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
)

var (
	shutdownSignals              = []os.Signal{os.Interrupt, syscall.SIGTERM}
	logHandler      slog.Handler = dlog.Default().Handler()
	nodeName                     = os.Getenv("NODE_NAME")
)

func main() {
	log := slog.New(logHandler).With("component", "main")
	log = log.With("node", nodeName)

	hostIP := os.Getenv("HOST_IP")
	if hostIP == "" {
		log.Error("HOST_IP environment variable is not set")
		os.Exit(1)
	}

	log = log.With("hostIP", hostIP)

	log.Info("Starting mirrorer")
	defer log.Info("Stopped")

	ctx := setupSignalHandler()
	ctx, cancel := context.WithCancel(ctx)

	log.Info("Waiting for signal")
	<-ctx.Done()
	cancel()
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
