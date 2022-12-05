/*
Copyright 2022 Flant JSC

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
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"
	"github.com/flant/constraint_exporter/pkg/kinds"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

var (
	// flags
	listenAddress string
	metricsPath   string
	interval      time.Duration

	trackValidationKinds     bool
	trackValidationResources bool
	trackObjectsCMName       string
)

func init() {
	flag.StringVar(&listenAddress, "server.telemetry-address", ":15060",
		"Address to listen on for telemetry")
	flag.StringVar(&metricsPath, "server.telemetry-path", "/metrics",
		"Path under which to expose metrics")
	flag.DurationVar(&interval, "server.interval", 30*time.Second,
		"Kubernetes API server polling interval")
	flag.BoolVar(&trackValidationKinds, "track-validation-match-kinds", false, "Tracked kinds for validation webhook")
	flag.BoolVar(&trackValidationResources, "track-validation-match-resource", true, "Tracked kinds for validation webhook are converted to the resources")
	flag.StringVar(&trackObjectsCMName, "track-objects-configmap", "constraint-exporter", "ConfigMap for export tracking resource kinds")
}

var (
	ticker *time.Ticker
	done   = make(chan bool)
)

type Exporter struct {
	client     *kubernetes.Clientset
	kubeConfig *rest.Config

	kindTracker *kinds.KindTracker

	metrics []prometheus.Metric
}

func NewExporter() *Exporter {
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Create kubernetes config failed: %+v\n", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Create kubernetes client failed: %+v\n", err)
	}

	return &Exporter{
		client:     client,
		kubeConfig: config,
		metrics:    make([]prometheus.Metric, 0),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- gatekeeper.Up
	ch <- gatekeeper.ConstraintViolation
	ch <- gatekeeper.ConstraintInformation
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		gatekeeper.Up, prometheus.GaugeValue, 1,
	)
	for _, m := range e.metrics {
		ch <- m
	}
}

func (e *Exporter) startScheduled(t time.Duration) {
	ticker = time.NewTicker(t)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				constraints, err := gatekeeper.GetConstraints(e.kubeConfig, e.client)
				if err != nil {
					klog.Warningf("Get constraints failed: %+v\n", err)
				}
				allMetrics := make([]prometheus.Metric, 0)
				violationMetrics := gatekeeper.ExportViolations(constraints)
				allMetrics = append(allMetrics, violationMetrics...)

				constraintInformationMetrics := gatekeeper.ExportConstraintInformation(constraints)
				allMetrics = append(allMetrics, constraintInformationMetrics...)

				e.metrics = allMetrics

				if e.kindTracker != nil {
					go e.kindTracker.UpdateTrackedObjects(constraints)
				}
			}
		}
	}()
}

func main() {
	flag.Parse()

	ns := os.Getenv("POD_NAMESPACE")
	if len(ns) == 0 {
		klog.Fatal("Pod namespace is not set")
	}

	exporter := NewExporter()
	if trackValidationKinds || trackValidationResources {
		err := exporter.initKindTracker(ns, trackObjectsCMName, trackValidationKinds, trackValidationResources)
		if err != nil {
			klog.Fatal(err)
		}
	}

	exporter.startScheduled(interval)
	prometheus.Unregister(collectors.NewGoCollector())
	prometheus.MustRegister(exporter)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	mux := http.NewServeMux()
	mux.Handle(metricsPath, promhttp.Handler())

	srv := &http.Server{
		Addr:    listenAddress,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Fatalf("listen: %s\n", err)
		}
	}()
	klog.Info("Server Started")

	<-done
	klog.Info("Server Stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer func() {
		// extra handling here
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		klog.Fatalf("Server Shutdown Failed:%+v", err)
	}
}

func (e *Exporter) initKindTracker(cmNS, cmName string, trackKinds, trackResources bool) error {
	kt := kinds.NewKindTracker(e.client, cmNS, cmName, trackKinds, trackResources)
	err := kt.FindInitialChecksum()
	if err != nil {
		return err
	}

	e.kindTracker = kt

	return nil
}
