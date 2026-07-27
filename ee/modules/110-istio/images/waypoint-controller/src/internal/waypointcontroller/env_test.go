//go:build !integration

/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"encoding/json"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func envByName(envs []corev1.EnvVar) map[string]*corev1.EnvVar {
	m := make(map[string]*corev1.EnvVar, len(envs))
	for i := range envs {
		m[envs[i].Name] = &envs[i]
	}
	return m
}

func TestWaypointEnv_AllRequiredKeys(t *testing.T) {
	envs, err := WaypointEnv(newWaypointPodSpecConfig())
	if err != nil {
		t.Fatalf("WaypointEnv returned error: %v", err)
	}
	got := envByName(envs)

	requiredKeys := []string{
		"ISTIO_META_SERVICE_ACCOUNT",
		"ISTIO_META_NODE_NAME",
		"PILOT_CERT_PROVIDER",
		"CA_ADDR",
		"POD_NAME",
		"POD_NAMESPACE",
		"INSTANCE_IP",
		"SERVICE_ACCOUNT",
		"HOST_IP",
		"ISTIO_CPU_LIMIT",
		"PROXY_CONFIG",
		"CLOUD_PLATFORM",
		"ISTIO_META_DNS_AUTO_ALLOCATE",
		"ISTIO_META_DNS_CAPTURE",
		"ISTIO_META_ENABLE_HBONE",
		"ISTIO_META_IDLE_TIMEOUT",
		"PROXY_CONFIG_XDS_AGENT",
		"GOMEMLIMIT",
		"GOMAXPROCS",
		"ISTIO_META_CLUSTER_ID",
		"ISTIO_META_NETWORK",
		"ISTIO_META_INTERCEPTION_MODE",
		"ISTIO_META_WORKLOAD_NAME",
		"ISTIO_META_OWNER",
		"ISTIO_META_MESH_ID",
		"TRUST_DOMAIN",
	}

	for _, key := range requiredKeys {
		if _, ok := got[key]; !ok {
			t.Errorf("missing env var %q", key)
		}
	}

	// Detect duplicates: every name in the env slice must appear exactly once.
	counts := map[string]int{}
	for _, e := range envs {
		counts[e.Name]++
	}
	for name, n := range counts {
		if n > 1 {
			t.Errorf("env var %q appears %d times, want 1", name, n)
		}
	}

	// Detect unexpected env vars: every produced env name must be in requiredKeys.
	want := make(map[string]struct{}, len(requiredKeys))
	for _, k := range requiredKeys {
		want[k] = struct{}{}
	}
	for name := range counts {
		if _, ok := want[name]; !ok {
			t.Errorf("unexpected env var %q", name)
		}
	}

	if len(envs) != len(requiredKeys) {
		t.Errorf("env var count = %d, want %d", len(envs), len(requiredKeys))
	}
}

func TestWaypointEnv_DownwardAPI(t *testing.T) {
	envs, err := WaypointEnv(newWaypointPodSpecConfig())
	if err != nil {
		t.Fatalf("WaypointEnv returned error: %v", err)
	}
	e := envByName(envs)

	cases := []struct {
		envName   string
		fieldPath string
	}{
		{"ISTIO_META_SERVICE_ACCOUNT", "spec.serviceAccountName"},
		{"ISTIO_META_NODE_NAME", "spec.nodeName"},
		{"POD_NAME", "metadata.name"},
		{"POD_NAMESPACE", "metadata.namespace"},
		{"INSTANCE_IP", "status.podIP"},
		{"SERVICE_ACCOUNT", "spec.serviceAccountName"},
		{"HOST_IP", "status.hostIP"},
	}

	for _, tc := range cases {
		t.Run(tc.envName, func(t *testing.T) {
			env, ok := e[tc.envName]
			if !ok {
				t.Fatalf("env var %q not found", tc.envName)
			}
			if env.ValueFrom == nil || env.ValueFrom.FieldRef == nil {
				t.Fatalf("env var %q: expected FieldRef, got nil", tc.envName)
			}
			if env.ValueFrom.FieldRef.FieldPath != tc.fieldPath {
				t.Errorf("env var %q FieldPath = %q, want %q",
					tc.envName, env.ValueFrom.FieldRef.FieldPath, tc.fieldPath)
			}
		})
	}
}

func TestWaypointEnv_ResourceFieldRefs(t *testing.T) {
	envs, err := WaypointEnv(newWaypointPodSpecConfig())
	if err != nil {
		t.Fatalf("WaypointEnv returned error: %v", err)
	}
	e := envByName(envs)

	cases := []struct {
		envName  string
		resource string
	}{
		{"ISTIO_CPU_LIMIT", "limits.cpu"},
		{"GOMEMLIMIT", "limits.memory"},
		{"GOMAXPROCS", "limits.cpu"},
	}

	for _, tc := range cases {
		t.Run(tc.envName, func(t *testing.T) {
			env, ok := e[tc.envName]
			if !ok {
				t.Fatalf("env var %q not found", tc.envName)
			}
			if env.ValueFrom == nil || env.ValueFrom.ResourceFieldRef == nil {
				t.Fatalf("env var %q: expected ResourceFieldRef, got nil", tc.envName)
			}
			if env.ValueFrom.ResourceFieldRef.Resource != tc.resource {
				t.Errorf("env var %q Resource = %q, want %q",
					tc.envName, env.ValueFrom.ResourceFieldRef.Resource, tc.resource)
			}
		})
	}
}

