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
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	sigsyaml "sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// TestNeedDrain: only a lone control-plane node, or the node of the Deckhouse pod with no
// other Ready node in its group, is interrupted without a drain. Any other group is drained,
// a group of one included.
func TestNeedDrain(t *testing.T) {
	tests := []struct {
		name          string
		ng            *v1.NodeGroup
		runsDeckhouse bool
		want          bool
	}{
		{
			name: "a group of one is drained",
			ng:   nodeGroupWithNodes(1),
			want: true,
		},
		{
			name: "a lone control-plane node is not drained",
			ng: func() *v1.NodeGroup {
				ng := nodeGroupWithNodes(1)
				ng.Name = "master"
				return ng
			}(),
			want: false,
		},
		{
			name: "a control plane of three is drained",
			ng: func() *v1.NodeGroup {
				ng := nodeGroupWithNodes(3)
				ng.Name = "master"
				return ng
			}(),
			want: true,
		},
		{
			name: "the node of the Deckhouse pod with no other Ready node is not drained",
			ng: func() *v1.NodeGroup {
				ng := nodeGroupWithNodes(2)
				ng.Status.Ready = 1
				return ng
			}(),
			runsDeckhouse: true,
			want:          false,
		},
		{
			name: "the node of the Deckhouse pod with another Ready node is drained",
			ng: func() *v1.NodeGroup {
				ng := nodeGroupWithNodes(2)
				ng.Status.Ready = 2
				return ng
			}(),
			runsDeckhouse: true,
			want:          true,
		},
		{
			name: "a group whose nodes have not been counted yet is drained",
			ng:   nodeGroupWithNodes(0),
			want: true,
		},
		{
			name: "a group of many is drained",
			ng:   nodeGroupWithNodes(50),
			want: true,
		},
		{
			name: "the operator can turn the drain off",
			ng: func() *v1.NodeGroup {
				ng := nodeGroupWithNodes(50)
				ng.Spec.Disruptions = &v1.DisruptionsSpec{
					Automatic: &v1.AutomaticDisruptionSpec{DrainBeforeApproval: ptr.To(false)},
				}
				return ng
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, needDrain(tt.ng, tt.runsDeckhouse))
		})
	}
}

// An operation is owned by the node and outlives the NodeConfig by a day, so a
// recreated config — counting from generation 1 again — matched the operation of
// its predecessor and waited for a disruption nobody would carry out, holding a
// rollout slot of its group until the operation was collected.
func TestFindApprovalIgnoresAnOperationOfAnEarlierNodeConfig(t *testing.T) {
	const previousUID, recreatedUID = "5f5d1c7e-0000-4000-8000-000000000001", "5f5d1c7e-0000-4000-8000-000000000002"

	inFlight := approval("disruption-worker-0-abcde", previousUID, 2, true, v1alpha1.NodeOperationPhaseInProgress)
	r := &Reconciler{sources: &sourceReader{Reader: operationsClient(t, inFlight)}}

	recreated, err := r.approvalsFor(t.Context(), nodeConfigOf("worker-0", recreatedUID, 2))
	require.NoError(t, err)
	require.Nil(t, recreated.current, "the operation is carried out for an object that no longer exists")

	previous, err := r.approvalsFor(t.Context(), nodeConfigOf("worker-0", previousUID, 2))
	require.NoError(t, err)
	require.NotNil(t, previous.current, "the object it was issued for must still find it, or it is approved twice")
}

// A NodeOperation fails on a deadline or on a pod nothing can evict. The node
// keeps asking, so a failed attempt must not be mistaken for one in flight:
// that leaves the node holding a rollout slot for a disruption nobody will
// carry out.
func TestFindApprovalAllowsAFreshAttemptAfterAFailure(t *testing.T) {
	const uid = "5f5d1c7e-0000-4000-8000-000000000003"

	failed := approval("disruption-worker-0-fghij", uid, 2, true, v1alpha1.NodeOperationPhaseFailed)
	r := &Reconciler{sources: &sourceReader{Reader: operationsClient(t, failed)}}

	found, err := r.approvalsFor(t.Context(), nodeConfigOf("worker-0", uid, 2))
	require.NoError(t, err)
	require.Nil(t, found.current, "a failed operation must not stand in for one in flight")
	require.Equal(t, 1, found.failures, "it is an attempt spent, and the attempts are counted")
}

