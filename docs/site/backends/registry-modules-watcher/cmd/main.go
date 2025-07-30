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
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"registry-modules-watcher/internal/backends"
	registryscanner "registry-modules-watcher/internal/backends/pkg/registry-scanner"
	"registry-modules-watcher/internal/backends/pkg/sender"
	handler "registry-modules-watcher/internal/http"
	"registry-modules-watcher/internal/metrics"
	"registry-modules-watcher/internal/watcher"
	registryclient "registry-modules-watcher/pkg/registry-client"
	"strings"
	"syscall"
	"time"

	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/deckhouse/deckhouse/pkg/log"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
)

func main() {
	logger := log.NewLogger(
		log.WithLevel(log.LogLevelFromStr(os.Getenv("LOG_LEVEL")).Level()),
		log.WithHandlerType(log.TextHandlerType),
	)

	registries := flag.String("watch-registries", "", "a list for followed registries")
	scanInterval := flag.Duration("scan-interval", 15*time.Minute, "interval for scanning the images. default 15 minutes")
	flag.Parse()

	if *registries == "" {
		logger.Fatal("watch-registries is empty")
	}

	ctx, stopNotify := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopNotify()

	// * * * * * * * * *
	// Metric storage
	metricStorage := metricstorage.NewMetricStorage("watcher")
	err := metrics.RegisterMetrics(metricStorage)
	if err != nil {
		logger.Fatal(err.Error())
	}
	// metricStorage.HistogramObserve()

	// * * * * * * * * *
	// New handlers
	h := handler.NewHandler(logger.Named("http"))
	srv := &http.Server{
		Addr:    "localhost:8080",
		Handler: h,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen: %w", err)
		}
	}()

	metricHandler := handler.NewMetricHandler(logger.Named("http-metrics"), metricStorage)
	metricServer := &http.Server{
		Addr:    "localhost:9090",
		Handler: metricHandler,
	}
	go func() {
		if err := metricServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen: %w", err)
		}
	}()

	// * * * * * * * * *
	// dockerconfigjson
	regsecretRaw := os.Getenv("REGISTRY_AUTHS")
	if regsecretRaw == "" {
		logger.Fatal("registry auths not set")
	}

	// * * * * * * * * *
	// Connect to registry
	clients := make([]registryscanner.Client, 0)
	for _, registry := range strings.Split(*registries, ",") {
		logger.Info("watch modules", slog.String("source", registry))

		client, err := registryclient.NewClient(registry, metricStorage,
			registryclient.WithAuth(regsecretRaw),
		)
		if err != nil {
			logger.Warn("no dockercfg auth set, skipping", slog.String("source", registry))
			continue
		}

		// TODO: some registry ping to check credentials
		clients = append(clients, client)
	}

	if len(clients) == 0 {
		logger.Fatal("no registries to watch")
	}

	registryscanner := registryscanner.New(logger.Named("registry-scanner"), clients...)
	registryscanner.Subscribe(ctx, *scanInterval)

	// * * * * * * * * *
	// New sender
	sender := sender.New(logger.Named("sender"))

	// * * * * * * * * *
	// New backends service
	backends := backends.New(registryscanner, sender, logger)

	// * * * * * * * * *
	// New metric ticker
	go func() {
		// 30 second ticker
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cache := registryscanner.GetCache()
				metrics.ObserveCache(metricStorage, cache)
			case <-ctx.Done():
				return
			}
		}
	}()

	// * * * * * * * * *
	// Init kube client
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("get kubernetes config", log.Err(err))
	}

	kClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal("init kubernetes client", log.Err(err))
	}

	// * * * * * * * * *
	// Watch lease
	namespace := os.Getenv("POD_NAMESPACE")
	wather := watcher.New(kClient, namespace, logger.Named("watcher"))
	wather.Watch(ctx, backends.Add, backends.Delete)

	<-ctx.Done()
}
