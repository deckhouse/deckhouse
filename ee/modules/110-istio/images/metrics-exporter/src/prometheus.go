/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/deckhouse/deckhouse/pkg/log"
	"net/http"
	"time"

	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusExporterMetrics struct {
	clusterUp   *prometheus.GaugeVec
	istiodToken *authv1.TokenRequest
	clientset   *kubernetes.Clientset
}

func RegisterMetrics(reg prometheus.Registerer) *PrometheusExporterMetrics {
	// Set metrics
	p := &PrometheusExporterMetrics{
		clusterUp: prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "istio_remote_cluster_up",
			Help: "Indicates if remote cluster is synced (1 = yes, 0 = no)",
		},
		[]string{"istiod", "cluster_id", "secret"},
		),
	}
	reg.MustRegister(p.clusterUp)

	// Kube client
	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Error("Failed to get cluster config: %v", err)
	}
	p.clientset, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error("Failed to create clientset: %v", err)
	}

	return p
}

func StartPrometheusServer(addr string, reg *prometheus.Registry, ctx context.Context) {
	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Metrics
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Info("Starting Prometheus metrics server at %s", addr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Prometheus server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Info("Prometheus server shutdown initiated...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Prometheus server graceful shutdown failed: %v", err)
	} else {
		log.Info("Prometheus server shut down cleanly")
	}
}

func (p *PrometheusExporterMetrics) getToken(ctx context.Context, namespace, saName string) string {

	if p.istiodToken == nil || p.istiodToken.Status.ExpirationTimestamp.Time.Before(time.Now()) {
		log.Info("Generating new service account token for istiod communication")
		newToken := p.newTokenRequest(ctx, namespace, saName)
		if newToken == nil {
			log.Error("Failed to obtain new token; returning empty string")
			return ""
		}
		p.istiodToken = newToken
	}

	return p.istiodToken.Status.Token
}

func (p *PrometheusExporterMetrics) newTokenRequest(ctx context.Context, namespace, saName string) *authv1.TokenRequest {
	// 1. Get fresh token
	tokenResp, err := p.clientset.CoreV1().ServiceAccounts(namespace).CreateToken(ctx, saName, &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences: []string{"istio-ca"},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		log.Error("Failed to create token: %v", err)
	}

	return tokenResp
}

func (p *PrometheusExporterMetrics) GetIstiodRemoteClustersStatus(ctx context.Context, namespace, saName string, ips []string) {
	token := p.getToken(ctx, namespace, saName)
	if token == "" {
		log.Error("Empty token, skipping polling.")
		return
	}

	clientHTTP := &http.Client{Timeout: 3 * time.Second}

	for _, ip := range ips {
		url := fmt.Sprintf("http://%s:15014/debug/clusterz", ip)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			log.Error("Failed to build request for %s: %v", ip, err)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := clientHTTP.Do(req)
		if err != nil {
			log.Error("Error polling %s: %v", ip, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Error("Non-200 response from %s: %s", ip, resp.Status)
			resp.Body.Close()
			continue
		}

		var data []ClusterDebugInfo
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			log.Error("Error decoding response from %s: %v", ip, err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		for _, info := range data {
			p.setClusterStatus(ip, info.ID, info.SecretName, info.SyncStatus)
		}
	}
}

func (p *PrometheusExporterMetrics) setClusterStatus(istiod, clusterID, secretName, syncStatus string) {
	val := 0.0
	if syncStatus == "synced" {
		val = 1.0
	}
	p.clusterUp.WithLabelValues(istiod, clusterID, secretName).Set(val)
}
