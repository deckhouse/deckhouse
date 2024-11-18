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
)

var (
	shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
)

func main() {
	log := slog.Default().With("component", "main")

	log.Info("Starting")
	defer log.Info("Stopped")

	log.Info("Setup signal handler")
	ctx := waitForExit()

	log.Info("Waiting for signal to exit")
	<-ctx.Done()
}

func waitForExit() context.Context {
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
