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

package instance

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/predicate"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	machineNamespace = "d8-cloud-instance-manager"
	finalizerName    = "node-manager.hooks.deckhouse.io/instance-controller"
	nodeGroupLabel   = "node.deckhouse.io/group"
)

func init() {
	dynr.RegisterReconciler(rcname.Instance, &deckhousev1alpha1.Instance{}, &InstanceReconciler{})
}

var _ dynr.Reconciler = (*InstanceReconciler)(nil)

type InstanceReconciler struct {
	dynr.Base
}

func (r *InstanceReconciler) SetupWatches(w dynr.Watcher) {
	w.
		Watches(
			&mcmv1alpha1.Machine{},
			handler.EnqueueRequestsFromMapFunc(machineToInstance),
			builder.WithPredicates(predicate.InNamespace(machineNamespace)),
		).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(nodeToInstance),
		).
		Watches(
			&deckhousev1.NodeGroup{},
			handler.EnqueueRequestsFromMapFunc(nodeGroupToInstances),
		)
	// TODO: Add CAPI Machine watch when cluster-api dependency is added to go.mod
	// w.Watches(
	//     &capiv1beta1.Machine{},
	//     handler.EnqueueRequestsFromMapFunc(capiMachineToInstance),
	//     builder.WithPredicates(predicate.InNamespace(machineNamespace)),
	// )
}

func machineToInstance(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{
		{NamespacedName: client.ObjectKey{Name: obj.GetName()}},
	}
}

func nodeToInstance(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{
		{NamespacedName: client.ObjectKey{Name: obj.GetName()}},
	}
}

func nodeGroupToInstances(_ context.Context, _ client.Object) []reconcile.Request {
	return nil
}

func (r *InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Logger.WithValues("instance", req.Name)

	instance := &deckhousev1alpha1.Instance{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("get instance: %w", err)
	}
	instanceExists := err == nil

	machine := &mcmv1alpha1.Machine{}
	machineKey := client.ObjectKey{Namespace: machineNamespace, Name: req.Name}
	err = r.Client.Get(ctx, machineKey, machine)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("get mcm machine: %w", err)
	}
	machineExists := err == nil

	// TODO: Add CAPI Machine lookup when cluster-api dependency is added to go.mod

	if machineExists {
		ngName := machine.Spec.NodeTemplateSpec.Labels[nodeGroupLabel]
		if ngName == "" {
			log.Info("Machine has no node group label, skipping", "machine", machine.Name)
			return ctrl.Result{}, nil
		}

		ng := &deckhousev1.NodeGroup{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: ngName}, ng); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("NodeGroup not found", "name", ngName)
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, fmt.Errorf("get node group %s: %w", ngName, err)
		}

		if instanceExists {
			if err := r.updateInstanceStatus(ctx, instance, machine, ng); err != nil {
				return ctrl.Result{}, fmt.Errorf("update instance status: %w", err)
			}

			if err := r.handleInstanceDeletion(ctx, instance, machine); err != nil {
				return ctrl.Result{}, fmt.Errorf("handle instance deletion: %w", err)
			}
		} else {
			if err := r.createInstance(ctx, machine, ng); err != nil {
				return ctrl.Result{}, fmt.Errorf("create instance: %w", err)
			}
		}

		return ctrl.Result{}, nil
	}

	if instanceExists {
		if err := r.handleOrphanedInstance(ctx, instance); err != nil {
			return ctrl.Result{}, fmt.Errorf("handle orphaned instance: %w", err)
		}
	}

	return ctrl.Result{}, nil
}