// A disruption that failed fails the same way when it is tried again at once:
// the node is drained, cordoned and released for nothing, on every pass, until
// somebody notices. The retries wait, and then stop.
func TestReconcileDisruptionBacksOffAndGivesUp(t *testing.T) {
	const uid = "5f5d1c7e-0000-4000-8000-000000000004"
	const generation = int64(2)

	tests := []struct {
		name     string
		failures []time.Duration // how long ago each failed attempt finished
		wantOps  int
		wantWait bool
		// Reported on the group and on the node's config both: two objects, two
		// people looking at them.
		wantWarnings int
	}{
		{
			name:    "the first request is answered at once",
			wantOps: 1,
		},
		{
			name:     "a failure just now is not retried yet",
			failures: []time.Duration{time.Second},
			wantOps:  0,
			wantWait: true,
		},
		{
			name:     "the retry comes once the backoff is up",
			failures: []time.Duration{2 * time.Minute},
			wantOps:  1,
		},
		{
			name:     "the second wait is longer than the first",
			failures: []time.Duration{2 * time.Minute, 90 * time.Second},
			wantOps:  0,
			wantWait: true,
		},
		{
			name:         "after the last attempt the revision is left alone",
			failures:     []time.Duration{time.Hour, time.Hour, time.Hour},
			wantOps:      0,
			wantWarnings: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make([]client.Object, 0, len(tt.failures))
			for i, ago := range tt.failures {
				objects = append(objects, failedApproval(fmt.Sprintf("disruption-worker-0-%d", i), uid, generation, ago))
			}
			cl := operationsClient(t, objects...)

			recorder := record.NewFakeRecorder(10)
			r := &Reconciler{sources: &sourceReader{Reader: cl}}
			r.Client = cl
			r.Recorder = recorder

			nc := askingNodeConfig("worker-0", uid, generation)
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}

			result, err := r.reconcileDisruption(t.Context(), nodeGroupWithNodes(2), node, nc, logr.Discard())
			require.NoError(t, err)

			created := &v1alpha1.NodeOperationList{}
			require.NoError(t, cl.List(t.Context(), created))
			require.Len(t, created.Items, len(tt.failures)+tt.wantOps)

			require.Equal(t, tt.wantWait, result.RequeueAfter > 0, "the wait has to bring the next pass along itself")

			close(recorder.Events)
			var warnings []string
			for event := range recorder.Events {
				if strings.Contains(event, disruptionApprovalExhaustedEvent) {
					warnings = append(warnings, event)
				}
			}
			require.Len(t, warnings, tt.wantWarnings)
			for _, warning := range warnings {
				require.Equal(t, "Warning DisruptionApprovalExhausted 3 attempts at generation 2 failed on node worker-0. "+
					"No new NodeOperation is created until the configuration changes or the oldest attempt is deleted, 24 hours after it finished", warning)
			}
		})
	}
}

// NOP-3: the request is in the API from the moment the node asks, in both
// modes; the mode only decides who approves it.
func TestReconcileDisruptionAsksAtOnceInEveryMode(t *testing.T) {
	const uid = "5f5d1c7e-0000-4000-8000-000000000005"

	for _, mode := range []v1.DisruptionApprovalMode{v1.DisruptionApprovalModeAutomatic, v1.DisruptionApprovalModeManual} {
		t.Run(string(mode), func(t *testing.T) {
			cl := operationsClient(t)
			r := &Reconciler{sources: &sourceReader{Reader: cl}}
			r.Client = cl
			r.Recorder = record.NewFakeRecorder(10)

			ng := nodeGroupWithNodes(2)
			ng.Spec.Disruptions = &v1.DisruptionsSpec{ApprovalMode: mode}
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}

			_, err := r.reconcileDisruption(t.Context(), ng, node, askingNodeConfig("worker-0", uid, 4), logr.Discard())
			require.NoError(t, err)

			created := &v1alpha1.NodeOperationList{}
			require.NoError(t, cl.List(t.Context(), created))
			require.Len(t, created.Items, 1)
			op := created.Items[0]
			require.Equal(t, v1alpha1.NodeOperationTypeDisruption, op.Spec.Type)
			require.Equal(t, types.UID(uid), op.Spec.NodeConfigUID, "the node checks the identity of its NodeConfig")
			require.Equal(t, ptr.To(int64(4)), op.Spec.ConfigGeneration)
			require.Equal(t, ptr.To(mode == v1.DisruptionApprovalModeAutomatic), op.Spec.Approved)
			require.NotContains(t, op.Labels, "node-manager.deckhouse.io/node", "operations are found by spec.nodeName")
		})
	}
}

