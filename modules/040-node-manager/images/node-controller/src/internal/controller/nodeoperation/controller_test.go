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

package nodeoperation

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// The spec of a NodeOperation is CEL-immutable, so Failed is the end of it: an
// operation failed because a cache had not caught up can never be retried, and
// the node it names stays uninterrupted. The node is therefore read past the
// cache, like every other decision-before-write in this controller.
func TestANodeMissingFromTheCacheDoesNotKillTheOperation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "op",
			CreationTimestamp: metav1.Now(),
		},
		Spec: v1alpha1.NodeOperationSpec{
			Type:     v1alpha1.NodeOperationTypeReboot,
			NodeName: "worker",
			// The eviction is skipped so the pass reaches the hand-over without
			// writing to the node: what is under test is the read, not the drain.
			Drain: &v1alpha1.NodeOperationDrainSpec{Skip: true},
		},
	}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}

	// The cache holds the operation but not the node it names — the state
	// between a node joining and the informer catching up with it.
	cached := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(op.DeepCopy()).
		WithStatusSubresource(&v1alpha1.NodeOperation{}).
		Build()
	live := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(op.DeepCopy(), node).
		WithIndex(&v1alpha1.NodeOperation{}, v1alpha1.NodeOperationNodeNameField, func(obj client.Object) []string {
			return []string{obj.(*v1alpha1.NodeOperation).Spec.NodeName}
		}).
		Build()

	r := &Reconciler{apiReader: live}
	r.InjectClient(cached)

	ctx := context.Background()
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: op.Name}})
	require.NoError(t, err)

	fresh := &v1alpha1.NodeOperation{}
	require.NoError(t, cached.Get(ctx, types.NamespacedName{Name: op.Name}, fresh))
	require.NotEqual(t, v1alpha1.NodeOperationPhaseFailed, fresh.Status.Phase,
		"a node the cache has not seen yet must not fail an operation for good")
}

// The draining controller writes its error only onto the node, and it goes with
// the request once the operation gives up: the operation keeps it in its record.
func TestAnOperationTimingOutInItsEvictionKeepsTheEvictionError(t *testing.T) {
	node := groupedNode("worker")
	op := rebootOf(node)
	r, c, _ := phaseRecordingReconciler(t, node, op)
	recorder := record.NewFakeRecorder(10)
	r.InjectRecorder(recorder)

	reconcileOperation(t, r, op.Name)
	annotateNode(t, c, node.Name, nodecommon.DrainFailedAnnotation, evictionError)
	fresh := getOp(t, c, op.Name)
	fresh.Status.DrainDeadline = ptr.To(metav1.NewTime(time.Now().Add(-2 * operationTimeout)))
	require.NoError(t, c.Status().Update(context.Background(), fresh))
	reconcileOperation(t, r, op.Name)

	fresh = getOp(t, c, op.Name)
	require.Equal(t, v1alpha1.NodeOperationPhaseFailed, fresh.Status.Phase)
	require.Equal(t, "PreparationTimedOut", fresh.Status.Reason)
	require.Contains(t, fresh.Status.Message, "node worker was not drained by ")
	require.Contains(t, fresh.Status.Message, ". Last eviction error: "+evictionError)
	require.Equal(t, "Warning PreparationTimedOut "+fresh.Status.Message, <-recorder.Events)
}

// The draining controller writes a failed attempt only onto the node, so the
// watch has to let that change through for the operation to say so.
func TestNodeStateChangedFollowsTheDrainError(t *testing.T) {
	before := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}
	after := before.DeepCopy()
	after.Annotations = map[string]string{nodecommon.DrainFailedAnnotation: "cannot evict pod"}

	require.True(t, nodeStateChanged(before, after))
}

// An operation waiting on a reboot is woken by the node coming back, and not by
// every heartbeat of the kubelet.
func TestNodeStateChangedFollowsTheBoot(t *testing.T) {
	before := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Status: corev1.NodeStatus{
			NodeInfo:   corev1.NodeSystemInfo{BootID: "boot-1"},
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}},
		},
	}
	booted := before.DeepCopy()
	booted.Status.NodeInfo.BootID = "boot-2"
	ready := before.DeepCopy()
	ready.Status.Conditions[0].Status = corev1.ConditionTrue
	heartbeat := before.DeepCopy()
	heartbeat.Status.Conditions[0].LastHeartbeatTime = metav1.Now()

	require.True(t, nodeStateChanged(before, booted))
	require.True(t, nodeStateChanged(before, ready))
	require.False(t, nodeStateChanged(before, heartbeat))
}

