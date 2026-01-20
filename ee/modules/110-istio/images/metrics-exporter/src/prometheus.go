/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type PrometheusExporterMetrics struct {
	clusterUp   *prometheus.GaugeVec
	istiodToken *authv1.TokenRequest
	clientSet   *kubernetes.Clientset
}

func RegisterMetrics(clientSet *kubernetes.Clientset, reg prometheus.Registerer) *PrometheusExporterMetrics {
	// Set metrics
	p := &PrometheusExporterMetrics{
		clusterUp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "istio_remote_cluster_up",
				Help: "Indicates if remote cluster is synced (1 = yes, 0 = no)",
			},
			[]string{"istiod", "cluster_id", "secret"},
		),
		clientSet: clientSet,
	}
	reg.MustRegister(p.clusterUp)

	return p
}

func StartPrometheusServer(ctx context.Context, reg *prometheus.Registry, addr string) {
	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("ok"))
		if err != nil {
			log.Error(fmt.Sprintf("error writing healthz response: %v", err))
			return
		}
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("ok"))
		if err != nil {
			log.Error(fmt.Sprintf("failed to write readyz response: %v", err))
			return
		}
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

func (p *PrometheusExporterMetrics) getToken(ctx context.Context, namespace, saName string) (string, error) {
	if p.istiodToken == nil || p.istiodToken.Status.ExpirationTimestamp.Time.Before(time.Now()) {
		log.Info("Generating new service account token for istiod communication")
		newToken, err := p.newTokenRequest(ctx, namespace, saName)
		if err != nil {
			log.Error("Failed to generate new service account token: %v", err)
			return "", err
		}
		p.istiodToken = newToken
	}

	return p.istiodToken.Status.Token, nil
}

func (p *PrometheusExporterMetrics) newTokenRequest(ctx context.Context, namespace, saName string) (*authv1.TokenRequest, error) {
	// 1. Get fresh token
	tokenResp, err := p.clientSet.CoreV1().ServiceAccounts(namespace).CreateToken(ctx, saName, &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences: []string{"istio-ca"},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %v", err)
	}

	return tokenResp, nil
}

func (p *PrometheusExporterMetrics) GetIstiodRemoteClustersStatus(ctx context.Context, namespace, saName string, pods []IstioPodInfo) {
	token, err := p.getToken(ctx, namespace, saName)
	if err != nil {
		log.Error("Failed to get token: %v", err)
		return
	}
	if token == "" {
		log.Error("Empty token, skipping polling.")
		return
	}

	clientHTTP := &http.Client{Timeout: 3 * time.Second}

	for _, pod := range pods {
		data, err := fetchClusterStatusFromIstiod(ctx, clientHTTP, token, pod.IP)
		if err != nil {
			log.Error(fmt.Sprintf("Failed to fetch cluster status from pod %s (%s): %v", pod.Name, pod.IP, err))
			continue
		}

		for _, info := range data {
			p.setClusterStatus(pod.Name, info.ID, info.SecretName, info.SyncStatus)
		}
	}
}

func fetchClusterStatusFromIstiod(ctx context.Context, clientHTTP *http.Client, token, ip string) ([]ClusterDebugInfo, error) {
	url := fmt.Sprintf("http://%s:15014/debug/clusterz", ip)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request for %s: %w", ip, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := clientHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error polling %s: %w", ip, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response from %s: %s", ip, resp.Status)
	}

	var data []ClusterDebugInfo
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("error decoding response from %s: %w", ip, err)
	}

	return data, nil
}

func (p *PrometheusExporterMetrics) setClusterStatus(istiod, clusterID, secretName, syncStatus string) {
	val := 0.0
	if syncStatus == "synced" {
		val = 1.0
	}
	p.clusterUp.WithLabelValues(istiod, clusterID, secretName).Set(val)
}

func (p *PrometheusExporterMetrics) DeleteIstiodMetrics(podName string) {
	if podName == "" {
		return
	}

	removed := p.clusterUp.DeletePartialMatch(prometheus.Labels{"istiod": podName})
	if removed > 0 {
		log.Info(fmt.Sprintf("Removed %d istio_remote_cluster_up series for pod %s", removed, podName))
	}
}
