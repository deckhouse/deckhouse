/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	dlog "github.com/deckhouse/deckhouse/pkg/log"

	staticpodmanager "embeded-registry-manager/internal/static-pod"
)

var (
	shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
)

func main() {
	log := dlog.Default().With("component", "main")

	log.Info("Starting static pod manager")
	defer log.Info("Stopped")

	log.Info("Setup signal handler")
	ctx := setupSignalHandler()

	log.Info("Starting manager")
	err := staticpodmanager.Run(ctx)
	if err != nil {
		log.Error("Manager run error", "error", err)
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
