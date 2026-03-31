package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

type requestLogContextKey string

const requestLogMetaContextKey requestLogContextKey = "request_log_meta"

const (
	HeaderXRequestID     = "X-Request-ID"
	HeaderXCorrelationID = "X-Correlation-ID"
	HeaderXTraceID       = "X-Trace-ID"
	HeaderTraceparent    = "Traceparent"
)

type RequestLogMeta struct {
	RequestID     string
	TraceID       string
	CorrelationID string
	Method        string
	Path          string
	Route         string
	Host          string
	RemoteIP      string
	UserAgent     string
}

func WithRequestLogMeta(r *http.Request, route string) *http.Request {
	if r == nil {
		return nil
	}

	meta, ok := RequestLogMetaFromContext(r.Context())
	if ok {
		if meta.Route != "" || strings.TrimSpace(route) == "" {
			return r
		}

		meta.Route = strings.TrimSpace(route)
		return r.WithContext(context.WithValue(r.Context(), requestLogMetaContextKey, meta))
	}

	traceID := firstNonEmpty(
		strings.TrimSpace(r.Header.Get(HeaderXTraceID)),
		traceIDFromTraceparent(r.Header.Get(HeaderTraceparent)),
	)
	requestID := firstNonEmpty(
		strings.TrimSpace(r.Header.Get(HeaderXRequestID)),
		strings.TrimSpace(r.Header.Get(HeaderXCorrelationID)),
		traceID,
		newLogID(),
	)
	correlationID := firstNonEmpty(
		strings.TrimSpace(r.Header.Get(HeaderXCorrelationID)),
		requestID,
	)

	meta = RequestLogMeta{
		RequestID:     requestID,
		TraceID:       firstNonEmpty(traceID, requestID),
		CorrelationID: correlationID,
		Method:        r.Method,
		Path:          r.URL.Path,
		Route:         strings.TrimSpace(route),
		Host:          r.Host,
		RemoteIP:      requestRemoteIP(r),
		UserAgent:     strings.TrimSpace(r.UserAgent()),
	}

	return r.WithContext(context.WithValue(r.Context(), requestLogMetaContextKey, meta))
}

func RequestLogMetaFromContext(ctx context.Context) (RequestLogMeta, bool) {
	if ctx == nil {
		return RequestLogMeta{}, false
	}

	meta, ok := ctx.Value(requestLogMetaContextKey).(RequestLogMeta)
	return meta, ok
}

func RequestLogAttrs(r *http.Request) []slog.Attr {
	if r == nil {
		return nil
	}

	meta, ok := RequestLogMetaFromContext(r.Context())
	if !ok {
		r = WithRequestLogMeta(r, "")
		meta, _ = RequestLogMetaFromContext(r.Context())
	}

	attrs := []slog.Attr{
		slog.String("request_id", meta.RequestID),
		slog.String("trace_id", meta.TraceID),
		slog.String("correlation_id", meta.CorrelationID),
		slog.String("http_method", meta.Method),
		slog.String("http_path", meta.Path),
		slog.String("http_host", meta.Host),
		slog.String("remote_ip", meta.RemoteIP),
		slog.String("user_agent", meta.UserAgent),
	}

	if meta.Route != "" {
		attrs = append(attrs, slog.String("http_route", meta.Route))
	}

	return attrs
}

func requestRemoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	realIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}

	return strings.TrimSpace(r.RemoteAddr)
}

func traceIDFromTraceparent(traceparent string) string {
	parts := strings.Split(strings.TrimSpace(traceparent), "-")
	if len(parts) != 4 {
		return ""
	}

	traceID := strings.ToLower(strings.TrimSpace(parts[1]))
	if len(traceID) != 32 {
		return ""
	}

	for _, r := range traceID {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return ""
		}
	}

	return traceID
}

func newLogID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000000000000000000000000000"
	}

	return hex.EncodeToString(buf)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	return ""
}
