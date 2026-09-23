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

package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// TestValidateClusterTypeAgainstDocuments: nothing compared the two, and the mismatch was written
// into the cluster as a Secret the modules then believed.
func TestValidateClusterTypeAgainstDocuments(t *testing.T) {
	withClusterType := func(clusterType string) *MetaConfig {
		return &MetaConfig{
			ClusterType:   clusterType,
			ClusterConfig: map[string]json.RawMessage{"clusterType": json.RawMessage(`"` + clusterType + `"`)},
		}
	}

	t.Run("a static cluster with a provider document", func(t *testing.T) {
		meta := withClusterType(StaticClusterType)
		meta.ProviderClusterConfig = map[string]json.RawMessage{"layout": json.RawMessage(`"Standard"`)}

		err := validateClusterTypeAgainstDocuments(meta)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "a static cluster with no <Provider>ClusterConfiguration")
	})

	t.Run("a cloud cluster with a static document", func(t *testing.T) {
		meta := withClusterType(CloudClusterType)
		meta.StaticClusterConfig = map[string]json.RawMessage{
			"internalNetworkCIDRs": json.RawMessage(`["10.0.0.0/8"]`),
		}

		err := validateClusterTypeAgainstDocuments(meta)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "a cloud cluster with no StaticClusterConfiguration")
	})

	t.Run("each with its own document", func(t *testing.T) {
		cloud := withClusterType(CloudClusterType)
		cloud.ProviderClusterConfig = map[string]json.RawMessage{"layout": json.RawMessage(`"Standard"`)}
		assert.NoError(t, validateClusterTypeAgainstDocuments(cloud))

		static := withClusterType(StaticClusterType)
		static.StaticClusterConfig = map[string]json.RawMessage{
			"internalNetworkCIDRs": json.RawMessage(`["10.0.0.0/8"]`),
		}
		assert.NoError(t, validateClusterTypeAgainstDocuments(static))
	})

	t.Run("no ClusterConfiguration at all", func(t *testing.T) {
		// A cluster whose control plane dhctl did not create; there is no clusterType to compare.
		meta := &MetaConfig{ProviderClusterConfig: map[string]json.RawMessage{"layout": json.RawMessage(`"Standard"`)}}
		assert.NoError(t, validateClusterTypeAgainstDocuments(meta))
	})
}

// TestWarnAboutMasterReplicaParity: two masters tolerate no failure at all, which is what one
// does, with twice the machines to lose.
func TestWarnAboutMasterReplicaParity(t *testing.T) {
	tests := []struct {
		replicas int
		warns    bool
	}{
		{replicas: 0},
		{replicas: 1},
		{replicas: 2, warns: true},
		{replicas: 3},
		{replicas: 4, warns: true},
		{replicas: 5},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d masters", tt.replicas), func(t *testing.T) {
			var logged bytes.Buffer
			ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))

			warnAboutMasterReplicaParity(ctx, &MetaConfig{
				MasterNodeGroupSpec: MasterNodeGroupSpec{Replicas: tt.replicas},
			})

			warned := strings.Contains(logged.String(), "WARN")
			assert.Equal(t, tt.warns, warned, "log was:\n%s", logged.String())
			if tt.warns {
				assert.Contains(t, logged.String(), "an odd number of masters")
			}
		})
	}
}
