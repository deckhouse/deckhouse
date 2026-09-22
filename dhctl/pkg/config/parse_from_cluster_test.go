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

func TestCloudFiller_NeitherMarker(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	mc := mustMetaConfigForProvider(t, "yandex")
	filler := newFromClusterMetaConfigFiller(kubeCl, newSchemaStore(nil, nil))

	_, err := filler.Cloud(context.Background(), mc)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ModuleConfig")
	require.Contains(t, err.Error(), LegacyProviderClusterConfigSecret)
}

func mustSeedGlobalMC(t *testing.T, kubeCl *client.KubernetesClient, prefix string) {
	t.Helper()
	settings := map[string]interface{}{}
	if prefix != "" {
		settings["prefix"] = prefix
	}
	mc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata":   map[string]interface{}{"name": "global"},
		"spec": map[string]interface{}{
			"version":  float64(2),
			"settings": settings,
		},
	}}
	_, err := kubeCl.Dynamic().Resource(ModuleConfigGVR).Create(t.Context(), mc, metav1.CreateOptions{})
	require.NoError(t, err)
}

func TestLoadGlobalModuleConfig_Present(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	mustSeedGlobalMC(t, kubeCl, "mcprefix")

	gmc := loadGlobalModuleConfig(context.Background(), kubeCl)
	require.NotNil(t, gmc)
	require.Equal(t, "mcprefix", gmc.Spec.Settings["prefix"])
}

func TestLoadGlobalModuleConfig_Absent(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	// Missing global ModuleConfig must be a soft miss (nil), never an error:
	// converge/destroy fall back to ClusterConfiguration.cloud.prefix.
	gmc := loadGlobalModuleConfig(context.Background(), kubeCl)
	require.Nil(t, gmc)
}

func mustSeedControlPlaneManagerMC(t *testing.T, kubeCl *client.KubernetesClient, network map[string]interface{}) {
	t.Helper()
	settings := map[string]interface{}{}
	if network != nil {
		settings["network"] = network
	}
	mc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata":   map[string]interface{}{"name": "control-plane-manager"},
		"spec": map[string]interface{}{
			"version":  float64(3),
			"settings": settings,
		},
	}}
	_, err := kubeCl.Dynamic().Resource(ModuleConfigGVR).Create(t.Context(), mc, metav1.CreateOptions{})
	require.NoError(t, err)
}

func TestLoadControlPlaneManagerModuleConfig_Present(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	mustSeedControlPlaneManagerMC(t, kubeCl, map[string]interface{}{"podSubnetCIDR": "10.11.0.0/16"})

	cpm := loadControlPlaneManagerModuleConfig(context.Background(), kubeCl)
	require.NotNil(t, cpm)
	network, _ := cpm.Spec.Settings["network"].(map[string]interface{})
	require.Equal(t, "10.11.0.0/16", network["podSubnetCIDR"])
}

func TestLoadControlPlaneManagerModuleConfig_Absent(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	// Missing control-plane-manager ModuleConfig must be a soft miss (nil), never an error:
	// converge/destroy fall back to the deprecated ClusterConfiguration network fields.
	cpm := loadControlPlaneManagerModuleConfig(context.Background(), kubeCl)
	require.Nil(t, cpm)
}

// This is the regression the bug report was: on a cluster whose control-plane-manager
// ModuleConfig owns the network settings and has had them removed from ClusterConfiguration,
// Terraform's clusterConfiguration variable must still carry podSubnetCIDR - not the "object has
// no such attribute" a converge/destroy run got before Cloud() loaded this ModuleConfig at all.
func TestCloudFiller_LoadsControlPlaneManagerNetworkSettings(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	mustSeedCloudProviderMC(t, kubeCl, "yandex")
	mustSeedControlPlaneManagerMC(t, kubeCl, map[string]interface{}{
		"podSubnetCIDR":     "10.11.0.0/16",
		"serviceSubnetCIDR": "10.22.0.0/16",
	})

	// podSubnetCIDR/serviceSubnetCIDR already removed from ClusterConfiguration - only
	// ModuleConfig has them, exactly like the reported cluster.
	mc := mustMetaConfigForProvider(t, "yandex")
	mc.ClusterType = CloudClusterType
	filler := newFromClusterMetaConfigFiller(kubeCl, newSchemaStore(nil, nil))

	_, err := filler.Cloud(context.Background(), mc)
	require.NoError(t, err)

	infra := mc.clusterConfigForInfrastructure()
	var podSubnetCIDR, serviceSubnetCIDR string
	require.NoError(t, json.Unmarshal(infra["podSubnetCIDR"], &podSubnetCIDR),
		"var.clusterConfiguration.podSubnetCIDR must be present for Terraform")
	require.NoError(t, json.Unmarshal(infra["serviceSubnetCIDR"], &serviceSubnetCIDR))
	require.Equal(t, "10.11.0.0/16", podSubnetCIDR)
	require.Equal(t, "10.22.0.0/16", serviceSubnetCIDR)
}
