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

// Package nodeoperation carries a node-interrupting operation (reboot,
// eviction, permission to apply a disruptive config) from recorded intent to
// result: the controller evicts and hands over via InProgress, the node reports.
package nodeoperation

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController(controllerName, &v1alpha1.NodeOperation{}, &Reconciler{})
}

type Reconciler struct {
	register.Base

	// apiReader reads past the manager's cache, for every read a write is
	// decided from: a cache that has not caught up misses a node or an
	// operation that is there.
	apiReader client.Reader
}

func (r *Reconciler) Setup(ctx context.Context, mgr ctrl.Manager) error {
	r.apiReader = mgr.GetAPIReader()
	// The Node watch lists a node's operations from the cache by this index;
	// reads past the cache use the same field, selectable on the CRD.
	err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.NodeOperation{}, v1alpha1.NodeOperationNodeNameField,
		func(obj client.Object) []string {
			return []string{obj.(*v1alpha1.NodeOperation).Spec.NodeName}
		})
	if err != nil {
		return fmt.Errorf("index NodeOperations by %s: %w", v1alpha1.NodeOperationNodeNameField, err)
	}
	return nil
}

// SetupWatches follows what an operation waits on: the node it is draining or
// waiting to come back from a reboot. A failed Disruption also follows what its
// Retry is read from: the other attempts on the node and the node's NodeConfig.
func (r *Reconciler) SetupWatches(w register.Watcher) {
	// Predicated, and it has to be: the mapper lists the operations of the node,
	// and without a filter every kubelet heartbeat of every node would run that
	// list.
	w.Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		ops := &v1alpha1.NodeOperationList{}
		if err := r.Client.List(ctx, ops, client.MatchingFields{v1alpha1.NodeOperationNodeNameField: obj.GetName()}); err != nil {
			log.FromContext(ctx).Error(err, "list the operations of a node that changed", "node", obj.GetName())
			return nil
		}
		var requests []reconcile.Request
		for i := range ops.Items {
			// A finished operation waits out its retention on its own requeue, not on this.
			if !terminal(&ops.Items[i]) {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: ops.Items[i].Name}})
			}
		}
		return requests
	}), builder.WithPredicates(predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return nodeStateChanged(e.ObjectOld, e.ObjectNew)
		},
	}))

	// An attempt created or collected moves the Retry of the others. An update does
	// not: an attempt that ends gets its own Retry, and the others name it already.
	w.Watches(&v1alpha1.NodeOperation{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		op, ok := obj.(*v1alpha1.NodeOperation)
		if !ok || op.Spec.Type != v1alpha1.NodeOperationTypeDisruption {
			return nil
		}
		return r.failedDisruptionsOn(ctx, op.Spec.NodeName)
	}), builder.WithPredicates(predicate.Funcs{
		UpdateFunc:  func(event.UpdateEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}))

	// Named after its node, and predicated: the agent republishes its status far more
	// often than it changes what it asks for.
	w.Watches(&internalv1alpha1.NodeConfig{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		return r.failedDisruptionsOn(ctx, obj.GetName())
	}), builder.WithPredicates(predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return retryInputChanged(e.ObjectOld, e.ObjectNew)
		},
	}))
}

