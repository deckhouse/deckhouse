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

package gpu

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	gpuEnabledLabel   = "node.deckhouse.io/gpu"
	devicePluginLabel = "node.deckhouse.io/device-gpu.config"
	nodeGroupLabel    = "node.deckhouse.io/group"
	migConfigLabel    = "nvidia.com/mig.config"
	migDisabled       = "all-disabled"
)

func init() {
	dynr.RegisterReconciler(rcname.NodeGPU, &corev1.Node{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler patches GPU-specific labels on Nodes based on the NodeGroup GPU settings.
// It manages the following labels:
//   - node.deckhouse.io/gpu — marks the node as GPU-enabled
//   - node.deckhouse.io/device-gpu.config — GPU sharing mode (timeSlicing, mig, exclusive)
//   - nvidia.com/mig.config — MIG partition configuration name
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{gpuNodePredicate()}
}

func (r *Reconciler) SetupWatches(w dynr.Watcher) {
	w.Watches(
		&deckhousev1.NodeGroup{},
		handler.EnqueueRequestsFromMapFunc(r.nodeGroupToNodes),
		builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			ng, ok := obj.(*deckhousev1.NodeGroup)
			if !ok {
				return false
			}
			return ng.Spec.GPU != nil && ng.Spec.GPU.Mode != ""
		})),
	)
}

// nodeGroupToNodes maps a NodeGroup change to reconcile requests for all Nodes in that group.
func (r *Reconciler) nodeGroupToNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	ng, ok := obj.(*deckhousev1.NodeGroup)
	if !ok {
		return nil
	}

	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList, client.MatchingLabels{nodeGroupLabel: ng.Name}); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list nodes for node group", "nodeGroup", ng.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: node.Name},
		})
	}
	return requests
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

	// 2. Get the NodeGroup for this node.
	ngName := node.Labels[nodeGroupLabel]

	// 3. Get the NodeGroup.
	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: ngName}, ng); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("node group not found, skipping", "nodeGroup", ngName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get node group %s: %w", ngName, err)
	}

	// 4. Skip if GPU is not configured on the NodeGroup.
	if ng.Spec.GPU == nil || ng.Spec.GPU.Mode == "" {
		return ctrl.Result{}, nil
	}

	gpuMode := ng.Spec.GPU.Mode

	// 5. Build the labels patch.
	labels := make(map[string]interface{})

	// Handle MIG config label.
	if ng.Spec.GPU.MIG != nil && ng.Spec.GPU.MIG.Strategy != "" {
		labels[migConfigLabel] = ng.Spec.GPU.MIG.Strategy
	} else {
		// Remove MIG label if it is set and this is not a MIG node.
		if _, hasMIG := node.Labels[migConfigLabel]; hasMIG {
			labels[migConfigLabel] = migDisabled
		}
	}

	// Handle GPU enabled and device plugin labels.
	if _, hasGPU := node.Labels[gpuEnabledLabel]; hasGPU {
		// Node already has GPU label — update device plugin config if it differs.
		if currentMode, hasDP := node.Labels[devicePluginLabel]; hasDP {
			if currentMode != gpuMode {
				labels[devicePluginLabel] = gpuMode
			}
		}
	} else {
		// Node does not have GPU label — set both.
		labels[gpuEnabledLabel] = ""
		labels[devicePluginLabel] = gpuMode
	}

	if len(labels) == 0 {
		log.V(1).Info("GPU labels already up to date", "node", req.Name, "nodeGroup", ngName)
		return ctrl.Result{}, nil
	}

	// 6. Patch the node using merge patch.
	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": labels,
		},
	}

	patchData, err := json.Marshal(mergePatch)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("marshal GPU labels patch: %w", err)
	}

	if err := r.Client.Patch(ctx, node, client.RawPatch(types.MergePatchType, patchData)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch GPU labels on node %s: %w", req.Name, err)
	}

	log.Info("applied GPU labels", "node", req.Name, "nodeGroup", ngName, "mode", gpuMode)
	return ctrl.Result{}, nil
}

// gpuNodePredicate filters Node events: only nodes that belong to a node group.
func gpuNodePredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, ok := obj.GetLabels()[nodeGroupLabel]
		return ok
	})
}
