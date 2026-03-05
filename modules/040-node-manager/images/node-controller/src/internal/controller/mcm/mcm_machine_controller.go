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

package mcm

import (
	"context"
	"fmt"
	"time"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/common/machine"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type MCMMachineReconciler struct {
	client.Client
	machineFactory machine.MachineFactory
}

type mcmMachineReconcileData struct {
	mcmMachine    *mcmv1alpha1.Machine
	instanceName  string
	machineRef    *deckhousev1alpha2.MachineRef
	machineStatus machine.MachineStatus
	nodeGroup     string
}

type mcmReconcileState struct {
	req        ctrl.Request
	mcmMachine *mcmv1alpha1.Machine
	data       mcmMachineReconcileData
}

type mcmReconcileStep func(ctx context.Context, state *mcmReconcileState) (done bool, result ctrl.Result, err error)

const (
	mcmMachineRequeueInterval = time.Minute
)

func SetupMCMMachineController(mgr ctrl.Manager) error {
	if err := (&MCMMachineReconciler{
		Client:         mgr.GetClient(),
		machineFactory: machine.NewMachineFactory(),
	}).
		SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup mcm machine reconciler: %w", err)
	}

	return nil
}

func (r *MCMMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.machineFactory == nil {
		return fmt.Errorf("machineFactory is required")
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("mcm-machine-controller").
		For(&mcmv1alpha1.Machine{}).
		Watches(
			&deckhousev1alpha2.Instance{},
			handler.EnqueueRequestsFromMapFunc(mapInstanceToMCMMachine()),
			builder.WithPredicates(mcmInstanceWatchPredicate()),
		).
		Complete(r)
}

func (r *MCMMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx).WithValues("mcmMachine", req.NamespacedName.String())
	ctx = ctrl.LoggerInto(ctx, logger)
	logger.V(4).Info("tick", "op", "mcm.reconcile.start")

	state := &mcmReconcileState{
		req: req,
	}

	for _, step := range []mcmReconcileStep{
		// fetch current mcm machine object from api server
		r.reconcileMCMMachineFetch,
		// delete linked instance when mcm machine object is gone
		r.reconcileMCMMachineMissingInstanceDeletion,
		// build reconcile data from machine adapter
		r.reconcileMCMMachineData,
		// reconcile linked instance spec and status from machine data
		r.reconcileMCMMachineInstance,
	} {
		done, result, err := step(ctx, state)
		if err != nil {
			return ctrl.Result{}, err
		}
		if done {
			return result, nil
		}
	}

	logger.V(1).Info("reconcile complete", "status", state.data.machineStatus, "nodeGroup", state.data.nodeGroup)
	return ctrl.Result{RequeueAfter: mcmMachineRequeueInterval}, nil
}

func (r *MCMMachineReconciler) reconcileMCMMachineFetch(
	ctx context.Context,
	state *mcmReconcileState,
) (bool, ctrl.Result, error) {
	mcmMachine := &mcmv1alpha1.Machine{}
	if err := r.Get(ctx, state.req.NamespacedName, mcmMachine); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return false, ctrl.Result{}, err
		}

		state.mcmMachine = nil
		return false, ctrl.Result{}, nil
	}

	state.mcmMachine = mcmMachine
	return false, ctrl.Result{}, nil
}

func (r *MCMMachineReconciler) reconcileMCMMachineMissingInstanceDeletion(
	ctx context.Context,
	state *mcmReconcileState,
) (bool, ctrl.Result, error) {
	if state.mcmMachine != nil {
		return false, ctrl.Result{}, nil
	}

	logger := ctrl.LoggerFrom(ctx)
	logger.V(4).Info("tick", "op", "mcm.instance.delete.request")
	deleted, err := r.deleteInstanceIfExists(ctx, state.req.Name)
	if err != nil {
		return false, ctrl.Result{}, err
	}

	logger.V(1).Info("machine not found, linked instance delete handled", "instance", state.req.Name, "deleted", deleted)
	return true, ctrl.Result{}, nil
}

func (r *MCMMachineReconciler) reconcileMCMMachineData(
	_ context.Context,
	state *mcmReconcileState,
) (bool, ctrl.Result, error) {
	if state.mcmMachine == nil {
		return false, ctrl.Result{}, fmt.Errorf("mcm machine is nil in data step")
	}

	machineObj, err := r.machineFactory.NewMachine(state.mcmMachine)
	if err != nil {
		return false, ctrl.Result{}, fmt.Errorf("build reconcile data for mcm machine %q: %w", state.mcmMachine.Name, err)
	}
	state.data = mcmMachineReconcileData{
		mcmMachine:    state.mcmMachine,
		instanceName:  machineObj.GetName(),
		machineRef:    machineObj.GetMachineRef(),
		machineStatus: machineObj.GetStatus(),
		nodeGroup:     machineObj.GetNodeGroup(),
	}

	return false, ctrl.Result{}, nil
}

func (r *MCMMachineReconciler) reconcileMCMMachineInstance(
	ctx context.Context,
	state *mcmReconcileState,
) (bool, ctrl.Result, error) {
	if err := r.reconcileLinkedInstance(ctx, state.data); err != nil {
		if apierrors.IsConflict(err) {
			return true, ctrl.Result{Requeue: true}, nil
		}

		return false, ctrl.Result{}, err
	}

	return false, ctrl.Result{}, nil
}

func mapInstanceToMCMMachine() handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		instance, ok := obj.(*deckhousev1alpha2.Instance)
		if !ok {
			return nil
		}

		ref := instance.Spec.MachineRef
		if ref == nil || ref.Name == "" {
			return nil
		}
		if ref.Kind != "" && ref.Kind != "Machine" {
			return nil
		}
		if ref.APIVersion != mcmv1alpha1.SchemeGroupVersion.String() {
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

func mcmInstanceWatchPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isMCMMachineRef(e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isMCMMachineRef(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			instance, ok := e.ObjectNew.(*deckhousev1alpha2.Instance)
			if !ok || !isMCMMachineRef(instance) {
				return false
			}

			// self-heal - status is empty
			if instance.Status.Phase == "" || instance.Status.MachineStatus == "" {
				return true
			}

			// self-heal - condition is empty
			_, hasMachineReady := getConditionByType(instance.Status.Conditions, deckhousev1alpha2.InstanceConditionTypeMachineReady)
			return !hasMachineReady
		},
	}
}

func isMCMMachineRef(obj client.Object) bool {
	instance, ok := obj.(*deckhousev1alpha2.Instance)
	if !ok || instance == nil {
		return false
	}

	ref := instance.Spec.MachineRef
	if ref == nil || ref.Name == "" {
		return false
	}
	if ref.Kind != "" && ref.Kind != "Machine" {
		return false
	}

	return ref.APIVersion == mcmv1alpha1.SchemeGroupVersion.String()
}
