package logger

import (
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

func Configure(levelStr string, env string) {
    level := parseLogLevel(levelStr)
	w := os.Stdout
	var handler slog.Handler

	if env == "dev" || env == "development" {
		handler = tint.NewHandler(w, &tint.Options{Level: level})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(handler))
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

