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
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// nodeConfigWith builds one node's config: what it was asked to install, and
// what it reports back.
func nodeConfigWith(name string, spec []internalv1alpha1.Extension, status []internalv1alpha1.ExtensionStatus) *internalv1alpha1.NodeConfig {
	return &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       internalv1alpha1.NodeSpec{NodeName: name, Extensions: spec},
		Status:     internalv1alpha1.NodeConfigStatus{Extensions: status},
	}
}

// fleetOf is what a status pass knows about these nodes when none waits for an
// approval and no render held one back.
func fleetOf(configs ...*internalv1alpha1.NodeConfig) fleet {
	f := fleet{configs: map[string]*internalv1alpha1.NodeConfig{}, approvals: map[string]approvalState{}}
	for _, config := range configs {
		f.configs[config.Name] = config
	}
	return f
}

// The whole point of reading the nodes back: a request that resolves here can
// still be refused by every node it reaches, and until this was counted the two
// looked identical from the cluster.
func TestNEROutcomesCountWhatTheNodesReport(t *testing.T) {
	outcomes := nerOutcomes(fleetOf(
		// Two nodes refuse bob and accept bob-signed.
		nodeConfigWith("master-0",
			[]internalv1alpha1.Extension{
				{Name: "bob", Digest: "sha256:d1", RequestedBy: nerOwner("bob-request")},
				{Name: "bob-signed", Digest: "sha256:d2", RequestedBy: nerOwner("bob-signed-request")},
			},
			[]internalv1alpha1.ExtensionStatus{
				{Name: "bob", Digest: "sha256:d1", State: "Failed", Message: "Required key not available"},
				{Name: "bob-signed", Digest: "sha256:d2", State: "Ready"},
			}),
		nodeConfigWith("worker-0",
			[]internalv1alpha1.Extension{
				{Name: "bob", Digest: "sha256:d1", RequestedBy: nerOwner("bob-request")},
				{Name: "bob-signed", Digest: "sha256:d2", RequestedBy: nerOwner("bob-signed-request")},
			},
			[]internalv1alpha1.ExtensionStatus{
				{Name: "bob", Digest: "sha256:d1", State: "Failed", Message: "Required key not available"},
				{Name: "bob-signed", Digest: "sha256:d2", State: "Ready"},
			}),
		// A node that has not answered yet counts as neither.
		nodeConfigWith("worker-1",
			[]internalv1alpha1.Extension{{Name: "bob", Digest: "sha256:d1", RequestedBy: nerOwner("bob-request")}},
			nil),
		// A platform extension carries no requestedBy: no request owns it.
		nodeConfigWith("worker-2",
			[]internalv1alpha1.Extension{{Name: "containerd", Digest: "sha256:d3"}},
			[]internalv1alpha1.ExtensionStatus{{Name: "containerd", Digest: "sha256:d3", State: "Ready"}}),
	), map[string]string{"bob-request": "sha256:d1", "bob-signed-request": "sha256:d2"})

	require.Equal(t, map[string]string{"master-0": "Required key not available", "worker-0": "Required key not available"},
		outcomes["bob-request"].failed, "the reason must reach the cluster, so nobody has to read a node's journal for it")
	require.Empty(t, outcomes["bob-request"].applied)

	require.Len(t, outcomes["bob-signed-request"].applied, 2)
	require.Empty(t, outcomes["bob-signed-request"].failed)
	require.Len(t, outcomes, 2, "a platform extension is no request's")
}

// A node reporting Pending is still working; counting it either way would be an
// answer it has not given.
func TestNEROutcomesIgnorePendingNodes(t *testing.T) {
	outcomes := nerOutcomes(fleetOf(
		nodeConfigWith("worker-0",
			[]internalv1alpha1.Extension{{Name: "bob", Digest: "sha256:d1", RequestedBy: nerOwner("bob-request")}},
			[]internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: "sha256:d1", State: "Pending"}}),
	), map[string]string{"bob-request": "sha256:d1"})

	require.False(t, answered(outcomes["bob-request"], "worker-0"))
}

