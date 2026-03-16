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

package bashiblecleanup

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	bashibleFirstRunFinishedLabel = "node.deckhouse.io/bashible-first-run-finished"
	bashibleUninitializedTaintKey = "node.deckhouse.io/bashible-uninitialized"
)

func init() {
	dynr.RegisterReconciler(rcname.NodeBashibleCleanup, &corev1.Node{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler removes the bashible first-run-finished label and the
// bashible-uninitialized taint from Nodes once bashible has completed its
// initial run. This mirrors the logic from remove_bashible_completed_labels_and_taints.go.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{bashibleCleanupPredicate()}
}

func (r *Reconciler) SetupWatches(w dynr.Watcher) {
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Get the Node.
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get node %s: %w", req.Name, err)
	}

	// 2. Check if the label exists — this is the trigger for cleanup.
	if _, hasLabel := node.Labels[bashibleFirstRunFinishedLabel]; !hasLabel {
		return ctrl.Result{}, nil
	}

	// 3. Build a patched copy: remove label and taint.
	patch := client.MergeFrom(node.DeepCopy())

	// Remove the bashible first-run-finished label.
	delete(node.Labels, bashibleFirstRunFinishedLabel)

	// Remove the bashible-uninitialized taint if present.
	taints := make([]corev1.Taint, 0, len(node.Spec.Taints))
	for _, t := range node.Spec.Taints {
		if t.Key != bashibleUninitializedTaintKey {
			taints = append(taints, t)
		}
	}
	if len(taints) == 0 {
		node.Spec.Taints = nil
	} else {
		node.Spec.Taints = taints
	}

	// Clear status before patching — we only patch metadata + spec.
	node.Status = corev1.NodeStatus{}

	// 4. Patch the node.
	if err := r.Client.Patch(ctx, node, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch bashible cleanup on node %s: %w", req.Name, err)
	}

	log.Info("removed bashible artifacts", "node", req.Name)
	return ctrl.Result{}, nil
}

// bashibleCleanupPredicate filters Node events to only process nodes that have
// the bashible-first-run-finished label.
func bashibleCleanupPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			_, ok := e.Object.GetLabels()[bashibleFirstRunFinishedLabel]
			return ok
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			_, ok := e.ObjectNew.GetLabels()[bashibleFirstRunFinishedLabel]
			return ok
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
	}
}
