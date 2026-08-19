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
	"fmt"
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

	// Predicated, and it has to be: the mapper lists the operations of the node,
	// and without a filter every kubelet heartbeat of every node would run that
	// list.
	w.Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		ops := &v1alpha1.NodeOperationList{}
		if err := r.Client.List(ctx, ops, client.MatchingLabels{v1alpha1.NodeOperationNodeLabel: obj.GetName()}); err != nil {
			log.FromContext(ctx).Error(err, "list the operations of a node that changed", "node", obj.GetName())
			return nil
		}
		var requests []reconcile.Request
		for i := range ops.Items {
			// The node is checked again because the label is written by adopt and
			// an operation may still be carrying somebody else's. A finished
			// operation waits out its retention on its own requeue, not on this.
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

	if err := r.begin(ctx, op, node, logger); err != nil {
		return ctrl.Result{}, err
	}

	return r.carryOut(ctx, op, node, logger)
}

// retire is what becomes of a finished operation: history, kept for the record
// until old enough to collect. The node goes back to the scheduler — except
// after a Drain that got what it asked for, which was to keep it out.
func (r *Reconciler) retire(ctx context.Context, op *v1alpha1.NodeOperation, logger logr.Logger) (ctrl.Result, error) {
	// A Drain that failed gives its marker back like any other operation, or
	// the draining controller evicts for one that has given up.
	keptOut := op.Spec.Type == v1alpha1.NodeOperationTypeDrain && op.Status.Phase == v1alpha1.NodeOperationPhaseCompleted
	if !keptOut {
		if err := r.releaseNode(ctx, op, logger); err != nil {
			return ctrl.Result{}, err
		}
	}
	return r.collect(ctx, op, logger)
}

// carryOut moves a live operation one step towards its end: empty the node,
// hand it to the node, wait for the report — each stretch bounded.
func (r *Reconciler) carryOut(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node, logger logr.Logger) (ctrl.Result, error) {
	// Every stretch of the operation is bounded, not only the node's: a Drain
	// whose eviction never finishes would otherwise hold its parent Pending
	// forever, and with it the node out of the scheduler.
	if deadline := hardDeadline(op); time.Now().After(deadline) {
		return ctrl.Result{}, r.expire(ctx, op, deadline, logger)
	}

	// A Drain is the eviction itself: it asks the draining controller to empty
	// the node and is done once the workload is gone. The node stays
	// unschedulable until someone says otherwise.
	if op.Spec.Type == v1alpha1.NodeOperationTypeDrain {
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
	if op.Status.Phase != v1alpha1.NodeOperationPhaseInProgress {
		return ctrl.Result{RequeueAfter: operationTimeout}, r.setPhase(ctx, op, v1alpha1.NodeOperationPhaseInProgress, "NodePrepared",
			"The node may carry the operation out", logger)
	}

	// Requeue when the deadline is actually due, not a whole period later.
	// Floored: the deadline can pass while this pass runs, and a RequeueAfter
	// of zero is "never" rather than "at once".
	return ctrl.Result{RequeueAfter: max(time.Until(hardDeadline(op)), minRequeue)}, nil
}

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
	return r.setPhase(ctx, op, v1alpha1.NodeOperationPhasePending, "Queued",
		"The operation is queued", logger)
}

// reconcileDrain carries a Drain operation: ask the draining controller to
// empty the node, then wait for the answer it writes onto the node.
func (r *Reconciler) reconcileDrain(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node, logger logr.Logger) (ctrl.Result, error) {
	// The draining controller watches only nodes labelled with their group, so
	// a request written here would sit unread until someone labels the node —
	// and then evict for an operation that timed out long ago.
	if _, inGroup := node.Labels[nodecommon.NodeGroupLabel]; !inGroup {
		return ctrl.Result{}, r.fail(ctx, op, "NodeGroupMissing",
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
			return ctrl.Result{RequeueAfter: waitPollInterval}, nil
		}
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
	return ctrl.Result{}, r.setPhase(ctx, op, v1alpha1.NodeOperationPhaseCompleted, "Drained",
		"The workload has left the node, which stays unschedulable", logger)
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
				Type:     v1alpha1.NodeOperationTypeDrain,
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
	case v1alpha1.NodeOperationPhaseCompleted:
		return true, nil
	case v1alpha1.NodeOperationPhaseFailed:
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

func terminal(op *v1alpha1.NodeOperation) bool {
	return op.Status.Phase == v1alpha1.NodeOperationPhaseCompleted || op.Status.Phase == v1alpha1.NodeOperationPhaseFailed
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

func (r *Reconciler) stampFinished(ctx context.Context, op *v1alpha1.NodeOperation) error {
	return r.patchStatus(ctx, op, fmt.Sprintf("record when %s finished", op.Name), func() {
		now := metav1.Now()
		op.Status.FinishedAt = &now
	})
}

// patchStatus writes a status change under an optimistic lock. The lock is the
// point: the node and this controller both write here, so a patch built from a
// stale read would silently undo the other's.
func (r *Reconciler) patchStatus(ctx context.Context, op *v1alpha1.NodeOperation, what string, mutate func()) error {
	patch := client.MergeFromWithOptions(op.DeepCopy(), client.MergeFromWithOptimisticLock{})
	mutate()
	if err := r.Client.Status().Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return nil
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
	// The optimistic lock matters here beyond the usual: the guard above is
	// only as fresh as the cache, so a node reporting Completed just as the
	// deadline fires would otherwise be overwritten with Failed.
	err := r.patchStatus(ctx, op, fmt.Sprintf("set %s phase of %s", phase, op.Name), func() {
		op.Status.Phase = phase
		if phase == v1alpha1.NodeOperationPhaseInProgress && op.Status.StartedAt == nil {
			now := metav1.Now()
			op.Status.StartedAt = &now
		}
		if phase == v1alpha1.NodeOperationPhaseCompleted || phase == v1alpha1.NodeOperationPhaseFailed {
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
	})
	if err != nil {
		return err
	}
	logger.Info("operation phase", "operation", op.Name, "type", op.Spec.Type, "node", op.Spec.NodeName, "phase", phase)
	return nil
}

func (r *Reconciler) fail(ctx context.Context, op *v1alpha1.NodeOperation, reason, message string, logger logr.Logger) error {
	return r.setPhase(ctx, op, v1alpha1.NodeOperationPhaseFailed, reason, message, logger)
}
