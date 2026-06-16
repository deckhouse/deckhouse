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

// Package setreplicas keeps the replicas of MCM and CAPI MachineDeployments
// within the min/max bounds declared by their NodeGroup. It is the
// controller-runtime port of the set_replicas_on_machine_deployment hook.
package setreplicas

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	"github.com/deckhouse/node-controller/internal/register"
)

type reconciler struct {
	register.Base
}

var _ register.Reconciler = (*reconciler)(nil)

func (r *reconciler) SetupWatches(w register.Watcher) {
	w.Watches(ngcommon.NewUnstructured(ngcommon.MCMMachineDeploymentGVK), handler.EnqueueRequestsFromMapFunc(ngcommon.MachineDeploymentToNodeGroup))
	w.Watches(ngcommon.NewUnstructured(ngcommon.CAPIMachineDeploymentGVK), handler.EnqueueRequestsFromMapFunc(ngcommon.MachineDeploymentToNodeGroup))
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var ng deckhousev1.NodeGroup
	if err := r.Client.Get(ctx, req.NamespacedName, &ng); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	minReplicas, maxReplicas := nodeGroupBounds(&ng)

	for _, gvk := range []schema.GroupVersionKind{ngcommon.MCMMachineDeploymentGVK, ngcommon.CAPIMachineDeploymentGVK} {
		if err := r.alignReplicas(ctx, gvk, ng.Name, minReplicas, maxReplicas); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *reconciler) alignReplicas(ctx context.Context, gvk schema.GroupVersionKind, ngName string, minReplicas, maxReplicas int32) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
	if err := r.Client.List(ctx, list, client.InNamespace(ngcommon.MachineNamespace), client.MatchingLabels{"node-group": ngName}); err != nil {
		return fmt.Errorf("list %s for node group %s: %w", gvk.Kind, ngName, err)
	}

	for i := range list.Items {
		md := &list.Items[i]
		current, _, _ := unstructured.NestedInt64(md.Object, "spec", "replicas")
		desired := desiredReplicas(int32(current), minReplicas, maxReplicas)
		if desired == int32(current) {
			continue
		}

		target := ngcommon.NewUnstructured(gvk)
		target.SetNamespace(md.GetNamespace())
		target.SetName(md.GetName())
		patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, desired))
		if err := r.Client.Patch(ctx, target, client.RawPatch(types.MergePatchType, patch)); err != nil {
			return fmt.Errorf("patch replicas on %s %s: %w", gvk.Kind, md.GetName(), err)
		}
	}

	return nil
}

func nodeGroupBounds(ng *deckhousev1.NodeGroup) (int32, int32) {
	var minReplicas, maxReplicas int32
	if ng.Spec.StaticInstances != nil && ng.Spec.StaticInstances.Count != nil {
		minReplicas = *ng.Spec.StaticInstances.Count
		maxReplicas = *ng.Spec.StaticInstances.Count
	}
	if ng.Spec.CloudInstances != nil {
		minReplicas = ng.Spec.CloudInstances.MinPerZone
		maxReplicas = ng.Spec.CloudInstances.MaxPerZone
	}
	return minReplicas, maxReplicas
}

func desiredReplicas(current, minReplicas, maxReplicas int32) int32 {
	switch {
	case minReplicas >= maxReplicas:
		return maxReplicas
	case current == 0:
		return minReplicas
	case current <= minReplicas:
		return minReplicas
	case current > maxReplicas:
		return maxReplicas
	default:
		return current
	}
}

func init() {
	register.RegisterController("set-replicas-on-machine-deployment", &deckhousev1.NodeGroup{}, &reconciler{})
}
