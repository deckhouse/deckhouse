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

package metrics

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type ExporterMetrics struct {
	NodeEnabled          *prometheus.GaugeVec
	NodeThreshold        *prometheus.GaugeVec
	NamespacesEnabled    *prometheus.GaugeVec
	PodEnabled           *prometheus.GaugeVec
	PodThreshold         *prometheus.GaugeVec
	DaemonSetEnabled     *prometheus.GaugeVec
	DaemonSetThreshold   *prometheus.GaugeVec
	StatefulSetEnabled   *prometheus.GaugeVec
	StatefulSetThreshold *prometheus.GaugeVec
	DeploymentEnabled    *prometheus.GaugeVec
	DeploymentThreshold  *prometheus.GaugeVec
	IngressEnabled       *prometheus.GaugeVec
	IngressThreshold     *prometheus.GaugeVec
	CronJobEnabled       *prometheus.GaugeVec
}

func RegisterMetrics(reg prometheus.Registerer) *ExporterMetrics {
	m := &ExporterMetrics{
		NamespacesEnabled: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_enabled",
				Help: "Namespace enabled for extended monitoring",
			},
			[]string{"namespace"},
		),

		NodeEnabled: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_node_enabled",
				Help: "Node enabled for extended monitoring",
			},
			[]string{"node"},
		),
		NodeThreshold: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_node_threshold",
				Help: "Node thresholds for extended monitoring",
			},
			[]string{"node", "threshold"},
		),

		PodEnabled: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_pod_enabled",
				Help: "Pod enabled for extended monitoring",
			},
			[]string{"namespace", "pod"},
		),
		PodThreshold: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_pod_threshold",
				Help: "Pod thresholds for extended monitoring",
			},
			[]string{"namespace", "pod", "threshold"},
		),

		IngressEnabled: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_ingress_enabled",
				Help: "Ingress enabled for extended monitoring",
			},
			[]string{"namespace", "ingress"},
		),
		IngressThreshold: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_ingress_threshold",
				Help: "Ingress thresholds for extended monitoring",
			},
			[]string{"namespace", "ingress", "threshold"},
		),

		DeploymentEnabled: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_deployment_enabled",
				Help: "Deployment enabled for extended monitoring",
			},
			[]string{"namespace", "deployment"},
		),
		DeploymentThreshold: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_deployment_threshold",
				Help: "Deployment thresholds for extended monitoring",
			},
			[]string{"namespace", "deployment", "threshold"},
		),

		DaemonSetEnabled: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_daemonset_enabled",
				Help: "DaemonSet enabled for extended monitoring",
			},
			[]string{"namespace", "daemonset"},
		),
		DaemonSetThreshold: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_daemonset_threshold",
				Help: "DaemonSet thresholds for extended monitoring",
			},
			[]string{"namespace", "daemonset", "threshold"},
		),

		StatefulSetEnabled: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_statefulset_enabled",
				Help: "StatefulSet enabled for extended monitoring",
			},
			[]string{"namespace", "statefulset"},
		),
		StatefulSetThreshold: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_statefulset_threshold",
				Help: "StatefulSet thresholds for extended monitoring",
			},
			[]string{"namespace", "statefulset", "threshold"},
		),

		CronJobEnabled: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "extended_monitoring_cronjob_enabled",
				Help: "CronJob enabled for extended monitoring",
			},
			[]string{"namespace", "cronjob"},
		),
	}

	reg.MustRegister(
		m.NamespacesEnabled,
		m.NodeEnabled, m.NodeThreshold,
		m.PodEnabled, m.PodThreshold,
		m.IngressEnabled, m.IngressThreshold,
		m.DeploymentEnabled, m.DeploymentThreshold,
		m.DaemonSetEnabled, m.DaemonSetThreshold,
		m.StatefulSetEnabled, m.StatefulSetThreshold,
		m.CronJobEnabled,
	)

	return m
}

func StartPrometheusServer(ctx context.Context, reg *prometheus.Registry, addr string) {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if time.Since(GetLastObserved()) > timeOutHealthz {
			log.Printf("Fail if metrics were last collected more than %v", timeOutHealthz)
			http.Error(w, "metrics stale", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		if !GetIsPopulated() {
			http.Error(w, "metrics not populated", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/startup", func(w http.ResponseWriter, _ *http.Request) {
		if !GetIsPopulated() {
			http.Error(w, "metrics not populated", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("Starting Prometheus metrics server at %s", addr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Prometheus server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Print("Prometheus server shutdown initiated...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Prometheus server graceful shutdown failed: %v", err)
	} else {
		log.Print("Prometheus server shut down cleanly")
	}
}
