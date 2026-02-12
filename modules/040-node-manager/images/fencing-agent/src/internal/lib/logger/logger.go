package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func NewLogger(levelStr string) *log.Logger {
	level := getLogLevel(levelStr)

	logger := log.NewLogger(
		log.WithOutput(os.Stdout),
		log.WithLevel(level),
		log.WithHandlerType(log.JSONHandlerType),
	)
	return logger
}

func getLogLevel(levelStr string) slog.Level {
	switch strings.ToLower(levelStr) {
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
