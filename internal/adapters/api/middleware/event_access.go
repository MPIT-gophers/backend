package middleware

import (
	"net/http"

	"eventAI/internal/adapters/api/response"
	"eventAI/internal/infrastructure/authz"

	"github.com/go-chi/chi/v5"
)

func RequireEventAccess(authorizer *authz.Authorizer, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				response.Failure(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			eventID := chi.URLParam(r, "eventID")
			if eventID == "" {
				response.Failure(w, http.StatusBadRequest, "event id is required")
				return
			}

			allowed, err := authorizer.CanEvent(r.Context(), userID, eventID, action)
			if err != nil {
				response.Failure(w, http.StatusForbidden, "forbidden")
				return
			}
			if !allowed {
				response.Failure(w, http.StatusForbidden, "forbidden")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
