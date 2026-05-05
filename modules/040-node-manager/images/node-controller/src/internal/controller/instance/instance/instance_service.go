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

	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

type InstanceService struct {
	client         client.Client
	machineFactory machine.MachineFactory
}

func NewInstanceService(c client.Client) *InstanceService {
	return &InstanceService{
		client:         c,
		machineFactory: machine.NewMachineFactory(),
	}
}

func (s *InstanceService) ReconcileFinalization(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
	machineGone, err := s.reconcileLinkedMachineDeletion(ctx, instance)
	if err != nil {
		return false, err
	}
	if err := s.finalizeAfterMachineDeletion(ctx, instance, machineGone); err != nil {
		return false, err
	}
	return machineGone, nil
}
