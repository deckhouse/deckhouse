/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	dlog "github.com/deckhouse/deckhouse/pkg/log"

	staticpodmanager "embeded-registry-manager/internal/static-pod"
)

var (
	shutdownSignals  = []os.Signal{os.Interrupt, syscall.SIGTERM}
	healthListenAddr = ":8097"
)

func main() {
	log := dlog.Default().With("component", "main")

	log.Info("Starting static pod manager")
	defer log.Info("Stopped")

	log.Info("Setup signal handler")
	ctx := setupSignalHandler()

	ctx, cancel := context.WithCancel(ctx)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		if err := startHealthServer(ctx); err != nil {
			log.Error("Health server error", "error", err)
		}
	}()

	log.Info("Starting manager")
	err := staticpodmanager.Run(ctx)
	if err != nil {
		log.Error("Manager run error", "error", err)
	}

	log.Info("Waiting for background operations")
	cancel()
	wg.Wait()

	log.Info("Bye!")

	if err != nil {
		os.Exit(1)
	}
}

// startHealthServer starts a health server that provides readiness and liveness probes
func startHealthServer(ctx context.Context) error {
	okHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", okHandler)
	mux.HandleFunc("/readyz", okHandler)

	srv := &http.Server{
		Addr:    healthListenAddr,
		Handler: mux,
	}

	ctxListenStop := context.AfterFunc(ctx, func() {
		ctx, ctxDone := context.WithTimeout(context.Background(), 10*time.Second)
		defer ctxDone()

		srv.Shutdown(ctx)
	})

	defer ctxListenStop()

	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("listen and serve error: %w", err)
	}

	return nil
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
