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

package controller

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

const providerResourcesNodeGroups = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
  cloudInstances:
    minPerZone: 1
    classReference:
      kind: DVPInstanceClass
      name: master-class
`

func cloudMetaConfig() *config.MetaConfig {
	return &config.MetaConfig{
		ClusterConfig: map[string]json.RawMessage{
			"clusterType":       json.RawMessage(`"Cloud"`),
			"serviceSubnetCIDR": json.RawMessage(`"10.222.0.0/16"`),
			"clusterDomain":     json.RawMessage(`"cluster.local"`),
			"cloud":             json.RawMessage(`{"provider":"DVP","prefix":"test"}`),
		},
	}
}

// A node of a group is planned with the settings the group had when the node was created.
func TestGetNgMetaConfigPinsStoredSettingsOfLegacyConfig(t *testing.T) {
	clusterMetaConfig := cloudMetaConfig()
	clusterMetaConfig.ProviderClusterConfig = map[string]json.RawMessage{
		"layout":     json.RawMessage(`"Standard"`),
		"nodeGroups": json.RawMessage(`[{"name":"current","replicas":3}]`),
	}

	cfg, err := getNgMetaConfig(t.Context(), clusterMetaConfig, []byte(`{"name":"stored","replicas":1}`))
	require.NoError(t, err)

	require.JSONEq(t, `[{"name":"stored","replicas":1}]`, string(cfg.ProviderClusterConfig["nodeGroups"]))
	require.JSONEq(t, `[{"name":"current","replicas":3}]`, string(clusterMetaConfig.ProviderClusterConfig["nodeGroups"]),
		"the cluster configuration the copy was taken from stays as it was")
}

// A cluster that configures its provider through resources has no provider cluster configuration:
// settings left in the node state by an earlier legacy run must not panic the planning of a node.
func TestGetNgMetaConfigWithoutLegacyProviderConfig(t *testing.T) {
	clusterMetaConfig := cloudMetaConfig()
	clusterMetaConfig.ResourcesYAML = providerResourcesNodeGroups

	var cfg *config.MetaConfig
	require.NotPanics(t, func() {
		var err error
		cfg, err = getNgMetaConfig(t.Context(), clusterMetaConfig, []byte(`{"name":"stored","replicas":1}`))
		require.NoError(t, err)
	})

	require.Empty(t, cfg.ProviderClusterConfig, "the stored settings have nowhere to go: the groups come from the resources")
	require.Equal(t, 1, cfg.MasterNodeGroupSpec.Replicas)
}
