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

	capi "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type capiMachine struct {
	machine *capi.Machine
}

func (m *capiMachine) GetName() string {
	return m.machine.GetName()
}

func (m *capiMachine) GetNodeName() string {
	return m.machine.Status.NodeRef.Name
}

func (m *capiMachine) GetNodeGroup() string {
	if m.machine.Labels == nil {
		return ""
	}
	return m.machine.Labels["node-group"]
}

func (m *capiMachine) GetMachineRef() *deckhousev1alpha2.MachineRef {
	return newMachineRef(capi.GroupVersion.String(), m.machine.Name)
}

func (m *capiMachine) GetStatus() MachineStatus {
	phase := m.calculatePhase()
	state := calculateCAPIState(
		m.machine.Status.Conditions,
		capi.MachinePhase(m.machine.Status.Phase),
	)
	condition := buildMachineReadyCondition(state)

	return MachineStatus{
		Phase:                 phase,
		MachineStatus:         state.statusString,
		MachineReadyCondition: &condition,
	}
}

func (m *capiMachine) EnsureDeleted(ctx context.Context, c client.Client) (bool, error) {
	if !m.machine.DeletionTimestamp.IsZero() {
		return false, nil
	}

	if err := c.Delete(ctx, m.machine); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("delete capi machine %q: %w", m.machine.Name, err)
	}

	return false, nil
}

func (m *capiMachine) calculatePhase() deckhousev1alpha2.InstancePhase {
	if !m.machine.DeletionTimestamp.IsZero() {
		return deckhousev1alpha2.InstancePhaseTerminating
	}

	switch capi.MachinePhase(m.machine.Status.Phase) {
	case capi.MachinePhasePending:
		return deckhousev1alpha2.InstancePhasePending
	case capi.MachinePhaseProvisioning:
		return deckhousev1alpha2.InstancePhaseProvisioning
	case capi.MachinePhaseProvisioned:
		return deckhousev1alpha2.InstancePhaseProvisioned
	case capi.MachinePhaseRunning:
		return deckhousev1alpha2.InstancePhaseRunning
	case capi.MachinePhaseDeleting, capi.MachinePhaseDeleted:
		return deckhousev1alpha2.InstancePhaseTerminating
	default:
		return deckhousev1alpha2.InstancePhaseUnknown
	}
}
