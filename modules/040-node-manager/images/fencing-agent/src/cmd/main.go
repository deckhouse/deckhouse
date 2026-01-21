package main

import (
	"context"
	"fencing-agent/internal/app"
	fencingconfig "fencing-agent/internal/config"
	"fencing-agent/internal/lib/logger/sl"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var config fencingconfig.Config
	config.MustLoad()

	logger := NewLogger(config.LogLevel)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		s := <-sigChan
		close(sigChan)
		logger.Info("Got a signal", slog.String("signal", s.String()))
		cancel()
	}()

	application, err := app.NewApplication(ctx, logger, config)

	if err != nil {
		logger.Fatal("Unable to create an application", sl.Err(err))
	}

	if err = application.Run(ctx); err != nil {
		logger.Fatal("Unable to run the application", sl.Err(err))
	}
}

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