// The drain request is one slot shared with bashible and with hand drains; a
// waiting operation names whoever holds it, never a bare marker.
func TestEvictionHolder(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	holder := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "reboot-worker", UID: "holder-uid"},
		Spec:       v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeReboot, NodeName: "worker"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(holder).
		WithIndex(&v1alpha1.NodeOperation{}, v1alpha1.NodeOperationNodeNameField, func(obj client.Object) []string {
			return []string{obj.(*v1alpha1.NodeOperation).Spec.NodeName}
		}).
		Build()
	r := &Reconciler{}
	r.InjectClient(c)

	tests := []struct {
		requested string
		expHolder string
	}{
		{requested: "", expHolder: "a bashible update"},
		{requested: "bashible", expHolder: "a bashible update"},
		{requested: "user", expHolder: "a drain requested by user"},
		{requested: drainingSource + "/holder-uid", expHolder: "NodeOperation reboot-worker"},
		{requested: drainingSource + "/gone-uid", expHolder: "a NodeOperation that no longer exists (UID gone-uid)"},
	}
	for _, tc := range tests {
		t.Run(tc.requested, func(t *testing.T) {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Name: "worker", Annotations: map[string]string{nodecommon.DrainingAnnotation: tc.requested},
			}}
			got, err := r.evictionHolder(context.Background(), node)
			require.NoError(t, err)
			require.Equal(t, tc.expHolder, got)
		})
	}
}

// The phase alone tells an operation that waits for an operator from one that
// was approved and is still evicting: kubectl get shows nothing else.
func TestAnApprovalIsApprovedWhileTheNodeIsPrepared(t *testing.T) {
	node := groupedNode("worker")
	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "disruption-worker", UID: "disruption-uid"},
		Spec: v1alpha1.NodeOperationSpec{
			Type: v1alpha1.NodeOperationTypeDisruption, NodeName: node.Name,
			ConfigGeneration: ptr.To(int64(1)), NodeConfigUID: "config-uid", Approved: ptr.To(false),
		},
	}
	r, c, phases := phaseRecordingReconciler(t, node, op)
	ctx := context.Background()

	reconcileOperation(t, r, op.Name)
	reconcileOperation(t, r, op.Name)
	require.Equal(t, []v1alpha1.NodeOperationPhase{v1alpha1.NodeOperationPhasePending}, phases[op.Name],
		"nobody approved it, so it stays Pending")

	fresh := getOp(t, c, op.Name)
	fresh.Spec.Approved = ptr.To(true)
	require.NoError(t, c.Update(ctx, fresh))
	reconcileOperation(t, r, op.Name)
	require.Equal(t, []v1alpha1.NodeOperationPhase{v1alpha1.NodeOperationPhasePending, v1alpha1.NodeOperationPhaseApproved},
		phases[op.Name], "approved, and evicting the workload")

	finishEviction(t, c, node.Name)
	reconcileOperation(t, r, op.Name)
	require.Equal(t, []v1alpha1.NodeOperationPhase{
		v1alpha1.NodeOperationPhasePending, v1alpha1.NodeOperationPhaseApproved, v1alpha1.NodeOperationPhaseInProgress,
	}, phases[op.Name], "the node receives the operation once the workload has left")
	drained := meta.FindStatusCondition(getOp(t, c, op.Name).Status.Conditions, conditionDrained)
	require.NotNil(t, drained)
	require.Equal(t, metav1.ConditionTrue, drained.Status)
	require.Equal(t, "Drained", drained.Reason)
}

