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

// withConfigApplied stamps the condition the agent publishes on every pass,
// carrying the generation it describes.
func withConfigApplied(nc *internalv1alpha1.NodeConfig, status metav1.ConditionStatus, generation int64) *internalv1alpha1.NodeConfig {
	meta.SetStatusCondition(&nc.Status.Conditions, metav1.Condition{
		Type:               configurationAppliedCondition,
		Status:             status,
		Reason:             "ReconcileSucceeded",
		ObservedGeneration: generation,
	})
	return nc
}

// applied() is the rollout's "this node converged" test. It must key on the
// generation the node is RUNNING (appliedGeneration), not the one it has merely
// SEEN (observedGeneration): a held node has observed the current generation but
// is still running the previous one, and counting it as done would walk the
// change through the whole group while every node waits.
//
// It must also key on ConfigurationApplied rather than on the phase. The two
// answer different questions, and a node that is Degraded for a reason the
// rollout neither caused nor can fix must not hold the group's slot forever.
func TestApplied(t *testing.T) {
	tests := []struct {
		name       string
		nc         *internalv1alpha1.NodeConfig
		disruption bool
		want       bool
	}{
		{
			name: "running the current generation, Ready",
			nc:   withConfigApplied(nodeConfigAt(4, 4, 4, phaseReady), metav1.ConditionTrue, 4),
			want: true,
		},
		{
			name: "has seen gen 4 but is still running gen 3",
			nc:   withConfigApplied(nodeConfigAt(4, 4, 3, phaseReady), metav1.ConditionFalse, 4),
			want: false,
		},
		{
			name: "observedGeneration says 4 but appliedGeneration is 3 — not done",
			// The exact overstating agent the two-number split guards against.
			nc:   withConfigApplied(nodeConfigAt(4, 4, 3, phaseReady), metav1.ConditionTrue, 4),
			want: false,
		},
		{
			name: "rolled the config back: running gen 4's number but not its config",
			nc:   withConfigApplied(nodeConfigAt(4, 4, 4, "Degraded"), metav1.ConditionFalse, 4),
			want: false,
		},
		{
			// What the phase test got wrong. The node took the spec and is running
			// it; something else on it is broken. Holding the slot here froze every
			// other node in the group behind a fault the rollout cannot repair.
			name: "running the published config, but degraded for an unrelated reason",
			nc:   withConfigApplied(nodeConfigAt(4, 4, 4, "Degraded"), metav1.ConditionTrue, 4),
			want: true,
		},
		{
			name: "the condition is a generation stale",
			nc:   withConfigApplied(nodeConfigAt(4, 4, 4, phaseReady), metav1.ConditionTrue, 3),
			want: false,
		},
		{
			name:       "applied and Ready, but still asking to be interrupted",
			nc:         withConfigApplied(nodeConfigAt(4, 4, 4, phaseReady), metav1.ConditionTrue, 4),
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
