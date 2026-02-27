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

package capi

import (
	"context"
	"fmt"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/machine"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type CAPIMachineReconciler struct {
	client.Client
	machineFactory machine.MachineFactory
}

type capiMachineReconcileData struct {
	capiMachine   *capiv1beta2.Machine
	instanceName  string
	machineRef    *deckhousev1alpha2.MachineRef
	machineStatus machine.MachineStatus
	nodeGroup     string
}

func SetupCAPIMachineController(mgr ctrl.Manager) error {
	if err := (&CAPIMachineReconciler{
		Client:         mgr.GetClient(),
		machineFactory: machine.NewMachineFactory(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup capi machine reconciler: %w", err)
	}
	return nil
}

func (r *CAPIMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.machineFactory == nil {
		return fmt.Errorf("machineFactory is required")
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("capi-machine-controller").
		For(&capiv1beta2.Machine{}).
		Watches(&deckhousev1alpha2.Instance{}, handler.EnqueueRequestsFromMapFunc(mapInstanceToCAPIMachine())).
		Complete(r)
}

func (r *CAPIMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("capiMachine", req.NamespacedName.String())
	ctx = ctrl.LoggerInto(ctx, log)

	capiMachine := &capiv1beta2.Machine{}
	if err := r.Get(ctx, req.NamespacedName, capiMachine); err != nil {
		if client.IgnoreNotFound(err) == nil {
			deleted, delErr := r.deleteInstanceIfExists(ctx, req.Name)
			if delErr != nil {
				return ctrl.Result{}, delErr
			}
			log.V(1).Info("machine not found, linked instance delete handled", "instance", req.Name, "deleted", deleted)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	data, err := r.buildReconcileData(capiMachine)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("build reconcile data for capi machine %q: %w", capiMachine.Name, err)
	}

	if err := r.reconcileLinkedInstance(ctx, data); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	log.V(1).Info("reconcile complete", "status", data.machineStatus, "nodeGroup", data.nodeGroup)
	return ctrl.Result{}, nil
}

func (r *CAPIMachineReconciler) buildReconcileData(capiMachine *capiv1beta2.Machine) (capiMachineReconcileData, error) {
	machine, err := r.machineFactory.NewMachine(capiMachine)
	if err != nil {
		return capiMachineReconcileData{}, err
	}

	return capiMachineReconcileData{
		capiMachine:   capiMachine,
		instanceName:  machine.GetName(),
		machineRef:    machine.GetMachineRef(),
		machineStatus: machine.GetStatus(),
		nodeGroup:     machine.GetNodeGroup(),
	}, nil
}

func mapInstanceToCAPIMachine() handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		instance, ok := obj.(*deckhousev1alpha2.Instance)
		if !ok {
			return nil
		}

		ref := instance.Spec.MachineRef
		if ref == nil || ref.Name == "" {
			return nil
		}
		namespace := ref.Namespace
		if namespace == "" {
			namespace = machine.MachineNamespace
		}

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      ref.Name,
			},
		}}
	}
}
