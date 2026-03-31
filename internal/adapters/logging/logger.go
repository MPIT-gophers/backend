package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func New(serviceName, envName, level string) (*slog.Logger, error) {
	var slogLevel slog.Level

	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		return nil, fmt.Errorf("unsupported log level %q", level)
	}

	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slogLevel,
	})

	return slog.New(baseHandler).With(
		slog.String("service", serviceName),
		slog.String("env", envName),
	), nil
}
