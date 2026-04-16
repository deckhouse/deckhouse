// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/docs"
	v1 "github.com/flant/docs-builder/internal/http/v1"
	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/pkg/k8s"
)

// flags
var (
	listenAddress    string
	src              string
	dst              string
	metricsAddress   string
	highAvailability bool
)

func init() {
	flag.StringVar(&listenAddress, "address", ":8081", "Address to listen on")
	flag.StringVar(&src, "src", "/app/hugo/", "Directory to load source files")
	flag.StringVar(&dst, "dst", "/mount/", "Directory for site files")
	flag.StringVar(&metricsAddress, "metrics-address", ":9090", "Address to listen on metrics")
	flag.BoolVar(&highAvailability, "highAvailability", false, "high availability mod")
}

func main() {
	flag.Parse()

	ctx, stopNotify := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopNotify()

	logger := log.NewLogger(
		log.WithLevel(log.LogLevelFromStr(os.Getenv("LOG_LEVEL")).Level()),
		log.WithHandlerType(log.TextHandlerType),
	)

	mStorage := metricsstorage.NewMetricStorage()

	if err := metrics.RegisterMetrics(mStorage); err != nil {
		logger.Fatal("failed to register metrics", log.Err(err))
	}

	lManager, err := k8s.NewLeasesManager(logger)
	if err != nil {
		logger.Fatal("new leases manager", log.Err(err))
	}

	h := v1.NewHandler(docs.NewService(src, dst, highAvailability, logger, mStorage), logger.Named("v1"))

	srv := &http.Server{
		Addr:    listenAddress,
		Handler: h,
	}

	metricsSrv := &http.Server{
		Addr:    metricsAddress,
		Handler: mStorage.Handler(),
	}

	eg, ctx := errgroup.WithContext(ctx)

	logger.Info("starting application")

	eg.Go(srv.ListenAndServe)
	eg.Go(metricsSrv.ListenAndServe)
	eg.Go(lManager.Run(ctx))

	logger.Info("application started")

	<-ctx.Done()

	logger.Info("stopping application")

	// Create a new context with timeout for graceful shutdown.
	// The signal context (ctx) may already be canceled (e.g., if one of the goroutines failed).
	// In that case, shutdown would immediately exit — which is not what we want.
	// A separate context allows all shutdown goroutines to complete correctly,
	// even if one of them fails — the error won't cancel the context for others.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// New errgroup for shutdown — the main eg may have already exited with an error
	// (and consequently canceled its context). Reusing it for shutdown is not allowed.
	shutdownEg, shutdownCtx := errgroup.WithContext(shutdownCtx)

	shutdownEg.Go(func() error {
		return srv.Shutdown(shutdownCtx)
	})
	shutdownEg.Go(func() error {
		return metricsSrv.Shutdown(shutdownCtx)
	})

	waitErr := shutdownEg.Wait()
	if waitErr != nil && !errors.Is(waitErr, context.Canceled) {
		logger.Error("goroutine error", log.Err(waitErr))
	}

	// Remove the kubernetes lease to signal that this instance is no longer active.
	// Uses its own context with a 5-second timeout to ensure it completes even if
	// the shutdown context was already canceled. Do not remove this — the lease
	// must be released so that another instance can take over, otherwise we'll have
	// a split-brain situation where two instances think they are the leader.
	removeCtx, removeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer removeCancel()
	if err := lManager.Remove(removeCtx); err != nil {
		logger.Error("lease removing failed", log.Err(err))
	}

	logger.Info("application stopped")
}
