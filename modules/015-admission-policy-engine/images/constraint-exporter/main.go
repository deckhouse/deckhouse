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
	"bytes"
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/collectors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	listenAddress = flag.String("server.telemetry-address", ":15060",
		"Address to listen on for telemetry")
	metricsPath = flag.String("server.telemetry-path", "/metrics",
		"Path under which to expose metrics")
	interval = flag.Duration("server.interval", 15*time.Second,
		"Kubernetes API server polling interval")
	kindsCM = flag.String("match-kinds-configmap", "gatekeeper-match-kinds",
		"ConfigMap for export tracking resource kinds")

	ticker *time.Ticker
	done   = make(chan bool)

	metrics = make([]prometheus.Metric, 0)
)

type Exporter struct{}

func NewExporter() *Exporter {
	return &Exporter{}
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
	for _, m := range metrics {
		ch <- m
	}
}

func (e *Exporter) startScheduled(t time.Duration) {
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Create kubernetes config failed: %+v\n", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Create kubernetes client failed: %+v\n", err)
	}

	ticker = time.NewTicker(t)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				constraints, err := gatekeeper.GetConstraints(config, client)
				if err != nil {
					klog.Warningf("Get constraints failed: %+v\n", err)
				}
				allMetrics := make([]prometheus.Metric, 0)
				violationMetrics := gatekeeper.ExportViolations(constraints)
				allMetrics = append(allMetrics, violationMetrics...)

				constraintInformationMetrics := gatekeeper.ExportConstraintInformation(constraints)
				allMetrics = append(allMetrics, constraintInformationMetrics...)

				metrics = allMetrics

				if kindsCM != nil && len(*kindsCM) > 0 {
					updateCM(client, constraints)
				}
			}
		}
	}()
}

func main() {
	flag.Parse()

	exporter := NewExporter()
	exporter.startScheduled(*interval)
	prometheus.Unregister(collectors.NewGoCollector())
	prometheus.MustRegister(exporter)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	mux := http.NewServeMux()
	mux.Handle(*metricsPath, promhttp.Handler())

	srv := &http.Server{
		Addr:    *listenAddress,
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

func updateCM(client *kubernetes.Clientset, constraints []gatekeeper.Constraint) {
	if len(constraints) == 0 {
		return
	}

	hasher := sha256.New()           // writer
	buf := bytes.NewBuffer(nil)

	// deduplicate
	m := map[string]gatekeeper.MatchKind

	for _, con := range constraints {
		for _, k := range con.Spec.Match.Kinds {
			sort.Strings(k.APIGroups)
			sort.Strings(k.Kinds)
			key := fmt.Sprintf("%s:%s", strings.Join(k.APIGroups, ","), strings.Join(k.Kinds, ","))

			m[key] = con.Spec.Match.Kinds
		}
	}

}


