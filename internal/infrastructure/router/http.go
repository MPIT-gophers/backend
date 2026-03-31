package router

import (
	"log/slog"
	"net/http"

	"eventAI/internal/adapters/api/action"
	apimiddleware "eventAI/internal/adapters/api/middleware"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func New(logger *slog.Logger, health *action.HealthHandler, registrations *action.RegistrationHandler) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(apimiddleware.Logger(logger))
	r.Use(apimiddleware.Recoverer(logger))

	r.Get("/healthz", health.Get)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/registrations", registrations.List)
		r.Post("/registrations", registrations.Create)
	})

	return r
}
