/*
Copyright 2025 Flant JSC

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

package csinode

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	csiNotBootstrappedTaint = "node.deckhouse.io/csi-not-bootstrapped"
)

func init() {
	dynr.RegisterReconciler(rcname.CSITaint, &storagev1.CSINode{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler watches CSINode objects and removes the
// "node.deckhouse.io/csi-not-bootstrapped" taint from the corresponding Node
// once the CSINode has at least one driver registered.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		csiNode, ok := obj.(*storagev1.CSINode)
		if !ok {
			return false
		}
		// Only trigger when the CSINode has drivers.
		return len(csiNode.Spec.Drivers) > 0
	})}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the CSINode.
	csiNode := &storagev1.CSINode{}
	if err := r.Client.Get(ctx, req.NamespacedName, csiNode); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get csinode %s: %w", req.Name, err)
	}

	// Skip CSINodes without drivers.
	if len(csiNode.Spec.Drivers) == 0 {
		return ctrl.Result{}, nil
	}

	// Fetch the corresponding Node.
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: csiNode.Name}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get node %s: %w", csiNode.Name, err)
	}

	// Check if the node has the csi-not-bootstrapped taint.
	if !hasTaint(node, csiNotBootstrappedTaint) {
		return ctrl.Result{}, nil
	}

	// Remove the taint using a merge patch.
	newTaints := removeTaint(node.Spec.Taints, csiNotBootstrappedTaint)
	if err := r.patchNodeTaints(ctx, node, newTaints); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch node %s to remove CSI taint: %w", node.Name, err)
	}

	log.Info("removed csi-not-bootstrapped taint from node", "node", node.Name)
	return ctrl.Result{}, nil
}

// hasTaint returns true if the node has a taint with the given key.
func hasTaint(node *corev1.Node, key string) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == key {
			return true
		}
	}
	return false
}

// removeTaint returns a new slice of taints with the specified key removed.
func removeTaint(taints []corev1.Taint, key string) []corev1.Taint {
	result := make([]corev1.Taint, 0, len(taints))
	for _, taint := range taints {
		if taint.Key != key {
			result = append(result, taint)
		}
	}
	return result
}

// patchNodeTaints applies a merge patch to update the node's taints.
func (r *Reconciler) patchNodeTaints(ctx context.Context, node *corev1.Node, taints []corev1.Taint) error {
	patch := client.MergeFrom(node.DeepCopy())

	node.Spec.Taints = taints
	// Clear status to avoid patching it.
	node.Status = corev1.NodeStatus{}

	return r.Client.Patch(ctx, node, patch)
}
