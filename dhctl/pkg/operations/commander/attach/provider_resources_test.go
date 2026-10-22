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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestProviderResourcesYAML(t *testing.T) {
	newFormat := func() *config.MetaConfig {
		return &config.MetaConfig{
			ClusterType:  config.CloudClusterType,
			ProviderName: "dvp",
			CloudProviderVars: &config.CloudProviderVars{
				Settings: map[string]any{
					"apiVersion": "deckhouse.io/v1alpha1",
					"kind":       "ModuleConfig",
					"metadata": map[string]any{
						"name":            "cloud-provider-dvp",
						"resourceVersion": "42",
						"uid":             "u-1",
						"managedFields":   []any{map[string]any{"manager": "kubectl"}},
						"annotations": map[string]any{
							"kubectl.kubernetes.io/last-applied-configuration": "{}",
							"keep": "me",
						},
					},
					"spec":   map[string]any{"enabled": true, "version": int64(1)},
					"status": map[string]any{"status": "Ready"},
				},
				Secrets: map[string]map[string]any{
					"d8-cloud-provider-dvp/zeta":  {"kind": "Secret", "metadata": map[string]any{"name": "zeta"}},
					"d8-cloud-provider-dvp/alpha": {"kind": "Secret", "metadata": map[string]any{"name": "alpha"}},
				},
				InstanceClasses: map[string]map[string]any{
					"master": {"kind": "DVPInstanceClass", "metadata": map[string]any{"name": "master", "uid": "u-2"}},
				},
				NodeGroups: map[string]map[string]any{
					"master": {
						"kind":     "NodeGroup",
						"metadata": map[string]any{"name": "master", "creationTimestamp": "2026-01-01T00:00:00Z"},
						"status":   map[string]any{"nodes": int64(1)},
					},
				},
			},
		}
	}

	tests := map[string]struct {
		metaConfig *config.MetaConfig
		check      func(t *testing.T, documents []map[string]any)
		wantEmpty  bool
	}{
		"new format renders every kind in a stable order": {
			metaConfig: newFormat(),
			check: func(t *testing.T, documents []map[string]any) {
				kinds := make([]string, 0, len(documents))
				names := make([]string, 0, len(documents))
				for _, document := range documents {
					kinds = append(kinds, document["kind"].(string))
					names = append(names, document["metadata"].(map[string]any)["name"].(string))
				}

				assert.Equal(t, []string{"ModuleConfig", "Secret", "Secret", "DVPInstanceClass", "NodeGroup"}, kinds)
				assert.Equal(t, []string{"cloud-provider-dvp", "alpha", "zeta", "master", "master"}, names)
			},
		},
		"the status and the metadata set by the cluster are dropped": {
			metaConfig: newFormat(),
			check: func(t *testing.T, documents []map[string]any) {
				for _, document := range documents {
					assert.NotContains(t, document, "status")

					metadata := document["metadata"].(map[string]any)
					for _, field := range []string{"resourceVersion", "uid", "managedFields", "creationTimestamp"} {
						assert.NotContains(t, metadata, field)
					}
				}
			},
		},
		"annotations of the tool that applied the object are dropped, the others stay": {
			metaConfig: newFormat(),
			check: func(t *testing.T, documents []map[string]any) {
				annotations := documents[0]["metadata"].(map[string]any)["annotations"].(map[string]any)

				assert.Equal(t, map[string]any{"keep": "me"}, annotations)
			},
		},
		"a legacy provider configuration has no resources": {
			metaConfig: func() *config.MetaConfig {
				mc := newFormat()
				mc.ProviderClusterConfig = map[string]json.RawMessage{"layout": json.RawMessage(`"Standard"`)}
				return mc
			}(),
			wantEmpty: true,
		},
		"a static cluster has no resources": {
			metaConfig: func() *config.MetaConfig {
				mc := newFormat()
				mc.ClusterType = config.StaticClusterType
				return mc
			}(),
			wantEmpty: true,
		},
		"unread provider variables mean no resources": {
			metaConfig: func() *config.MetaConfig {
				mc := newFormat()
				mc.CloudProviderVars = nil
				return mc
			}(),
			wantEmpty: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := providerResourcesYAML(tt.metaConfig)
			require.NoError(t, err)

			if tt.wantEmpty {
				assert.Empty(t, got)
				return
			}

			var documents []map[string]any
			for _, part := range strings.Split(got, "---\n") {
				var document map[string]any
				require.NoError(t, yaml.Unmarshal([]byte(part), &document))
				documents = append(documents, document)
			}

			tt.check(t, documents)
		})
	}
}

func TestCleanResourceDoesNotChangeTheSource(t *testing.T) {
	source := map[string]any{
		"kind":     "NodeGroup",
		"metadata": map[string]any{"name": "master", "uid": "u-1"},
		"status":   map[string]any{"nodes": int64(1)},
	}

	cleaned, err := cleanResource(source)
	require.NoError(t, err)

	assert.NotContains(t, cleaned, "status")
	assert.Contains(t, source, "status")
	assert.Contains(t, source["metadata"], "uid")
}

// The rendered set is what Commander sends back to dhctl as the resources of the cluster, so dhctl
// has to read it as it was read from the cluster.
func TestProviderResourcesYAMLIsReadBackByTheResourcesParser(t *testing.T) {
	metaConfig := &config.MetaConfig{
		ClusterType:  config.CloudClusterType,
		ProviderName: "dvp",
		CloudProviderVars: &config.CloudProviderVars{
			Secrets: map[string]map[string]any{
				"d8-cloud-provider-dvp/credentials": {
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata":   map[string]any{"name": "credentials", "namespace": "d8-cloud-provider-dvp"},
					"type":       string(config.CloudProviderCredentialsSecretType),
					"stringData": map[string]any{"kubeconfig": "apiVersion: v1"},
				},
			},
			InstanceClasses: map[string]map[string]any{
				"master": {
					"apiVersion": "deckhouse.io/v1alpha1",
					"kind":       "DVPInstanceClass",
					"metadata":   map[string]any{"name": "master"},
					"spec":       map[string]any{"virtualMachine": map[string]any{"cpu": map[string]any{"cores": int64(4)}}},
				},
			},
			NodeGroups: map[string]map[string]any{
				"master": {
					"apiVersion": "deckhouse.io/v1",
					"kind":       "NodeGroup",
					"metadata":   map[string]any{"name": "master"},
					"spec":       map[string]any{"nodeType": "CloudPermanent", "cloudInstances": map[string]any{"minPerZone": int64(1)}},
				},
			},
		},
	}

	rendered, err := providerResourcesYAML(metaConfig)
	require.NoError(t, err)

	parsed, err := config.ParseResourcesYAML(rendered)
	require.NoError(t, err)

	assert.Contains(t, parsed.NodeGroups, "master")
	assert.Contains(t, parsed.InstanceClasses, "master")
	require.Len(t, parsed.Secrets, 1)
	assert.Contains(t, rendered, "cores: 4", "an integer stays an integer")
}
