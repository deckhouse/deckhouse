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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

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
			// DVP: {prefix}-master-additional-disk-0-0-abcdef
			// max prefix = 63 - len("-master-additional-disk-0-0-abcdef") = 63 - 34 = 29
			name: "DVP: short prefix with master node group passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 29),
				ProviderName:  "dvp",
			},
			expectError: false,
		},
		{
			name: "DVP: long prefix with master node group fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 30),
				ProviderName:  "dvp",
			},
			expectError: true,
			errContains: "exceeds 63 characters",
		},
		{
			// DVP: {prefix}-{long-node-group}-additional-disk-0-0-abcdef
			// "my-long-worker-group" = 20 chars, overhead = 1+20+28 = 49, max prefix = 14
			name: "DVP: long node group name fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 15),
				ProviderName:  "dvp",
				TerraNodeGroupSpecs: []config.TerraNodeGroupSpec{
					{Name: "my-long-worker-group"},
				},
			},
			expectError: true,
			errContains: "my-long-worker-group",
		},
		{
			name: "DVP: long node group name with short prefix passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 14),
				ProviderName:  "dvp",
				TerraNodeGroupSpecs: []config.TerraNodeGroupSpec{
					{Name: "my-long-worker-group"},
				},
			},
			expectError: false,
		},
		{
			// AWS: {prefix}-kubernetes-data-0
			// max prefix = 63 - len("-kubernetes-data-0") = 63 - 18 = 45
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
			// Zvirt: {prefix}-master-0-kubernetes-data
			// max prefix = 63 - len("-master-0-kubernetes-data") = 63 - 25 = 38
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
			name: "DVP: NodeGroup from ResourcesYAML with long name fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 20),
				ProviderName:  "dvp",
				ResourcesYAML: `apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: my-very-long-system-group
spec:
  nodeType: CloudEphemeral`,
			},
			expectError: true,
			errContains: "my-very-long-system-group",
		},
		{
			name: "DVP: NodeGroup from ResourcesYAML with short name passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 20),
				ProviderName:  "dvp",
				ResourcesYAML: `apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  nodeType: CloudEphemeral`,
			},
			expectError: false,
		},
		{
			name: "empty prefix passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: "",
				ProviderName:  "dvp",
			},
			expectError: false,
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
