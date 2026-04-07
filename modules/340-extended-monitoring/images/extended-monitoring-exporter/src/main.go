/*
Copyright 2025 Flant JSC

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
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/pkg/log"

	met "extended-monitoring/metrics"
	w "extended-monitoring/watcher"
)

func main() {
	var listenAddr = "127.0.0.1:8080"

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info("Shutdown signal received...")
		cancel()
	}()

	config, err := rest.InClusterConfig()
	if err != nil {
		cancel()
		log.Fatal("Error kubernetes config: %v\n", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		cancel()
		log.Fatal("Error getting kubernetes config: %v\n", err)
	}

	defer cancel()

	registry := prometheus.NewRegistry()

	metrics := met.RegisterMetrics(registry)

	watcher := w.NewWatcher(kubeClient, metrics)

	go watcher.StartNodeWatcher(ctx)

	go watcher.StartNamespaceWatcher(ctx)

	go met.StartPrometheusServer(ctx, registry, listenAddr)

	<-ctx.Done()
	log.Info("Main shutdown complete")
}
