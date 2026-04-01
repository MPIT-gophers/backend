package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eventAI/internal/adapters/logging"
	"eventAI/internal/config"
	infra "eventAI/internal/infrastructure"
	"eventAI/internal/infrastructure/database"
)

// @title mpit2026-reg API
// @version 0.1.0
// @description Backend API для сервиса mpit2026-reg.
// @description
// @description Основные возможности:
// @description - авторизация через MAX mini app и локальный JWT
// @description - получение и обновление профиля текущего пользователя
// @description - создание событий, получение invite token и вход по нему
// @description - управление гостями и фиксация статуса посещения
// @description - wishlist и фото внутри события
// @description
// @description Аутентификация:
// @description - защищённые методы требуют заголовок Authorization: Bearer <jwt>
// @description - JWT можно получить через /auth/max/login или через мобильный flow /auth/max/start -> /auth/max/complete -> /auth/max/exchange
// @description
// @description Swagger UI доступен по адресу /api/docs/.
// @host mpit-bot.kostya1024.ru
// @schemes https
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", slog.Any("error", err))
		os.Exit(1)
	}

	logger, err := logging.New(cfg.App.Name, cfg.App.Env, cfg.Log.Level)
	if err != nil {
		slog.Error("init logger", slog.Any("error", err))
		os.Exit(1)
	}

	if err := run(context.Background(), cfg, logger, os.Args[1:]); err != nil {
		logger.Error("application stopped with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger, args []string) error {
	if len(args) > 0 && args[0] == "migrate" {
		direction := "up"
		if len(args) > 1 {
			direction = args[1]
		}

		return database.RunMigrations(cfg.Postgres, direction)
	}

	app, err := infra.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer app.Close()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- app.Run()
	}()

	signalCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-signalCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return app.Shutdown(shutdownCtx)
	}
}
