// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// These tests intentionally exercise (*fromClusterMetaConfigFiller).Cloud
// without going through the full TestParseConfigFromCluster harness, which
// requires the werf-bundled /deckhouse/candi tree and is skipped locally.
// They verify the mc-flow vs legacy preference rules independently, so a
// regression in Cloud's loader is caught even outside CI.

func mustMetaConfigForProvider(t *testing.T, providerName string) *MetaConfig {
	t.Helper()
	cloud, err := json.Marshal(ClusterConfigCloudSpec{Provider: providerName})
	require.NoError(t, err)
	return &MetaConfig{
		ClusterConfig: map[string]json.RawMessage{
			"cloud": cloud,
		},
	}
}

func mustSeedCloudProviderMC(t *testing.T, kubeCl *client.KubernetesClient, providerName string) {
	t.Helper()
	// schemaStore=nil in these tests skips the validation path entirely,
	// so the content of settings does not matter for correctness. Keep it
	// empty to remain compatible should a future test wire a real schema
	// store: real cloud-provider-<name> schemas do not expose
	// nodes.parameters.layout.
	mc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata":   map[string]interface{}{"name": "cloud-provider-" + providerName},
		"spec": map[string]interface{}{
			"version":  float64(2),
			"enabled":  true,
			"settings": map[string]interface{}{},
		},
	}}
	_, err := kubeCl.Dynamic().Resource(ModuleConfigGVR).Create(t.Context(), mc, metav1.CreateOptions{})
	require.NoError(t, err)
}

func TestCloudFiller_McFlowOnly_NoLegacy(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	mustSeedCloudProviderMC(t, kubeCl, "yandex")

	mc := mustMetaConfigForProvider(t, "yandex")
	// An empty store isolates the preference logic from schema availability:
	// ModuleConfig validation tolerates ErrSchemaNotFound.
	filler := newFromClusterMetaConfigFiller(kubeCl, newSchemaStore(nil, nil))

	_, err := filler.Cloud(context.Background(), mc)
	require.NoError(t, err)
	require.Empty(t, mc.ProviderClusterConfig, "PCC must stay unset in mc-flow")
	require.Len(t, mc.ModuleConfigs, 1)
	require.Equal(t, "cloud-provider-yandex", mc.ModuleConfigs[0].GetName())
}

// mustSeedYandexSettingsMC creates cloud-provider-yandex with the given spec.enabled,
// or without the field when enabled is nil. The 2022 ConfigMap-to-ModuleConfig
// migration created such settings-only objects without spec.enabled.
func mustSeedYandexSettingsMC(t *testing.T, kubeCl *client.KubernetesClient, enabled *bool) {
	t.Helper()
	spec := map[string]interface{}{
		"version": float64(1),
		"settings": map[string]interface{}{
			"storageClass": map[string]interface{}{"default": "network-ssd"},
		},
	}
	if enabled != nil {
		spec["enabled"] = *enabled
	}
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata":   map[string]interface{}{"name": "cloud-provider-yandex"},
		"spec":       spec,
	}}
	_, err := kubeCl.Dynamic().Resource(ModuleConfigGVR).Create(t.Context(), obj, metav1.CreateOptions{})
	require.NoError(t, err)
}

func TestLoadCloudProviderModuleConfig_NotEnabledIsIgnored(t *testing.T) {
	disabled := false
	for name, enabled := range map[string]*bool{"without enabled": nil, "enabled false": &disabled} {
		t.Run(name, func(t *testing.T) {
			kubeCl := client.NewFakeKubernetesClient()
			mustSeedYandexSettingsMC(t, kubeCl, enabled)

			mc, err := loadCloudProviderModuleConfig(t.Context(), kubeCl, "yandex", newSchemaStore(nil, nil))
			require.NoError(t, err)
			require.Nil(t, mc)
		})
	}
}

func TestClusterUsesProviderModuleConfig_WithoutEnabled(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	testCreateKubeSystemSecret(t, kubeCl, "d8-cluster-configuration", map[string][]byte{
		"cluster-configuration.yaml": []byte("clusterType: Cloud\ncloud:\n  provider: Yandex\n"),
	})
	mustSeedYandexSettingsMC(t, kubeCl, nil)

	usesMC, err := ClusterUsesProviderModuleConfig(t.Context(), kubeCl)
	require.NoError(t, err)
	require.False(t, usesMC)
}

func TestCloudFiller_NeitherMarker(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	mc := mustMetaConfigForProvider(t, "yandex")
	filler := newFromClusterMetaConfigFiller(kubeCl, newSchemaStore(nil, nil))

	_, err := filler.Cloud(context.Background(), mc)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ModuleConfig")
	require.Contains(t, err.Error(), LegacyProviderClusterConfigSecret)
}
