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
	met "extended-monitoring/metrics"
	w "extended-monitoring/watcher"
	"os"
	"os/signal"
	"syscall"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	var listenAddr string = "127.0.0.1:8080"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info("Shutdown signal received...")
		cancel()
	}()

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Error kubernetes config: %v\n", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal("Error getting kubernetes config: %v\n", err)
	}

	registry := prometheus.NewRegistry()

	metrics := met.RegisterMetrics(registry)

	watcher := w.NewWatcher(kubeClient, metrics)

	go watcher.StartNodeWatcher(ctx)

	go watcher.StartNamespaceWatcher(ctx)

	go met.StartPrometheusServer(ctx, registry, listenAddr)

	<-ctx.Done()
	log.Info("Main shutdown complete")
}
