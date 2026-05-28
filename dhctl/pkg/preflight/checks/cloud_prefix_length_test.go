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

package checks

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func dvpProviderConfig(masterAdditionalDisks bool, nodeGroups ...dvpNodeGroup) map[string]json.RawMessage {
	master := dvpMasterNodeGroup{}
	if masterAdditionalDisks {
		master.InstanceClass.AdditionalDisks = []json.RawMessage{json.RawMessage(`{"size":"10Gi","storageClass":"sc"}`)}
	}

	masterJSON, _ := json.Marshal(master)
	result := map[string]json.RawMessage{
		"masterNodeGroup": masterJSON,
	}

	if len(nodeGroups) > 0 {
		ngJSON, _ := json.Marshal(nodeGroups)
		result["nodeGroups"] = ngJSON
	}

	return result
}

func TestCloudDiskNameLength(t *testing.T) {
	tests := []struct {
		name        string
		metaConfig  *config.MetaConfig
		expectError bool
		errContains string
	}{
		{
			name:        "nil MetaConfig returns error",
			metaConfig:  nil,
			expectError: true,
			errContains: "meta config is nil",
		},
		{
			name: "DVP master without additionalDisks: short prefix passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix:         strings.Repeat("a", 31),
				ProviderName:          "dvp",
				ProviderClusterConfig: dvpProviderConfig(false),
			},
			expectError: false,
		},
		{
			name: "DVP master without additionalDisks: long prefix fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix:         strings.Repeat("a", 32),
				ProviderName:          "dvp",
				ProviderClusterConfig: dvpProviderConfig(false),
			},
			expectError: true,
			errContains: "exceeds 63 characters",
		},
		{
			name: "DVP master with additionalDisks: short prefix passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix:         strings.Repeat("a", 29),
				ProviderName:          "dvp",
				ProviderClusterConfig: dvpProviderConfig(true),
			},
			expectError: false,
		},
		{
			name: "DVP master with additionalDisks: long prefix fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix:         strings.Repeat("a", 30),
				ProviderName:          "dvp",
				ProviderClusterConfig: dvpProviderConfig(true),
			},
			expectError: true,
			errContains: "exceeds 63 characters",
		},
		{
			name: "DVP nodeGroup without additionalDisks: passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 38),
				ProviderName:  "dvp",
				ProviderClusterConfig: dvpProviderConfig(false, dvpNodeGroup{
					Name: "cloud-permanent",
				}),
			},
			expectError: false,
		},
		{
			name: "DVP nodeGroup without additionalDisks: fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 39),
				ProviderName:  "dvp",
				ProviderClusterConfig: dvpProviderConfig(false, dvpNodeGroup{
					Name: "cloud-permanent",
				}),
			},
			expectError: true,
			errContains: "cloud-permanent",
		},
		{
			name: "DVP nodeGroup with additionalDisks: passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 18),
				ProviderName:  "dvp",
				ProviderClusterConfig: dvpProviderConfig(false, dvpNodeGroup{
					Name: "cloud-permanent",
					InstanceClass: dvpInstanceClass{
						AdditionalDisks: []json.RawMessage{json.RawMessage(`{"size":"10Gi","storageClass":"sc"}`)},
					},
				}),
			},
			expectError: false,
		},
		{
			name: "DVP nodeGroup with additionalDisks: fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 19),
				ProviderName:  "dvp",
				ProviderClusterConfig: dvpProviderConfig(false, dvpNodeGroup{
					Name: "cloud-permanent",
					InstanceClass: dvpInstanceClass{
						AdditionalDisks: []json.RawMessage{json.RawMessage(`{"size":"10Gi","storageClass":"sc"}`)},
					},
				}),
			},
			expectError: true,
			errContains: "cloud-permanent",
		},
		{
			name: "AWS: prefix at max length passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("b", 45),
				ProviderName:  "aws",
			},
			expectError: false,
		},
		{
			name: "AWS: prefix exceeds max fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("b", 46),
				ProviderName:  "aws",
			},
			expectError: true,
			errContains: "exceeds 63 characters",
		},
		{
			name: "Zvirt: prefix at max length passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("c", 38),
				ProviderName:  "zvirt",
			},
			expectError: false,
		},
		{
			name: "Zvirt: prefix exceeds max fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("c", 39),
				ProviderName:  "zvirt",
			},
			expectError: true,
			errContains: "exceeds 63 characters",
		},
		{
			name: "OpenStack: both disks checked, longer one fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("d", 43),
				ProviderName:  "openstack",
			},
			expectError: true,
			errContains: "master-root-volume",
		},
		{
			name: "empty prefix passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix:         "",
				ProviderName:          "dvp",
				ProviderClusterConfig: dvpProviderConfig(false),
			},
			expectError: false,
		},
		{
			name: "unknown provider passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("x", 60),
				ProviderName:  "unknown",
			},
			expectError: false,
		},
		{
			name: "AWS with 11 replicas: prefix at max length passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix:       strings.Repeat("b", 44),
				ProviderName:        "aws",
				MasterNodeGroupSpec: config.MasterNodeGroupSpec{Replicas: 11},
			},
			expectError: false,
		},
		{
			name: "AWS with 11 replicas: prefix exceeds max fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix:       strings.Repeat("b", 45),
				ProviderName:        "aws",
				MasterNodeGroupSpec: config.MasterNodeGroupSpec{Replicas: 11},
			},
			expectError: true,
			errContains: "exceeds 63 characters",
		},
		{
			name: "DVP master with 11 replicas: short prefix passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix:         strings.Repeat("a", 30),
				ProviderName:          "dvp",
				MasterNodeGroupSpec:   config.MasterNodeGroupSpec{Replicas: 11},
				ProviderClusterConfig: dvpProviderConfig(false),
			},
			expectError: false,
		},
		{
			name: "DVP master with 11 replicas: long prefix fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix:         strings.Repeat("a", 31),
				ProviderName:          "dvp",
				MasterNodeGroupSpec:   config.MasterNodeGroupSpec{Replicas: 11},
				ProviderClusterConfig: dvpProviderConfig(false),
			},
			expectError: true,
			errContains: "exceeds 63 characters",
		},
		{
			name: "DVP nodeGroup with 11 replicas: passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 37),
				ProviderName:  "dvp",
				ProviderClusterConfig: dvpProviderConfig(false, dvpNodeGroup{
					Name:     "cloud-permanent",
					Replicas: 11,
				}),
			},
			expectError: false,
		},
		{
			name: "DVP nodeGroup with 11 replicas: fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 38),
				ProviderName:  "dvp",
				ProviderClusterConfig: dvpProviderConfig(false, dvpNodeGroup{
					Name:     "cloud-permanent",
					Replicas: 11,
				}),
			},
			expectError: true,
			errContains: "cloud-permanent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := CloudDiskNameLengthCheck{MetaConfig: tt.metaConfig}
			err := check.Run(context.Background())

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