// nodeStateChanged reports whether an update touched what an operation reads off its node: the
// two drain markers, the draining controller's error, whether the node is out of the scheduler,
// its bootID and whether it is Ready.
func nodeStateChanged(before, after client.Object) bool {
	oldNode, okOld := before.(*corev1.Node)
	newNode, okNew := after.(*corev1.Node)
	if !okOld || !okNew {
		return true
	}
	if oldNode.Spec.Unschedulable != newNode.Spec.Unschedulable {
		return true
	}
	if oldNode.Status.NodeInfo.BootID != newNode.Status.NodeInfo.BootID || nodecommon.NodeReady(oldNode) != nodecommon.NodeReady(newNode) {
		return true
	}
	for _, key := range []string{nodecommon.DrainingAnnotation, nodecommon.DrainedAnnotation, nodecommon.DrainFailedAnnotation} {
		if oldNode.Annotations[key] != newNode.Annotations[key] {
			return true
		}
	}
	return false
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	op := &v1alpha1.NodeOperation{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name}, op); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// An ApproveDisruption written before the type was renamed: the CRD enum no
	// longer has it, so the API server would refuse every write to it. It waits
	// for an operator to delete it.
	if !knownType(op.Spec.Type) {
		logger.V(1).Info("skipped an operation of an unknown type", "operation", op.Name, "type", op.Spec.Type, "node", op.Spec.NodeName)
		return ctrl.Result{}, nil
	}

	if terminal(op) {
		return r.retire(ctx, op, logger)
	}

	// Past the cache: the spec is immutable, so failing an operation is final,
	// and a node the informer has not caught up with would end it for good.
	node := &corev1.Node{}
	if err := r.apiReader.Get(ctx, types.NamespacedName{Name: op.Spec.NodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.fail(ctx, op, "NodeNotFound",
				fmt.Sprintf("node %s does not exist", op.Spec.NodeName), logger)
		}
		return ctrl.Result{}, err
	}

	if err := r.adopt(ctx, op, node); err != nil {
		return ctrl.Result{}, err
	}

	// Nothing touches the node before the approval: the cordon is recorded and
	// the clock started only once the operation may go ahead.
	if waitingForApproval(op) {
		message, err := r.approvalRequest(ctx, op)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, r.setStatus(ctx, op, v1alpha1.NodeOperationPhasePending, "WaitingForApproval", message, logger)
	}

	if err := r.begin(ctx, op, node, logger); err != nil {
		return ctrl.Result{}, err
	}

	return r.carryOut(ctx, op, node, logger)
}

// retire is what becomes of a finished operation: history, kept for the record
// until old enough to collect. The node goes back to the scheduler — except
// after a Drain that got what it asked for, which was to keep it out. A
// failed Disruption also says whether another attempt follows.
func (r *Reconciler) retire(ctx context.Context, op *v1alpha1.NodeOperation, logger logr.Logger) (ctrl.Result, error) {
	// A Drain that failed gives its marker back like any other operation, or
	// the draining controller evicts for one that has given up.
	keptOut := op.Spec.Type == v1alpha1.NodeOperationTypeDrain && op.Status.Phase == v1alpha1.NodeOperationPhaseCompleted
	if !keptOut {
		if err := r.releaseNode(ctx, op, logger); err != nil {
			return ctrl.Result{}, err
		}
	}
	result, err := r.collect(ctx, op, logger)
	// No requeue means it was collected.
	if err != nil || result.RequeueAfter == 0 || op.Spec.Type != v1alpha1.NodeOperationTypeDisruption {
		return result, err
	}
	requestedAt, err := r.recordRetry(ctx, op)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Nothing else may happen when the wait runs out, so the Retry is read again then.
	if wait := time.Until(requestedAt); wait > 0 {
		result.RequeueAfter = min(result.RequeueAfter, wait)
	}
	return result, nil
}

