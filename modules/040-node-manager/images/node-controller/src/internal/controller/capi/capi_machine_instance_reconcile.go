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
	"github.com/deckhouse/node-controller/internal/controller/common"
	"github.com/deckhouse/node-controller/internal/controller/machine"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *CAPIMachineReconciler) reconcileLinkedInstance(ctx context.Context, data capiMachineReconcileData) error {
	logger := log.FromContext(ctx)

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

	logger.V(1).Info(
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

	return common.EnsureInstanceExists(ctx, r.Client, name, spec)
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
	machineStatus machine.MachineStatus,
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

func (r *CAPIMachineReconciler) deleteInstanceIfExists(ctx context.Context, name string) (bool, error) {
	instance := &deckhousev1alpha2.Instance{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := r.Delete(ctx, instance); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}
		return false, fmt.Errorf("delete instance %q: %w", name, err)
	}

	return true, nil
}

func isBeingDeleted(ts *metav1.Time) bool {
	return ts != nil && !ts.IsZero()
}