// The node acts on an operation by its type name, and an operator finds it by
// its name prefix, so both are pinned as written rather than through a constant.
func TestReconcileDisruptionCreatesADisruption(t *testing.T) {
	cl := operationsClient(t)
	r := &Reconciler{sources: &sourceReader{Reader: cl}}
	r.Client = cl
	r.Recorder = record.NewFakeRecorder(10)
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}
	nc := askingNodeConfig("worker-0", "5f5d1c7e-0000-4000-8000-000000000011", 4)

	_, err := r.reconcileDisruption(t.Context(), nodeGroupWithNodes(2), node, nc, logr.Discard())
	require.NoError(t, err)

	created := &v1alpha1.NodeOperationList{}
	require.NoError(t, cl.List(t.Context(), created))
	require.Len(t, created.Items, 1)
	require.Equal(t, v1alpha1.NodeOperationType("Disruption"), created.Items[0].Spec.Type)
	require.Equal(t, "disruption-worker-0-", created.Items[0].GenerateName)
}

// The node of a deleted Cluster API machine is going away: a new interruption would only be
// ended at once with MachineDeleted, so none is asked for while the machine's Drain exists.
func TestReconcileDisruptionAsksNothingForTheNodeOfADeletedMachine(t *testing.T) {
	const uid = "5f5d1c7e-0000-4000-8000-000000000011"
	for _, phase := range []v1alpha1.NodeOperationPhase{v1alpha1.NodeOperationPhaseInProgress, v1alpha1.NodeOperationPhaseCompleted} {
		t.Run(string(phase), func(t *testing.T) {
			ended := failedApproval("disruption-worker-0-ended", uid, 4, 5*time.Minute)
			ended.Status.Reason = "MachineDeleted"
			drain := &v1alpha1.NodeOperation{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "drain-worker-0-machine",
					Labels: map[string]string{v1alpha1.NodeOperationMachineUIDLabel: "8e1f0c2a-0000-4000-8000-000000000001"},
				},
				Spec:   v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeDrain, NodeName: "worker-0"},
				Status: v1alpha1.NodeOperationStatus{Phase: phase},
			}
			cl := operationsClient(t, ended, drain)
			r := &Reconciler{sources: &sourceReader{Reader: cl}}
			r.Client = cl
			r.Recorder = record.NewFakeRecorder(10)
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}

			_, err := r.reconcileDisruption(t.Context(), nodeGroupWithNodes(2), node, askingNodeConfig("worker-0", uid, 4), logr.Discard())
			require.NoError(t, err)

			require.ElementsMatch(t, []string{"disruption-worker-0-ended", "drain-worker-0-machine"}, operationNames(t, cl))
		})
	}
}

// A group of one is drained like any other: only a lone control-plane node, or the node of the
// Deckhouse pod with no other Ready node, is interrupted without a drain.
func TestReconcileDisruptionDrainsAGroupOfOne(t *testing.T) {
	deckhousePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "deckhouse-0", Namespace: "d8-system", Labels: map[string]string{"app": "deckhouse"}},
		Spec:       corev1.PodSpec{NodeName: "engine-0"},
	}
	tests := []struct {
		name     string
		objects  []client.Object
		wantSkip bool
	}{
		{name: "a group of one", wantSkip: false},
		{name: "the node of the Deckhouse pod, the only Ready one", objects: []client.Object{deckhousePod}, wantSkip: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := operationsClient(t, tt.objects...)
			r := &Reconciler{sources: &sourceReader{Reader: cl}}
			r.Client = cl
			r.Recorder = record.NewFakeRecorder(10)
			ng := nodeGroupWithNodes(1)
			ng.Name = "engine"
			ng.Status.Ready = 1
			nc := askingNodeConfig("engine-0", "5f5d1c7e-0000-4000-8000-000000000021", 4)

			_, err := r.reconcileDisruption(t.Context(), ng, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "engine-0"}}, nc, logr.Discard())
			require.NoError(t, err)

			created := &v1alpha1.NodeOperationList{}
			require.NoError(t, cl.List(t.Context(), created))
			require.Len(t, created.Items, 1)
			require.Equal(t, tt.wantSkip, created.Items[0].Spec.Drain.Skip)
		})
	}
}