// The request for approval quotes the node's failure, which can be a whole script
// output. The message stays under 4096 bytes, cut inside the quoted failure.
func TestAnApprovalRequestCutsTheFailureItQuotes(t *testing.T) {
	node := groupedNode("worker")
	attempt := func(name string, approved bool) *v1alpha1.NodeOperation {
		return &v1alpha1.NodeOperation{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: v1alpha1.NodeOperationSpec{
				Type: v1alpha1.NodeOperationTypeDisruption, NodeName: node.Name,
				ConfigGeneration: ptr.To(int64(1)), NodeConfigUID: "config-uid", Approved: ptr.To(approved),
			},
		}
	}
	failed := attempt("disruption-worker-a", true)
	failed.Status = v1alpha1.NodeOperationStatus{Phase: v1alpha1.NodeOperationPhaseFailed, Reason: "NodeFailed",
		Message: strings.Repeat("\u043c\u043e\u0434\u0443\u043b\u044c \u043d\u0435 \u0437\u0430\u0433\u0440\u0443\u0436\u0435\u043d ", 1000)}
	retry := attempt("disruption-worker-b", false)
	r, c, _ := phaseRecordingReconciler(t, node, failed, retry)

	reconcileOperation(t, r, retry.Name)

	message := getOp(t, c, retry.Name).Status.Message
	require.LessOrEqual(t, len(message), 4096)
	require.True(t, utf8.ValidString(message))
	require.True(t, strings.HasPrefix(message, "Attempt 2 of 3. The previous attempt disruption-worker-a failed: \u043c\u043e\u0434\u0443\u043b\u044c \u043d\u0435 \u0437\u0430\u0433\u0440\u0443\u0436\u0435\u043d"))
	require.True(t, strings.HasSuffix(message, "… (truncated). Waiting for an operator to set spec.approved to true"))
}

// The console reads the cause off the operation it asks to approve. A retry keeps it
// at the end, and a cause of multibyte characters does not break the cut.
func TestAnApprovalRequestEndsWithTheCause(t *testing.T) {
	const cause = "kernel module(s) example would be loaded into the running kernel"
	node := groupedNode("worker")
	attempt := func(name string, approved bool, cause string) *v1alpha1.NodeOperation {
		return &v1alpha1.NodeOperation{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: v1alpha1.NodeOperationSpec{
				Type: v1alpha1.NodeOperationTypeDisruption, NodeName: node.Name, Cause: cause,
				ConfigGeneration: ptr.To(int64(1)), NodeConfigUID: "config-uid", Approved: ptr.To(approved),
			},
		}
	}

	first := attempt("disruption-worker-a", false, cause)
	r, c, _ := phaseRecordingReconciler(t, node, first)
	reconcileOperation(t, r, first.Name)
	require.Equal(t, "Waiting for an operator to set spec.approved to true. Cause: "+cause, getOp(t, c, first.Name).Status.Message)

	failed := attempt("disruption-worker-b", true, cause)
	failed.Status = v1alpha1.NodeOperationStatus{Phase: v1alpha1.NodeOperationPhaseFailed, Reason: "NodeFailed",
		Message: strings.Repeat("\u043c\u043e\u0434\u0443\u043b\u044c \u043d\u0435 \u0437\u0430\u0433\u0440\u0443\u0436\u0435\u043d ", 1000)}
	// Four bytes a character: the CRD lets this cause take the whole budget.
	retry := attempt("disruption-worker-c", false, strings.Repeat("𝔞", 1024))
	r, c, _ = phaseRecordingReconciler(t, groupedNode("worker"), failed, retry)
	reconcileOperation(t, r, retry.Name)

	message := getOp(t, c, retry.Name).Status.Message
	require.True(t, utf8.ValidString(message))
	require.True(t, strings.HasPrefix(message, "Attempt 2 of 3. The previous attempt disruption-worker-b failed: … (truncated)"))
	require.True(t, strings.HasSuffix(message, ". Waiting for an operator to set spec.approved to true. Cause: "+retry.Spec.Cause))
}

// An approved operation evicts the workload itself. A second operation next to
// it in kubectl get would look like something else waiting for the operator.
func TestAnApprovedOperationCreatesNoOtherOperation(t *testing.T) {
	node := groupedNode("worker")
	op := rebootOf(node)
	r, c, _ := phaseRecordingReconciler(t, node, op)

	reconcileOperation(t, r, op.Name)
	reconcileOperation(t, r, op.Name)

	require.Equal(t, []string{op.Name}, operationNamesOn(t, c, node.Name))
}

