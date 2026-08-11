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
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
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

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController(controllerName, &v1alpha1.NodeOperation{}, &Reconciler{})
}

type Reconciler struct {
	register.Base

	// apiReader reads past the manager's cache. Deciding whether to create a
	// child operation from a cached list creates a second one whenever the
	// cache has not caught up with the first.
	apiReader client.Reader
}

// Setup wires the uncached reader.
func (r *Reconciler) Setup(_ context.Context, mgr ctrl.Manager) error {
	r.apiReader = mgr.GetAPIReader()
	return nil
}

// SetupWatches follows what an operation waits on: the node it is draining, and
// the Drain operation it spawned to do the eviction.
func (r *Reconciler) SetupWatches(w register.Watcher) {
	// A child finishing is what lets its parent hand the node over. The child
	// eviction is created with a controller reference to its parent, which is
	// exactly what Owns follows.
	w.Owns(&v1alpha1.NodeOperation{})

	// Predicated, and it has to be: the mapper lists every operation in the
	// cluster, and without a filter every kubelet heartbeat of every node
	// would run that list.
	w.Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		ops := &v1alpha1.NodeOperationList{}
		if err := r.Client.List(ctx, ops); err != nil {
			log.FromContext(ctx).Error(err, "list the operations of a node that changed", "node", obj.GetName())
			return nil
		}
		var requests []reconcile.Request
		for i := range ops.Items {
			// A finished operation is only waiting out its retention, and it
			// requeues itself for that. Waking it on somebody else's node write
			// costs a reconcile that can only decide to wait again.
			if ops.Items[i].Spec.NodeName == obj.GetName() && !terminal(&ops.Items[i]) {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: ops.Items[i].Name}})
			}
		}
		return requests
	}), builder.WithPredicates(predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return nodeDrainStateChanged(e.ObjectOld, e.ObjectNew)
		},
	}))
}

// nodeDrainStateChanged reports whether an update touched the only three things
// an operation reads off its node: the two drain markers and whether the node is
// out of the scheduler.
func nodeDrainStateChanged(before, after client.Object) bool {
	oldNode, okOld := before.(*corev1.Node)
	newNode, okNew := after.(*corev1.Node)
	if !okOld || !okNew {
		return true
	}
	if oldNode.Spec.Unschedulable != newNode.Spec.Unschedulable {
		return true
	}
	return oldNode.Annotations[nodecommon.DrainingAnnotation] != newNode.Annotations[nodecommon.DrainingAnnotation] ||
		oldNode.Annotations[nodecommon.DrainedAnnotation] != newNode.Annotations[nodecommon.DrainedAnnotation]
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

	// A finished operation is history: kept for the record, never acted on
	// again until old enough to collect. The node goes back to the scheduler —
	// except after a Drain, which was asked for precisely to keep it out.
	if terminal(op) {
		if op.Spec.Type != v1alpha1.NodeOperationDrain {
			if err := r.releaseNode(ctx, op, logger); err != nil {
				return ctrl.Result{}, err
			}
		}
		return r.collect(ctx, op, logger)
	}

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: op.Spec.NodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.fail(ctx, op, "NodeNotFound",
				fmt.Sprintf("node %s does not exist", op.Spec.NodeName), logger)
		}
		return ctrl.Result{}, err
	}

	if err := r.adopt(ctx, op, node); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.begin(ctx, op, node, logger); err != nil {
		return ctrl.Result{}, err
	}

	// Every stretch of the operation is bounded, not only the node's: a Drain
	// whose eviction never finishes would otherwise hold its parent Pending
	// forever, and with it the node out of the scheduler.
	if deadline := hardDeadline(op); time.Now().After(deadline) {
		return ctrl.Result{}, r.expire(ctx, op, deadline, logger)
	}

	// A Drain is the eviction itself: it asks the draining controller to empty
	// the node and is done once the workload is gone. The node stays
	// unschedulable until someone says otherwise.
	if op.Spec.Type == v1alpha1.NodeOperationDrain {
		return r.reconcileDrain(ctx, op, node, logger)
	}

	// Every other operation interrupts the node, so the workload leaves first
	// through a Drain operation of its own — a visible step with its own
	// phases, carried out by the one piece of code that knows how.
	if !skipDrain(op) {
		done, err := r.ensureDrained(ctx, op, logger)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !done {
			return ctrl.Result{RequeueAfter: waitPollInterval}, nil
		}
	}

	// Handing the operation to the node: from here the node carries it out and
	// reports back through the same object.
	if op.Status.Phase != v1alpha1.NodeOperationInProgress {
		return ctrl.Result{RequeueAfter: operationTimeout}, r.setPhase(ctx, op, v1alpha1.NodeOperationInProgress, "NodePrepared",
			"The node may carry the operation out", logger)
	}

	// Requeue when the deadline is actually due, not a whole period later.
	// Floored: the deadline can pass while this pass runs, and a RequeueAfter
	// of zero is "never" rather than "at once".
	requeue := time.Until(hardDeadline(op))
	if requeue < minRequeue {
		requeue = minRequeue
	}
	return ctrl.Result{RequeueAfter: requeue}, nil
}