// The console reads why the node is interrupted from the operation: its users may
// not read the NodeConfig. The prefix every request of the node opens with says nothing.
func TestReconcileDisruptionRecordsTheCause(t *testing.T) {
	cl := operationsClient(t)
	r := &Reconciler{sources: &sourceReader{Reader: cl}}
	r.Client = cl
	r.Recorder = record.NewFakeRecorder(10)
	nc := askingNodeConfig("worker-0", "5f5d1c7e-0000-4000-8000-000000000012", 4)
	nc.Status.Conditions[0].Message = "waiting for approval to interrupt this node: kernel module(s) example would be loaded into the running kernel"

	_, err := r.reconcileDisruption(t.Context(), nodeGroupWithNodes(2), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}, nc, logr.Discard())
	require.NoError(t, err)

	created := &v1alpha1.NodeOperationList{}
	require.NoError(t, cl.List(t.Context(), created))
	require.Len(t, created.Items, 1)
	require.Equal(t, "kernel module(s) example would be loaded into the running kernel", created.Items[0].Spec.Cause)
}

// A cause longer than spec.cause holds would fail every create, so it is cut to the
// CRD's bound, and the bound here is the CRD's.
func TestDisruptionCauseFitsTheCRD(t *testing.T) {
	raw, err := os.ReadFile(testenv.NodeManagerCRDPaths(testenv.NodeOperationCRDFile)[0])
	require.NoError(t, err)
	crd := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, sigsyaml.Unmarshal(raw, crd))
	maxLength := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["cause"].MaxLength
	require.Equal(t, ptr.To(int64(maxCause)), maxLength)

	nc := askingNodeConfig("worker-0", "5f5d1c7e-0000-4000-8000-000000000013", 4)
	nc.Status.Conditions[0].Message = approvalRequestPrefix + strings.Repeat("\u0436", maxCause)
	cause := disruptionCause(nc)
	require.LessOrEqual(t, len(cause), maxCause)
	require.True(t, utf8.ValidString(cause))
	require.True(t, strings.HasSuffix(cause, "… (truncated)"))
}

// The node publishes DisruptionApproved at rest too; only False/ApprovalPending
// for the revision it holds is a request.
func TestDisruptionRequested(t *testing.T) {
	tests := []struct {
		name       string
		status     metav1.ConditionStatus
		reason     string
		generation int64
		want       bool
	}{
		{name: "waiting for the revision it holds", status: metav1.ConditionFalse, reason: internalv1alpha1.NodeConfigReasonApprovalPending, generation: 3, want: true},
		{name: "waiting for a revision it was moved off", status: metav1.ConditionFalse, reason: internalv1alpha1.NodeConfigReasonApprovalPending, generation: 2},
		{name: "nothing to approve", status: metav1.ConditionTrue, reason: "NoDisruptionPending", generation: 3},
		{name: "applying what was approved", status: metav1.ConditionTrue, reason: "Approved", generation: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nc := nodeConfigOf("worker-0", "5f5d1c7e-0000-4000-8000-000000000006", 3)
			nc.Status.Conditions = []metav1.Condition{{
				Type: internalv1alpha1.NodeConfigConditionTypeDisruptionApproved, Status: tt.status, Reason: tt.reason, ObservedGeneration: tt.generation,
			}}
			require.Equal(t, tt.want, disruptionRequested(nc))
		})
	}
}

