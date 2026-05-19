//go:build !integration

/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package waypointcontroller

import (
	"testing"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

func TestEffectiveMinReplicas(t *testing.T) {
	cases := []struct {
		name string
		spec func() *networkv1alpha1.WaypointInstanceSpec
		want int32
	}{
		{
			name: "nil_ReplicasManagement",
			spec: func() *networkv1alpha1.WaypointInstanceSpec {
				return &newInstance("main", "ns").Spec
			},
			want: 1,
		},
		{
			name: "static_replicas_3",
			spec: func() *networkv1alpha1.WaypointInstanceSpec {
				return &newInstance("main", "ns", withStaticReplicas(3)).Spec
			},
			want: 3,
		},
		{
			name: "hpa_minReplicas_5",
			spec: func() *networkv1alpha1.WaypointInstanceSpec {
				return &newInstance("main", "ns", withHPAMode(5, 10, 70)).Spec
			},
			want: 5,
		},
		{
			name: "empty_mode_defaults_to_1",
			spec: func() *networkv1alpha1.WaypointInstanceSpec {
				return &newInstance("main", "ns", withReplicasManagementMode("")).Spec
			},
			want: 1,
		},
		{
			name: "unknown_mode_defaults_to_1",
			spec: func() *networkv1alpha1.WaypointInstanceSpec {
				return &newInstance("main", "ns", withReplicasManagementMode("garbage")).Spec
			},
			want: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := effectiveMinReplicas(tc.spec())
			if got != tc.want {
				t.Errorf("effectiveMinReplicas() = %d, want %d", got, tc.want)
			}
		})
	}
}