// begin queues an operation nobody has started yet, once.
func (r *Reconciler) begin(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node, logger logr.Logger) error {
	if op.Status.Phase != "" {
		return nil
	}
	// Recorded before anything touches the node, so releasing it later gives
	// back the state the operator left it in rather than always making it
	// schedulable.
	if err := r.rememberCordon(ctx, op, node); err != nil {
		return err
	}
	return r.setPhase(ctx, op, v1alpha1.NodeOperationPending, "Queued",
		"The operation is queued", logger)
}

// expire fails an operation that ran out of time and says so. Announced only
// once the phase is actually written: an event for a transition a conflict
// rolled back would be a lie, and the retry would emit it again.
func (r *Reconciler) expire(ctx context.Context, op *v1alpha1.NodeOperation, deadline time.Time, logger logr.Logger) error {
	reason, message := timedOut(op, deadline)
	if err := r.fail(ctx, op, reason, message, logger); err != nil {
		return err
	}
	r.Recorder.Event(op, corev1.EventTypeWarning, reason, message)
	return nil
}

// reconcileDrain carries a Drain operation: ask the draining controller to
// empty the node, then wait for the answer it writes onto the node.
func (r *Reconciler) reconcileDrain(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node, logger logr.Logger) (ctrl.Result, error) {
	// The request is re-issued whenever the node carries no drain of this
	// operation's: markers removed by anything else would otherwise be
	// waited on forever. Asking again does not move the pinned deadline.
	if !drainRequested(op) || idle(op, node) {
		if err := r.startDrain(ctx, op, node, logger); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: waitPollInterval}, r.recordDrainRequested(ctx, op, node)
	}
	// The Node event carrying the marker is the normal wake-up; one dropped
	// event must not strand the operation until the manager's next full
	// resync, hence the requeue.
	if !drained(op, node) {
		return ctrl.Result{RequeueAfter: waitPollInterval}, nil
	}
	return ctrl.Result{}, r.setPhase(ctx, op, v1alpha1.NodeOperationCompleted, "Drained",
		"The workload has left the node, which stays unschedulable", logger)
}

// hardDeadline is the moment this operation runs out of time. Read off the
// operation alone, never recomputed from the cluster: a later NodeGroup edit
// or deletion must not cut short a drain that is still legitimately running.
func hardDeadline(op *v1alpha1.NodeOperation) time.Time {
	switch {
	case op.Status.StartedAt != nil:
		// The node has the work and owes an answer.
		return op.Status.StartedAt.Time.Add(operationTimeout)
	case op.Status.DrainDeadline != nil:
		// Waiting for an eviction that was given until then, plus the margin
		// every stretch gets for the plumbing around it.
		return op.Status.DrainDeadline.Time.Add(operationTimeout)
	default:
		return op.CreationTimestamp.Time.Add(operationTimeout)
	}
}

// drainTimeout is the bound the draining controller will use for this node's
// group, resolved the way it resolves it. Read once, when the eviction is asked
// for, and pinned from there on.
func (r *Reconciler) drainTimeout(ctx context.Context, node *corev1.Node) time.Duration {
	ngName := node.Labels[nodecommon.NodeGroupLabel]
	if ngName == "" {
		return defaultDrainTimeout
	}
	ng, err := nodecommon.GetNodeGroup(ctx, r.Client, ngName)
	if err != nil {
		log.FromContext(ctx).V(1).Info("could not read the group's drain timeout, using the default",
			"nodeGroup", ngName, "default", defaultDrainTimeout, "error", err.Error())
		return defaultDrainTimeout
	}
	return drainTimeoutOf(ng)
}

