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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := CloudDiskNameLengthCheck{MetaConfig: tt.metaConfig}
			err := check.Run(t.Context())

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
