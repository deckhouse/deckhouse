/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var (
		healthBindAddress  string
		metricsBindAddress string
		leaderElect        bool
		verbosity          int
	)

	flag.StringVar(&healthBindAddress, "health-probe-bind-address", "0.0.0.0:4292", "address the health probe endpoint binds to")
	flag.StringVar(&metricsBindAddress, "metrics-bind-address", "127.0.0.1:4291", "address the metrics endpoint binds to")
	flag.BoolVar(&leaderElect, "leader-elect", false, "enable leader election for controller manager")
	flag.IntVar(&verbosity, "v", 0, "log verbosity level")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger, healthBindAddress, leaderElect); err != nil {
		logger.Error("fencing-controller failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, healthBindAddress string, leaderElect bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	logger.Info("starting fencing-controller 2.0", "leader_elect", leaderElect, "health_probe_bind_address", healthBindAddress)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:              healthBindAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
