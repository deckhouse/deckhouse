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

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// nodeConfigWithPod builds one node's config: the static pod it was given, and
// what it reports back about it.
func nodeConfigWithPod(name string, pods []internalv1alpha1.StaticPod, podStatus []internalv1alpha1.StaticPodStatus) *internalv1alpha1.NodeConfig {
	return &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       internalv1alpha1.NodeSpec{NodeName: name, StaticPods: pods},
		Status:     internalv1alpha1.NodeConfigStatus{StaticPods: podStatus},
	}
}

// writtenPod is the entry a node publishes once the manifest is on its disk.
func writtenPod(name, manifest string) internalv1alpha1.StaticPodStatus {
	return internalv1alpha1.StaticPodStatus{Name: name, State: "Written", ManifestDigest: manifestDigest(manifest)}
}

// The manifest on disk is the whole of what a node owes a static pod, so it is
// the whole of what "applied" means, and a node that has not reported is
// counted neither way: calling it a refusal would turn every rollout into a red
// object for a minute, and calling it applied would say a pod is on a node it
// has not reached.
func TestNodeConfigOutcomesCountWhatTheNodesReport(t *testing.T) {
	agent := []internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: podManifest("registry-agent")}}

	outcomes := nsprOutcomes(fleetOf(
		nodeConfigWithPod("worker-0", agent,
			[]internalv1alpha1.StaticPodStatus{writtenPod("registry-agent", podManifest("registry-agent"))}),
		nodeConfigWithPod("worker-1", agent,
			[]internalv1alpha1.StaticPodStatus{{
				Name: "registry-agent", State: "Failed",
				Reason: "WriteFailed", Message: "read-only file system",
			}}),
		// Has not answered at all: the node is not refusing, it has not spoken.
		nodeConfigWithPod("worker-2", agent, nil),
		// Every node's refusal is kept; a state outside the enum counts as nothing.
		nodeConfigWithPod("worker-4", agent,
			[]internalv1alpha1.StaticPodStatus{{Name: "registry-agent", State: "Failed", Reason: "RemoveFailed", Message: "busy"}}),
		nodeConfigWithPod("worker-5", agent,
			[]internalv1alpha1.StaticPodStatus{{Name: "registry-agent", State: "Pending"}}),
		// Somebody else's static pod must not land on this one's counters.
		nodeConfigWithPod("worker-3",
			[]internalv1alpha1.StaticPod{{Name: "somebody-else", Manifest: podManifest("somebody-else")}},
			[]internalv1alpha1.StaticPodStatus{{Name: "somebody-else", State: "Failed", Reason: "ManifestRejected"}}),
	), map[string]string{"registry-agent": podManifest("registry-agent"), "somebody-else": podManifest("somebody-else")})

	require.Equal(t, map[string]struct{}{"worker-0": {}}, outcomes["registry-agent"].applied)
	// The reason has to reach the cluster, so nobody has to read a node's
	// journal to learn whether to edit the object or to go and look at the node.
	require.Equal(t, map[string]string{"worker-1": "WriteFailed: read-only file system", "worker-4": "RemoveFailed: busy"},
		outcomes["registry-agent"].failed)
	require.Equal(t, map[string]string{"worker-3": "ManifestRejected"}, outcomes["somebody-else"].failed)
	require.Empty(t, outcomes["somebody-else"].applied)
}

// Every reason is a refusal: this source counts them alike and only carries the
// difference into the message. RemoveFailed is the one an object may never see —
// the name has left the spec by then — but it is a node holding a pod the
// cluster stopped asking for, and it must not be silently dropped.
func TestNodeConfigOutcomesCountEveryReasonAsARefusal(t *testing.T) {
	for _, reason := range []string{"ManifestRejected", "WriteFailed", "RemoveFailed"} {
		t.Run(reason, func(t *testing.T) {
			outcomes := nsprOutcomes(fleetOf(
				nodeConfigWithPod("worker-0", nil,
					[]internalv1alpha1.StaticPodStatus{{Name: "agent", State: "Failed", Reason: reason}}),
			), map[string]string{"agent": podManifest("agent")})

			require.Equal(t, map[string]string{"worker-0": reason}, outcomes["agent"].failed,
				"a refusal with no message still has to say which of the three it was")
			require.Empty(t, outcomes["agent"].applied)
		})
	}
}

// A fleet that reports nothing is a fleet with no counters, and the status pass
// has to be able to tell that from "every node wrote it".
func TestNodeConfigOutcomesOfASilentFleet(t *testing.T) {
	require.Empty(t, nsprOutcomes(fleetOf(), map[string]string{"agent": podManifest("agent")}))
}