// A node that refused or rolled back the revision carrying a request has refused the request,
// whether or not it named the extension: it will not run it until the configuration changes.
// The node's own Failed entry stays its word about that extension.
func TestNEROutcomesCountANodeThatRefusedTheRevisionCarryingTheRequest(t *testing.T) {
	refused := nodeConfigWith("master-0",
		[]internalv1alpha1.Extension{
			{Name: "containerd", Digest: "sha256:d3"},
			{Name: "bob-signed", Digest: "sha256:d2", RequestedBy: nerOwner("bob-signed-request")},
			{Name: "bob", Digest: "sha256:d1", RequestedBy: nerOwner("bob-request")},
		},
		[]internalv1alpha1.ExtensionStatus{
			{Name: "containerd", Digest: "sha256:d3", State: "Ready"},
			{Name: "bob", Digest: "sha256:d1", State: "Failed", Message: "its roothash signature is not one this node trusts"},
		})
	rolledBack := nodeConfigWith("worker-0",
		[]internalv1alpha1.Extension{{Name: "bob-signed", Digest: "sha256:d2", RequestedBy: nerOwner("bob-signed-request")}},
		nil)
	// Applied the generation it reports on: a stale refusal is no answer about the current one.
	current := nodeConfigWith("worker-1",
		[]internalv1alpha1.Extension{{Name: "bob-signed", Digest: "sha256:d2", RequestedBy: nerOwner("bob-signed-request")}},
		nil)
	for config, reason := range map[*internalv1alpha1.NodeConfig]string{
		refused: "Rejected", rolledBack: "RolledBackToLastKnownGood", current: "Rejected",
	} {
		config.Generation = 7
		config.Status.AppliedGeneration = 6
		config.Status.Conditions = []metav1.Condition{{
			Type: internalv1alpha1.NodeConfigConditionTypeConfigurationApplied, Status: metav1.ConditionFalse, Reason: reason,
			ObservedGeneration: 7, Message: "bob: config rejected",
		}}
	}
	current.Status.AppliedGeneration = 7

	outcomes := nerOutcomes(fleetOf(refused, rolledBack, current),
		map[string]string{"bob-request": "sha256:d1", "bob-signed-request": "sha256:d2"})

	require.Equal(t, map[string]string{"master-0": "its roothash signature is not one this node trusts"}, outcomes["bob-request"].failed)
	require.Equal(t, map[string]string{
		"master-0": "Rejected: bob: config rejected",
		"worker-0": "RolledBackToLastKnownGood: bob: config rejected",
	}, outcomes["bob-signed-request"].failed)
	require.Empty(t, outcomes["bob-signed-request"].applied)
}

// Applied means this request's digest: a node still on another one — the spec not
// rolled out to it yet, or a rollback — has not answered the request.
func TestNEROutcomesCountOnlyTheDigestOfTheRequest(t *testing.T) {
	outcomes := nerOutcomes(fleetOf(
		// Waits for a rollout slot: its spec still carries the old digest, and it runs it.
		nodeConfigWith("worker-0",
			[]internalv1alpha1.Extension{{Name: "bob", Digest: "sha256:d1", RequestedBy: nerOwner("bob-request")}},
			[]internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: "sha256:d1", State: "Ready"}}),
		// Rolled back: the spec says d2, the node runs d1.
		nodeConfigWith("worker-1",
			[]internalv1alpha1.Extension{{Name: "bob", Digest: "sha256:d2", RequestedBy: nerOwner("bob-request")}},
			[]internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: "sha256:d1", State: "Ready"}}),
		// Refused the old digest; that is not an answer about the new one.
		nodeConfigWith("worker-2",
			[]internalv1alpha1.Extension{{Name: "bob", Digest: "sha256:d1", RequestedBy: nerOwner("bob-request")}},
			[]internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: "sha256:d1", State: "Failed", Message: "old"}}),
		nodeConfigWith("worker-3",
			[]internalv1alpha1.Extension{{Name: "bob", Digest: "sha256:d2", RequestedBy: nerOwner("bob-request")}},
			[]internalv1alpha1.ExtensionStatus{{Name: "bob", Digest: "sha256:d2", State: "Ready"}}),
	), map[string]string{"bob-request": "sha256:d2"})

	require.Equal(t, map[string]struct{}{"worker-3": {}}, outcomes["bob-request"].applied, "only worker-3 runs the digest the request asks for")
	require.Empty(t, outcomes["bob-request"].failed)
}

