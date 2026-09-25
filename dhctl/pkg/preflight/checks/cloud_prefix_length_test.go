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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

func TestCloudDiskNameLength(t *testing.T) {
	tests := []struct {
		name          string
		metaConfig    *config.MetaConfig
		expectError   bool
		errContains   string
		notApplicable bool
	}{
		{
			name:        "nil MetaConfig returns error",
			metaConfig:  nil,
			expectError: true,
			errContains: "no configuration was loaded from --config",
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
			errContains: "the limit is 63",
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
			errContains: "the limit is 63",
		},
		{
			name: "VCD: prefix at max length passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("e", 44),
				ProviderName:  "vcd",
			},
			expectError: false,
		},
		{
			// VCD names the etcd disk, not the kubernetes-data one, and its suffix is the
			// longest of any provider bar DVP.
			name: "VCD: prefix exceeds max fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("e", 45),
				ProviderName:  "vcd",
			},
			expectError: true,
			errContains: "etcd-disk",
		},
		{
			name: "DVP: prefix at max length passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("f", 29),
				ProviderName:  "dvp",
			},
			expectError: false,
		},
		{
			// DVP was missing from the switch entirely, so a prefix too long for it passed
			// here and the cluster API rejected the disk during base infrastructure instead.
			// Its template carries a hash, which costs eight more characters than anyone
			// else's.
			name: "DVP: prefix exceeds max fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("f", 30),
				ProviderName:  "dvp",
			},
			expectError: true,
			errContains: "master-kubernetes-data",
		},
		{
			// The index is the last master's, not the first: with three masters the longest
			// name is the one that has not been generated yet.
			name: "the node index comes from the replica count",
			metaConfig: &config.MetaConfig{
				ClusterPrefix:       strings.Repeat("g", 45),
				ProviderName:        "vcd",
				MasterNodeGroupSpec: config.MasterNodeGroupSpec{Replicas: 3},
			},
			expectError: true,
			errContains: "master-2-etcd-disk",
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
				ClusterPrefix: "",
				ProviderName:  "aws",
			},
			expectError: false,
		},
		{
			name: "unknown provider passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("x", 60),
				ProviderName:  "unknown",
			},
			// A provider whose disk naming this installer does not know is a question it cannot
			// ask, not a question it asked and liked the answer to.
			notApplicable: true,
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
			errContains: "the limit is 63",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := CloudDiskNameLengthCheck{MetaConfig: tt.metaConfig}
			_, err := check.Run(t.Context())

			switch {
			case tt.expectError:
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			case tt.notApplicable:
				assert.ErrorIs(t, err, preflight.ErrNotApplicable)
			default:
				assert.NoError(t, err)
			}
		})
	}
}
