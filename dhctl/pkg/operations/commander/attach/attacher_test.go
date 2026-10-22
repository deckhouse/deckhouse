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

package attach

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func TestSSHPublicKeyFromMetaConfig(t *testing.T) {
	const key = "ssh-ed25519 AAAA test@example"

	moduleSettings := func(parameters map[string]any) *config.CloudProviderVars {
		return &config.CloudProviderVars{Settings: map[string]any{
			"spec": map[string]any{
				"settings": map[string]any{
					"nodes": map[string]any{"parameters": parameters},
				},
			},
		}}
	}

	tests := map[string]struct {
		metaConfig *config.MetaConfig
		want       string
		wantErr    string
	}{
		"legacy provider configuration": {
			metaConfig: &config.MetaConfig{
				ProviderName:          "DVP",
				ProviderClusterConfig: map[string]json.RawMessage{"sshPublicKey": json.RawMessage(`"` + key + `"`)},
			},
			want: key,
		},
		"legacy provider configuration without the key": {
			metaConfig: &config.MetaConfig{
				ProviderName:          "DVP",
				ProviderClusterConfig: map[string]json.RawMessage{"layout": json.RawMessage(`"Standard"`)},
			},
			wantErr: "sshPublicKey not found in cloud provider config",
		},
		"legacy GCP uses sshKey": {
			metaConfig: &config.MetaConfig{
				ProviderName:          "GCP",
				ProviderClusterConfig: map[string]json.RawMessage{"sshKey": json.RawMessage(`"` + key + `"`)},
			},
			want: key,
		},
		"new format reads the key from the provider module settings": {
			metaConfig: &config.MetaConfig{
				ProviderName:      "DVP",
				CloudProviderVars: moduleSettings(map[string]any{"sshPublicKey": key}),
			},
			want: key,
		},
		"new format without the key is not an error": {
			metaConfig: &config.MetaConfig{
				ProviderName:      "DVP",
				CloudProviderVars: moduleSettings(map[string]any{"layout": "Standard"}),
			},
		},
		"new format without module settings is not an error": {
			metaConfig: &config.MetaConfig{ProviderName: "DVP"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := sshPublicKeyFromMetaConfig(tt.metaConfig)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestDropLeftoverLegacyProviderConfig(t *testing.T) {
	legacyDocument := map[string]json.RawMessage{"layout": json.RawMessage(`"Standard"`)}

	providerModuleConfig := func(settings map[string]any) *config.ModuleConfig {
		moduleConfig := &config.ModuleConfig{}
		moduleConfig.SetName("cloud-provider-dvp")
		moduleConfig.Spec.Enabled = ptr.To(true)
		moduleConfig.Spec.Settings = settings

		return moduleConfig
	}

	newModelSettings := map[string]any{"nodes": map[string]any{"parameters": map[string]any{"layout": "Standard"}}}

	migratedCluster := func() *config.MetaConfig {
		return &config.MetaConfig{
			ClusterType:           config.CloudClusterType,
			ProviderName:          "dvp",
			ProviderClusterConfig: legacyDocument,
			ModuleConfigs:         []*config.ModuleConfig{providerModuleConfig(newModelSettings)},
		}
	}

	tests := map[string]struct {
		metaConfig *config.MetaConfig
		migrating  bool
		wantLegacy bool
	}{
		"a migrated cluster drops the leftover legacy document": {
			metaConfig: migratedCluster(),
		},
		"a migration in progress keeps the legacy document": {
			metaConfig: migratedCluster(),
			migrating:  true,
			wantLegacy: true,
		},
		"a module only switched on keeps the legacy document of a legacy cluster": {
			metaConfig: func() *config.MetaConfig {
				mc := migratedCluster()
				mc.ModuleConfigs = []*config.ModuleConfig{providerModuleConfig(nil)}
				return mc
			}(),
			wantLegacy: true,
		},
		"a cluster without the provider module config keeps the legacy document": {
			metaConfig: func() *config.MetaConfig {
				mc := migratedCluster()
				mc.ModuleConfigs = nil
				return mc
			}(),
			wantLegacy: true,
		},
		"a static cluster is left alone": {
			metaConfig: func() *config.MetaConfig {
				mc := migratedCluster()
				mc.ClusterType = config.StaticClusterType
				return mc
			}(),
			wantLegacy: true,
		},
		"a cluster in the new format has nothing to drop": {
			metaConfig: func() *config.MetaConfig {
				mc := migratedCluster()
				mc.ProviderClusterConfig = nil
				return mc
			}(),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			kubeClient := client.NewFakeKubernetesClient()
			if tt.migrating {
				_, err := kubeClient.CoreV1().ConfigMaps("d8-cloud-provider-dvp").Create(
					t.Context(),
					&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: providerMigrationMarkerConfigMap, Namespace: "d8-cloud-provider-dvp"}},
					metav1.CreateOptions{},
				)
				require.NoError(t, err)
			}

			require.NoError(t, dropLeftoverLegacyProviderConfig(t.Context(), kubeClient, tt.metaConfig))

			assert.Equal(t, tt.wantLegacy, tt.metaConfig.HasLegacyProviderConfig())
		})
	}
}

func TestCheckCommanderModeParams(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		scanResult    *ScanResult
		wantResources string
	}{
		"a cluster in the new format gives the check the resources read by the scan": {
			scanResult: &ScanResult{
				ClusterConfiguration:                 "kind: ClusterConfiguration",
				ProviderSpecificClusterConfiguration: "",
				Resources:                            "kind: ModuleConfig\n---\nkind: NodeGroup",
			},
			wantResources: "kind: ModuleConfig\n---\nkind: NodeGroup",
		},
		"a cluster with the legacy provider configuration gives the check no resources": {
			scanResult: &ScanResult{
				ClusterConfiguration:                 "kind: ClusterConfiguration",
				ProviderSpecificClusterConfiguration: "kind: DVPClusterConfiguration",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			params := checkCommanderModeParams(tt.scanResult)

			assert.Equal(t, []byte(tt.scanResult.ClusterConfiguration), params.ClusterConfigurationData)
			assert.Equal(t, []byte(tt.scanResult.ProviderSpecificClusterConfiguration), params.ProviderClusterConfigurationData)
			assert.Equal(t, tt.wantResources, string(params.ResourcesData))
		})
	}
}
