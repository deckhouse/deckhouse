/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

const (
	allianceHealthcheckUserAgent = "alliance-healthcheck/1.0"
	healthzBodyReadLimit         = 4096
)

const (
	AllianceKindMulticluster string = "IstioMulticluster"
	AllianceKindFederation   string = "IstioFederation"
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
			Help: "Data-plane connectivity health check (1 = healthy, 0 = unhealthy). Label remote_cluster_uuid identifies the peer cluster.",
		},
		[]string{"alliance_kind", "name", "remote_cluster_uuid"},
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

		remoteUUID, err := c.extractRemoteClusterUUID(item)
		if err != nil {
			logger.Printf("Federation cluster '%s' check: cannot determine remote UUID: %v", name, err)
			c.metric.WithLabelValues(AllianceKindFederation, name, "unknown").Set(0)
			c.patchDataplaneConnectionCondition(ctx, federationGVR, name, metav1.ConditionFalse, "MetadataUnavailable", err.Error())
			continue
		}

		target, err := c.findFederationHealthcheckTarget(item)
		if err != nil {
			logger.Printf("Federation cluster '%s' check: cannot determine healthcheck target: %v", name, err)
			c.metric.WithLabelValues(AllianceKindFederation, name, remoteUUID).Set(0)
			c.patchDataplaneConnectionCondition(ctx, federationGVR, name, metav1.ConditionFalse, "ConfigurationError", err.Error())
			continue
		}

		url := fmt.Sprintf("http://%s:80/healthz", target)
		healthy, msg := c.curlHealthz(ctx, url)

		c.metric.WithLabelValues(AllianceKindFederation, name, remoteUUID).Set(boolToFloat(healthy))
		c.patchDataplaneConnectionCondition(ctx, federationGVR, name, healthToConditionStatus(healthy), healthToReason(healthy), msg)

		logger.Printf("Federation cluster '%s' check: target=%s remoteUUID=%s healthy=%v msg=%s", name, target, remoteUUID, healthy, msg)
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
			c.metric.WithLabelValues(AllianceKindMulticluster, name, "unknown").Set(0)
			c.patchDataplaneConnectionCondition(ctx, multiclusterGVR, name, metav1.ConditionFalse, "MetadataUnavailable", err.Error())
			continue
		}

		target := fmt.Sprintf("alliance-healthcheck-%s.d8-istio.svc.%s", remoteUUID, c.config.ClusterDomain)
		url := fmt.Sprintf("http://%s:80/healthz", target)
		healthy, msg := c.curlHealthz(ctx, url)

		c.metric.WithLabelValues(AllianceKindMulticluster, name, remoteUUID).Set(boolToFloat(healthy))
		c.patchDataplaneConnectionCondition(ctx, multiclusterGVR, name, healthToConditionStatus(healthy), healthToReason(healthy), msg)

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
		if strings.HasPrefix(svc.Hostname, "alliance-healthcheck-") && strings.Contains(svc.Hostname, ".d8-istio.svc.") {
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

func (c *Checker) curlHealthz(ctx context.Context, url string) (bool, string) {
	ok, _, msg := c.curlHealthzWithBody(ctx, url)
	return ok, msg
}

func (c *Checker) curlHealthzWithBody(ctx context.Context, url string) (bool, string, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, "", fmt.Sprintf("request build failed: %v", err)
	}
	req.Header.Set("User-Agent", allianceHealthcheckUserAgent)
	req.Header.Set("X-alliance-healthcheck-from", c.config.ClusterUUID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, "", fmt.Sprintf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, healthzBodyReadLimit)
	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return false, "", fmt.Sprintf("read body failed: %v", err)
	}

	return true, strings.TrimSpace(string(bodyBytes)), "ok"
}

const conditionTypeDataplaneConnectionReady = "DataplaneConnectionReady"

func healthToConditionStatus(healthy bool) metav1.ConditionStatus {
	if healthy {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func healthToReason(healthy bool) string {
	if healthy {
		return "Succeeded"
	}
	return "ProbeFailed"
}

func priorDataplaneConditionMap(conds []interface{}) map[string]interface{} {
	for _, c := range conds {
		m, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := m["type"].(string); t == conditionTypeDataplaneConnectionReady {
			return m
		}
	}
	return nil
}

func buildDataplaneConditionRow(prior map[string]interface{}, status, reason, message string, probe time.Time) map[string]interface{} {
	if reason == "" {
		reason = "Unknown"
	}
	probeStr := probe.UTC().Format(time.RFC3339)
	transition := probeStr
	if prior != nil {
		if ps, ok := prior["status"].(string); ok && ps == status {
			if pr, ok := prior["reason"].(string); ok && pr == reason {
				if pm, ok := prior["message"].(string); ok && pm == message {
					if ts, ok := prior["lastTransitionTime"].(string); ok && ts != "" {
						transition = ts
					}
				}
			}
		}
	}
	return map[string]interface{}{
		"type":               conditionTypeDataplaneConnectionReady,
		"status":             status,
		"reason":             reason,
		"message":            message,
		"lastTransitionTime": transition,
		"lastProbeTime":      probeStr,
	}
}

func mergeDataplaneConditionIntoConditions(conds []interface{}, newRow map[string]interface{}) []interface{} {
	out := make([]interface{}, 0, len(conds)+1)
	replaced := false
	for _, c := range conds {
		m, ok := c.(map[string]interface{})
		if ok {
			if t, _ := m["type"].(string); t == conditionTypeDataplaneConnectionReady {
				out = append(out, newRow)
				replaced = true
				continue
			}
		}
		out = append(out, c)
	}
	if !replaced {
		out = append(out, newRow)
	}
	return out
}

func (c *Checker) patchDataplaneConnectionCondition(ctx context.Context, gvr schema.GroupVersionResource, name string, status metav1.ConditionStatus, reason, message string) {
	probe := time.Now().UTC()

	obj, err := c.dynClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Printf("Failed to get %s/%s for DataplaneConnectionReady patch: %v", gvr.Resource, name, err)
		return
	}

	var condsSlice []interface{}
	if conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions"); err == nil && found {
		condsSlice = conditions
	}

	prior := priorDataplaneConditionMap(condsSlice)
	newRow := buildDataplaneConditionRow(prior, string(status), reason, message, probe)
	newConds := mergeDataplaneConditionIntoConditions(condsSlice, newRow)

	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": newConds,
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
