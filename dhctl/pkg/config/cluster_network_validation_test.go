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
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func networkMetaConfig(cluster map[string]string) *MetaConfig {
	raw := map[string]json.RawMessage{}
	for key, value := range cluster {
		encoded, _ := json.Marshal(value)
		raw[key] = encoded
	}
	return &MetaConfig{ClusterConfig: raw}
}

// TestValidateClusterNetworking covers the checks that moved out of preflight. They read nothing
// but the documents, and as preflight checks they did not run for `dhctl config`, did not run for
// converge, and were turned off by --preflight-skip-all-checks.
func TestValidateClusterNetworking(t *testing.T) {
	tests := []struct {
		name    string
		cluster map[string]string
		wantErr string
	}{
		{
			name:    "disjoint subnets",
			cluster: map[string]string{"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/16"},
		},
		{
			name:    "the pod subnet contains the service subnet",
			cluster: map[string]string{"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.111.128.0/17"},
			wantErr: "10.111.0.0/16 overlaps 10.111.128.0/17",
		},
		{
			name:    "the service subnet contains the pod subnet",
			cluster: map[string]string{"podSubnetCIDR": "10.111.128.0/17", "serviceSubnetCIDR": "10.111.0.0/16"},
			wantErr: "overlaps",
		},
		{
			name:    "an address without a prefix length",
			cluster: map[string]string{"podSubnetCIDR": "10.111.0.0", "serviceSubnetCIDR": "10.222.0.0/16"},
			wantErr: `expected: an IPv4 CIDR`,
		},
		{
			// getDNSAddress takes the eleventh address of the service subnet and returns "" when
			// the subnet has no eleventh address — silently, leaving the cluster with no DNS.
			name:    "a service subnet too small for the cluster DNS address",
			cluster: map[string]string{"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/29"},
			wantErr: "narrower than /28",
		},
		{
			name:    "a service subnet exactly wide enough",
			cluster: map[string]string{"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/28"},
		},
		{
			name: "a per-node prefix that is not larger than the pod subnet",
			cluster: map[string]string{
				"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/16",
				"podSubnetNodeCIDRPrefix": "16",
			},
			wantErr: `a value between 17 and 28`,
		},
		{
			name: "a per-node prefix that leaves a node no addresses",
			cluster: map[string]string{
				"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/16",
				"podSubnetNodeCIDRPrefix": "30",
			},
			wantErr: "expected: 28 or lower",
		},
		{
			name: "a per-node prefix that is not a number",
			cluster: map[string]string{
				"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/16",
				"podSubnetNodeCIDRPrefix": "twenty-four",
			},
			wantErr: "a prefix length written as a number",
		},
		{
			name: "the default per-node prefix",
			cluster: map[string]string{
				"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/16",
				"podSubnetNodeCIDRPrefix": "24",
			},
		},
		{
			name:    "no ClusterConfiguration at all",
			cluster: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateClusterNetworking(t.Context(), networkMetaConfig(tt.cluster))

			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestValidateClusterNetworkingAgainstInternalNetworks is the static-cidr-intersection check,
// which now runs wherever the configuration is loaded.
func TestValidateClusterNetworkingAgainstInternalNetworks(t *testing.T) {
	metaConfig := networkMetaConfig(map[string]string{
		"podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/16",
	})
	metaConfig.StaticClusterConfig = map[string]json.RawMessage{
		"internalNetworkCIDRs": json.RawMessage(`["192.168.1.0/24", "10.111.32.0/24"]`),
	}

	err := validateClusterNetworking(t.Context(), metaConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10.111.0.0/16 overlaps entry 10.111.32.0/24")
}

// TestValidatePublicDomainTemplate: the comparison that used to be strings.Contains, which
// rejected two documented configurations and let a third through on case.
func TestValidatePublicDomainTemplate(t *testing.T) {
	metaConfigWith := func(clusterDomain, template string) *MetaConfig {
		mc := &ModuleConfig{Spec: ModuleConfigSpec{Settings: SettingsValues{
			"modules": map[string]any{"publicDomainTemplate": template},
		}}}
		mc.ObjectMeta = metav1.ObjectMeta{Name: "global"}
		return &MetaConfig{ClusterDomain: clusterDomain, ModuleConfigs: []*ModuleConfig{mc}}
	}

	tests := []struct {
		name          string
		clusterDomain string
		template      string
		wantErr       bool
	}{
		{name: "outside the cluster zone", clusterDomain: "cluster.local", template: "%s.example.com"},
		{name: "the cluster zone itself", clusterDomain: "cluster.local", template: "cluster.local", wantErr: true},
		{name: "a subdomain of it", clusterDomain: "cluster.local", template: "%s.cluster.local", wantErr: true},
		{name: "case does not save it", clusterDomain: "cluster.local", template: "%s.Cluster.Local", wantErr: true},

		// The two the old strings.Contains rejected, both perfectly valid.
		{name: "a TLD that merely contains the string", clusterDomain: "cluster.local", template: "%s.mycluster.localnet.io"},
		{name: "the documented k8s.internal example", clusterDomain: "k8s.internal", template: "%s.k8s.internal.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePublicDomainTemplate(metaConfigWith(tt.clusterDomain, tt.template))

			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "is inside")
		})
	}
}

// #22688 moved podSubnetCIDR, serviceSubnetCIDR and podSubnetNodeCIDRPrefix to ModuleConfig
// control-plane-manager, leaving the ClusterConfiguration fields as a deprecated fallback. This
// validation read the document directly, so for every cluster that had migrated it found no
// values, returned early and validated nothing — silently, which is the part that matters: a pod
// subnet overlapping the service subnet cannot be changed after the cluster is created.
func TestValidateClusterNetworkingReadsTheModuleConfig(t *testing.T) {
	networkModuleConfig := func(network map[string]interface{}) *MetaConfig {
		return &MetaConfig{
			ClusterConfig: map[string]json.RawMessage{"clusterType": json.RawMessage(`"Static"`)},
			ModuleConfigs: []*ModuleConfig{{
				ObjectMeta: metav1.ObjectMeta{Name: "control-plane-manager"},
				Spec:       ModuleConfigSpec{Settings: SettingsValues{"network": network}},
			}},
		}
	}

	t.Run("overlapping subnets are still refused", func(t *testing.T) {
		err := validateClusterNetworking(context.Background(), networkModuleConfig(map[string]interface{}{
			"podSubnetCIDR":     "10.111.0.0/16",
			"serviceSubnetCIDR": "10.111.128.0/17",
		}))

		require.Error(t, err)
		require.Contains(t, err.Error(), "overlaps")
		require.Contains(t, err.Error(), "ModuleConfig control-plane-manager",
			"the message must name the document the value actually came from")
	})

	t.Run("a bad per-node prefix is still refused", func(t *testing.T) {
		err := validateClusterNetworking(context.Background(), networkModuleConfig(map[string]interface{}{
			"podSubnetCIDR":           "10.111.0.0/16",
			"serviceSubnetCIDR":       "10.222.0.0/16",
			"podSubnetNodeCIDRPrefix": "16",
		}))

		require.ErrorContains(t, err, "a value between 17 and 28")
	})

	t.Run("disjoint subnets pass", func(t *testing.T) {
		require.NoError(t, validateClusterNetworking(context.Background(), networkModuleConfig(map[string]interface{}{
			"podSubnetCIDR":     "10.111.0.0/16",
			"serviceSubnetCIDR": "10.222.0.0/16",
		})))
	})
}
