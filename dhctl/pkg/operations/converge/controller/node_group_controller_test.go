// Copyright 2024 Flant JSC
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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func Test_sortNodeNames(t *testing.T) {
	tests := []struct {
		unsorted []string
		sorted   []string
		error    bool
	}{
		{
			unsorted: []string{"node-3", "node-2", "node-1"},
			sorted:   []string{"node-3", "node-2", "node-1"},
		},
		{
			unsorted: []string{"node-2", "node-1", "node-3"},
			sorted:   []string{"node-3", "node-2", "node-1"},
		},
		{
			unsorted: []string{"node-1", "node-2", "node-3"},
			sorted:   []string{"node-3", "node-2", "node-1"},
		},
		{
			unsorted: []string{"node-1", "node-3", "node-2"},
			sorted:   []string{"node-3", "node-2", "node-1"},
		},
		{
			unsorted: []string{"node-a-2", "node-b-3", "node-c-1"},
			sorted:   []string{"node-b-3", "node-a-2", "node-c-1"},
		},
		{
			unsorted: []string{"node-a", "node-b", "node-c"},
			error:    true,
		},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.unsorted, " "), func(t *testing.T) {
			state := make(map[string][]byte)

			for _, node := range test.unsorted {
				state[node] = nil
			}

			sorted, err := sortNodeNames(state)

			if test.error {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			names := make([]string, 0, len(sorted))
			for _, node := range sorted {
				names = append(names, node.name)
			}

			require.Equal(t, test.sorted, names)
		})
	}
}

func Test_getNodesToDeleteInfo(t *testing.T) {
	tests := []struct {
		name     string
		replicas int
		nodes    []string
		deleted  []string
		error    bool
	}{
		{
			name:     "empty",
			replicas: 0,
			nodes:    []string{},
			deleted:  []string{},
		},
		{
			name:     "zero replicas and no deleted nodes",
			replicas: 0,
			nodes:    []string{"node-3", "node-2", "node-1"},
			deleted:  []string{"node-3", "node-2", "node-1"},
		},
		{
			name:     "1 replica and 2 deleted nodes",
			replicas: 1,
			nodes:    []string{"node-2", "node-1", "node-3"},
			deleted:  []string{"node-3", "node-2"},
		},
		{
			name:     "2 replicas and 1 deleted node",
			replicas: 2,
			nodes:    []string{"node-1", "node-2", "node-3"},
			deleted:  []string{"node-3"},
		},
		{
			name:     "3 replicas and no deleted nodes",
			replicas: 3,
			nodes:    []string{"node-1", "node-3", "node-2"},
			deleted:  []string{},
		},
		{
			name:  "validation error",
			nodes: []string{"node-a", "node-b", "node-c"},
			error: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := make(map[string][]byte)

			for _, node := range test.nodes {
				state[node] = []byte(node)
			}

			nodeToDeleteInfo, err := getNodesToDeleteInfo(t.Context(), test.replicas, state)

			if test.error {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			deleted := make([]string, 0, len(nodeToDeleteInfo))

			for _, node := range nodeToDeleteInfo {
				deleted = append(deleted, node.name)
				require.Equal(t, []byte(node.name), node.state, "the node to delete carries its own state")
			}

			require.Equal(t, test.deleted, deleted)

			// The nodes to delete leave the passed states: the masters are updated and the
			// SSH inventory is refreshed from what remains, and a node about to be destroyed
			// must not come back into it.
			remaining := make([]string, 0, len(state))
			for node := range state {
				remaining = append(remaining, node)
			}

			for _, node := range deleted {
				require.NotContains(t, remaining, node)
			}
			require.Len(t, remaining, len(test.nodes)-len(deleted))
		})
	}
}

// The nodes of a group are deleted with the settings the group had when they were created, but only
// for the legacy provider configuration: a cluster that configures its provider through resources has
// no document to put the settings into.
func Test_nodeGroupMetaConfig(t *testing.T) {
	storedSettings := []byte(`{"name":"stored","replicas":1}`)

	tests := []struct {
		name     string
		shared   *config.MetaConfig
		settings []byte
		// sameConfig reports that the configuration is used as it is, without a copy.
		sameConfig bool
		// nodeGroups is the "nodeGroups" document of the returned configuration; empty means there is none.
		nodeGroups string
	}{
		{
			name:       "legacy configuration gets the stored settings in a copy",
			shared:     legacyCloudMetaConfig(),
			settings:   storedSettings,
			nodeGroups: `[{"name":"stored","replicas":1}]`,
		},
		{
			name:       "legacy configuration without stored settings is used as it is",
			shared:     legacyCloudMetaConfig(),
			sameConfig: true,
			nodeGroups: currentNodeGroups,
		},
		{
			name:       "provider configured through resources is used as it is",
			shared:     resourcesCloudMetaConfig(),
			settings:   storedSettings,
			sameConfig: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sharedBefore := string(test.shared.ProviderClusterConfig["nodeGroups"])

			var cfg *config.MetaConfig
			require.NotPanics(t, func() {
				var err error
				cfg, err = nodeGroupMetaConfig(t.Context(), test.shared, test.settings)
				require.NoError(t, err)
			})

			if test.sameConfig {
				require.Same(t, test.shared, cfg)
			} else {
				require.NotSame(t, test.shared, cfg)
			}

			if test.nodeGroups == "" {
				require.Empty(t, cfg.ProviderClusterConfig["nodeGroups"])
			} else {
				require.JSONEq(t, test.nodeGroups, string(cfg.ProviderClusterConfig["nodeGroups"]))
			}

			require.Equal(t, sharedBefore, string(test.shared.ProviderClusterConfig["nodeGroups"]),
				"the configuration shared by the converge run stays as it was")
		})
	}
}

const currentNodeGroups = `[{"name":"current","replicas":3}]`

const masterNodeGroupResource = `
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

func legacyCloudMetaConfig() *config.MetaConfig {
	metaConfig := cloudMetaConfig()
	metaConfig.ProviderClusterConfig = map[string]json.RawMessage{
		"layout":     json.RawMessage(`"Standard"`),
		"nodeGroups": json.RawMessage(currentNodeGroups),
	}

	return metaConfig
}

func resourcesCloudMetaConfig() *config.MetaConfig {
	metaConfig := cloudMetaConfig()
	metaConfig.ResourcesYAML = masterNodeGroupResource

	return metaConfig
}