// drainTimeoutOf clamps as well as reads: an object stored before the CRD
// bounded nodeDrainTimeoutSecond can carry a value whose seconds-to-Duration
// multiplication overflows into a negative deadline.
func drainTimeoutOf(ng *v1.NodeGroup) time.Duration {
	if ng.Spec.NodeDrainTimeoutSecond == nil {
		return defaultDrainTimeout
	}
	seconds := int64(*ng.Spec.NodeDrainTimeoutSecond)
	if seconds <= 0 {
		return defaultDrainTimeout
	}
	if seconds > int64(maxDrainTimeout/time.Second) {
		return maxDrainTimeout
	}
	return time.Duration(seconds) * time.Second
}

// timedOut names what the operation was still waiting for when the deadline
// passed, so the record says which node is holding the group up.
func timedOut(op *v1alpha1.NodeOperation, deadline time.Time) (string, string) {
	when := deadline.UTC().Format(time.RFC3339)
	if op.Status.StartedAt != nil {
		return "NodeTimedOut", fmt.Sprintf("node %s did not report back by %s", op.Spec.NodeName, when)
	}
	return "PreparationTimedOut", fmt.Sprintf("node %s was not prepared by %s: its workload has not finished leaving", op.Spec.NodeName, when)
}

func terminal(op *v1alpha1.NodeOperation) bool {
	return op.Status.Phase == v1alpha1.NodeOperationCompleted || op.Status.Phase == v1alpha1.NodeOperationFailed
}

// collect deletes a finished operation once it is older than the retention, and
// otherwise asks to be called again when it is. The parent takes its child
// Drain with it, since the child is owned by it.
func (r *Reconciler) collect(ctx context.Context, op *v1alpha1.NodeOperation, logger logr.Logger) (ctrl.Result, error) {
	// The node reports a Reboot or an ApproveDisruption finished by writing the
	// phase itself, which is not the path that stamps the time. Whoever ended
	// the operation, it ended when this controller first saw it ended.
	if op.Status.FinishedAt == nil {
		if err := r.stampFinished(ctx, op); err != nil {
			return ctrl.Result{}, err
		}
	}

	if age := time.Since(op.Status.FinishedAt.Time); age < retention {
		return ctrl.Result{RequeueAfter: retention - age}, nil
	}

	if err := r.Client.Delete(ctx, op); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("collect the finished operation %s: %w", op.Name, err)
	}
	logger.Info("collected a finished operation", "operation", op.Name, "type", op.Spec.Type, "node", op.Spec.NodeName)
	return ctrl.Result{}, nil
}

// stampFinished records when a finished operation finished, for the operations
// whose last phase was written by the node rather than by this controller.
func (r *Reconciler) stampFinished(ctx context.Context, op *v1alpha1.NodeOperation) error {
	patch := client.MergeFromWithOptions(op.DeepCopy(), client.MergeFromWithOptimisticLock{})
	now := metav1.Now()
	op.Status.FinishedAt = &now
	if err := r.Client.Status().Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("record when %s finished: %w", op.Name, err)
	}
	return nil
}

// rememberCordon records whether the node was already cordoned for a reason of
// its own, so release does not undo an operator's cordon. A holder's recorded
// answer is copied instead of live state, which may be the holder's own cordon.
func (r *Reconciler) rememberCordon(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node) error {
	if op.Status.NodeWasUnschedulable != nil {
		return nil
	}
	recorded, err := r.cordonRecordedByHolder(ctx, op)
	if err != nil {
		return err
	}
	wasUnschedulable := node.Spec.Unschedulable
	if recorded != nil {
		wasUnschedulable = *recorded
	}

	patch := client.MergeFromWithOptions(op.DeepCopy(), client.MergeFromWithOptimisticLock{})
	op.Status.NodeWasUnschedulable = ptr.To(wasUnschedulable)
	if err := r.Client.Status().Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("record how %s found the node: %w", op.Name, err)
	}
	return nil
}

// cordonRecordedByHolder returns what an operation already holding this node
// recorded; one that has not recorded yet has not cordoned either, so nil. The
// read goes straight to the API server: a stale cache reads as "nobody here".
func (r *Reconciler) cordonRecordedByHolder(ctx context.Context, op *v1alpha1.NodeOperation) (*bool, error) {
	others, err := r.operationsOfNode(ctx, op.Spec.NodeName)
	if err != nil {
		return nil, fmt.Errorf("list the operations of %s: %w", op.Spec.NodeName, err)
	}
	for i := range others {
		other := &others[i]
		if other.UID == op.UID || terminal(other) {
			continue
		}
		if other.Status.NodeWasUnschedulable != nil {
			return other.Status.NodeWasUnschedulable, nil
		}
	}
	return nil, nil
}

