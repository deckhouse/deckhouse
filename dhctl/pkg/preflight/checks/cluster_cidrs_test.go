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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// metaConfigWithModuleConfigNetwork builds a configuration whose network parameters live where
// #22688 moved them. It came with that change, in cidr_intersection_test.go; that file went with
// the checks it tested — they read only the documents and are part of loading the configuration
// now — and network_single_source_test.go, which stayed, still needs this.
func metaConfigWithModuleConfigNetwork(t *testing.T, network map[string]interface{}) *config.MetaConfig {
	t.Helper()

	return &config.MetaConfig{
		ModuleConfigs: []*config.ModuleConfig{{
			ObjectMeta: metav1.ObjectMeta{Name: "control-plane-manager"},
			Spec:       config.ModuleConfigSpec{Settings: config.SettingsValues{"network": network}},
		}},
	}
}

// The checks that compare the cluster CIDRs against a node's networks or a cloud's node network
// read them through getCIDRs. It used to read ClusterConfiguration directly, which after #22688
// is empty for every cluster that declares them in ModuleConfig instead — and an empty CIDR
// overlaps nothing, so those checks would have passed by finding nothing to compare.
func TestGetCIDRsReadsWhereverTheyAreDeclared(t *testing.T) {
	t.Run("in the ModuleConfig", func(t *testing.T) {
		meta := metaConfigWithModuleConfigNetwork(t, map[string]interface{}{
			"podSubnetCIDR":     "10.111.0.0/16",
			"serviceSubnetCIDR": "10.222.0.0/16",
		})

		pod, service, err := getCIDRs(meta)

		require.NoError(t, err)
		assert.Equal(t, "10.111.0.0/16", pod)
		assert.Equal(t, "10.222.0.0/16", service)
	})

	t.Run("in the deprecated ClusterConfiguration fields", func(t *testing.T) {
		meta := &config.MetaConfig{ClusterConfig: map[string]json.RawMessage{
			"podSubnetCIDR":     json.RawMessage(`"10.111.0.0/16"`),
			"serviceSubnetCIDR": json.RawMessage(`"10.222.0.0/16"`),
		}}

		pod, service, err := getCIDRs(meta)

		require.NoError(t, err)
		assert.Equal(t, "10.111.0.0/16", pod)
		assert.Equal(t, "10.222.0.0/16", service)
	})

	t.Run("in neither", func(t *testing.T) {
		_, _, err := getCIDRs(&config.MetaConfig{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "control-plane-manager")
	})
}
