/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"
	"log/slog"
	"mirrorer/internal/config"
	"mirrorer/internal/mirrorer"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

var (
	shutdownSignals              = []os.Signal{os.Interrupt, syscall.SIGTERM}
	logHandler      slog.Handler = dlog.Default().Handler()
)

func main() {
	logger := slog.New(logHandler)
	log := logger.With("component", "main")

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <config file>\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	configFile := os.Args[1]
	log.Debug("Loading config", "config_file", configFile)

	cfg, err := config.FromFile(configFile)
	if err != nil {
		log.Error("Cannot load config file", "config_file", configFile, "error", err)
		os.Exit(1)
	}

	err = cfg.Validate()
	if err != nil {
		log.Error("Config validation error", "config_file", configFile, "error", err)
		os.Exit(1)
	}

	worker, err := mirrorer.New(logger, cfg)
	if err != nil {
		log.Error("Cannot create mirrorer", "error", err)
		os.Exit(2)
	}

	log.Info("Starting mirrorer")
	defer log.Info("Stopped")

	ctx := setupSignalHandler()
	err = worker.Run(ctx)
	if err != nil {
		log.Error("Mirrorer error", "error", err)
		os.Exit(3)
	}
}

func setupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return ctx
}
