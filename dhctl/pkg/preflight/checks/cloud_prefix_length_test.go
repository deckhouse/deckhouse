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

func TestCloudPrefixLength(t *testing.T) {
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
			name: "DVP: prefix at max length (26 chars) passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 26),
				ProviderName:  "dvp",
			},
			expectError: false,
		},
		{
			name: "DVP: prefix exceeds max (27 chars) fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("a", 27),
				ProviderName:  "dvp",
			},
			expectError: true,
			errContains: "too long for provider",
		},
		{
			name: "AWS: prefix at max length (44 chars) passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("b", 44),
				ProviderName:  "aws",
			},
			expectError: false,
		},
		{
			name: "AWS: prefix exceeds max (45 chars) fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("b", 45),
				ProviderName:  "aws",
			},
			expectError: true,
			errContains: "too long for provider",
		},
		{
			name: "vSphere: prefix at max length (54 chars) passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("c", 54),
				ProviderName:  "vsphere",
			},
			expectError: false,
		},
		{
			name: "vSphere: prefix exceeds max (55 chars) fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("c", 55),
				ProviderName:  "vsphere",
			},
			expectError: true,
			errContains: "too long for provider",
		},
		{
			name: "Zvirt: prefix at max length (37 chars) passes",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("d", 37),
				ProviderName:  "zvirt",
			},
			expectError: false,
		},
		{
			name: "Zvirt: prefix exceeds max (38 chars) fails",
			metaConfig: &config.MetaConfig{
				ClusterPrefix: strings.Repeat("d", 38),
				ProviderName:  "zvirt",
			},
			expectError: true,
			errContains: "too long for provider",
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
