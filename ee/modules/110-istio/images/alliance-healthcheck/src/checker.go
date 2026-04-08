/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

var (
	federationGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "istiofederations",
	}
	multiclusterGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "istiomulticlusters",
	}
)

type CheckerConfig struct {
	ClusterUUID         string
	ClusterDomain       string
	FederationEnabled   bool
	MulticlusterEnabled bool
	CheckInterval       time.Duration
	RequestTimeout      time.Duration
}

type Checker struct {
	dynClient  dynamic.Interface
	httpClient *http.Client
	config     CheckerConfig
	metric     *prometheus.GaugeVec
}

func NewChecker(dynClient dynamic.Interface, reg prometheus.Registerer, cfg CheckerConfig) *Checker {
	metric := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "d8_istio_alliance_dataplane_health_check",
			Help: "Data-plane connectivity health check (1 = healthy, 0 = unhealthy)",
		},
		[]string{"type", "name", "target"},
	)
	reg.MustRegister(metric)

	return &Checker{
		dynClient: dynClient,
		httpClient: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
		config: cfg,
		metric: metric,
	}
}

func (c *Checker) Run(ctx context.Context) {
	logger.Println("Starting health checker loop")
	c.runOnce(ctx)

	ticker := time.NewTicker(c.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Println("Health checker stopped")
			return
		case <-ticker.C:
			c.runOnce(ctx)
		}
	}
}

func (c *Checker) runOnce(ctx context.Context) {
	c.metric.Reset()

	if c.config.FederationEnabled {
		c.checkFederations(ctx)
	}
	if c.config.MulticlusterEnabled {
		c.checkMulticlusters(ctx)
	}
}

func (c *Checker) checkFederations(ctx context.Context) {
	list, err := c.dynClient.Resource(federationGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Printf("Failed to list IstioFederations: %v", err)
		return
	}

	for _, item := range list.Items {
		name := item.GetName()

		target, err := c.findFederationHealthcheckTarget(item)
		if err != nil {
			logger.Printf("Federation cluster '%s' check: cannot determine healthcheck target: %v", name, err)
			c.metric.WithLabelValues("federation", name, "unknown").Set(0)
			c.patchDataPlaneHealth(ctx, federationGVR, name, false, err.Error())
			continue
		}

		url := fmt.Sprintf("http://%s:80/healthz", target)
		healthy, msg := c.curlHealthz(url)

		c.metric.WithLabelValues("federation", name, target).Set(boolToFloat(healthy))
		c.patchDataPlaneHealth(ctx, federationGVR, name, healthy, msg)

		logger.Printf("Federation cluster '%s' check: %s target=%s healthy=%v msg=%s", name, target, healthy, msg)
	}
}

func (c *Checker) checkMulticlusters(ctx context.Context) {
	list, err := c.dynClient.Resource(multiclusterGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Printf("Failed to list IstioMulticlusters: %v", err)
		return
	}

	for _, item := range list.Items {
		name := item.GetName()

		remoteUUID, err := c.extractRemoteClusterUUID(item)
		if err != nil {
			logger.Printf("Multicluster '%s' check: cannot determine remote UUID: %v", name, err)
			c.metric.WithLabelValues("multicluster", name, "unknown").Set(0)
			c.patchDataPlaneHealth(ctx, multiclusterGVR, name, false, err.Error())
			continue
		}

		target := fmt.Sprintf("alliance-healthcheck-%s.d8-istio.svc.%s", remoteUUID, c.config.ClusterDomain)
		url := fmt.Sprintf("http://%s:80/healthz", target)
		healthy, msg := c.curlHealthz(url)

		c.metric.WithLabelValues("multicluster", name, target).Set(boolToFloat(healthy))
		c.patchDataPlaneHealth(ctx, multiclusterGVR, name, healthy, msg)

		logger.Printf("Multicluster '%s' check: target=%s healthy=%v msg=%s", name, target, healthy, msg)
	}
}

func (c *Checker) findFederationHealthcheckTarget(obj unstructured.Unstructured) (string, error) {
	privateRaw, found, err := unstructured.NestedMap(obj.Object, "status", "metadataCache", "private")
	if err != nil || !found {
		return "", fmt.Errorf("private metadata not available yet")
	}

	privateJSON, err := json.Marshal(privateRaw)
	if err != nil {
		return "", fmt.Errorf("cannot marshal private metadata: %w", err)
	}

	var pm FederationPrivateMetadata
	if err := json.Unmarshal(privateJSON, &pm); err != nil {
		return "", fmt.Errorf("cannot unmarshal private metadata: %w", err)
	}

	if pm.PublicServices == nil {
		return "", fmt.Errorf("no public services in private metadata")
	}

	for _, svc := range *pm.PublicServices {
		if strings.HasPrefix(svc.Hostname, "alliance-healthcheck.d8-istio.svc.") {
			return svc.Hostname, nil
		}
	}

	return "", fmt.Errorf("alliance-healthcheck service not found in remote public services")
}

func (c *Checker) extractRemoteClusterUUID(obj unstructured.Unstructured) (string, error) {
	publicRaw, found, err := unstructured.NestedMap(obj.Object, "status", "metadataCache", "public")
	if err != nil || !found {
		return "", fmt.Errorf("public metadata not available yet")
	}

	publicJSON, err := json.Marshal(publicRaw)
	if err != nil {
		return "", fmt.Errorf("cannot marshal public metadata: %w", err)
	}

	var pm AlliancePublicMetadata
	if err := json.Unmarshal(publicJSON, &pm); err != nil {
		return "", fmt.Errorf("cannot unmarshal public metadata: %w", err)
	}

	if pm.ClusterUUID == "" {
		return "", fmt.Errorf("remote clusterUUID is empty")
	}

	return pm.ClusterUUID, nil
}

func (c *Checker) curlHealthz(url string) (bool, string) {
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return false, fmt.Sprintf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, "ok"
	}
	return false, fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
}

func (c *Checker) patchDataPlaneHealth(ctx context.Context, gvr schema.GroupVersionResource, name string, connected bool, message string) {
	now := time.Now().UTC().Format(time.RFC3339)

	status := DataPlaneHealthStatus{
		Connected:          connected,
		LastCheckTimestamp: now,
		Message:            message,
	}
	if connected {
		status.LastSuccessTimestamp = now
	}

	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"dataPlaneHealth": status,
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		logger.Printf("Failed to marshal status patch for %s/%s: %v", gvr.Resource, name, err)
		return
	}

	_, err = c.dynClient.Resource(gvr).Patch(ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		logger.Printf("Failed to patch status for %s/%s: %v", gvr.Resource, name, err)
	}
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