// The request carries the operation's own marker, so the answer comes back to
// it. The deadline is pinned at the first ask and not moved by asking again.
func TestAnApprovedOperationAsksForTheEvictionItself(t *testing.T) {
	node := groupedNode("worker")
	op := rebootOf(node)
	r, c, _ := phaseRecordingReconciler(t, node, op)

	reconcileOperation(t, r, op.Name)

	require.Equal(t, drainingSource+"/reboot-uid", getNodeOf(t, c, node.Name).Annotations[nodecommon.DrainingAnnotation])
	fresh := getOp(t, c, op.Name)
	require.Equal(t, v1alpha1.NodeOperationPhaseApproved, fresh.Status.Phase)
	require.Equal(t, "Draining", fresh.Status.Reason)
	require.Equal(t, "The workload is being evicted from node worker", fresh.Status.Message)
	drained := meta.FindStatusCondition(fresh.Status.Conditions, conditionDrained)
	require.NotNil(t, drained)
	require.Equal(t, metav1.ConditionFalse, drained.Status)
	require.Equal(t, "Draining", drained.Reason)
	require.NotNil(t, fresh.Status.DrainDeadline)
	pinned := *fresh.Status.DrainDeadline

	removeNodeAnnotation(t, c, node.Name, nodecommon.DrainingAnnotation)
	reconcileOperation(t, r, op.Name)
	require.Equal(t, drainingSource+"/reboot-uid", getNodeOf(t, c, node.Name).Annotations[nodecommon.DrainingAnnotation],
		"a request removed by something else is asked for again")
	require.True(t, pinned.Equal(getOp(t, c, op.Name).Status.DrainDeadline), "asking again does not move the deadline")
}

// The draining controller writes a failed attempt only onto the node; the
// operation is where the operator reads it.
func TestTheEvictionErrorReachesTheOperation(t *testing.T) {
	node := groupedNode("worker")
	op := rebootOf(node)
	r, c, _ := phaseRecordingReconciler(t, node, op)

	reconcileOperation(t, r, op.Name)
	annotateNode(t, c, node.Name, nodecommon.DrainFailedAnnotation, evictionError)
	reconcileOperation(t, r, op.Name)

	fresh := getOp(t, c, op.Name)
	require.Equal(t, v1alpha1.NodeOperationPhaseApproved, fresh.Status.Phase)
	require.Equal(t, "Draining", fresh.Status.Reason)
	require.Equal(t, "the eviction of node worker is failing: "+evictionError, fresh.Status.Message)
}

// The node has the work once the workload is gone. A drained marker removed
// afterwards, by a bashible drain for one, must not send it back to evicting.
func TestAnOperationAtWorkDoesNotEvictAgain(t *testing.T) {
	node := groupedNode("worker")
	op := rebootOf(node)
	r, c, _ := phaseRecordingReconciler(t, node, op)

	reconcileOperation(t, r, op.Name)
	finishEviction(t, c, node.Name)
	reconcileOperation(t, r, op.Name)
	require.Equal(t, v1alpha1.NodeOperationPhaseInProgress, getOp(t, c, op.Name).Status.Phase)

	removeNodeAnnotation(t, c, node.Name, nodecommon.DrainedAnnotation)
	reconcileOperation(t, r, op.Name)

	require.Equal(t, v1alpha1.NodeOperationPhaseInProgress, getOp(t, c, op.Name).Status.Phase)
	require.NotContains(t, getNodeOf(t, c, node.Name).Annotations, nodecommon.DrainingAnnotation)
}

// The drain request is one slot. An operation that finds it taken waits, and
// says for whom, instead of taking it.
func TestAnOperationWaitsForTheEvictionAnotherOneHolds(t *testing.T) {
	node := groupedNode("worker")
	node.Annotations = map[string]string{nodecommon.DrainingAnnotation: drainingSource + "/drain-uid"}
	holder := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "drain-worker", UID: "drain-uid"},
		Spec:       v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeDrain, NodeName: node.Name},
		Status:     v1alpha1.NodeOperationStatus{Phase: v1alpha1.NodeOperationPhaseInProgress, Reason: "Draining"},
	}
	op := rebootOf(node)
	r, c, _ := phaseRecordingReconciler(t, node, holder, op)

	reconcileOperation(t, r, op.Name)

	fresh := getOp(t, c, op.Name)
	require.Equal(t, v1alpha1.NodeOperationPhaseApproved, fresh.Status.Phase)
	require.Equal(t, "WaitingForOperation", fresh.Status.Reason)
	require.Equal(t, "the workload of node worker is already being evicted for NodeOperation drain-worker. "+
		"Waiting for that eviction to finish", fresh.Status.Message)
	require.Equal(t, drainingSource+"/drain-uid", getNodeOf(t, c, node.Name).Annotations[nodecommon.DrainingAnnotation])
}

