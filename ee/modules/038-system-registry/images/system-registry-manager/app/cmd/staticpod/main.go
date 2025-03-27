/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	dlog "github.com/deckhouse/deckhouse/pkg/log"

	"embeded-registry-manager/internal/staticpod"
)

var (
	shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

	logHandler slog.Handler = dlog.Default().Handler()

	nodeName = os.Getenv("NODE_NAME")
)

func main() {
	log := slog.New(logHandler).With("component", "main")
	log = log.With("node", nodeName)

	hostIP := os.Getenv("HOST_IP")
	if hostIP == "" {
		log.Error("HOST_IP environment variable is not set")
		os.Exit(1)
	}

	if nodeName == "" {
		log.Error("NODE_NAME environment variable is not set")
		os.Exit(1)
	}

	log.Info("Starting Node Services manager")
	defer log.Info("Stopped")

	log.Info("Setup signal handler")
	ctx := setupSignalHandler()

	log.Info("Starting application")
	err := staticpod.Run(ctx, hostIP, nodeName)
	if err != nil {
		log.Error("Application error", "error", err)
	}

	log.Info("Bye!")

	if err != nil {
		os.Exit(1)
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
