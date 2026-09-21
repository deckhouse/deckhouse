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

package hooks

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestResolveNetwork(t *testing.T) {
	const (
		mcPod    = "10.1.0.0/16"
		mcSvc    = "10.2.0.0/16"
		mcPrefix = "25"

		ccPod    = "10.111.0.0/16"
		ccSvc    = "10.222.0.0/16"
		ccPrefix = "26"
	)

	tests := []struct {
		name string
		mc   networkSettings
		cc   networkSettings

		wantPod      string
		wantSvc      string
		wantPrefix   string
		wantFallback []string
	}{
		{
			name:       "ModuleConfig wins over ClusterConfiguration",
			mc:         networkSettings{mcPod, mcSvc, mcPrefix},
			cc:         networkSettings{ccPod, ccSvc, ccPrefix},
			wantPod:    mcPod,
			wantSvc:    mcSvc,
			wantPrefix: mcPrefix,
		},
		{
			name:         "ClusterConfiguration only",
			cc:           networkSettings{ccPod, ccSvc, ccPrefix},
			wantPod:      ccPod,
			wantSvc:      ccSvc,
			wantPrefix:   ccPrefix,
			wantFallback: []string{"podSubnetCIDR", "serviceSubnetCIDR", "podSubnetNodeCIDRPrefix"},
		},
		{
			name:       "ModuleConfig only",
			mc:         networkSettings{mcPod, mcSvc, mcPrefix},
			wantPod:    mcPod,
			wantSvc:    mcSvc,
			wantPrefix: mcPrefix,
		},
		{
			// Operators pass through this state one parameter at a time, so it has to work.
			name:         "half migrated: prefix still in ClusterConfiguration",
			mc:           networkSettings{PodSubnetCIDR: mcPod, ServiceSubnetCIDR: mcSvc},
			cc:           networkSettings{ccPod, ccSvc, ccPrefix},
			wantPod:      mcPod,
			wantSvc:      mcSvc,
			wantPrefix:   ccPrefix,
			wantFallback: []string{"podSubnetNodeCIDRPrefix"},
		},
		{
			name:         "half migrated: only the pod CIDR moved",
			mc:           networkSettings{PodSubnetCIDR: mcPod},
			cc:           networkSettings{ccPod, ccSvc, ccPrefix},
			wantPod:      mcPod,
			wantSvc:      ccSvc,
			wantPrefix:   ccPrefix,
			wantFallback: []string{"serviceSubnetCIDR", "podSubnetNodeCIDRPrefix"},
		},
		{
			// Post-release bootstrap with the values only in ModuleConfig: the deprecated field is
			// gone from ClusterConfiguration and the prefix falls back to the code constant.
			name:       "nothing anywhere: only the prefix has a default",
			wantPod:    "",
			wantSvc:    "",
			wantPrefix: defaultPodSubnetNodeCIDRPrefix,
		},
		{
			name:       "prefix absent from both documents",
			mc:         networkSettings{PodSubnetCIDR: mcPod, ServiceSubnetCIDR: mcSvc},
			cc:         networkSettings{PodSubnetCIDR: ccPod, ServiceSubnetCIDR: ccSvc},
			wantPod:    mcPod,
			wantSvc:    mcSvc,
			wantPrefix: defaultPodSubnetNodeCIDRPrefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveNetwork(tt.mc, tt.cc)

			require.Equal(t, tt.wantPod, got.PodSubnetCIDR)
			require.Equal(t, tt.wantSvc, got.ServiceSubnetCIDR)
			require.Equal(t, tt.wantPrefix, got.PodSubnetNodeCIDRPrefix)
			require.Equal(t, tt.wantFallback, got.FromClusterConfiguration)
		})
	}
}

func TestApplyControlPlaneManagerNetworkFilter(t *testing.T) {
	tests := []struct {
		name   string
		object map[string]interface{}
		want   networkSettings
	}{
		{
			name: "full network group",
			object: map[string]interface{}{
				"spec": map[string]interface{}{
					"settings": map[string]interface{}{
						"network": map[string]interface{}{
							"podSubnetCIDR":           "10.1.0.0/16",
							"serviceSubnetCIDR":       "10.2.0.0/16",
							"podSubnetNodeCIDRPrefix": "25",
						},
					},
				},
			},
			want: networkSettings{"10.1.0.0/16", "10.2.0.0/16", "25"},
		},
		{
			name: "absent network group is not an error",
			object: map[string]interface{}{
				"spec": map[string]interface{}{
					"settings": map[string]interface{}{"kubernetesVersion": "1.34"},
				},
			},
			want: networkSettings{},
		},
		{
			name:   "absent settings",
			object: map[string]interface{}{"spec": map[string]interface{}{"enabled": true}},
			want:   networkSettings{},
		},
		{
			name:   "absent spec",
			object: map[string]interface{}{},
			want:   networkSettings{},
		},
		{
			name: "partial group leaves the rest unset, keeping the fallback reachable",
			object: map[string]interface{}{
				"spec": map[string]interface{}{
					"settings": map[string]interface{}{
						"network": map[string]interface{}{"podSubnetCIDR": "10.1.0.0/16"},
					},
				},
			},
			want: networkSettings{PodSubnetCIDR: "10.1.0.0/16"},
		},
		{
			// The schema keeps these strings; a non-string is dropped rather than coerced, so a stray
			// number cannot be laundered into a value the admission webhook would have rejected.
			name: "non-string values are dropped",
			object: map[string]interface{}{
				"spec": map[string]interface{}{
					"settings": map[string]interface{}{
						"network": map[string]interface{}{
							"podSubnetCIDR":           "10.1.0.0/16",
							"podSubnetNodeCIDRPrefix": int64(24),
						},
					},
				},
			},
			want: networkSettings{PodSubnetCIDR: "10.1.0.0/16"},
		},
		{
			name: "network of the wrong type is not an error",
			object: map[string]interface{}{
				"spec": map[string]interface{}{
					"settings": map[string]interface{}{"network": "nonsense"},
				},
			},
			want: networkSettings{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyControlPlaneManagerNetworkFilter(&unstructured.Unstructured{Object: tt.object})

			// Never errors: an error discards the snapshot and takes down the only publisher of the
			// resolved values for the whole cluster.
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