// An operation the previous controller started has a child Drain asking under
// the parent's marker. Nothing new is created for it: the parent reads the
// answer itself, and the child completes on its own without asking again.
func TestAnEvictionLeftByThePreviousControllerFinishesBoth(t *testing.T) {
	marker := drainingSource + "/reboot-uid"
	node := groupedNode("worker")
	node.Annotations = map[string]string{nodecommon.DrainingAnnotation: marker}
	deadline := metav1.NewTime(time.Now().Add(time.Hour))
	draining := []metav1.Condition{{Type: conditionDrained, Status: metav1.ConditionFalse, Reason: "Draining",
		Message: "evicting", LastTransitionTime: metav1.Now()}}
	parent := rebootOf(node)
	parent.Status = v1alpha1.NodeOperationStatus{
		Phase: v1alpha1.NodeOperationPhaseApproved, Reason: "Draining",
		NodeWasUnschedulable: ptr.To(false), DrainDeadline: &deadline, Conditions: draining,
	}
	child := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "reboot-worker-drain-x", UID: "child-uid",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1alpha1.GroupVersion.String(), Kind: nodeOperationKind,
				Name: parent.Name, UID: parent.UID, Controller: ptr.To(true),
			}},
		},
		Spec: v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeDrain, NodeName: node.Name},
		Status: v1alpha1.NodeOperationStatus{
			Phase: v1alpha1.NodeOperationPhaseInProgress, Reason: "Draining", StartedAt: ptr.To(metav1.Now()),
			NodeWasUnschedulable: ptr.To(false), DrainDeadline: &deadline, Conditions: draining,
		},
	}
	r, c, _ := phaseRecordingReconciler(t, node, parent, child)

	reconcileOperation(t, r, parent.Name)
	require.ElementsMatch(t, []string{parent.Name, child.Name}, operationNamesOn(t, c, node.Name))

	finishEviction(t, c, node.Name)
	reconcileOperation(t, r, parent.Name)
	require.Equal(t, v1alpha1.NodeOperationPhaseInProgress, getOp(t, c, parent.Name).Status.Phase,
		"the child's answer carries the parent's marker")

	reconcileOperation(t, r, child.Name)
	require.Equal(t, v1alpha1.NodeOperationPhaseCompleted, getOp(t, c, child.Name).Status.Phase)
	annotations := getNodeOf(t, c, node.Name).Annotations
	require.NotContains(t, annotations, nodecommon.DrainingAnnotation, "the child does not ask for a second eviction")
	require.Equal(t, marker, annotations[nodecommon.DrainedAnnotation])
}

// A finished operation gives the node back only when no other one needs it out
// of the scheduler. One still evicting does: pods must not return onto it.
func TestAReleaseKeepsTheNodeOutWhileAnotherOperationEvicts(t *testing.T) {
	node := groupedNode("worker")
	first := rebootOf(node)
	second := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "second-reboot", UID: "second-uid"},
		Spec:       v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeReboot, NodeName: node.Name},
	}
	r, c, _ := phaseRecordingReconciler(t, node, first, second)
	ctx := context.Background()

	reconcileOperation(t, r, first.Name)
	finishEviction(t, c, node.Name)
	reconcileOperation(t, r, first.Name)
	require.Equal(t, v1alpha1.NodeOperationPhaseInProgress, getOp(t, c, first.Name).Status.Phase)

	reconcileOperation(t, r, second.Name)
	require.Equal(t, v1alpha1.NodeOperationPhaseApproved, getOp(t, c, second.Name).Status.Phase)

	fresh := getOp(t, c, first.Name)
	meta.SetStatusCondition(&fresh.Status.Conditions, metav1.Condition{
		Type: conditionNodeReported, Status: metav1.ConditionTrue, Reason: "Succeeded", Message: "done"})
	require.NoError(t, c.Status().Update(ctx, fresh))
	reconcileOperation(t, r, first.Name)
	reconcileOperation(t, r, first.Name)
	require.Equal(t, v1alpha1.NodeOperationPhaseCompleted, getOp(t, c, first.Name).Status.Phase)

	released := getNodeOf(t, c, node.Name)
	require.NotContains(t, released.Annotations, nodecommon.DrainedAnnotation, "the finished operation took its marker back")
	require.True(t, released.Spec.Unschedulable, "the other operation is still evicting")
}

