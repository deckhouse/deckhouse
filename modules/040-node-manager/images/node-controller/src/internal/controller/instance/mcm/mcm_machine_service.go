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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

type MCMMachineService struct {
	client         client.Client
	machineFactory machine.MachineFactory
}

func NewMCMMachineService(c client.Client) *MCMMachineService {
	return &MCMMachineService{
		client:         c,
		machineFactory: machine.NewMachineFactory(),
	}
}

func (s *MCMMachineService) EnsureInstanceFromMachine(ctx context.Context, name types.NamespacedName) (bool, error) {
	mcmMachine := &mcmv1alpha1.Machine{}
	if err := s.client.Get(ctx, name, mcmMachine); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}
		return false, err
	}

	machineObj, err := s.machineFactory.NewMachine(mcmMachine)
	if err != nil {
		return false, fmt.Errorf("build machine for mcm %q: %w", mcmMachine.Name, err)
	}

	spec := deckhousev1alpha2.InstanceSpec{}
	if ref := machineObj.GetMachineRef(); ref != nil {
		refCopy := *ref
		spec.MachineRef = &refCopy
	}
	if _, err := instancecommon.EnsureInstanceExists(ctx, s.client, machineObj.GetName(), spec); err != nil {
		return false, err
	}
	return true, nil
}
