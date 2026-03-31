package router

import (
	"log/slog"
	"net/http"

	"eventAI/internal/adapters/api/action"
	apimiddleware "eventAI/internal/adapters/api/middleware"
	"eventAI/internal/config"
	"eventAI/internal/infrastructure/authz"
	"eventAI/internal/infrastructure/database"
	"eventAI/internal/infrastructure/jwt"
	"eventAI/internal/infrastructure/max"
	"eventAI/internal/service"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(cfg config.Config, logger *slog.Logger, db *pgxpool.Pool) (http.Handler, error) {
	healthHandler := action.NewHealthHandler(db)

	eventRepo := database.NewEventRepository(db)
	eventService := service.NewEventService(eventRepo)
	eventHandler := action.NewEventHandler(eventService)

	authRepo := database.NewAuthRepository(db)

	maxClient, err := max.NewClient(cfg.MAX.BotToken)
	if err != nil {
		return nil, err
	}

	jwtManager, err := jwt.NewManager(cfg.JWT.Secret, cfg.JWT.TTL)
	if err != nil {
		return nil, err
	}

	authService := service.NewAuthService(authRepo, maxClient, jwtManager, cfg.MAX.BotUsername)
	authHandler := action.NewAuthHandler(authService)

	authorizer, err := authz.New(cfg.Casbin.ModelPath, cfg.Casbin.PolicyPath, eventRepo)
	if err != nil {
		return nil, err
	}

	authMiddleware := apimiddleware.Auth(func(token string) (string, error) {
		claims, err := jwtManager.Verify(token)
		if err != nil {
			return "", err
		}

		return claims.Subject, nil
	})

	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(apimiddleware.Logger(logger))
	r.Use(apimiddleware.Recoverer(logger))

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/healthz", healthHandler.Get)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/max/start", authHandler.StartMAXAuth)
			r.Post("/max/login", authHandler.LoginWithMAX)
			r.Post("/max/complete", authHandler.CompleteMAXAuth)
			r.Get("/max/session/{sessionID}", authHandler.GetMAXAuthSession)
			r.Post("/max/exchange", authHandler.ExchangeMAXAuth)
		})

		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)

			r.Patch("/me", authHandler.UpdateMe)
			r.Get("/events/my", eventHandler.ListMine)
			r.Post("/events", eventHandler.Create)
			r.Post("/events/join-by-token", eventHandler.JoinByToken)

			r.Route("/events/{eventID}", func(r chi.Router) {
				r.Use(apimiddleware.RequireEventAccess(authorizer, "read"))
				r.Get("/", eventHandler.GetByID)
			})
		})
	})

	return r, nil
}