// operationsOfNode lists every operation recorded against a node. The read goes
// straight to the API server: a cached list missing a just-created one is how a
// node ended up with two operations that each thought they were alone.
func (r *Reconciler) operationsOfNode(ctx context.Context, nodeName string) ([]v1alpha1.NodeOperation, error) {
	list := &v1alpha1.NodeOperationList{}
	if err := r.apiReader.List(ctx, list, client.MatchingLabels{v1alpha1.NodeOperationNodeLabel: nodeName}); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// adopt gives an operation its node owner reference (node deletion collects
// it) and node label (how siblings find each other). An operator's arrives
// with neither, and unlabelled it would leave the node cordoned for good.
func (r *Reconciler) adopt(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node) error {
	if len(op.OwnerReferences) > 0 && op.Labels[v1alpha1.NodeOperationNodeLabel] == op.Spec.NodeName {
		return nil
	}
	patch := client.MergeFrom(op.DeepCopy())
	if op.Labels == nil {
		op.Labels = map[string]string{}
	}
	op.Labels[v1alpha1.NodeOperationNodeLabel] = op.Spec.NodeName
	if len(op.OwnerReferences) == 0 {
		op.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "v1",
			Kind:       "Node",
			Name:       node.Name,
			UID:        node.UID,
		}}
	}
	if err := r.Client.Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("adopt %s onto %s: %w", op.Name, node.Name, err)
	}
	return nil
}

// ensureDrained runs the eviction this operation needs as a Drain operation of
// its own, and reports whether it has finished. The child belongs to its
// parent: deleting the parent takes the record of its eviction with it.
func (r *Reconciler) ensureDrained(ctx context.Context, op *v1alpha1.NodeOperation, logger logr.Logger) (bool, error) {
	child, err := r.drainOf(ctx, op)
	if err != nil {
		return false, err
	}

	if child == nil {
		child = &v1alpha1.NodeOperation{
			ObjectMeta: metav1.ObjectMeta{
				// Generated, not derived from the parent's name: a name this
				// controller computed could already belong to an operation
				// someone else created, and that one is not ours to touch.
				GenerateName: op.Name + "-drain-",
				Labels:       map[string]string{v1alpha1.NodeOperationNodeLabel: op.Spec.NodeName},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1alpha1.GroupVersion.String(),
					Kind:       nodeOperationKind,
					Name:       op.Name,
					UID:        op.UID,
					Controller: ptr.To(true),
				}},
			},
			Spec: v1alpha1.NodeOperationSpec{
				Type:     v1alpha1.NodeOperationDrain,
				NodeName: op.Spec.NodeName,
			},
		}
		if err := r.Client.Create(ctx, child); err != nil {
			return false, fmt.Errorf("evict the workload of %s: %w", op.Spec.NodeName, err)
		}
		logger.Info("evicting the workload before the operation", "operation", op.Name, "drain", child.Name)
		return false, nil
	}

	switch child.Status.Phase {
	case v1alpha1.NodeOperationCompleted:
		return true, nil
	case v1alpha1.NodeOperationFailed:
		return false, r.fail(ctx, op, "DrainFailed",
			fmt.Sprintf("the workload could not be evicted, see NodeOperation %s", child.Name), logger)
	default:
		// The parent waits exactly as long as the eviction it is waiting for.
		if child.Status.DrainDeadline != nil && !child.Status.DrainDeadline.Equal(op.Status.DrainDeadline) {
			return false, r.adoptDrainDeadline(ctx, op, child.Status.DrainDeadline)
		}
		return false, nil
	}
}

// drainOf finds the eviction this operation spawned, by ownership rather than
// name. The read goes straight to the API server: a cached list missing a
// just-created child is how one operation got two evictions of the same node.
func (r *Reconciler) drainOf(ctx context.Context, op *v1alpha1.NodeOperation) (*v1alpha1.NodeOperation, error) {
	children, err := r.operationsOfNode(ctx, op.Spec.NodeName)
	if err != nil {
		return nil, fmt.Errorf("list the drains of %s: %w", op.Name, err)
	}
	for i := range children {
		child := &children[i]
		if child.Spec.Type == v1alpha1.NodeOperationDrain && ownedBy(child, op) {
			return child, nil
		}
	}
	return nil, nil
}