// An approved operation waits for its eviction, not for an operator: the request
// status must not send the operator to approve it a second time.
func TestApprovalWaitNamesAnApprovedOperation(t *testing.T) {
	tests := []struct {
		phase deckhousev1alpha1.NodeOperationPhase
		exp   string
	}{
		{phase: deckhousev1alpha1.NodeOperationPhasePending, exp: "waiting for approval by NodeOperation disruption-worker-0-1 in phase Pending"},
		{phase: deckhousev1alpha1.NodeOperationPhaseApproved, exp: "NodeOperation disruption-worker-0-1 is approved, the node is being prepared"},
	}
	for _, tc := range tests {
		t.Run(string(tc.phase), func(t *testing.T) {
			op := approvalOf("worker-0", "disruption-worker-0-1", "uid-0", 3, tc.phase,
				"NodeOperation disruption-worker-0-1-drain-x is evicting the workload")
			require.Equal(t, tc.exp, approvalWait(approvalState{current: op}))
		})
	}
}

// Between attempts nothing is waited for in the cluster, so the refusal says when node-manager
// asks again.
func TestNodeRefusalSaysWhenTheNextAttemptIsRequested(t *testing.T) {
	finished := time.Date(2026, time.January, 1, 8, 0, 0, 0, time.UTC)
	attempt := func(name string, phase deckhousev1alpha1.NodeOperationPhase, finishedAt time.Time) *deckhousev1alpha1.NodeOperation {
		op := approvalOf("worker-0", name, "uid-0", 4, phase, "module example: no example.ko found")
		op.Status.FinishedAt = ptr.To(metav1.NewTime(finishedAt))
		return op
	}
	tests := []struct {
		name string
		ops  []client.Object
		exp  string
	}{
		{
			name: "after a failed attempt",
			ops:  []client.Object{attempt("disruption-worker-0-0", deckhousev1alpha1.NodeOperationPhaseFailed, finished)},
			exp: "NodeOperation disruption-worker-0-0 failed: module example: no example.ko found. " +
				"Attempt 2 of 3 is requested at 2026-01-01T08:01:00Z",
		},
		{
			name: "after two, the wait is longer and runs from the newest",
			ops: []client.Object{
				attempt("disruption-worker-0-0", deckhousev1alpha1.NodeOperationPhaseFailed, finished.Add(-time.Hour)),
				attempt("disruption-worker-0-1", deckhousev1alpha1.NodeOperationPhaseFailed, finished),
			},
			exp: "NodeOperation disruption-worker-0-1 failed: module example: no example.ko found. " +
				"Attempt 3 of 3 is requested at 2026-01-01T08:02:00Z",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := nodeConfigWith("worker-0", nil, nil)
			waitingForApproval(config, "uid-0", 4)
			r := &Reconciler{sources: &sourceReader{Reader: operationsClient(t, tc.ops...)}}
			approvals, err := r.approvalsFor(t.Context(), config)
			require.NoError(t, err)

			refusal, refused := nodeRefusal(config, approvals)
			require.True(t, refused)
			require.Equal(t, tc.exp, refusal)
		})
	}
}

// A window the node released is not a refusal, with the revision applied or not: a node that
// will not apply it says so in its NodeConfig (configRefused). A failure before that window is
// over once the window completed.
func TestNodeRefusalIsNotACompletedAttempt(t *testing.T) {
	finished := time.Now().Add(-time.Hour)
	attempt := func(name string, phase deckhousev1alpha1.NodeOperationPhase, finishedAt time.Time) *deckhousev1alpha1.NodeOperation {
		op := approvalOf("worker-0", name, "uid-0", 4, phase, "module example: no example.ko found")
		op.Status.FinishedAt = ptr.To(metav1.NewTime(finishedAt))
		return op
	}
	tests := []struct {
		name string
		ops  []client.Object
	}{
		{
			name: "a completed attempt",
			ops:  []client.Object{attempt("disruption-worker-0-0", deckhousev1alpha1.NodeOperationPhaseCompleted, finished)},
		},
		{
			name: "a completed attempt after a failed one",
			ops: []client.Object{
				attempt("disruption-worker-0-0", deckhousev1alpha1.NodeOperationPhaseFailed, finished.Add(-time.Hour)),
				attempt("disruption-worker-0-1", deckhousev1alpha1.NodeOperationPhaseCompleted, finished),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := nodeConfigWith("worker-0", nil, nil)
			waitingForApproval(config, "uid-0", 4)
			r := &Reconciler{sources: &sourceReader{Reader: operationsClient(t, tc.ops...)}}
			approvals, err := r.approvalsFor(t.Context(), config)
			require.NoError(t, err)

			refusal, refused := nodeRefusal(config, approvals)
			require.False(t, refused)
			require.Empty(t, refusal)
		})
	}
}