// An unapproved request for an older generation stays Pending next to the current one,
// and approving it drains the node for nothing. It is deleted, with an event on the
// NodeConfig. An approved one is left alone, since the drain for it may already run. So
// are the request for the current generation and the requests of a previous NodeConfig
// object.
func TestReconcileDisruptionDeletesAnUnapprovedRequestOfAnOlderRevision(t *testing.T) {
	const uid, recreatedUID = "5f5d1c7e-0000-4000-8000-000000000007", "5f5d1c7e-0000-4000-8000-000000000008"

	cl := operationsClient(t,
		approval("older-unapproved", uid, 2, false, v1alpha1.NodeOperationPhasePending),
		approval("previous-object", recreatedUID, 2, false, v1alpha1.NodeOperationPhasePending),
		approval("older-approved", uid, 2, true, v1alpha1.NodeOperationPhaseApproved),
		approval("current-unapproved", uid, 3, false, v1alpha1.NodeOperationPhasePending),
	)
	recorder := record.NewFakeRecorder(10)
	recorder.IncludeObject = true
	r := &Reconciler{sources: &sourceReader{Reader: cl}}
	r.Client = cl
	r.Recorder = recorder

	// No disruption is requested: the newer generation may need none.
	nc := nodeConfigOf("worker-0", uid, 3)
	nc.TypeMeta = metav1.TypeMeta{Kind: "NodeConfig", APIVersion: internalv1alpha1.GroupVersion.String()}
	_, err := r.reconcileDisruption(t.Context(), nodeGroupWithNodes(2), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}, nc, logr.Discard())
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"previous-object", "older-approved", "current-unapproved"}, operationNames(t, cl))
	require.Equal(t, []string{"Normal DisruptionRequestSuperseded " +
		"NodeOperation older-unapproved for generation 2 is deleted: the NodeConfig moved to generation 3 before it was approved. " +
		"NodeOperation current-unapproved requests approval for generation 3" +
		" involvedObject{kind=NodeConfig,apiVersion=" + internalv1alpha1.GroupVersion.String() + "}"}, recordedEvents(recorder))
}

// The newer generation has no request yet, so the event names none. The request for it is
// created in the same pass.
func TestReconcileDisruptionDeletesAStaleRequestBeforeTheNewOneExists(t *testing.T) {
	const uid = "5f5d1c7e-0000-4000-8000-000000000009"

	cl := operationsClient(t, approval("disruption-worker-0-old", uid, 5, false, v1alpha1.NodeOperationPhasePending))
	recorder := record.NewFakeRecorder(10)
	r := &Reconciler{sources: &sourceReader{Reader: cl}}
	r.Client = cl
	r.Recorder = recorder

	_, err := r.reconcileDisruption(t.Context(), nodeGroupWithNodes(2), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}, askingNodeConfig("worker-0", uid, 6), logr.Discard())
	require.NoError(t, err)

	names := operationNames(t, cl)
	require.Len(t, names, 1)
	require.NotEqual(t, "disruption-worker-0-old", names[0], "the request for generation 6 replaces it")
	require.Contains(t, recordedEvents(recorder), "Normal DisruptionRequestSuperseded "+
		"NodeOperation disruption-worker-0-old for generation 5 is deleted: the NodeConfig moved to generation 6 before it was approved")
}

// An operator approving the stale request between the read and the delete wins: the
// delete carries the resourceVersion that was read, and the approved request stays.
func TestReconcileDisruptionLeavesAStaleRequestApprovedDuringTheDelete(t *testing.T) {
	const uid = "5f5d1c7e-0000-4000-8000-000000000010"

	cl := operationsClient(t, approval("disruption-worker-0-old", uid, 2, false, v1alpha1.NodeOperationPhasePending))
	deletes := 0
	racing := interceptor.NewClient(cl.(client.WithWatch), interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			deletes++
			approved := &v1alpha1.NodeOperation{}
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(obj), approved))
			approved.Spec.Approved = ptr.To(true)
			require.NoError(t, c.Update(ctx, approved))
			return c.Delete(ctx, obj, opts...)
		},
	})
	recorder := record.NewFakeRecorder(10)
	r := &Reconciler{sources: &sourceReader{Reader: cl}}
	r.Client = racing
	r.Recorder = recorder

	_, err := r.reconcileDisruption(t.Context(), nodeGroupWithNodes(2), &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}, nodeConfigOf("worker-0", uid, 3), logr.Discard())
	require.NoError(t, err)

	require.Equal(t, 1, deletes, "the delete was tried")
	require.Equal(t, ptr.To(true), operationNamed(t, cl, "disruption-worker-0-old").Spec.Approved)
	require.Empty(t, recordedEvents(recorder))
}

