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

// Package nodeoperation carries an operation that interrupts a node — a
// reboot, an eviction, or the permission a node needs to apply a configuration
// it cannot apply without a pause — from the intent someone recorded to the
// state the node ends up in.
//
// The controller owns the preparation: it evicts the workload and then hands
// the operation to the node by moving it to InProgress. The node owns the work
// itself and reports how it ended. Both halves are visible in one object, which
// is the point of doing this through a resource instead of an annotation: an
// operator can see what is happening to a node, and ask for the same thing by
// hand.
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

	// APIReader reads past the manager's cache. Deciding whether to create a
	// child operation from a cached list creates a second one whenever the
	// cache has not caught up with the first.
	APIReader client.Reader
}

// Setup wires the uncached reader.
func (r *Reconciler) Setup(mgr ctrl.Manager) error {
	r.APIReader = mgr.GetAPIReader()
	return nil
}

func (r *Reconciler) reader() client.Reader {
	if r.APIReader != nil {
		return r.APIReader
	}
	return r.Client
}

// SetupWatches follows what an operation waits on: the node it is draining, and
// the Drain operation it spawned to do the eviction.
func (r *Reconciler) SetupWatches(w register.Watcher) {
	// A child finishing is what lets its parent hand the node over.
	w.Watches(&v1alpha1.NodeOperation{}, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
		for _, owner := range obj.GetOwnerReferences() {
			if owner.Kind == "NodeOperation" {
				return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: owner.Name}}}
			}
		}
		return nil
	}))

	w.Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		ops := &v1alpha1.NodeOperationList{}
		if err := r.Client.List(ctx, ops); err != nil {
			return nil
		}
		var requests []reconcile.Request
		for i := range ops.Items {
			if ops.Items[i].Spec.NodeName == obj.GetName() {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: ops.Items[i].Name}})
			}
		}
		return requests
	}))
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

	// A finished operation is history: it is kept for the record and never acted
	// on again, until it is old enough to collect. The node goes back to the
	// scheduler — except after a Drain, which was asked for precisely to keep it
	// out.
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

	if err := r.ownedByNode(ctx, op, node); err != nil {
		return ctrl.Result{}, err
	}

	if op.Status.Phase == "" {
		if err := r.setPhase(ctx, op, v1alpha1.NodeOperationPending, "Queued",
			"The operation is queued", logger); err != nil {
			return ctrl.Result{}, err
		}
	}

	// A Drain is the eviction itself: it asks the draining controller to empty
	// the node and is done once the workload is gone. The node stays
	// unschedulable until someone says otherwise.
	if op.Spec.Type == v1alpha1.NodeOperationDrain {
		if !drained(node) {
			return ctrl.Result{}, r.startDrain(ctx, node, logger)
		}
		return ctrl.Result{}, r.setPhase(ctx, op, v1alpha1.NodeOperationCompleted, "Drained",
			"The workload has left the node, which stays unschedulable", logger)
	}

	// Every other operation interrupts the node, so the workload leaves first —
	// through a Drain operation of its own rather than a side effect of this
	// one. The eviction is then a step anyone can see, with its own phases, and
	// it is carried out by the one piece of code that knows how.
	if !skipDrain(op) {
		done, err := r.ensureDrained(ctx, op, logger)
		if err != nil || !done {
			return ctrl.Result{}, err
		}
	}

	// Handing the operation to the node: from here the node carries it out and
	// reports back through the same object.
	if op.Status.Phase != v1alpha1.NodeOperationInProgress {
		return ctrl.Result{RequeueAfter: operationTimeout}, r.setPhase(ctx, op, v1alpha1.NodeOperationInProgress, "NodePrepared",
			"The node may carry the operation out", logger)
	}

	// The node has had it for a while and said nothing. Something is wrong with
	// the node, and leaving the operation open would keep it out of the
	// scheduler forever, with no sign that nothing is coming.
	if waited := r.since(op); waited > operationTimeout {
		return ctrl.Result{}, r.fail(ctx, op, "NodeTimedOut",
			fmt.Sprintf("the node did not report back within %s", operationTimeout), logger)
	}
	return ctrl.Result{RequeueAfter: operationTimeout}, nil
}

// since is how long the operation has been waiting for the node.
func (r *Reconciler) since(op *v1alpha1.NodeOperation) time.Duration {
	if op.Status.StartedAt == nil {
		return 0
	}
	return time.Since(op.Status.StartedAt.Time)
}

func terminal(op *v1alpha1.NodeOperation) bool {
	return op.Status.Phase == v1alpha1.NodeOperationCompleted || op.Status.Phase == v1alpha1.NodeOperationFailed
}