// carryOut moves a live operation one step towards its end: empty the node,
// hand it to the node, wait for the report — each stretch bounded.
func (r *Reconciler) carryOut(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node, logger logr.Logger) (ctrl.Result, error) {
	// The report is what the deadline waits for, so it wins over a deadline
	// that fell due in the meantime.
	if phase, reason, report := reportedOutcome(op); report != nil {
		if reason == reasonRebooting {
			return r.awaitReboot(ctx, op, node, logger)
		}
		message := report.Message
		// The operation names the node, the NodeConfig keeps the cause.
		if reason == reasonReleased {
			message = fmt.Sprintf("Node %s is released without generation %d applied. Its NodeConfig says why",
				op.Spec.NodeName, ptr.Deref(op.Spec.ConfigGeneration, 0))
		}
		return ctrl.Result{}, r.setStatus(ctx, op, phase, reason, message, logger)
	}

	// Every stretch of the operation is bounded, not only the node's: an
	// eviction that never finishes would otherwise hold the operation Approved
	// forever, and with it the node out of the scheduler.
	if deadline := hardDeadline(op); time.Now().After(deadline) {
		return ctrl.Result{}, r.expire(ctx, op, node, deadline, logger)
	}

	// A Drain is the eviction itself: it asks the draining controller to empty
	// the node and is done once the workload is gone. The node stays
	// unschedulable until someone says otherwise.
	if op.Spec.Type == v1alpha1.NodeOperationTypeDrain {
		return r.reconcileDrain(ctx, op, node, logger)
	}

	// Every other operation interrupts the node, so the workload leaves first.
	// The operation evicts it itself and stays Approved meanwhile: the node acts
	// on InProgress, and its workload is still on it.
	drainedCondition := metav1.Condition{Type: conditionDrained, Status: metav1.ConditionTrue,
		Reason: "DrainSkipped", Message: "The operation interrupts the node without evicting its workload"}
	if !skipDrain(op) {
		// Only until the hand-over: a drained marker that something clears
		// later must not send a node at work back to an eviction.
		if awaitingEviction(op) {
			drained, err := r.evict(ctx, op, node, v1alpha1.NodeOperationPhaseApproved, logger)
			if err != nil {
				return ctrl.Result{}, err
			}
			if !drained {
				return ctrl.Result{RequeueAfter: waitPollInterval}, nil
			}
		}
		drainedCondition.Reason = "Drained"
		drainedCondition.Message = "The workload is evicted from the node"
	}

	// Handing the operation to the node: from here the node carries it out and
	// reports back through its NodeReported condition.
	if op.Status.Phase != v1alpha1.NodeOperationPhaseInProgress {
		// The deadline this write publishes: the wait for the node starts with it.
		// ponytail: setStatus stamps startedAt a moment later, so across a second
		// boundary the message is a second early; pass the instant in if it matters.
		message := handedOverMessage(op, time.Now().Add(operationTimeout))
		return ctrl.Result{RequeueAfter: operationTimeout}, r.setStatus(ctx, op, v1alpha1.NodeOperationPhaseInProgress, "WaitingForNode",
			message, logger, drainedCondition)
	}

	// Requeue when the deadline is actually due, not a whole period later.
	// Floored: the deadline can pass while this pass runs, and a RequeueAfter
	// of zero is "never" rather than "at once".
	return ctrl.Result{RequeueAfter: max(time.Until(hardDeadline(op)), minRequeue)}, nil
}

// begin queues an operation nobody has started yet, once. An approval waited
// on is Pending already, and is begun when it is approved.
func (r *Reconciler) begin(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node, logger logr.Logger) error {
	// Recorded before anything touches the node, so releasing it later gives
	// back the state the operator left it in rather than always making it
	// schedulable.
	if err := r.rememberCordon(ctx, op, node); err != nil {
		return err
	}
	if op.Status.Phase != "" {
		return nil
	}
	return r.setStatus(ctx, op, v1alpha1.NodeOperationPhasePending, "Queued",
		"The operation is queued", logger)
}

// reconcileDrain carries a Drain operation. The eviction is its work, so it is
// InProgress from the first pass until the workload has left.
func (r *Reconciler) reconcileDrain(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node, logger logr.Logger) (ctrl.Result, error) {
	drained, err := r.evict(ctx, op, node, v1alpha1.NodeOperationPhaseInProgress, logger)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !drained {
		return ctrl.Result{RequeueAfter: waitPollInterval}, nil
	}
	// A Drain the previous controller created for another operation goes back
	// to the scheduler with that operation, so it must not say the node stays out.
	message := "The workload is evicted from the node. The node stays unschedulable"
	if lineageUID(op) != op.UID {
		message = "The workload is evicted from the node"
	}
	return ctrl.Result{}, r.setStatus(ctx, op, v1alpha1.NodeOperationPhaseCompleted, "Drained", message, logger,
		metav1.Condition{Type: conditionDrained, Status: metav1.ConditionTrue, Reason: "Drained", Message: "The workload is evicted from the node"})
}

