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

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *InstanceReconciler) reconcileInstanceDeletion(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
	machineGone, err := r.reconcileLinkedMachineDeletion(ctx, instance)
	if err != nil {
		return false, err
	}

	if !controllerutil.ContainsFinalizer(instance, instanceControllerFinalizer) {
		return false, nil
	}
	if !machineGone {
		return true, nil
	}

	return false, r.removeInstanceFinalizer(ctx, instance)
}

func (r *InstanceReconciler) ensureInstanceFinalizer(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	if controllerutil.ContainsFinalizer(instance, instanceControllerFinalizer) {
		return nil
	}

	updated := instance.DeepCopy()
	controllerutil.AddFinalizer(updated, instanceControllerFinalizer)
	if err := r.Patch(ctx, updated, client.MergeFrom(instance)); err != nil {
		return fmt.Errorf("ensure finalizer on instance %q: %w", instance.Name, err)
	}

	*instance = *updated
	return nil
}

func (r *InstanceReconciler) removeInstanceFinalizer(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	if !controllerutil.ContainsFinalizer(instance, instanceControllerFinalizer) {
		return nil
	}

	updated := instance.DeepCopy()
	controllerutil.RemoveFinalizer(updated, instanceControllerFinalizer)
	if err := r.Patch(ctx, updated, client.MergeFrom(instance)); err != nil {
		return fmt.Errorf("remove finalizer from instance %q: %w", instance.Name, err)
	}

	*instance = *updated
	return nil
}

func (r *InstanceReconciler) reconcileLinkedMachineDeletion(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
	ref := instance.Spec.MachineRef
	if ref == nil || ref.Name == "" {
		return true, nil
	}

	machine, err := r.machineFactory.NewMachineFromRef(ctx, r.Client, ref)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	return machine.EnsureDeleted(ctx, r.Client)
}