func TestWaypointEnv_StaticValues(t *testing.T) {
	cfg := newWaypointPodSpecConfig()
	envs, err := WaypointEnv(cfg)
	if err != nil {
		t.Fatalf("WaypointEnv returned error: %v", err)
	}
	e := envByName(envs)

	cases := []struct {
		envName string
		want    string
	}{
		{"PILOT_CERT_PROVIDER", "istiod"},
		{"CA_ADDR", fmt.Sprintf("istiod-%s.d8-istio.svc:15012", cfg.IstioRevision)},
		{"CLOUD_PLATFORM", cfg.IstioCloudPlatform},
		{"ISTIO_META_DNS_AUTO_ALLOCATE", "true"},
		{"ISTIO_META_DNS_CAPTURE", "true"},
		{"ISTIO_META_ENABLE_HBONE", "true"},
		{"ISTIO_META_IDLE_TIMEOUT", "1h"},
		{"PROXY_CONFIG_XDS_AGENT", "true"},
		{"ISTIO_META_CLUSTER_ID", cfg.IstioClusterID},
		{"ISTIO_META_NETWORK", cfg.IstioNetworkName},
		{"ISTIO_META_INTERCEPTION_MODE", "REDIRECT"},
		{"ISTIO_META_WORKLOAD_NAME", "d8-waypoint-main"},
		{"ISTIO_META_OWNER", fmt.Sprintf("kubernetes://apis/apps/v1/namespaces/%s/deployments/d8-waypoint-%s", cfg.Namespace, cfg.InstanceName)},
		{"ISTIO_META_MESH_ID", "d8-istio-mesh"},
		{"TRUST_DOMAIN", cfg.ClusterDomain},
	}

	for _, tc := range cases {
		t.Run(tc.envName, func(t *testing.T) {
			env, ok := e[tc.envName]
			if !ok {
				t.Fatalf("env var %q not found", tc.envName)
			}
			if env.Value != tc.want {
				t.Errorf("env var %q = %q, want %q", tc.envName, env.Value, tc.want)
			}
		})
	}
}

func TestWaypointEnv_ProxyConfigJSON(t *testing.T) {
	cfg := newWaypointPodSpecConfig()
	envs, err := WaypointEnv(cfg)
	if err != nil {
		t.Fatalf("WaypointEnv returned error: %v", err)
	}
	e := envByName(envs)

	env, ok := e["PROXY_CONFIG"]
	if !ok {
		t.Fatal("PROXY_CONFIG env var not found")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(env.Value), &parsed); err != nil {
		t.Fatalf("PROXY_CONFIG is not valid JSON: %v", err)
	}

	if parsed["discoveryAddress"] != fmt.Sprintf("istiod-%s.d8-istio.svc:15012", cfg.IstioRevision) {
		t.Errorf("discoveryAddress = %v", parsed["discoveryAddress"])
	}
	if parsed["holdApplicationUntilProxyStarts"] != false {
		t.Errorf("holdApplicationUntilProxyStarts = %v, want false", parsed["holdApplicationUntilProxyStarts"])
	}
	if parsed["meshId"] != "d8-istio-mesh" {
		t.Errorf("meshId = %v, want d8-istio-mesh", parsed["meshId"])
	}

	proxyMeta, ok := parsed["proxyMetadata"].(map[string]interface{})
	if !ok {
		t.Fatal("proxyMetadata missing or not an object")
	}

	expectedProxyMeta := map[string]string{
		"CLOUD_PLATFORM":               cfg.IstioCloudPlatform,
		"ISTIO_META_DNS_AUTO_ALLOCATE": "true",
		"ISTIO_META_DNS_CAPTURE":       "true",
		"ISTIO_META_ENABLE_HBONE":      "true",
		"ISTIO_META_IDLE_TIMEOUT":      "1h",
		"PROXY_CONFIG_XDS_AGENT":       "true",
	}
	for key, want := range expectedProxyMeta {
		got, ok := proxyMeta[key]
		if !ok {
			t.Errorf("proxyMetadata[%q] missing", key)
			continue
		}
		if got != want {
			t.Errorf("proxyMetadata[%q] = %v, want %q", key, got, want)
		}
	}

	// Ensure no unexpected keys in proxyMetadata
	if len(proxyMeta) != len(expectedProxyMeta) {
		t.Errorf("proxyMetadata has %d keys, want %d", len(proxyMeta), len(expectedProxyMeta))
	}
}
