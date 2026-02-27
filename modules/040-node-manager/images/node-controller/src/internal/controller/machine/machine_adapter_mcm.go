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

package machine

import (
	"context"
	"fmt"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const mcmNodeGroupLabelKey = "node.deckhouse.io/group"

type mcmMachine struct {
	machine *mcmv1alpha1.Machine
}

func (m *mcmMachine) GetName() string {
	return m.machine.GetName()
}

func (m *mcmMachine) GetNodeName() string {
	return m.machine.Status.Node
}

func (m *mcmMachine) GetNodeGroup() string {
	return m.machine.Spec.NodeTemplateSpec.Labels[mcmNodeGroupLabelKey]
}

func (m *mcmMachine) GetMachineRef() *deckhousev1alpha2.MachineRef {
	return newMachineRef(mcmv1alpha1.SchemeGroupVersion.String(), m.machine.Name)
}

func (m *mcmMachine) GetStatus() MachineStatus {
	return MachineStatus{
		Phase:         deckhousev1alpha2.InstancePhaseUnknown,
		MachineStatus: MachineStatusProgressing,
	}
}

func (m *mcmMachine) Exists(ctx context.Context, c client.Client) (bool, error) {
	return true, nil
}

func (m *mcmMachine) EnsureDeleted(ctx context.Context, c client.Client) (bool, error) {
	if !m.machine.DeletionTimestamp.IsZero() {
		return false, nil
	}

	if err := c.Delete(ctx, m.machine); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("delete mcm machine %q: %w", m.machine.Name, err)
	}

	return false, nil
}
