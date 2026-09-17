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

package csitaint

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("csi-taint", &corev1.Node{}, &Reconciler{})
}

const csiNotBootstrappedTaintKey = "node.deckhouse.io/csi-not-bootstrapped"

type Reconciler struct {
	register.Base
}

// ForPredicates keeps kubelet heartbeats out of the workqueue: only a node that carries the taint
// has anything to do here. It also ends the reconcile chain after the patch — the patched node no
// longer passes this filter, so removing the taint does not enqueue the node again.
func (r *Reconciler) ForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		node, ok := obj.(*corev1.Node)
		if !ok {
			return false
		}
		return hasCSITaint(node)
	})}
}

// SetupWatches maps a CSINode to the Node of the same name: a CSINode is named after its node,
// and driver registration is what makes the taint removable. The For predicate above does not
// apply here, so a driver registering later still reaches the reconciler.
func (r *Reconciler) SetupWatches(w register.Watcher) {
	w.Watches(&storagev1.CSINode{}, handler.EnqueueRequestsFromMapFunc(
		func(_ context.Context, obj client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName()}}}
		},
	))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !hasCSITaint(node) {
		logger.V(1).Info("skipping: node has no csi-not-bootstrapped taint", "node", node.Name)
		return ctrl.Result{}, nil
	}

	csiNode := &storagev1.CSINode{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: node.Name}, csiNode); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("waiting: no CSINode for the node yet", "node", node.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get CSINode %s: %w", node.Name, err)
	}

	if len(csiNode.Spec.Drivers) == 0 {
		logger.V(1).Info("waiting: CSINode registers no drivers yet", "node", node.Name)
		return ctrl.Result{}, nil
	}

	logger.Info("CSI driver registered, removing taint", "node", node.Name, "taint", csiNotBootstrappedTaintKey)

	base := node.DeepCopy()
	node.Spec.Taints = slices.DeleteFunc(node.Spec.Taints, isCSITaint)

	// Optimistic lock: a merge patch replaces the whole taint list, so without the resourceVersion
	// guard a taint added concurrently by another actor would be silently dropped.
	if err := r.Client.Patch(ctx, node, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch taints of node %s: %w", node.Name, err)
	}

	logger.Info("taint removed", "node", node.Name)
	return ctrl.Result{}, nil
}

func hasCSITaint(node *corev1.Node) bool {
	return slices.ContainsFunc(node.Spec.Taints, isCSITaint)
}

func isCSITaint(taint corev1.Taint) bool {
	return taint.Key == csiNotBootstrappedTaintKey
}
