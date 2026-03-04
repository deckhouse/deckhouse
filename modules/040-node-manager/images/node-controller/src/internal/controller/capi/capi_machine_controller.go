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

package capi

import (
	"context"
	"fmt"
	"time"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
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

type CAPIMachineReconciler struct {
	client.Client
	machineFactory machine.MachineFactory
}

type capiMachineReconcileData struct {
	capiMachine   *capiv1beta2.Machine
	instanceName  string
	nodeName      string
	machineRef    *deckhousev1alpha2.MachineRef
	machineStatus machine.MachineStatus
	nodeGroup     string
}

type capiReconcileState struct {
	req         ctrl.Request
	capiMachine *capiv1beta2.Machine
	data        capiMachineReconcileData
}

type capiReconcileStep func(ctx context.Context, state *capiReconcileState) (done bool, result ctrl.Result, err error)

const capiMachineRequeueInterval = time.Minute

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
		Watches(
			&deckhousev1alpha2.Instance{},
			handler.EnqueueRequestsFromMapFunc(mapInstanceToCAPIMachine()),
			builder.WithPredicates(capiInstanceWatchPredicate()),
		).
		Complete(r)
}

func (r *CAPIMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("capiMachine", req.NamespacedName.String())
	ctx = ctrl.LoggerInto(ctx, log)
	log.V(4).Info("tick", "op", "capi.reconcile.start")

	state := &capiReconcileState{
		req: req,
	}

	for _, step := range []capiReconcileStep{
		r.reconcileCAPIMachineFetch,
		r.reconcileCAPIMachineMissingInstanceDeletion,
		r.reconcileCAPIMachineData,
		r.reconcileCAPIMachineInstance,
	} {
		done, result, err := step(ctx, state)
		if err != nil {
			return ctrl.Result{}, err
		}
		if done {
			return result, nil
		}
	}

	log.V(1).Info("reconcile complete", "status", state.data.machineStatus, "nodeGroup", state.data.nodeGroup)
	return ctrl.Result{RequeueAfter: capiMachineRequeueInterval}, nil
}

func (r *CAPIMachineReconciler) reconcileCAPIMachineFetch(
	ctx context.Context,
	state *capiReconcileState,
) (bool, ctrl.Result, error) {
	capiMachine := &capiv1beta2.Machine{}
	if err := r.Get(ctx, state.req.NamespacedName, capiMachine); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return false, ctrl.Result{}, err
		}

		state.capiMachine = nil
		return false, ctrl.Result{}, nil
	}

	state.capiMachine = capiMachine
	return false, ctrl.Result{}, nil
}

func (r *CAPIMachineReconciler) reconcileCAPIMachineMissingInstanceDeletion(
	ctx context.Context,
	state *capiReconcileState,
) (bool, ctrl.Result, error) {
	if state.capiMachine != nil {
		return false, ctrl.Result{}, nil
	}

	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("tick", "op", "capi.instance.delete.request")
	deleted, err := r.deleteInstanceIfExists(ctx, state.req.Name)
	if err != nil {
		return false, ctrl.Result{}, err
	}

	log.V(1).Info("machine not found, linked instance delete handled", "instance", state.req.Name, "deleted", deleted)
	return true, ctrl.Result{}, nil
}

func (r *CAPIMachineReconciler) reconcileCAPIMachineData(
	_ context.Context,
	state *capiReconcileState,
) (bool, ctrl.Result, error) {
	if state.capiMachine == nil {
		return false, ctrl.Result{}, fmt.Errorf("capi machine is nil in data step")
	}

	machineObj, err := r.machineFactory.NewMachine(state.capiMachine)
	if err != nil {
		return false, ctrl.Result{}, fmt.Errorf("build reconcile data for capi machine %q: %w", state.capiMachine.Name, err)
	}
	state.data = capiMachineReconcileData{
		capiMachine:   state.capiMachine,
		instanceName:  machineObj.GetName(),
		nodeName:      machineObj.GetNodeName(),
		machineRef:    machineObj.GetMachineRef(),
		machineStatus: machineObj.GetStatus(),
		nodeGroup:     machineObj.GetNodeGroup(),
	}

	return false, ctrl.Result{}, nil
}

func (r *CAPIMachineReconciler) reconcileCAPIMachineInstance(
	ctx context.Context,
	state *capiReconcileState,
) (bool, ctrl.Result, error) {
	if err := r.reconcileLinkedInstance(ctx, state.data); err != nil {
		if apierrors.IsConflict(err) {
			return true, ctrl.Result{Requeue: true}, nil
		}

		return false, ctrl.Result{}, err
	}

	return false, ctrl.Result{}, nil
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
		if ref.Kind != "" && ref.Kind != "Machine" {
			return nil
		}
		if ref.APIVersion != capiv1beta2.GroupVersion.String() {
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

func capiInstanceWatchPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isCAPIMachineRef(e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isCAPIMachineRef(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			instance, ok := e.ObjectNew.(*deckhousev1alpha2.Instance)
			if !ok || !isCAPIMachineRef(instance) {
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

func isCAPIMachineRef(obj client.Object) bool {
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

	return ref.APIVersion == capiv1beta2.GroupVersion.String()
}
