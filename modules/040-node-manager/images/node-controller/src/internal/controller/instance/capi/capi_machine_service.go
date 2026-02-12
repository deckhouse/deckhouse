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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

type CAPIMachineService struct {
	client         client.Client
	machineFactory machine.MachineFactory
}

func NewCAPIMachineService(c client.Client) *CAPIMachineService {
	return &CAPIMachineService{
		client:         c,
		machineFactory: machine.NewMachineFactory(),
	}
}

func (s *CAPIMachineService) EnsureInstanceFromMachine(ctx context.Context, name types.NamespacedName) (bool, error) {
	capiMachine := &capiv1beta2.Machine{}
	if err := s.client.Get(ctx, name, capiMachine); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}
		return false, err
	}

	machineObj, err := s.machineFactory.NewMachine(capiMachine)
	if err != nil {
		return false, fmt.Errorf("build machine for capi %q: %w", capiMachine.Name, err)
	}

	spec := deckhousev1alpha2.InstanceSpec{}
	if nodeName := machineObj.GetNodeName(); nodeName != "" {
		spec.NodeRef = deckhousev1alpha2.NodeRef{Name: nodeName}
	}
	if ref := machineObj.GetMachineRef(); ref != nil {
		refCopy := *ref
		spec.MachineRef = &refCopy
	}
	if _, err := instancecommon.EnsureInstanceExists(ctx, s.client, machineObj.GetName(), spec); err != nil {
		return false, err
	}
	return true, nil
}