func operationNamed(t *testing.T, cl client.Client, name string) *v1alpha1.NodeOperation {
	t.Helper()
	op := &v1alpha1.NodeOperation{}
	require.NoError(t, cl.Get(t.Context(), types.NamespacedName{Name: name}, op))
	return op
}

func operationNames(t *testing.T, cl client.Client) []string {
	t.Helper()
	ops := &v1alpha1.NodeOperationList{}
	require.NoError(t, cl.List(t.Context(), ops))
	names := make([]string, 0, len(ops.Items))
	for _, op := range ops.Items {
		names = append(names, op.Name)
	}
	return names
}

func recordedEvents(recorder *record.FakeRecorder) []string {
	close(recorder.Events)
	var events []string
	for event := range recorder.Events {
		events = append(events, event)
	}
	return events
}

func approval(name, uid string, generation int64, approved bool, phase v1alpha1.NodeOperationPhase) *v1alpha1.NodeOperation {
	return &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.NodeOperationSpec{
			Type:             v1alpha1.NodeOperationTypeDisruption,
			NodeName:         "worker-0",
			ConfigGeneration: ptr.To(generation),
			NodeConfigUID:    types.UID(uid),
			Approved:         ptr.To(approved),
		},
		Status: v1alpha1.NodeOperationStatus{Phase: phase},
	}
}

func failedApproval(name, uid string, generation int64, finishedAgo time.Duration) *v1alpha1.NodeOperation {
	op := approval(name, uid, generation, true, v1alpha1.NodeOperationPhaseFailed)
	op.Status.FinishedAt = ptr.To(metav1.NewTime(time.Now().Add(-finishedAgo)))
	return op
}

// operationsClient serves NodeOperations the way the API server does: by
// spec.nodeName, which the CRD declares selectable.
func operationsClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).
		WithStatusSubresource(&v1alpha1.NodeOperation{}).
		WithIndex(&v1alpha1.NodeOperation{}, v1alpha1.NodeOperationNodeNameField, func(obj client.Object) []string {
			return []string{obj.(*v1alpha1.NodeOperation).Spec.NodeName}
		}).
		Build()
}

// askingNodeConfig is a NodeConfig whose node waits for permission to apply
// the revision it holds.
func askingNodeConfig(name, uid string, generation int64) *internalv1alpha1.NodeConfig {
	nc := nodeConfigOf(name, uid, generation)
	nc.Status.Conditions = []metav1.Condition{{
		Type:               internalv1alpha1.NodeConfigConditionTypeDisruptionApproved,
		Status:             metav1.ConditionFalse,
		Reason:             internalv1alpha1.NodeConfigReasonApprovalPending,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: generation,
	}}
	return nc
}

func nodeConfigOf(name, uid string, generation int64) *internalv1alpha1.NodeConfig {
	return &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(uid), Generation: generation},
	}
}

func nodeGroupWithNodes(count int32) *v1.NodeGroup {
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Status.Nodes = count
	return ng
}

// Two attempts that ended in the same second are told apart by name, as the Retry condition of
// the nodeoperation controller orders them: both name the same attempt as the newest.
func TestApprovalsNameTheNewestFailureAsRetryDoes(t *testing.T) {
	const uid = "5f5d1c7e-0000-4000-8000-000000000018"
	finished := metav1.NewTime(time.Now().Add(-time.Hour).Truncate(time.Second))
	var objects []client.Object
	for _, name := range []string{"disruption-worker-0-b", "disruption-worker-0-a"} {
		op := approval(name, uid, 4, true, v1alpha1.NodeOperationPhaseFailed)
		op.Status.FinishedAt = ptr.To(finished)
		objects = append(objects, op)
	}
	r := &Reconciler{sources: &sourceReader{Reader: operationsClient(t, objects...)}}

	approvals, err := r.approvalsFor(t.Context(), nodeConfigOf("worker-0", uid, 4))
	require.NoError(t, err)
	require.Equal(t, 2, approvals.failures)
	require.Equal(t, "disruption-worker-0-b", approvals.lastFailed.Name)
}