// Approved is the preparation of the node. An operation that prepares nothing
// goes straight from Pending to InProgress, and a Drain is the preparation itself.
func TestAnOperationWithNothingToPrepareIsNeverApproved(t *testing.T) {
	t.Run("a Reboot that skips the eviction", func(t *testing.T) {
		node := groupedNode("worker")
		op := &v1alpha1.NodeOperation{
			ObjectMeta: metav1.ObjectMeta{Name: "reboot-worker", UID: "reboot-uid"},
			Spec: v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeReboot, NodeName: node.Name,
				Drain: &v1alpha1.NodeOperationDrainSpec{Skip: true}},
		}
		r, _, phases := phaseRecordingReconciler(t, node, op)

		reconcileOperation(t, r, op.Name)
		require.Equal(t, []v1alpha1.NodeOperationPhase{v1alpha1.NodeOperationPhasePending, v1alpha1.NodeOperationPhaseInProgress},
			phases[op.Name])
	})

	t.Run("a Drain created by an operator", func(t *testing.T) {
		node := groupedNode("worker")
		op := &v1alpha1.NodeOperation{
			ObjectMeta: metav1.ObjectMeta{Name: "drain-worker", UID: "drain-uid"},
			Spec:       v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeDrain, NodeName: node.Name},
		}
		r, c, phases := phaseRecordingReconciler(t, node, op)

		reconcileOperation(t, r, op.Name)
		finishEviction(t, c, node.Name)
		reconcileOperation(t, r, op.Name)
		require.Equal(t, []v1alpha1.NodeOperationPhase{
			v1alpha1.NodeOperationPhasePending, v1alpha1.NodeOperationPhaseInProgress, v1alpha1.NodeOperationPhaseCompleted,
		}, phases[op.Name])

		// Retired, it keeps what it asked for: the node out of the scheduler.
		reconcileOperation(t, r, op.Name)
		require.Equal(t, []string{op.Name}, operationNamesOn(t, c, node.Name))
		require.True(t, getNodeOf(t, c, node.Name).Spec.Unschedulable)
	})
}

// The preparation deadline bounds the Approved stretch: an eviction that never
// finishes ends the operation as a preparation that ran out of time, not as a
// node that did not answer.
func TestAnApprovedOperationRunsOutOfPreparationTime(t *testing.T) {
	node := groupedNode("worker")
	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "reboot-worker", UID: "reboot-uid"},
		Spec:       v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeReboot, NodeName: node.Name},
	}
	r, c, phases := phaseRecordingReconciler(t, node, op)
	ctx := context.Background()

	reconcileOperation(t, r, op.Name)
	require.Equal(t, []v1alpha1.NodeOperationPhase{v1alpha1.NodeOperationPhasePending, v1alpha1.NodeOperationPhaseApproved},
		phases[op.Name])

	fresh := &v1alpha1.NodeOperation{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: op.Name}, fresh))
	fresh.Status.DrainDeadline = ptr.To(metav1.NewTime(time.Now().Add(-2 * operationTimeout)))
	require.NoError(t, c.Status().Update(ctx, fresh))
	reconcileOperation(t, r, op.Name)

	require.Equal(t, []v1alpha1.NodeOperationPhase{
		v1alpha1.NodeOperationPhasePending, v1alpha1.NodeOperationPhaseApproved, v1alpha1.NodeOperationPhaseFailed,
	}, phases[op.Name])
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: op.Name}, fresh))
	require.Equal(t, "PreparationTimedOut", fresh.Status.Reason)
}

// An ApproveDisruption written before the type was renamed fails the CRD enum
// on every write, so the controller leaves it and its node untouched.
func TestAnOperationOfAnUnknownTypeIsLeftAlone(t *testing.T) {
	node := groupedNode("worker")
	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "approve-worker-abcde", UID: "old-uid"},
		Spec: v1alpha1.NodeOperationSpec{
			Type: "ApproveDisruption", NodeName: node.Name,
			ConfigGeneration: ptr.To(int64(1)), NodeConfigUID: "config-uid", Approved: ptr.To(true),
		},
	}
	r, c, phases := phaseRecordingReconciler(t, node, op)
	opBefore := getOp(t, c, op.Name)
	nodeBefore := getNodeOf(t, c, node.Name)

	reconcileOperation(t, r, op.Name)

	require.Empty(t, phases[op.Name], "no phase is written")
	require.Equal(t, opBefore.ResourceVersion, getOp(t, c, op.Name).ResourceVersion, "the operation is not written at all")
	require.Equal(t, nodeBefore.ResourceVersion, getNodeOf(t, c, node.Name).ResourceVersion, "the node is neither cordoned nor drained")
}

