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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
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
	mc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata":   map[string]interface{}{"name": "cloud-provider-" + providerName},
		"spec": map[string]interface{}{
			"version": float64(2),
			"enabled": true,
			"settings": map[string]interface{}{
				"nodes": map[string]interface{}{
					"parameters": map[string]interface{}{"layout": "Standard"},
				},
			},
		},
	}}
	_, err := kubeCl.Dynamic().Resource(ModuleConfigGVR).Create(t.Context(), mc, metav1.CreateOptions{})
	require.NoError(t, err)
}

func mustSeedLegacyPCCSecret(t *testing.T, kubeCl *client.KubernetesClient, payload string) {
	t.Helper()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      legacyProviderClusterConfigSecretName,
			Namespace: global.ConfigsNS,
		},
		Data: map[string][]byte{
			"cloud-provider-cluster-configuration.yaml": []byte(payload),
		},
	}
	_, err := kubeCl.CoreV1().Secrets(global.ConfigsNS).Create(t.Context(), secret, metav1.CreateOptions{})
	require.NoError(t, err)
}

const legacyPCCMinimalYAML = `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithoutNAT
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: imageId
    platform: standard-v2
sshPublicKey: ssh-rsa AAAAB3NzaC
nodeNetworkCIDR: 10.100.0.0/21
provider:
  cloudID: cloudId
  folderID: folderId
  serviceAccountJSON: "{}"
`

func TestCloudFiller_McFlowOnly_NoLegacy(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	mustSeedCloudProviderMC(t, kubeCl, "yandex")

	mc := mustMetaConfigForProvider(t, "yandex")
	// schemaStore=nil disables validation: moduleConfigFromUnstructured
	// accepts the document, which is what we want when isolating the
	// preference logic from schema availability.
	filler := newFromClusterMetaConfigFiller(kubeCl, nil)

	_, err := filler.Cloud(context.Background(), mc)
	require.NoError(t, err)
	require.Empty(t, mc.ProviderClusterConfig, "PCC must stay unset in mc-flow")
	require.Len(t, mc.ModuleConfigs, 1)
	require.Equal(t, "cloud-provider-yandex", mc.ModuleConfigs[0].GetName())
}

func TestCloudFiller_McFlowWinsOverLegacy(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	mustSeedCloudProviderMC(t, kubeCl, "yandex")
	mustSeedLegacyPCCSecret(t, kubeCl, legacyPCCMinimalYAML)

	mc := mustMetaConfigForProvider(t, "yandex")
	filler := newFromClusterMetaConfigFiller(kubeCl, nil)

	_, err := filler.Cloud(context.Background(), mc)
	require.NoError(t, err)
	// Critical regression check: legacy PCC must be ignored when MC is
	// present, otherwise extractProviderClusterFields keeps using the
	// stale pre-migration values forever.
	require.Empty(t, mc.ProviderClusterConfig, "legacy PCC must be ignored when MC is present")
	require.Len(t, mc.ModuleConfigs, 1)
}

func TestCloudFiller_LegacyOnly_NoMC(t *testing.T) {
	// We need a real SchemaStore for legacy PCC validation; the harness
	// for that lives in TestParseConfigFromCluster which is CI-only.
	// Skip locally — TestParseConfigFromCluster covers this path when
	// the werf-bundled candi tree is available.
	t.Skip("legacy-only validation requires SchemaStore — covered by TestParseConfigFromCluster on CI")
}

func TestCloudFiller_NeitherMarker(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()
	mc := mustMetaConfigForProvider(t, "yandex")
	filler := newFromClusterMetaConfigFiller(kubeCl, nil)

	_, err := filler.Cloud(context.Background(), mc)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ModuleConfig")
	require.Contains(t, err.Error(), legacyProviderClusterConfigSecretName)
}