// startDrain hands the node to the draining controller, which evicts the pods
// and reports back through the drained annotation. The request carries this
// operation's marker, so the answer comes back carrying it too.
func (r *Reconciler) startDrain(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node, logger logr.Logger) error {
	patch := client.MergeFrom(node.DeepCopy())
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
	node.Annotations[nodecommon.DrainingAnnotation] = drainMarker(op)
	if err := r.Client.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("start drain of %s: %w", node.Name, err)
	}
	logger.Info("draining the node for the operation", "node", node.Name, "operation", op.Name)
	return nil
}

func drainRequested(op *v1alpha1.NodeOperation) bool {
	return meta.IsStatusConditionTrue(op.Status.Conditions, conditionDrainRequested)
}

// drainMarker is the value this operation writes into the node's drain
// annotations and the only value it reads back or erases: the identity lets
// operations share a node without clearing each other's (or bashible's) marker.
func drainMarker(op *v1alpha1.NodeOperation) string {
	return drainingSource + "/" + string(lineageUID(op))
}

func lineageUID(op *v1alpha1.NodeOperation) types.UID {
	for _, owner := range op.OwnerReferences {
		if owner.Kind == nodeOperationKind {
			return owner.UID
		}
	}
	return op.UID
}

// idle reports that the node carries no drain of this operation's — neither a
// request waiting to be picked up nor an answer. Something removed them, so the
// eviction is not going to arrive unless it is asked for again.
func idle(op *v1alpha1.NodeOperation, node *corev1.Node) bool {
	marker := drainMarker(op)
	return node.Annotations[nodecommon.DrainingAnnotation] != marker &&
		node.Annotations[nodecommon.DrainedAnnotation] != marker
}

// recordDrainRequested remembers that the eviction has been asked for and, the
// first time only, until when it may run. The deadline is written once and
// never moved: a deadline renewed on every re-issue is no deadline.
func (r *Reconciler) recordDrainRequested(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node) error {
	patch := client.MergeFromWithOptions(op.DeepCopy(), client.MergeFromWithOptimisticLock{})
	meta.SetStatusCondition(&op.Status.Conditions, metav1.Condition{
		Type:               conditionDrainRequested,
		Status:             metav1.ConditionTrue,
		Reason:             "Draining",
		Message:            fmt.Sprintf("the draining controller was asked to empty %s", op.Spec.NodeName),
		ObservedGeneration: op.Generation,
	})
	if op.Status.DrainDeadline == nil {
		op.Status.DrainDeadline = ptr.To(metav1.NewTime(time.Now().Add(r.drainTimeout(ctx, node))))
	}
	if err := r.Client.Status().Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("record that %s asked for its eviction: %w", op.Name, err)
	}
	return nil
}

// adoptDrainDeadline gives a parent the deadline its child eviction runs to;
// otherwise the parent fails out from under a still-running drain, which then
// finishes onto a node whose operation has already released it.
func (r *Reconciler) adoptDrainDeadline(ctx context.Context, op *v1alpha1.NodeOperation, deadline *metav1.Time) error {
	patch := client.MergeFromWithOptions(op.DeepCopy(), client.MergeFromWithOptimisticLock{})
	op.Status.DrainDeadline = deadline.DeepCopy()
	if err := r.Client.Status().Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("take over the eviction deadline of %s: %w", op.Name, err)
	}
	return nil
}

