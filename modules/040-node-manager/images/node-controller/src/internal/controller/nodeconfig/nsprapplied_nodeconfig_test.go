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
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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

// nodeConfigScheme is all this source reads: no Nodes, no NodeGroups. Every node
// has a NodeConfig, so this reader needs nothing else to know whose reports it
// is counting.
func nodeConfigScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))
	return scheme
}

// The manifest on disk is the whole of what a node owes a static pod, so it is
// the whole of what "applied" means, and a node that has not reported is
// counted neither way: calling it a refusal would turn every rollout into a red
// object for a minute, and calling it applied would say a pod is on a node it
// has not reached.
func TestNodeConfigOutcomesCountWhatTheNodesReport(t *testing.T) {
	agent := []internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: podManifest("registry-agent")}}

	cl := fake.NewClientBuilder().WithScheme(nodeConfigScheme(t)).WithObjects(
		nodeConfigWithPod("worker-0", agent,
			[]internalv1alpha1.StaticPodStatus{{Name: "registry-agent", State: "Written"}}),
		nodeConfigWithPod("worker-1", agent,
			[]internalv1alpha1.StaticPodStatus{{
				Name: "registry-agent", State: "Failed",
				Reason: "WriteFailed", Message: "read-only file system",
			}}),
		// Has not answered at all: the node is not refusing, it has not spoken.
		nodeConfigWithPod("worker-2", agent, nil),
		// A second refusal keeps the first message; a state outside the enum counts as nothing.
		nodeConfigWithPod("worker-4", agent,
			[]internalv1alpha1.StaticPodStatus{{Name: "registry-agent", State: "Failed", Reason: "RemoveFailed", Message: "busy"}}),
		nodeConfigWithPod("worker-5", agent,
			[]internalv1alpha1.StaticPodStatus{{Name: "registry-agent", State: "Pending"}}),
		// Somebody else's static pod must not land on this one's counters.
		nodeConfigWithPod("worker-3",
			[]internalv1alpha1.StaticPod{{Name: "somebody-else", Manifest: podManifest("somebody-else")}},
			[]internalv1alpha1.StaticPodStatus{{Name: "somebody-else", State: "Failed", Reason: "ManifestRejected"}}),
	).Build()

	outcomes, err := readNodeConfigOutcomes(context.Background(), cl)
	require.NoError(t, err)

	require.Equal(t, int32(1), outcomes["registry-agent"].applied)
	require.Equal(t, int32(2), outcomes["registry-agent"].failed)
	// The reason has to reach the cluster, so nobody has to read a node's
	// journal to learn whether to edit the object or to go and look at the node.
	require.Equal(t, "WriteFailed: read-only file system", outcomes["registry-agent"].message)

	require.Equal(t, int32(1), outcomes["somebody-else"].failed)
	require.Equal(t, int32(0), outcomes["somebody-else"].applied)
}

// Every reason is a refusal: this source counts them alike and only carries the
// difference into the message. RemoveFailed is the one an object may never see —
// the name has left the spec by then — but it is a node holding a pod the
// cluster stopped asking for, and it must not be silently dropped.
func TestNodeConfigOutcomesCountEveryReasonAsARefusal(t *testing.T) {
	for _, reason := range []string{"ManifestRejected", "WriteFailed", "RemoveFailed"} {
		t.Run(reason, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(nodeConfigScheme(t)).WithObjects(
				nodeConfigWithPod("worker-0", nil,
					[]internalv1alpha1.StaticPodStatus{{Name: "agent", State: "Failed", Reason: reason}}),
			).Build()

			outcomes, err := readNodeConfigOutcomes(context.Background(), cl)
			require.NoError(t, err)
			require.Equal(t, int32(1), outcomes["agent"].failed)
			require.Equal(t, int32(0), outcomes["agent"].applied)
			require.Equal(t, reason, outcomes["agent"].message,
				"a refusal with no message still has to say which of the three it was")
		})
	}
}

// A fleet that reports nothing is a fleet with no counters, and the status pass
// has to be able to tell that from "every node wrote it".
func TestNodeConfigOutcomesOfASilentFleet(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nodeConfigScheme(t)).Build()

	outcomes, err := readNodeConfigOutcomes(context.Background(), cl)
	require.NoError(t, err)
	require.Empty(t, outcomes)
}

// A report counts only for the generation that carries the current manifest: a
// node still running the previous one reports the old manifest Written, and a
// node refusing its current generation whole has refused the pod in it.
func TestNodeConfigOutcomesFollowTheCurrentGeneration(t *testing.T) {
	agent := []internalv1alpha1.StaticPod{{Name: "registry-agent", Manifest: podManifest("registry-agent")}}
	written := []internalv1alpha1.StaticPodStatus{{Name: "registry-agent", State: "Written"}}
	atGeneration := func(name string, generation, applied int64, conditions ...metav1.Condition) *internalv1alpha1.NodeConfig {
		config := nodeConfigWithPod(name, agent, written)
		config.Generation = generation
		config.Status.AppliedGeneration = applied
		config.Status.Conditions = conditions
		return config
	}
	notApplied := func(reason string, generation int64) metav1.Condition {
		return metav1.Condition{
			Type: configurationAppliedCondition, Status: metav1.ConditionFalse,
			ObservedGeneration: generation, Reason: reason, Message: "the node said why",
		}
	}
	held := atGeneration("held", 3, 2, notApplied("DisruptionApprovalPending", 3), metav1.Condition{
		Type: disruptionRequiredCondition, Status: metav1.ConditionTrue, ObservedGeneration: 3, Reason: "DisruptionPending",
	})

	cl := fake.NewClientBuilder().WithScheme(nodeConfigScheme(t)).WithObjects(
		atGeneration("current", 3, 3),
		atGeneration("behind", 3, 2),
		atGeneration("rolled-back", 3, 2, notApplied("RolledBackToLastKnownGood", 3)),
		atGeneration("stale-refusal", 3, 2, notApplied("Rejected", 2)),
		held,
	).Build()

	outcomes, err := readNodeConfigOutcomes(context.Background(), cl)
	require.NoError(t, err)
	require.Equal(t, int32(1), outcomes["registry-agent"].applied)
	require.Equal(t, int32(1), outcomes["registry-agent"].failed)
	require.Equal(t, "RolledBackToLastKnownGood: the node said why", outcomes["registry-agent"].message)
}
