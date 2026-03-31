package middleware

import (
	"log/slog"
	"net/http"

	"eventAI/internal/adapters/api/response"
	"eventAI/internal/utils"
)

func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					r = utils.WithRequestLogMeta(r, "")
					attrs := utils.RequestLogAttrs(r)
					attrs = append(attrs, slog.Any("panic", rec))

					logger.LogAttrs(r.Context(), slog.LevelError, "panic recovered", attrs...)
					response.Failure(w, http.StatusInternalServerError, "internal server error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
