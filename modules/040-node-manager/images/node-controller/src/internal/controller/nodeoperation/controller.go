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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
}

// SetupWatches follows the nodes an operation waits on: the drain finishing is
// what lets the operation move on.
func (r *Reconciler) SetupWatches(w register.Watcher) {
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

	// A finished operation is history: it is kept for the record and never
	// acted on again. The node goes back to the scheduler — except after a
	// Drain, which was asked for precisely to keep it out.
	if op.Status.Phase == v1alpha1.NodeOperationCompleted || op.Status.Phase == v1alpha1.NodeOperationFailed {
		if op.Spec.Type == v1alpha1.NodeOperationDrain {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, r.releaseNode(ctx, op, logger)
	}

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: op.Spec.NodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.fail(ctx, op, "NodeNotFound",
				fmt.Sprintf("node %s does not exist", op.Spec.NodeName), logger)
		}
		return ctrl.Result{}, err
	}

	if op.Status.Phase == "" {
		if err := r.setPhase(ctx, op, v1alpha1.NodeOperationPending, "Queued",
			"The operation is queued", logger); err != nil {
			return ctrl.Result{}, err
		}
	}

	// The workload leaves before the node is interrupted, unless whoever
	// created the operation asked for it not to.
	if !skipDrain(op) && !drained(node) {
		return ctrl.Result{}, r.startDrain(ctx, node, logger)
	}

	// A Drain is done once the workload is gone: there is nothing for the node
	// to carry out, and it stays unschedulable until someone says otherwise.
	if op.Spec.Type == v1alpha1.NodeOperationDrain {
		return ctrl.Result{}, r.setPhase(ctx, op, v1alpha1.NodeOperationCompleted, "Drained",
			"The workload has left the node, which stays unschedulable", logger)
	}

	// Handing the operation to the node: from here the node carries it out and
	// reports back through the same object.
	return ctrl.Result{}, r.setPhase(ctx, op, v1alpha1.NodeOperationInProgress, "NodePrepared",
		"The node may carry the operation out", logger)
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
	patch := client.MergeFrom(op.DeepCopy())
	op.Status.Phase = phase
	op.Status.ObservedGeneration = op.Generation
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