// evict asks the draining controller to empty the node, then waits for the
// answer it writes onto the node, and reports whether the workload has left.
// Until then the operation says how the eviction goes, in the given phase.
func (r *Reconciler) evict(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node, phase v1alpha1.NodeOperationPhase, logger logr.Logger) (bool, error) {
	// The draining controller watches only nodes labelled with their group, so
	// a request written here would sit unread until someone labels the node —
	// and then evict for an operation that timed out long ago.
	if _, inGroup := node.Labels[nodecommon.NodeGroupLabel]; !inGroup {
		return false, r.fail(ctx, op, "NodeGroupMissing",
			fmt.Sprintf("node %s has no %s label, so its workload cannot be evicted", node.Name, nodecommon.NodeGroupLabel), logger)
	}

	// The request is re-issued whenever the node carries no drain of this
	// operation's: markers removed by anything else would otherwise be
	// waited on forever. Asking again does not move the pinned deadline.
	if !drainRequested(op) || idle(op, node) {
		// The request is a single slot: taking it from whoever holds it makes
		// both operations keep taking it back. Waiting is bounded by this
		// operation's own deadline.
		if requestedByAnother(op, node) {
			holder, err := r.evictionHolder(ctx, node)
			if err != nil {
				return false, err
			}
			return false, r.setStatus(ctx, op, phase, "WaitingForOperation",
				fmt.Sprintf("the workload of node %s is already being evicted for %s. Waiting for that eviction to finish", node.Name, holder), logger)
		}
		if err := r.startDrain(ctx, op, node, logger); err != nil {
			return false, err
		}
		if err := r.recordDrainRequested(ctx, op, node); err != nil {
			return false, err
		}
	} else if drained(op, node) {
		return true, nil
	}

	message := fmt.Sprintf("The workload is being evicted from node %s", node.Name)
	if failure := evictionFailure(op, node); failure != "" {
		message = fmt.Sprintf("the eviction of node %s is failing: %s", node.Name, failure)
	}
	// The Node event carrying the answer is the normal wake-up; the caller's
	// requeue covers a dropped one, which would otherwise strand the operation
	// until the next resync.
	return false, r.setStatus(ctx, op, phase, "Draining", message, logger)
}

// waitingForApproval reports a Disruption nobody has approved yet. The other
// types are approved by being created.
func waitingForApproval(op *v1alpha1.NodeOperation) bool {
	return op.Spec.Type == v1alpha1.NodeOperationTypeDisruption && !ptr.Deref(op.Spec.Approved, false)
}

func knownType(t v1alpha1.NodeOperationType) bool {
	switch t {
	case v1alpha1.NodeOperationTypeReboot, v1alpha1.NodeOperationTypeDrain, v1alpha1.NodeOperationTypeDisruption:
		return true
	default:
		return false
	}
}

// reportedOutcome returns the phase and reason the node's report ends the
// operation with, and the report. Only an operation handed to the node is
// reported on, and a reason this controller does not know is left to the deadline.
func reportedOutcome(op *v1alpha1.NodeOperation) (v1alpha1.NodeOperationPhase, string, *metav1.Condition) {
	if op.Spec.Type == v1alpha1.NodeOperationTypeDrain || op.Status.Phase != v1alpha1.NodeOperationPhaseInProgress {
		return "", "", nil
	}
	report := meta.FindStatusCondition(op.Status.Conditions, conditionNodeReported)
	if report == nil || report.Status != metav1.ConditionTrue {
		return "", "", nil
	}
	switch report.Reason {
	case "Succeeded":
		return v1alpha1.NodeOperationPhaseCompleted, report.Reason, report
	case reasonReleased:
		// The node gave up on the revision, and the interruption itself went
		// through. Only a Disruption carries a revision to give up on.
		if op.Spec.Type != v1alpha1.NodeOperationTypeDisruption {
			return "", "", nil
		}
		return v1alpha1.NodeOperationPhaseCompleted, reasonReleased, report
	case "RebootStarted":
		// The node is not back to say more, so awaitReboot reads the rest off the Node.
		return v1alpha1.NodeOperationPhaseInProgress, reasonRebooting, report
	case "Failed", "Refused":
		// Prefixed: a bare "Failed" next to the phase of the same name says nothing.
		return v1alpha1.NodeOperationPhaseFailed, "Node" + report.Reason, report
	default:
		return "", "", nil
	}
}

