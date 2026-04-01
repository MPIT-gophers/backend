package middleware

import (
	"context"
	"net/http"
	"strings"

	"eventAI/internal/adapters/api/response"
)

type contextKey string

const userIDContextKey contextKey = "user_id"

func Auth(verify func(token string) (string, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" {
				response.Failure(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			const prefix = "Bearer "
			if !strings.HasPrefix(authHeader, prefix) {
				response.Failure(w, http.StatusUnauthorized, "invalid bearer token")
				return
			}

			userID, err := verify(strings.TrimSpace(strings.TrimPrefix(authHeader, prefix)))
			if err != nil || strings.TrimSpace(userID) == "" {
				response.Failure(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			ctx := context.WithValue(r.Context(), userIDContextKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey).(string)
	return userID, ok
}

// WithUserID is a helper primarily used for testing handlers that depend on UserIDFromContext.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}