// A completed attempt is a window the node released, with the revision applied or not. It is
// not counted, and a node that asks again after it gets a new window at once. Failed attempts
// still count, back off from the newest failure and run out.
func TestReconcileDisruptionCountsOnlyFailedAttempts(t *testing.T) {
	const uid = "5f5d1c7e-0000-4000-8000-000000000017"
	type attempt struct {
		phase v1alpha1.NodeOperationPhase
		ago   time.Duration
	}
	completed := func(ago time.Duration) attempt { return attempt{v1alpha1.NodeOperationPhaseCompleted, ago} }
	failed := func(ago time.Duration) attempt { return attempt{v1alpha1.NodeOperationPhaseFailed, ago} }
	tests := []struct {
		name     string
		attempts []attempt
		wantOps  int
		// wantWait bounds the wait before the next attempt, zero when there is none.
		wantWait      time.Duration
		wantExhausted bool
	}{
		{name: "a new window at once after a completed attempt", attempts: []attempt{completed(10 * time.Second)}, wantOps: 1},
		{
			name:     "a completed attempt after two failures neither counts nor waits",
			attempts: []attempt{failed(2 * time.Hour), failed(time.Hour), completed(10 * time.Second)},
			wantOps:  1,
		},
		{
			name:     "a completed attempt does not lengthen the wait after a failure",
			attempts: []attempt{completed(time.Hour), failed(10 * time.Second)},
			wantWait: time.Minute,
		},
		{
			name:          "three failures run out with a completed attempt among them",
			attempts:      []attempt{failed(4 * time.Hour), completed(3 * time.Hour), failed(2 * time.Hour), failed(time.Hour)},
			wantExhausted: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make([]client.Object, 0, len(tt.attempts))
			for i, a := range tt.attempts {
				op := approval(fmt.Sprintf("disruption-worker-0-%d", i), uid, 4, true, a.phase)
				finished := time.Now().Add(-a.ago)
				op.Status.StartedAt = ptr.To(metav1.NewTime(finished.Add(-time.Minute)))
				op.Status.FinishedAt = ptr.To(metav1.NewTime(finished))
				objects = append(objects, op)
			}
			cl := operationsClient(t, objects...)
			recorder := record.NewFakeRecorder(10)
			r := &Reconciler{sources: &sourceReader{Reader: cl}}
			r.Client = cl
			r.Recorder = recorder

			result, err := r.reconcileDisruption(t.Context(), nodeGroupWithNodes(2),
				&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}, askingNodeConfig("worker-0", uid, 4), logr.Discard())
			require.NoError(t, err)

			require.Len(t, operationNames(t, cl), len(tt.attempts)+tt.wantOps)
			require.Equal(t, tt.wantWait > 0, result.RequeueAfter > 0)
			require.LessOrEqual(t, result.RequeueAfter, tt.wantWait)
			var exhausted []string
			for _, event := range recordedEvents(recorder) {
				if strings.Contains(event, disruptionApprovalExhaustedEvent) {
					exhausted = append(exhausted, event)
				}
			}
			if !tt.wantExhausted {
				require.Empty(t, exhausted)
				return
			}
			require.Equal(t, []string{
				"Warning DisruptionApprovalExhausted 3 attempts at generation 4 failed on node worker-0. " +
					"No new NodeOperation is created until the configuration changes or the oldest attempt is deleted, 24 hours after it finished",
				"Warning DisruptionApprovalExhausted 3 attempts at generation 4 failed on node worker-0. " +
					"No new NodeOperation is created until the configuration changes or the oldest attempt is deleted, 24 hours after it finished",
			}, exhausted)
		})
	}
}

// spec.cause is immutable, and the time nodelet appends while the backoff runs is over once
// the attempt it announced exists.
func TestDisruptionCauseLeavesOutTheScheduledAttempt(t *testing.T) {
	nc := &internalv1alpha1.NodeConfig{}
	nc.Status.Conditions = []metav1.Condition{{
		Type:    internalv1alpha1.NodeConfigConditionTypeDisruptionApproved,
		Status:  metav1.ConditionFalse,
		Reason:  "ApprovalPending",
		Message: approvalRequestPrefix + "kernel module(s) example would be loaded into the running kernel; Attempt 2 of 3 is requested at 2026-01-01T08:01:00Z",
	}}
	require.Equal(t, "kernel module(s) example would be loaded into the running kernel", disruptionCause(nc))
}