// awaitReboot holds a node that reported its reboot until it is back: a new
// bootID and Ready. The node stays cordoned meanwhile, and the deadline of the
// hand-over still bounds the wait.
func (r *Reconciler) awaitReboot(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node, logger logr.Logger) (ctrl.Result, error) {
	// Being back is what the deadline waits for, so it wins over a deadline
	// that fell due in the meantime. The bootID is recorded with the reason.
	rebooting := op.Status.Reason == reasonRebooting
	if rebooting && node.Status.NodeInfo.BootID != op.Status.BootID && nodecommon.NodeReady(node) {
		return ctrl.Result{}, r.setStatus(ctx, op, v1alpha1.NodeOperationPhaseCompleted, "Succeeded",
			fmt.Sprintf("Node %s rebooted and is Ready", node.Name), logger)
	}

	deadline := hardDeadline(op)
	if time.Now().After(deadline) {
		return ctrl.Result{}, r.expire(ctx, op, node, deadline, logger)
	}

	if !rebooting {
		message := fmt.Sprintf("Node %s is rebooting. Waiting for it to come back, until %s",
			node.Name, deadline.UTC().Format(time.RFC3339))
		err := r.patchStatus(ctx, op, fmt.Sprintf("record that the node of %s is rebooting", op.Name), func() {
			op.Status.Reason = reasonRebooting
			op.Status.Message = message
			op.Status.BootID = node.Status.NodeInfo.BootID
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("node is rebooting", "operation", op.Name, "node", node.Name, "bootID", op.Status.BootID)
	}
	return ctrl.Result{RequeueAfter: max(time.Until(deadline), minRequeue)}, nil
}

func terminal(op *v1alpha1.NodeOperation) bool {
	return op.Status.Phase == v1alpha1.NodeOperationPhaseCompleted || op.Status.Phase == v1alpha1.NodeOperationPhaseFailed
}

// collect deletes a finished operation once it is older than the retention, and
// otherwise asks to be called again when it is.
func (r *Reconciler) collect(ctx context.Context, op *v1alpha1.NodeOperation, logger logr.Logger) (ctrl.Result, error) {
	// A phase written past setStatus (by anyone with status rights) comes
	// without the time. Whoever ended the operation, it ended when this
	// controller first saw it ended.
	if op.Status.FinishedAt == nil {
		if err := r.stampFinished(ctx, op); err != nil {
			return ctrl.Result{}, err
		}
	}

	if age := time.Since(op.Status.FinishedAt.Time); age < nodecommon.NodeOperationRetention {
		return ctrl.Result{RequeueAfter: nodecommon.NodeOperationRetention - age}, nil
	}

	if err := r.Client.Delete(ctx, op); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("collect the finished operation %s: %w", op.Name, err)
	}
	logger.Info("collected a finished operation", "operation", op.Name, "type", op.Spec.Type, "node", op.Spec.NodeName)
	return ctrl.Result{}, nil
}

func (r *Reconciler) stampFinished(ctx context.Context, op *v1alpha1.NodeOperation) error {
	return r.patchStatus(ctx, op, fmt.Sprintf("record when %s finished", op.Name), func() {
		now := metav1.Now()
		op.Status.FinishedAt = &now
	})
}

// patchStatus writes a status change under an optimistic lock, and republishes
// the deadline it implies. The lock is the point: the node and this controller
// both write here, so a patch built from a stale read would undo the other's.
func (r *Reconciler) patchStatus(ctx context.Context, op *v1alpha1.NodeOperation, what string, mutate func()) error {
	before := op.DeepCopy()
	mutate()
	op.Status.Deadline = publishedDeadline(op)
	if equality.Semantic.DeepEqual(before.Status, op.Status) {
		return nil
	}
	patch := client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})
	if err := r.Client.Status().Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return nil
}

// setStatus moves the operation to a phase for a reason and records the given
// conditions with it; writing what is already there is a no-op.
func (r *Reconciler) setStatus(ctx context.Context, op *v1alpha1.NodeOperation, phase v1alpha1.NodeOperationPhase, reason, message string, logger logr.Logger, conditions ...metav1.Condition) error {
	// How an operation ended is decided once. The CRD refuses to reopen it as
	// well, but a pass that read the end must not try.
	if terminal(op) {
		return nil
	}
	previous := op.Status.Phase
	// The optimistic lock matters here beyond the usual: the guard above is
	// only as fresh as the read, so a node reporting just as the deadline
	// fires would otherwise be overwritten with Failed.
	err := r.patchStatus(ctx, op, fmt.Sprintf("set %s phase of %s", phase, op.Name), func() {
		op.Status.Phase = phase
		op.Status.Reason = reason
		op.Status.Message = message
		if handedOver(phase, reason) && op.Status.StartedAt == nil {
			now := metav1.Now()
			op.Status.StartedAt = &now
		}
		if terminal(op) {
			now := metav1.Now()
			op.Status.FinishedAt = &now
		}
		for _, condition := range conditions {
			condition.ObservedGeneration = op.Generation
			meta.SetStatusCondition(&op.Status.Conditions, condition)
		}
	})
	if err != nil {
		return err
	}
	if previous == phase {
		return nil
	}
	logger.Info("operation phase", "operation", op.Name, "type", op.Spec.Type, "node", op.Spec.NodeName, "phase", phase, "reason", reason)
	// Announced only once the phase is actually written: an event for a
	// transition a conflict rolled back would be a lie, and the retry would
	// emit it again.
	if terminal(op) {
		eventType := corev1.EventTypeNormal
		if phase == v1alpha1.NodeOperationPhaseFailed {
			eventType = corev1.EventTypeWarning
		}
		r.Recorder.Event(op, eventType, reason, message)
	}
	return nil
}