// releaseNode puts a node back the way the operation found it. It runs on
// every reconcile of a terminal operation until collection, so it touches only
// this operation's own markers and a cordon no other operation relies on.
func (r *Reconciler) releaseNode(ctx context.Context, op *v1alpha1.NodeOperation, logger logr.Logger) error {
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: op.Spec.NodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	marker := drainMarker(op)
	ours := node.Annotations[nodecommon.DrainingAnnotation] == marker ||
		node.Annotations[nodecommon.DrainedAnnotation] == marker
	if !ours {
		return nil
	}

	// An operation from before this was recorded falls back to the old
	// behaviour: leaving such a node cordoned forever is worse than the cordon
	// this restores, and finished operations are collected within a day.
	restored := op.Status.NodeWasUnschedulable != nil && *op.Status.NodeWasUnschedulable
	if !restored {
		// Lowering it is only ours while nobody else needs it raised: a Drain,
		// or an interruption mid-eviction, needs the node kept out of the
		// scheduler, and releasing would put pods back onto an emptying node.
		held, err := r.heldUnschedulableByAnother(ctx, op)
		if err != nil {
			return err
		}
		if held {
			restored = node.Spec.Unschedulable
		}
	}

	// Spelled out rather than computed from a mutated copy: annotations carry
	// omitempty, so removing the last key makes the merge patch say
	// "annotations: null", deleting every annotation on the node.
	annotations := map[string]any{}
	for _, key := range []string{nodecommon.DrainingAnnotation, nodecommon.DrainedAnnotation} {
		if node.Annotations[key] == marker {
			annotations[key] = nil
		}
	}
	body, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"annotations": annotations},
		"spec":     map[string]any{"unschedulable": restored},
	})
	if err != nil {
		return fmt.Errorf("build the release patch for %s: %w", node.Name, err)
	}
	if err := r.Client.Patch(ctx, node, client.RawPatch(types.MergePatchType, body)); err != nil {
		return fmt.Errorf("release %s after its operation: %w", node.Name, err)
	}
	logger.Info("node released after its operation",
		"node", node.Name, "operation", op.Name, "unschedulable", restored)
	return nil
}

// heldUnschedulableByAnother reports whether an operation of another lineage
// still needs this node out of the scheduler: a Drain, or an interruption
// mid-eviction. An operation's own child eviction shares its lineage.
func (r *Reconciler) heldUnschedulableByAnother(ctx context.Context, op *v1alpha1.NodeOperation) (bool, error) {
	others, err := r.operationsOfNode(ctx, op.Spec.NodeName)
	if err != nil {
		return false, fmt.Errorf("list the operations of %s: %w", op.Spec.NodeName, err)
	}
	lineage := lineageUID(op)
	for i := range others {
		other := &others[i]
		if lineageUID(other) == lineage || terminal(other) {
			continue
		}
		if other.Spec.Type == v1alpha1.NodeOperationDrain || drainRequested(other) {
			return true, nil
		}
	}
	return false, nil
}

func (r *Reconciler) setPhase(ctx context.Context, op *v1alpha1.NodeOperation, phase v1alpha1.NodeOperationPhase, reason, message string, logger logr.Logger) error {
	if op.Status.Phase == phase {
		return nil
	}
	// How an operation ended is decided once. The node and this controller both
	// write here, so without this a late report could reopen a failed operation
	// and hand a node over that nobody prepared.
	if terminal(op) {
		return nil
	}
	// Under an optimistic lock, because the guard above is only as fresh as
	// the cache: a node reporting Completed just as the deadline fires would
	// otherwise be overwritten with Failed.
	patch := client.MergeFromWithOptions(op.DeepCopy(), client.MergeFromWithOptimisticLock{})
	op.Status.Phase = phase
	op.Status.ObservedGeneration = op.Generation
	if phase == v1alpha1.NodeOperationInProgress && op.Status.StartedAt == nil {
		now := metav1.Now()
		op.Status.StartedAt = &now
	}
	if phase == v1alpha1.NodeOperationCompleted || phase == v1alpha1.NodeOperationFailed {
		now := metav1.Now()
		op.Status.FinishedAt = &now
	}
	meta.SetStatusCondition(&op.Status.Conditions, metav1.Condition{
		Type:               conditionProgress,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: op.Generation,
	})
	if err := r.Client.Status().Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("set %s phase of %s: %w", phase, op.Name, err)
	}
	logger.Info("operation phase", "operation", op.Name, "type", op.Spec.Type, "node", op.Spec.NodeName, "phase", phase)
	return nil
}

func (r *Reconciler) fail(ctx context.Context, op *v1alpha1.NodeOperation, reason, message string, logger logr.Logger) error {
	return r.setPhase(ctx, op, v1alpha1.NodeOperationFailed, reason, message, logger)
}

func skipDrain(op *v1alpha1.NodeOperation) bool {
	return op.Spec.Drain != nil && op.Spec.Drain.Skip
}

func drained(op *v1alpha1.NodeOperation, node *corev1.Node) bool {
	return node.Annotations[nodecommon.DrainedAnnotation] == drainMarker(op)
}

// ownedBy reports whether the child was created for this exact operation, not
// for an earlier one of the same name.
func ownedBy(child, parent *v1alpha1.NodeOperation) bool {
	return slices.ContainsFunc(child.OwnerReferences, func(owner metav1.OwnerReference) bool {
		return owner.UID == parent.UID
	})
}
