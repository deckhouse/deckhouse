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

package controller

import (
	"context"
	"fmt"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CAPIMachineReconciler struct {
	client.Client
	machineFactory MachineFactory
}

type capiMachineReconcileData struct {
	capiMachine   *capiv1beta2.Machine
	instanceName  string
	machineRef    *deckhousev1alpha2.MachineRef
	machineStatus MachineStatus
	nodeGroup     string
}

func SetupCAPIMachineController(mgr ctrl.Manager) error {
	if err := (&CAPIMachineReconciler{
		Client:         mgr.GetClient(),
		machineFactory: NewMachineFactory(),
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
		Complete(r)
}

func (r *CAPIMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("capiMachine", req.NamespacedName.String())
	ctx = ctrl.LoggerInto(ctx, log)

	capiMachine := &capiv1beta2.Machine{}
	if err := r.Get(ctx, req.NamespacedName, capiMachine); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// req.Name == machine.GetName() == instanceName по инварианту buildReconcileData.
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

func (r *CAPIMachineReconciler) reconcileLinkedInstance(ctx context.Context, data capiMachineReconcileData) error {
	log := ctrl.LoggerFrom(ctx)

	instance, err := r.ensureInstanceExists(ctx, data.instanceName, data.machineRef)
	if err != nil {
		return err
	}

	instance, specUpdated, err := r.syncInstanceSpec(ctx, instance, data.machineRef)
	if err != nil {
		return err
	}

	machineDeleteRequested, err := r.ensureMachineDeletionForDeletingInstance(ctx, data.capiMachine, instance)
	if err != nil {
		return err
	}

	statusUpdated, err := r.syncInstanceStatus(ctx, instance, data.machineStatus)
	if err != nil {
		return err
	}

	log.V(1).Info(
		"linked instance reconciled",
		"instance", instance.Name,
		"specUpdated", specUpdated,
		"statusUpdated", statusUpdated,
		"machineDeleteRequested", machineDeleteRequested,
	)
	return nil
}

func (r *CAPIMachineReconciler) ensureInstanceExists(
	ctx context.Context,
	name string,
	machineRef *deckhousev1alpha2.MachineRef,
) (*deckhousev1alpha2.Instance, error) {
	spec := deckhousev1alpha2.InstanceSpec{
		NodeRef: deckhousev1alpha2.NodeRef{Name: name},
	}
	if machineRef != nil {
		refCopy := *machineRef
		spec.MachineRef = &refCopy
	}

	return ensureInstanceExists(ctx, r.Client, name, spec)
}

func (r *CAPIMachineReconciler) syncInstanceSpec(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
	machineRef *deckhousev1alpha2.MachineRef,
) (*deckhousev1alpha2.Instance, bool, error) {
	updated := instance.DeepCopy()
	if machineRef == nil {
		updated.Spec.MachineRef = nil
	} else {
		refCopy := *machineRef
		updated.Spec.MachineRef = &refCopy
	}

	if apiequality.Semantic.DeepEqual(instance.Spec, updated.Spec) {
		return instance, false, nil
	}

	if err := r.Patch(ctx, updated, client.MergeFrom(instance)); err != nil {
		return nil, false, fmt.Errorf("patch instance %q spec: %w", instance.Name, err)
	}
	return updated, true, nil
}

func (r *CAPIMachineReconciler) syncInstanceStatus(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
	machineStatus MachineStatus,
) (bool, error) {
	updated := instance.DeepCopy()
	updated.Status.Phase = machineStatus.Phase
	updated.Status.MachineStatus = machineStatus.MachineStatus
	updated.Status.Conditions = machineStatus.Conditions

	if apiequality.Semantic.DeepEqual(instance.Status, updated.Status) {
		return false, nil
	}

	if err := r.Status().Patch(ctx, updated, client.MergeFrom(instance)); err != nil {
		return false, fmt.Errorf("patch instance %q status: %w", instance.Name, err)
	}
	return true, nil
}

func (r *CAPIMachineReconciler) ensureMachineDeletionForDeletingInstance(
	ctx context.Context,
	capiMachine *capiv1beta2.Machine,
	instance *deckhousev1alpha2.Instance,
) (bool, error) {
	if !isBeingDeleted(instance.DeletionTimestamp) || isBeingDeleted(capiMachine.DeletionTimestamp) {
		return false, nil
	}

	if err := r.Delete(ctx, capiMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("delete capi machine %q for deleting instance %q: %w", capiMachine.Name, instance.Name, err)
	}
	return true, nil
}

func isBeingDeleted(ts *metav1.Time) bool {
	return ts != nil && !ts.IsZero()
}