// handedOver reports a phase in which somebody else now has the work: the
// draining controller for a Drain, the node for the other types. A Drain still
// waiting for the eviction slot has handed nothing over yet.
func handedOver(phase v1alpha1.NodeOperationPhase, reason string) bool {
	return phase == v1alpha1.NodeOperationPhaseInProgress && reason != "WaitingForOperation"
}

func (r *Reconciler) fail(ctx context.Context, op *v1alpha1.NodeOperation, reason, message string, logger logr.Logger) error {
	return r.setStatus(ctx, op, v1alpha1.NodeOperationPhaseFailed, reason, message, logger)
}

// waitingForApprovalMessage names the field, not a kubectl line: kubectl -o yaml
// folds a long value and doubles its single quotes, so a copied command breaks.
const waitingForApprovalMessage = "Waiting for an operator to set spec.approved to true"

// maxApprovalRequest bounds the request for approval: the failure it quotes is the
// node's own text and can run to a whole script output.
const maxApprovalRequest = 4096

// approvalRequest asks an operator for the approval and ends with the cause when the
// operation has one. A retry also says which attempt it is and why the previous one
// failed, so the operator does not approve it blind.
func (r *Reconciler) approvalRequest(ctx context.Context, op *v1alpha1.NodeOperation) (string, error) {
	request := waitingForApprovalMessage
	if op.Spec.Cause != "" {
		request += ". Cause: " + op.Spec.Cause
	}

	// From the cache: only the text of the request depends on it.
	ops, err := operationsOfNode(ctx, r.Client, op.Spec.NodeName)
	if err != nil {
		return "", fmt.Errorf("list the operations of %s: %w", op.Spec.NodeName, err)
	}
	failed := v1alpha1.FailedDisruptions(ops, op.Spec.NodeConfigUID, ptr.Deref(op.Spec.ConfigGeneration, 0))
	if len(failed) == 0 {
		return request, nil
	}
	previous := failed[len(failed)-1]

	head := fmt.Sprintf("Attempt %d of %d. The previous attempt %s failed: ", len(failed)+1, nodecommon.MaxDisruptionAttempts, previous.Name)
	tail := ". " + request
	// The CRD bounds the cause in characters, so a multibyte one can take the whole budget.
	failure := nodecommon.Truncate(cmp.Or(previous.Status.Message, previous.Status.Reason), maxApprovalRequest-len(head)-len(tail))
	return head + failure + tail, nil
}

// handedOverMessage says what was done to the node, what the node is to do now, and by
// when it reports. A Reboot is approved by being created, a Disruption by an approval.
func handedOverMessage(op *v1alpha1.NodeOperation, deadline time.Time) string {
	until := deadline.UTC().Format(time.RFC3339)
	task := "reboot"
	if op.Spec.Type == v1alpha1.NodeOperationTypeDisruption {
		task = fmt.Sprintf("apply generation %d", ptr.Deref(op.Spec.ConfigGeneration, 0))
	}
	message := fmt.Sprintf("Waiting for node %s to %s and report, until %s", op.Spec.NodeName, task, until)
	if !skipDrain(op) {
		message = fmt.Sprintf("Node %s is drained and cordoned. Waiting for it to %s and report, until %s", op.Spec.NodeName, task, until)
	}
	if op.Spec.Type == v1alpha1.NodeOperationTypeDisruption {
		message = "Approved. " + message
	}
	return message
}
