package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"eventAI/internal/utils"

	"github.com/go-chi/chi/v5"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *loggingResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *loggingResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	size, err := w.ResponseWriter.Write(data)
	w.size += size
	return size, err
}

func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			lrw := &loggingResponseWriter{ResponseWriter: w}
			r = utils.WithRequestLogMeta(r, "")

			if meta, ok := utils.RequestLogMetaFromContext(r.Context()); ok {
				w.Header().Set(utils.HeaderXRequestID, meta.RequestID)
				w.Header().Set(utils.HeaderXCorrelationID, meta.CorrelationID)
				w.Header().Set(utils.HeaderXTraceID, meta.TraceID)
			}

			next.ServeHTTP(lrw, r)

			if lrw.status == 0 {
				lrw.status = http.StatusOK
			}

			route := ""
			if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
				route = routeContext.RoutePattern()
			}
			r = utils.WithRequestLogMeta(r, route)

			attrs := utils.RequestLogAttrs(r)
			attrs = append(attrs,
				slog.Int("http_status", lrw.status),
				slog.Int("response_bytes", lrw.size),
				slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
			)

			switch {
			case lrw.status >= http.StatusInternalServerError:
				logger.LogAttrs(r.Context(), slog.LevelError, "request finished", attrs...)
			case lrw.status >= http.StatusBadRequest:
				logger.LogAttrs(r.Context(), slog.LevelWarn, "request finished", attrs...)
			default:
				logger.LogAttrs(r.Context(), slog.LevelInfo, "request finished", attrs...)
			}
		})
	}
}
