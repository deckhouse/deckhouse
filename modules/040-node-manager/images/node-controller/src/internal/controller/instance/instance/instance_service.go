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

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

// InstanceService provides reconcile logic for Instance objects.
// It is stateless (no Setup/Reconcile loop) and is called by the top-level controller.
type InstanceService struct {
	client         client.Client
	machineFactory machine.MachineFactory
}

// NewInstanceService creates an InstanceService with the given client and default machine factory.
func NewInstanceService(c client.Client) *InstanceService {
	return &InstanceService{
		client:         c,
		machineFactory: machine.NewMachineFactory(),
	}
}

// ReconcileHeartbeat refreshes the BashibleReady condition heartbeat based on elapsed time.
func (s *InstanceService) ReconcileHeartbeat(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (bool, ctrl.Result, error) {
	if err := s.reconcileBashibleHeartbeat(ctx, instance); err != nil {
		return false, ctrl.Result{}, err
	}
	return false, ctrl.Result{}, nil
}

// ReconcileBashibleStatus derives BashibleStatus and Message from current conditions.
func (s *InstanceService) ReconcileBashibleStatus(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (bool, ctrl.Result, error) {
	if err := s.reconcileBashibleStatus(ctx, instance); err != nil {
		return false, ctrl.Result{}, err
	}
	return false, ctrl.Result{}, nil
}

// ReconcileEnsureFinalizer ensures the controller finalizer is present on the Instance.
func (s *InstanceService) ReconcileEnsureFinalizer(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (bool, ctrl.Result, error) {
	ctrl.LoggerFrom(ctx).V(4).Info("tick", "op", "instance.reconcile.active")
	if err := s.ensureInstanceFinalizer(ctx, instance); err != nil {
		return false, ctrl.Result{}, err
	}
	return false, ctrl.Result{}, nil
}

// ReconcileSourceExistence deletes the Instance when both linked Machine and Node are gone.
func (s *InstanceService) ReconcileSourceExistence(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (bool, ctrl.Result, error) {
	deleted, err := s.reconcileLinkedSourceExistence(ctx, instance)
	if err != nil {
		return false, ctrl.Result{}, err
	}
	if deleted {
		return true, ctrl.Result{}, nil
	}
	return false, ctrl.Result{}, nil
}

// ReconcileFinalization runs the finalization flow for a deleting Instance.
// Returns fastRequeue=true when the Machine deletion is in progress and we must wait.
func (s *InstanceService) ReconcileFinalization(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (fastRequeue bool, err error) {
	machineGone, err := s.reconcileLinkedMachineDeletion(ctx, instance)
	if err != nil {
		return false, err
	}
	return s.finalizeAfterMachineDeletion(ctx, instance, machineGone)
}
