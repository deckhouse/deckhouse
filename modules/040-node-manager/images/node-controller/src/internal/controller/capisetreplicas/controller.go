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

package capisetreplicas

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("capi-set-replicas", &deckhousev1.NodeGroup{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// Watch MCM MachineDeployments — map to owning NodeGroup.
	mcmMD := &unstructured.Unstructured{}
	mcmMD.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "MachineDeployment",
	})
	w.Watches(mcmMD, handler.EnqueueRequestsFromMapFunc(mdToNodeGroup))

	// Watch CAPI MachineDeployments — map to owning NodeGroup.
	w.Watches(&capiv1beta2.MachineDeployment{}, handler.EnqueueRequestsFromMapFunc(mdToNodeGroup))
}

func mdToNodeGroup(_ context.Context, obj client.Object) []reconcile.Request {
	ng, ok := obj.GetLabels()["node-group"]
	if !ok || ng == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ng}}}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, ng); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get NodeGroup: %w", err)
	}

	minReplicas, maxReplicas := getMinMax(ng)

	// Reconcile MCM MachineDeployments.
	if err := r.reconcileMDs(ctx, logger, ng.Name, minReplicas, maxReplicas, mcmMDGVK()); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile CAPI MachineDeployments.
	if err := r.reconcileMDs(ctx, logger, ng.Name, minReplicas, maxReplicas, capiMDGVK()); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcileMDs(ctx context.Context, logger interface{ Info(string, ...any) }, ngName string, minReplicas, maxReplicas int32, gvk schema.GroupVersionKind) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)

	if err := r.Client.List(ctx, list,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		return fmt.Errorf("list %s: %w", gvk.Kind, err)
	}

	for i := range list.Items {
		md := &list.Items[i]
		replicas, _, _ := unstructured.NestedInt64(md.Object, "spec", "replicas")
		current := int32(replicas)

		desired := calculateReplicas(current, minReplicas, maxReplicas)
		if desired == current {
			continue
		}

		patch := &unstructured.Unstructured{}
		patch.SetGroupVersionKind(md.GroupVersionKind())
		patch.SetName(md.GetName())
		patch.SetNamespace(md.GetNamespace())
		if err := unstructured.SetNestedField(patch.Object, int64(desired), "spec", "replicas"); err != nil {
			return fmt.Errorf("set replicas field: %w", err)
		}

		if err := r.Client.Patch(ctx, patch, client.Apply, client.FieldOwner("capi-set-replicas"), client.ForceOwnership); err != nil {
			return fmt.Errorf("patch %s/%s replicas: %w", gvk.Kind, md.GetName(), err)
		}
		logger.Info("patched replicas", "kind", gvk.Kind, "name", md.GetName(), "from", current, "to", desired)
	}
	return nil
}

func getMinMax(ng *deckhousev1.NodeGroup) (min, max int32) {
	if ng.Spec.StaticInstances != nil && ng.Spec.StaticInstances.Count != nil {
		count := *ng.Spec.StaticInstances.Count
		return count, count
	}
	if ng.Spec.CloudInstances != nil {
		min = ng.Spec.CloudInstances.MinPerZone
		max = ng.Spec.CloudInstances.MaxPerZone
	}
	return min, max
}

func calculateReplicas(current, min, max int32) int32 {
	switch {
	case min >= max:
		return max
	case current == 0:
		return min
	case current <= min:
		return min
	case current > max:
		return max
	default:
		return current
	}
}

func mcmMDGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "MachineDeploymentList"}
}

func capiMDGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineDeploymentList"}
}
