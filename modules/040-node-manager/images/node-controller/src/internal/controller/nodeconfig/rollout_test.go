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

package nodeconfig

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

func nodeConfigAt(generation, observed, applied int64, phase string) *internalv1alpha1.NodeConfig {
	nc := &internalv1alpha1.NodeConfig{}
	nc.Generation = generation
	nc.Status.ObservedGeneration = observed
	nc.Status.AppliedGeneration = applied
	nc.Status.Phase = phase
	return nc
}

// applied() is the rollout's "this node converged" test. It must key on the
// generation the node is RUNNING (appliedGeneration), not the one it has merely
// SEEN (observedGeneration): a held node has observed the current generation but
// is still running the previous one, and counting it as done would walk the
// change through the whole group while every node waits.
func TestApplied(t *testing.T) {
	tests := []struct {
		name       string
		nc         *internalv1alpha1.NodeConfig
		disruption bool
		want       bool
	}{
		{
			name: "running the current generation, Ready",
			nc:   nodeConfigAt(4, 4, 4, phaseReady),
			want: true,
		},
		{
			name: "has seen gen 4 but is still running gen 3",
			nc:   nodeConfigAt(4, 4, 3, phaseReady),
			want: false,
		},
		{
			name: "observedGeneration says 4 but appliedGeneration is 3 — not done",
			// The exact overstating agent the two-number split guards against.
			nc:   nodeConfigAt(4, 4, 3, phaseReady),
			want: false,
		},
		{
			name: "applied the generation but not Ready yet",
			nc:   nodeConfigAt(4, 4, 4, "Pending"),
			want: false,
		},
		{
			name:       "applied and Ready, but still asking to be interrupted",
			nc:         nodeConfigAt(4, 4, 4, phaseReady),
			disruption: true,
			want:       false,
		},
		{
			name: "never reported: appliedGeneration 0",
			nc:   nodeConfigAt(4, 0, 0, ""),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.disruption {
				meta.SetStatusCondition(&tt.nc.Status.Conditions, metav1.Condition{
					Type:               disruptionRequiredCondition,
					Status:             metav1.ConditionTrue,
					Reason:             "DisruptionPending",
					ObservedGeneration: tt.nc.Generation,
				})
			}
			if got := applied(tt.nc); got != tt.want {
				t.Fatalf("applied() = %v, want %v", got, tt.want)
			}
		})
	}
}
