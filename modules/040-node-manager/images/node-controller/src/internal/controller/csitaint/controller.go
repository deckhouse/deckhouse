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

// Package csitaint reconciles the node.deckhouse.io/csi-not-bootstrapped taint.
//
// A freshly bootstrapped node carries the taint until its CSI driver is
// registered. Registration is observed through the node's CSINode object: once
// CSINode.spec.drivers is non-empty the driver is up and the taint must go.
//
// This replaces the shell-operator hook hooks/remove_csi_taints.go. Unlike the
// hook, whose Node binding was passive (it only reconciled on OnBeforeHelm
// converge or CSINode filter-result changes), the controller watches Node
// reactively, so the taint is removed promptly on the CSINode registration
// event or on any node change.
package csitaint

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/node-controller/internal/register"
)

const csiNotBootstrappedTaintKey = "node.deckhouse.io/csi-not-bootstrapped"

func init() {
	register.RegisterController("node-csi-taint", &corev1.Node{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// A CSINode is named after its node, so a registration event maps directly
	// to the node of the same name. This is what unblocks a bootstrapping node
	// whose taint stays put until its driver registers (no Node change happens
	// at that moment).
	w.Watches(&storagev1.CSINode{}, handler.EnqueueRequestsFromMapFunc(csiNodeToNode))
}

func csiNodeToNode(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName()}}}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !hasCSITaint(node) {
		return ctrl.Result{}, nil
	}

	csiNode := &storagev1.CSINode{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: node.Name}, csiNode); err != nil {
		if apierrors.IsNotFound(err) {
			// Node is still bootstrapping: no CSINode yet. Keep the taint.
			logger.V(1).Info("CSINode not found, keeping taint", "node", node.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if len(csiNode.Spec.Drivers) == 0 {
		// Driver not registered yet. Keep the taint.
		logger.V(1).Info("no CSI drivers registered, keeping taint", "node", node.Name)
		return ctrl.Result{}, nil
	}

	patch := client.MergeFrom(node.DeepCopy())
	node.Spec.Taints = removeCSITaint(node.Spec.Taints)
	if err := r.Client.Patch(ctx, node, patch); err != nil {
		logger.Error(err, "failed to remove csi-not-bootstrapped taint", "node", node.Name)
		return ctrl.Result{}, err
	}

	logger.Info("removed csi-not-bootstrapped taint", "node", node.Name)
	return ctrl.Result{}, nil
}

func hasCSITaint(node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == csiNotBootstrappedTaintKey {
			return true
		}
	}
	return false
}

func removeCSITaint(taints []corev1.Taint) []corev1.Taint {
	result := make([]corev1.Taint, 0, len(taints))
	for _, taint := range taints {
		if taint.Key != csiNotBootstrappedTaintKey {
			result = append(result, taint)
		}
	}
	return result
}
