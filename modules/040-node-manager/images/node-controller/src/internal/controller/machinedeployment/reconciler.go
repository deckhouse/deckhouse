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

package machinedeployment

import (
	"context"
	"fmt"

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
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	nodeGroupLabel             = "node-group"
	machineDeploymentNamespace = "d8-cloud-instance-manager"
)

func init() {
	dynr.RegisterReconciler(rcname.MachineDeployment, &mcmv1alpha1.MachineDeployment{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler synchronizes replicas between NodeGroup and MachineDeployment.
// When a NodeGroup's minPerZone/maxPerZone changes, the corresponding
// MachineDeployment replicas are clamped to the [min, max] range.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupWatches(w dynr.Watcher) {
	w.Watches(
		&deckhousev1.NodeGroup{},
		handler.EnqueueRequestsFromMapFunc(r.nodeGroupToMachineDeployments),
		builder.WithPredicates(nodeGroupCloudInstancesChangedPredicate()),
	)
}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, ok := obj.GetLabels()[nodeGroupLabel]
		return ok && obj.GetNamespace() == machineDeploymentNamespace
	})}
}

// nodeGroupToMachineDeployments maps a NodeGroup change to reconcile requests
// for all MachineDeployments that belong to that NodeGroup (via the "node-group" label).
func (r *Reconciler) nodeGroupToMachineDeployments(ctx context.Context, obj client.Object) []reconcile.Request {
	ng, ok := obj.(*deckhousev1.NodeGroup)
	if !ok {
		return nil
	}

	mdList := &mcmv1alpha1.MachineDeploymentList{}
	if err := r.Client.List(ctx, mdList,
		client.InNamespace(machineDeploymentNamespace),
		client.MatchingLabels{nodeGroupLabel: ng.Name},
	); err != nil {
		logf.FromContext(ctx).Error(err, "failed to list machine deployments for node group", "nodeGroup", ng.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(mdList.Items))
	for _, md := range mdList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: md.Namespace,
				Name:      md.Name,
			},
		})
	}
	return requests
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Get the MachineDeployment.
	md := &mcmv1alpha1.MachineDeployment{}
	if err := r.Client.Get(ctx, req.NamespacedName, md); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get machine deployment %s: %w", req.NamespacedName, err)
	}

	// 2. Determine the owning NodeGroup from the label.
	ngName := md.Labels[nodeGroupLabel]

	// 3. Get the NodeGroup.
	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: ngName}, ng); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("node group not found, skipping", "nodeGroup", ngName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get node group %s: %w", ngName, err)
	}

	// 4. Determine min/max from the NodeGroup spec.
	minReplicas, maxReplicas := getMinMaxReplicas(ng)

	// 5. Calculate desired replicas.
	desiredReplicas := clampReplicas(md.Spec.Replicas, minReplicas, maxReplicas)

	if desiredReplicas == md.Spec.Replicas {
		log.V(1).Info("replicas already in range, no update needed",
			"machineDeployment", req.NamespacedName,
			"replicas", md.Spec.Replicas,
			"min", minReplicas,
			"max", maxReplicas,
		)
		return ctrl.Result{}, nil
	}

	// 6. Patch the MachineDeployment replicas.
	oldReplicas := md.Spec.Replicas
	patch := client.MergeFrom(md.DeepCopy())
	md.Spec.Replicas = desiredReplicas
	if err := r.Client.Patch(ctx, md, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch machine deployment %s replicas: %w", req.NamespacedName, err)
	}

	log.Info("updated machine deployment replicas",
		"machineDeployment", req.NamespacedName,
		"oldReplicas", oldReplicas,
		"newReplicas", desiredReplicas,
		"min", minReplicas,
		"max", maxReplicas,
	)
	return ctrl.Result{}, nil
}

// getMinMaxReplicas extracts min and max replicas from the NodeGroup spec.
func getMinMaxReplicas(ng *deckhousev1.NodeGroup) (min, max int32) {
	if ng.Spec.CloudInstances != nil {
		min = ng.Spec.CloudInstances.MinPerZone
		max = ng.Spec.CloudInstances.MaxPerZone
	}

	if ng.Spec.StaticInstances != nil && ng.Spec.StaticInstances.Count != nil {
		count := *ng.Spec.StaticInstances.Count
		min = count
		max = count
	}

	return min, max
}

// clampReplicas adjusts the current replicas to fit within [min, max].
func clampReplicas(current, min, max int32) int32 {
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
