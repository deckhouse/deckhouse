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

package providerid

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	nodeTypeLabel         = "node.deckhouse.io/type"
	uninitializedTaintKey = "node.cloudprovider.kubernetes.io/uninitialized"
)

func init() {
	dynr.RegisterReconciler(rcname.NodeProviderID, &corev1.Node{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler sets spec.providerID to "static://" on Static nodes that do not have
// the cloud-provider uninitialized taint and have an empty providerID.
// This mirrors the logic from the set_provider_id_on_static_nodes.go hook.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{providerIDPredicate()}
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

	// 2. Patch the node with providerID using merge patch.
	mergePatch := map[string]interface{}{
		"spec": map[string]interface{}{
			"providerID": "static://",
		},
	}

	patchData, err := json.Marshal(mergePatch)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("marshal providerID patch: %w", err)
	}

	if err := r.Client.Patch(ctx, node, client.RawPatch(types.MergePatchType, patchData)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch providerID on node %s: %w", req.Name, err)
	}

	log.Info("set providerID on static node", "node", req.Name)
	return ctrl.Result{}, nil
}

// needsProviderIDPatch returns true if the node is a Static node without
// the cloud-provider uninitialized taint and with an empty providerID.
func needsProviderIDPatch(node *corev1.Node) bool {
	// Must be a Static node.
	if node.Labels[nodeTypeLabel] != "Static" {
		return false
	}

	// Must not have providerID already set.
	if node.Spec.ProviderID != "" {
		return false
	}

	// Must not have the cloud-provider uninitialized taint.
	for _, taint := range node.Spec.Taints {
		if taint.Key == uninitializedTaintKey {
			return false
		}
	}

	return true
}

// providerIDPredicate filters Node events to only process nodes that might need
// a providerID patch: Static nodes with empty providerID.
func providerIDPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			node, ok := e.Object.(*corev1.Node)
			if !ok {
				return false
			}
			return needsProviderIDPatch(node)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			node, ok := e.ObjectNew.(*corev1.Node)
			if !ok {
				return false
			}
			return needsProviderIDPatch(node)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
	}
}
