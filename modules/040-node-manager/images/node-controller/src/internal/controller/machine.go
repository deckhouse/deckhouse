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
	"fmt"

	capi "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	mcmNodeGroupLabelKey     = "node.deckhouse.io/group"
	capiNodeGroupLabelKey    = "node-group"
	capiFallbackNodeGroupKey = "node.deckhouse.io/group"
)

// MachineFactory creates adapters for concrete machine APIs.
type MachineFactory interface {
	NewMachine(obj client.Object) (Machine, error)
}

// Machine is a normalized machine adapter.
type Machine interface {
	GetStatus() MachineStatus
	GetNodeGroup() string
}

// MachineStatus is a normalized machine status payload.
type MachineStatus struct {
	Phase string
	Raw   any
}

type machineFactory struct{}

func NewMachineFactory() MachineFactory {
	return &machineFactory{}
}

func (f *machineFactory) NewMachine(obj client.Object) (Machine, error) {
	switch m := obj.(type) {
	case *mcmv1alpha1.Machine:
		return &mcmMachine{machine: m}, nil
	case *capi.Machine:
		return &capiMachine{machine: m}, nil
	default:
		return nil, fmt.Errorf("unsupported machine type: %T", obj)
	}
}

type mcmMachine struct {
	machine *mcmv1alpha1.Machine
}

func (m *mcmMachine) GetStatus() MachineStatus {
	return MachineStatus{
		Phase: string(m.machine.Status.CurrentStatus.Phase),
		Raw:   m.machine.Status.LastOperation,
	}
}

func (m *mcmMachine) GetNodeGroup() string {
	return m.machine.Spec.NodeTemplateSpec.Labels[mcmNodeGroupLabelKey]
}

type capiMachine struct {
	machine *capi.Machine
}

func (m *capiMachine) GetStatus() MachineStatus {
	return MachineStatus{
		Phase: m.machine.Status.Phase,
		Raw:   filterCAPIConditions(m.machine.Status.Conditions),
	}
}

func (m *capiMachine) GetNodeGroup() string {
	return getCAPINodeGroup(m.machine.Labels)
}

func getCAPINodeGroup(labels map[string]string) string {
	if labels == nil {
		return ""
	}

	if ng := labels[capiNodeGroupLabelKey]; ng != "" {
		return ng
	}

	return labels[capiFallbackNodeGroupKey]
}

func filterCAPIConditions(conditions []metav1.Condition) []metav1.Condition {
	result := make([]metav1.Condition, 0, 3)

	for i := range conditions {
		conditionType := conditions[i].Type
		if !isRelevantCAPIConditionType(conditionType) {
			continue
		}

		result = append(result, conditions[i])
	}

	return result
}

func isRelevantCAPIConditionType(conditionType string) bool {
	switch conditionType {
	case capi.InfrastructureReadyCondition, capi.ReadyCondition, capi.DeletingCondition:
		return true
	default:
		return false
	}
}
