//go:build !integration

/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