// retention is how long a finished operation is kept. It is the record of what
// was done to a node, which is worth having while someone is still looking into
// what happened, and worth nothing a day later — a cluster that reboots nodes
// or rolls configs out produces one of these per node per change, so without a
// limit the list only grows.
const retention = 24 * time.Hour

// collect deletes a finished operation once it is older than the retention, and
// otherwise asks to be called again when it is. The parent takes its child
// Drain with it, since the child is owned by it.
func (r *Reconciler) collect(ctx context.Context, op *v1alpha1.NodeOperation, logger logr.Logger) (ctrl.Result, error) {
	// An operation that finished before this field existed, or whose finish was
	// never recorded, is measured from its creation instead of being kept
	// forever.
	finished := op.CreationTimestamp.Time
	if op.Status.FinishedAt != nil {
		finished = op.Status.FinishedAt.Time
	}

	if age := time.Since(finished); age < retention {
		return ctrl.Result{RequeueAfter: retention - age}, nil
	}

	if err := r.Client.Delete(ctx, op); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("collect the finished operation %s: %w", op.Name, err)
	}
	logger.Info("collected a finished operation", "operation", op.Name, "type", op.Spec.Type, "node", op.Spec.NodeName)
	return ctrl.Result{}, nil
}

// ownedByNode makes the node the owner of an operation created for it, so that
// a node leaving the cluster takes the operations addressed to it along.
// Operations this controller creates already carry an owner; the ones an
// operator writes by hand do not.
func (r *Reconciler) ownedByNode(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node) error {
	if len(op.OwnerReferences) > 0 {
		return nil
	}
	patch := client.MergeFrom(op.DeepCopy())
	op.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "Node",
		Name:       node.Name,
		UID:        node.UID,
	}}
	if err := r.Client.Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("set the owner of %s: %w", op.Name, err)
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
				Labels:       map[string]string{operationNodeLabel: op.Spec.NodeName},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1alpha1.GroupVersion.String(),
					Kind:       "NodeOperation",
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
		return false, nil
	}
}

// drainOf finds the eviction this operation spawned, by ownership rather than
// by a name that anyone could have taken.
//
// The read goes straight to the API server. A cached list can still be missing
// a child created moments ago, and the caller creates one when it finds none —
// which is how a single operation ended up with two evictions of the same node.
func (r *Reconciler) drainOf(ctx context.Context, op *v1alpha1.NodeOperation) (*v1alpha1.NodeOperation, error) {
	children := &v1alpha1.NodeOperationList{}
	if err := r.reader().List(ctx, children, client.MatchingLabels{operationNodeLabel: op.Spec.NodeName}); err != nil {
		return nil, fmt.Errorf("list the drains of %s: %w", op.Name, err)
	}
	for i := range children.Items {
		child := &children.Items[i]
		if child.Spec.Type == v1alpha1.NodeOperationDrain && ownedBy(child, op) {
			return child, nil
		}
	}
	return nil, nil
}

// startDrain hands the node to the draining controller, which evicts the pods
// and reports back through the drained annotation.
func (r *Reconciler) startDrain(ctx context.Context, node *corev1.Node, logger logr.Logger) error {
	if node.Annotations[nodecommon.DrainingAnnotation] == drainingSource {
		return nil
	}
	patch := client.MergeFrom(node.DeepCopy())
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
	node.Annotations[nodecommon.DrainingAnnotation] = drainingSource
	if err := r.Client.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("start drain of %s: %w", node.Name, err)
	}
	logger.Info("draining the node for the operation", "node", node.Name)
	return nil
}

// releaseNode gives a node back to the scheduler once the operation that took
// it away is over, however it ended.
func (r *Reconciler) releaseNode(ctx context.Context, op *v1alpha1.NodeOperation, logger logr.Logger) error {
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: op.Spec.NodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if node.Annotations[nodecommon.DrainingAnnotation] != drainingSource &&
		node.Annotations[nodecommon.DrainedAnnotation] != drainingSource {
		return nil
	}

	patch := client.MergeFrom(node.DeepCopy())
	delete(node.Annotations, nodecommon.DrainingAnnotation)
	delete(node.Annotations, nodecommon.DrainedAnnotation)
	node.Spec.Unschedulable = false
	if err := r.Client.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("release %s after its operation: %w", node.Name, err)
	}
	logger.Info("node returned to the scheduler after its operation", "node", node.Name, "operation", op.Name)
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
	patch := client.MergeFrom(op.DeepCopy())
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

func drained(node *corev1.Node) bool {
	return node.Annotations[nodecommon.DrainedAnnotation] == drainingSource
}

// ownedBy reports whether the child was created for this exact operation, not
// for an earlier one of the same name.
func ownedBy(child, parent *v1alpha1.NodeOperation) bool {
	for _, owner := range child.OwnerReferences {
		if owner.UID == parent.UID {
			return true
		}
	}
	return false
}
