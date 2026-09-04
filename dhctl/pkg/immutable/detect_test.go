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

package immutable

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestIsImmutableMaster(t *testing.T) {
	nodeGroup := func(systemType string) map[string]any {
		spec := map[string]any{"nodeType": "CloudPermanent"}
		if systemType != "" {
			spec["systemType"] = systemType
		}
		return map[string]any{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "NodeGroup",
			"spec":       spec,
		}
	}

	tests := []struct {
		name       string
		nodeGroups map[string]map[string]any
		want       bool
	}{
		{name: "no resources at all"},
		{
			name:       "immutable master among other groups",
			nodeGroups: map[string]map[string]any{"master": nodeGroup("Immutable"), "worker": nodeGroup("")},
			want:       true,
		},
		{
			name:       "master without systemType",
			nodeGroups: map[string]map[string]any{"master": nodeGroup("")},
		},
		{
			name:       "master asking for a mutable system",
			nodeGroups: map[string]map[string]any{"master": nodeGroup("Mutable")},
		},
		{
			// Only the master decides how the first node is created.
			name:       "only an immutable worker",
			nodeGroups: map[string]map[string]any{"worker": nodeGroup("Immutable")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaConfig := &config.MetaConfig{}
			if tt.nodeGroups != nil {
				metaConfig.CloudProviderVars = &config.CloudProviderVars{NodeGroups: tt.nodeGroups}
			}

			require.Equal(t, tt.want, IsImmutableMaster(t.Context(), metaConfig))
		})
	}

	t.Run("a config nothing has parsed yet", func(t *testing.T) {
		require.False(t, IsImmutableMaster(t.Context(), &config.MetaConfig{}))
	})
}

// A static cluster runs no BaseInfra phase, so it never reports the master's
// address and the handoff has nothing to dial. Refused before anything is
// created: every phase before it passes, and every rerun dies the same way.
func TestValidateClusterType(t *testing.T) {
	require.NoError(t, ValidateClusterType(t.Context(), &config.MetaConfig{ClusterType: config.CloudClusterType}))

	err := ValidateClusterType(t.Context(), &config.MetaConfig{ClusterType: config.StaticClusterType})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Immutable")
	require.Contains(t, err.Error(), config.StaticClusterType)
}
