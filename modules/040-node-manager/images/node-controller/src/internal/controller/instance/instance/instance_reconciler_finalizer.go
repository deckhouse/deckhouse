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

package instance

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
)

func (s *InstanceService) finalizeAfterMachineDeletion(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
	machineGone bool,
) error {
	if !controllerutil.ContainsFinalizer(instance, instancecommon.InstanceControllerFinalizer) {
		return nil
	}
	if !machineGone {
		return nil
	}

	return s.removeInstanceFinalizer(ctx, instance)
}

func (s *InstanceService) EnsureInstanceFinalizer(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	if controllerutil.ContainsFinalizer(instance, instancecommon.InstanceControllerFinalizer) {
		return nil
	}
	log.FromContext(ctx).V(4).Info("tick", "op", "instance.finalizer.add.patch")

	updated := instance.DeepCopy()
	controllerutil.AddFinalizer(updated, instancecommon.InstanceControllerFinalizer)
	if err := s.client.Patch(ctx, updated, client.MergeFrom(instance)); err != nil {
		return fmt.Errorf("ensure finalizer on instance %q: %w", instance.Name, err)
	}

	*instance = *updated
	return nil
}

func (s *InstanceService) removeInstanceFinalizer(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	return instancecommon.RemoveInstanceControllerFinalizer(ctx, s.client, instance)
}

func (s *InstanceService) reconcileLinkedMachineDeletion(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
	ref := instance.Spec.MachineRef
	if ref == nil || ref.Name == "" {
		return true, nil
	}

	machine, err := s.machineFactory.NewMachineFromRef(ctx, s.client, ref)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	log.FromContext(ctx).V(4).Info("tick", "op", "instance.machine.ensure_deleted")
	return machine.EnsureDeleted(ctx, s.client)
}