// A node runs the request when it reports the request's manifest written, by digest, whatever
// generation it runs: a held node whose older generation carries the same manifest runs it. Another
// digest is no answer, and a node refusing the generation that carries the manifest refused it.
func TestNodeConfigOutcomesCountTheManifestTheNodeWrote(t *testing.T) {
	current, previous := podManifest("registry-agent"), podManifest("registry-agent-v1")
	atGeneration := func(name, specified, written string, generation, applied int64, conditions ...metav1.Condition) *internalv1alpha1.NodeConfig {
		config := nodeConfigWithPod(name,
			[]internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: specified}},
			[]internalv1alpha1.StaticPodStatus{writtenPod("registry-agent", written)})
		config.Generation = generation
		config.Status.AppliedGeneration = applied
		config.Status.Conditions = conditions
		return config
	}
	notApplied := func(reason string, generation int64) metav1.Condition {
		return metav1.Condition{
			Type: internalv1alpha1.NodeConfigConditionTypeConfigurationApplied, Status: metav1.ConditionFalse,
			ObservedGeneration: generation, Reason: reason, Message: "the node said why",
		}
	}

	outcomes := nsprOutcomes(fleetOf(
		atGeneration("current", current, current, 3, 3),
		// Held for an approval of a change elsewhere in its config: the manifest is unchanged.
		atGeneration("held", current, current, 3, 2, notApplied("DisruptionApprovalPending", 3), metav1.Condition{
			Type: internalv1alpha1.NodeConfigConditionTypeDisruptionApproved, Status: metav1.ConditionFalse, ObservedGeneration: 3, Reason: "ApprovalPending",
		}),
		// The rollout slot held its spec back: it still runs the previous manifest.
		atGeneration("queued", previous, previous, 2, 2),
		atGeneration("behind", current, previous, 3, 2),
		atGeneration("rolled-back", current, previous, 3, 2, notApplied("RolledBackToLastKnownGood", 3)),
		atGeneration("stale-refusal", current, previous, 3, 2, notApplied("Rejected", 2)),
		// A pass that failed to save: no refusal of the pods, and its entries are the old generation's.
		atGeneration("save-failed", current, previous, 3, 2, notApplied("PartiallyApplied", 3)),
		// An older agent publishes no digest: it has not said which manifest it wrote.
		atGeneration("no-digest", current, "", 3, 3),
	), map[string]string{"registry-agent": current})

	require.Equal(t, map[string]struct{}{"current": {}, "held": {}}, outcomes["registry-agent"].applied)
	require.Equal(t, map[string]string{"rolled-back": "RolledBackToLastKnownGood: the node said why"}, outcomes["registry-agent"].failed)
}

// ConfigurationApplied=False is more than a refusal: a self-update restart, a bootstrap wait, fields
// the node does not take, a failed package (PartiallyApplied) next to written pods. Only a refusal
// fails every pod in the generation; otherwise the per-pod entries answer, or each fault paints all red.
func TestOnlyARefusedConfigFailsItsStaticPods(t *testing.T) {
	agent := []internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: podManifest("registry-agent")}}
	for _, tc := range []struct {
		reason  string
		refused bool
	}{
		{reason: "RestartPending"},
		{reason: "Pending"},
		{reason: "FieldsNotApplicable"},
		{reason: "UnknownFields"},
		{reason: "Rejected", refused: true},
		{reason: "RolledBackToLastKnownGood", refused: true},
		{reason: "RollbackFailed", refused: true},
		{reason: "PartiallyApplied"},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			config := nodeConfigWithPod("worker-0", agent,
				[]internalv1alpha1.StaticPodStatus{writtenPod("registry-agent", podManifest("registry-agent-v1"))})
			config.Generation = 2
			config.Status.AppliedGeneration = 1
			config.Status.Conditions = []metav1.Condition{{
				Type: internalv1alpha1.NodeConfigConditionTypeConfigurationApplied, Status: metav1.ConditionFalse,
				ObservedGeneration: 2, Reason: tc.reason, Message: "the node said why",
			}}

			outcomes := nsprOutcomes(fleetOf(config), map[string]string{"registry-agent": podManifest("registry-agent")})

			if tc.refused {
				require.Equal(t, map[string]string{"worker-0": tc.reason + ": the node said why"}, outcomes["registry-agent"].failed)
				return
			}
			require.False(t, answered(outcomes["registry-agent"], "worker-0"), "the per-pod entry answers, and it is about another manifest")
		})
	}
}
