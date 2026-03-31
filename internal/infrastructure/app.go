package infrastructure

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"eventAI/internal/adapters/api/action"
	"eventAI/internal/config"
	"eventAI/internal/infrastructure/database"
	"eventAI/internal/infrastructure/router"
	"eventAI/internal/usecase"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	cfg    config.Config
	logger *slog.Logger
	db     *pgxpool.Pool
	server *http.Server
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	if cfg.Postgres.AutoMigrate {
		if err := database.RunMigrations(cfg.Postgres, "up"); err != nil {
			return nil, fmt.Errorf("auto migrate: %w", err)
		}
	}

	db, err := database.NewPool(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}

	registrationRepo := database.NewRegistrationRepository(db)
	registrationUseCase := usecase.NewRegistrationUseCase(registrationRepo)

	healthHandler := action.NewHealthHandler(db)
	registrationHandler := action.NewRegistrationHandler(registrationUseCase)

	httpRouter := router.New(logger, healthHandler, registrationHandler)
	server := &http.Server{
		Addr:              cfg.HTTP.Addr(),
		Handler:           httpRouter,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &App{
		cfg:    cfg,
		logger: logger,
		db:     db,
		server: server,
	}, nil
}

func (a *App) Run() error {
	a.logger.Info("http server started",
		slog.String("app", a.cfg.App.Name),
		slog.String("env", a.cfg.App.Env),
		slog.String("addr", a.cfg.HTTP.Addr()),
	)

	return a.server.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	if err := a.server.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}

func (a *App) Close() {
	a.db.Close()
}
