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

	"embeded-registry-manager/internal/staticpod"

	"k8s.io/client-go/rest"
)

var (
	shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

	nodeName = os.Getenv("NODE_NAME")
)

func main() {
	log := slog.With("component", "main")
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

	// Load Kubernetes configuration
	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Error("Unable to get kubeconfig", "error", err)
		os.Exit(1)
	}

	log.Info("Starting Node Services manager")
	defer log.Info("Stopped")

	log.Info("Setup signal handler")
	ctx := setupSignalHandler()

	log.Info("Starting application")

	if err = staticpod.Run(ctx, cfg, hostIP, nodeName); err != nil {
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
