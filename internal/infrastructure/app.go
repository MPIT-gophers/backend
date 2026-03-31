package infrastructure

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"eventAI/internal/config"
	"eventAI/internal/infrastructure/database"
	"eventAI/internal/infrastructure/router"

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
		logger.Info("postgres migrations started",
			slog.String("direction", "up"),
			slog.String("db_host", cfg.Postgres.Host),
			slog.Int("db_port", cfg.Postgres.Port),
			slog.String("db_name", cfg.Postgres.DBName),
			slog.String("migrations_path", cfg.Postgres.MigrationsPath),
		)
		if err := database.RunMigrations(cfg.Postgres, "up"); err != nil {
			logger.Error("postgres migrations failed",
				slog.Any("error", err),
				slog.String("direction", "up"),
				slog.String("db_host", cfg.Postgres.Host),
				slog.Int("db_port", cfg.Postgres.Port),
				slog.String("db_name", cfg.Postgres.DBName),
			)
			return nil, fmt.Errorf("auto migrate: %w", err)
		}
		logger.Info("postgres migrations completed",
			slog.String("direction", "up"),
			slog.String("db_host", cfg.Postgres.Host),
			slog.Int("db_port", cfg.Postgres.Port),
			slog.String("db_name", cfg.Postgres.DBName),
		)
	} else {
		logger.Info("postgres migrations skipped",
			slog.Bool("auto_migrate", false),
			slog.String("db_host", cfg.Postgres.Host),
			slog.Int("db_port", cfg.Postgres.Port),
			slog.String("db_name", cfg.Postgres.DBName),
		)
	}

	logger.Info("postgres connection started",
		slog.String("db_host", cfg.Postgres.Host),
		slog.Int("db_port", cfg.Postgres.Port),
		slog.String("db_name", cfg.Postgres.DBName),
		slog.Int64("max_conns", int64(cfg.Postgres.MaxConns)),
	)
	db, err := database.NewPool(ctx, cfg.Postgres)
	if err != nil {
		logger.Error("postgres connection failed",
			slog.Any("error", err),
			slog.String("db_host", cfg.Postgres.Host),
			slog.Int("db_port", cfg.Postgres.Port),
			slog.String("db_name", cfg.Postgres.DBName),
		)
		return nil, err
	}
	logger.Info("postgres connection established",
		slog.String("db_host", cfg.Postgres.Host),
		slog.Int("db_port", cfg.Postgres.Port),
		slog.String("db_name", cfg.Postgres.DBName),
	)

	httpRouter, err := router.New(cfg, logger, db)
	if err != nil {
		db.Close()
		return nil, err
	}
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
