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

package check

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// TestProviderConfigurationChecks pins the new-format branch: an empty desired
// provider configuration next to a provider ModuleConfig, in the cluster or in the
// desired resources, skips the provider comparison (the set is applied through
// resource groups) and never fails with "'provider cluster config' ... cannot be
// empty". A legacy desired configuration keeps the legacy comparison and ignores a
// ModuleConfig left in the cluster by a rollback; an empty desired configuration
// with the ModuleConfig nowhere still fails loudly.
func TestProviderConfigurationChecks(t *testing.T) {
	legacyMetaConfig := func(providerYAML string) *config.MetaConfig {
		mc := &config.MetaConfig{
			ClusterType:  config.CloudClusterType,
			ProviderName: "yandex",
		}
		if providerYAML != "" {
			mc.ProviderClusterConfig = mustProviderConfigMap(t, providerYAML)
		}
		return mc
	}

	tests := map[string]struct {
		desired         *config.MetaConfig
		inCluster       *config.MetaConfig
		expectSkipped   bool
		expectBuildErr  string
		expectDriftPart string
	}{
		"new format skips the comparison": {
			desired:       legacyMetaConfig(""),
			inCluster:     withModuleConfig(legacyMetaConfig("")),
			expectSkipped: true,
		},
		"migrated cluster with the leftover legacy document skips the comparison": {
			desired:       legacyMetaConfig(""),
			inCluster:     withModuleConfig(legacyMetaConfig("layout: WithoutNAT\n")),
			expectSkipped: true,
		},
		"new format in the desired resources skips the comparison before the cluster has the module config": {
			desired:       withModuleConfig(legacyMetaConfig("")),
			inCluster:     legacyMetaConfig("layout: WithoutNAT\n"),
			expectSkipped: true,
		},
		"empty desired without the in-cluster module config is never a false in sync": {
			desired:        legacyMetaConfig(""),
			inCluster:      legacyMetaConfig("layout: WithoutNAT\n"),
			expectBuildErr: "cannot be empty",
		},
		"legacy desired ignores an in-cluster module config left by a rollback": {
			desired:   legacyMetaConfig("layout: WithoutNAT\n"),
			inCluster: withModuleConfig(legacyMetaConfig("layout: WithoutNAT\n")),
		},
		"legacy desired with a differing legacy document drifts": {
			desired:         legacyMetaConfig("layout: WithoutNAT\n"),
			inCluster:       legacyMetaConfig("layout: Standard\n"),
			expectDriftPart: "provider configuration",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			checks, err := providerConfigurationChecks(t.Context(), tt.desired, tt.inCluster)

			if tt.expectBuildErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectBuildErr)
				return
			}

			require.NoError(t, err)

			if tt.expectSkipped {
				assert.Empty(t, checks)
				return
			}

			require.NotEmpty(t, checks)

			for _, check := range checks {
				checkErr := check()

				if tt.expectDriftPart == "" {
					require.NoError(t, checkErr)
					continue
				}

				require.Error(t, checkErr)
				assert.Contains(t, checkErr.Error(), tt.expectDriftPart)
			}
		})
	}
}

func withModuleConfig(metaConfig *config.MetaConfig) *config.MetaConfig {
	moduleConfig := &config.ModuleConfig{}
	moduleConfig.SetName("cloud-provider-yandex")
	moduleConfig.Spec.Enabled = ptr.To(true)
	metaConfig.ModuleConfigs = []*config.ModuleConfig{moduleConfig}
	return metaConfig
}

func mustProviderConfigMap(t *testing.T, providerYAML string) map[string]json.RawMessage {
	t.Helper()

	var result map[string]json.RawMessage
	require.NoError(t, yaml.Unmarshal([]byte(providerYAML), &result))
	return result
}