// phaseRecordingReconciler runs the controller over a fake cluster and records
// every phase each operation is moved to, in order. A read after the pass
// misses a phase that the same pass wrote and then replaced.
func phaseRecordingReconciler(t *testing.T, objects ...client.Object) (*Reconciler, client.Client, map[string][]v1alpha1.NodeOperationPhase) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	// Retry of a failed attempt reads off the node's NodeConfig whether the node still asks.
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))

	phases := map[string][]v1alpha1.NodeOperationPhase{}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&v1alpha1.NodeOperation{}).
		WithIndex(&v1alpha1.NodeOperation{}, v1alpha1.NodeOperationNodeNameField, func(obj client.Object) []string {
			return []string{obj.(*v1alpha1.NodeOperation).Spec.NodeName}
		}).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourcePatch: func(ctx context.Context, c client.Client, subResource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
				if err := c.SubResource(subResource).Patch(ctx, obj, patch, opts...); err != nil {
					return err
				}
				op, ok := obj.(*v1alpha1.NodeOperation)
				if !ok || op.Status.Phase == "" {
					return nil
				}
				seen := phases[op.Name]
				if len(seen) == 0 || seen[len(seen)-1] != op.Status.Phase {
					phases[op.Name] = append(seen, op.Status.Phase)
				}
				return nil
			},
		}).
		Build()
	r := &Reconciler{apiReader: c}
	r.InjectClient(c)
	r.InjectRecorder(record.NewFakeRecorder(100))
	return r, c, phases
}

func reconcileOperation(t *testing.T, r *Reconciler, name string) {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	require.NoError(t, err)
}

func rebootOf(node *corev1.Node) *v1alpha1.NodeOperation {
	return &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: "reboot-" + node.Name, UID: "reboot-uid"},
		Spec:       v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeReboot, NodeName: node.Name},
	}
}

func getOp(t *testing.T, c client.Client, name string) *v1alpha1.NodeOperation {
	t.Helper()
	op := &v1alpha1.NodeOperation{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: name}, op))
	return op
}

func getNodeOf(t *testing.T, c client.Client, name string) *corev1.Node {
	t.Helper()
	node := &corev1.Node{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: name}, node))
	return node
}

// operationNamesOn names every operation recorded against the node.
func operationNamesOn(t *testing.T, c client.Client, nodeName string) []string {
	t.Helper()
	list := &v1alpha1.NodeOperationList{}
	require.NoError(t, c.List(context.Background(), list, client.MatchingFields{v1alpha1.NodeOperationNodeNameField: nodeName}))
	var names []string
	for i := range list.Items {
		names = append(names, list.Items[i].Name)
	}
	return names
}

func annotateNode(t *testing.T, c client.Client, nodeName, key, value string) {
	t.Helper()
	node := getNodeOf(t, c, nodeName)
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
	node.Annotations[key] = value
	require.NoError(t, c.Update(context.Background(), node))
}

func removeNodeAnnotation(t *testing.T, c client.Client, nodeName, key string) {
	t.Helper()
	node := getNodeOf(t, c, nodeName)
	delete(node.Annotations, key)
	require.NoError(t, c.Update(context.Background(), node))
}

func groupedNode(name string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name: name, UID: "node-uid", Labels: map[string]string{nodecommon.NodeGroupLabel: "worker"},
	}}
}

// finishEviction plays the draining controller: it answers the request on the
// node with the same marker.
func finishEviction(t *testing.T, c client.Client, nodeName string) {
	t.Helper()
	ctx := context.Background()
	node := &corev1.Node{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: nodeName}, node))
	requested := node.Annotations[nodecommon.DrainingAnnotation]
	require.NotEmpty(t, requested, "the draining controller only answers a request")
	delete(node.Annotations, nodecommon.DrainingAnnotation)
	node.Annotations[nodecommon.DrainedAnnotation] = requested
	node.Spec.Unschedulable = true
	require.NoError(t, c.Update(ctx, node))
}
