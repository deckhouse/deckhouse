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
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	mcmNodeGroupLabelKey  = "node.deckhouse.io/group"
	capiNodeGroupLabelKey = "node-group"

	MachineStatusProgressing = "Progressing"
	MachineStatusReady       = "Ready"
	MachineStatusBlocked     = "Blocked"
	MachineStatusRebooting   = "Rebooting"
	MachineStatusError       = "Error"

	machineReadyConditionType       = "MachineReady"
	waitingForInfrastructureMessage = "Waiting for infrastructure"
	reasonWaitingForInfra           = "WaitingForInfrastructure"
	reasonNotReady                  = "NotReady"
	reasonReady                     = "Ready"

	mcmAdapterNotImplementedMessage = "MCM machine adapter is not implemented yet"
)

type MachineFactory interface {
	NewMachine(obj client.Object) (Machine, error)
}

type Machine interface {
	GetName() string
	GetNodeName() string
	GetNodeGroup() string
	GetMachineRef() *deckhousev1alpha2.MachineRef
	GetStatus() MachineStatus
}

type MachineStatus struct {
	Phase         deckhousev1alpha2.InstancePhase
	MachineStatus string
	Message       string
	Conditions    []deckhousev1alpha2.InstanceCondition
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
	return &deckhousev1alpha2.MachineRef{
		Kind:       "Machine",
		APIVersion: mcmv1alpha1.SchemeGroupVersion.String(),
		Name:       m.machine.Name,
		Namespace:  MachineNamespace,
	}
}

func (m *mcmMachine) GetStatus() MachineStatus {
	return MachineStatus{
		Phase:         deckhousev1alpha2.InstancePhaseUnknown,
		MachineStatus: MachineStatusProgressing,
		Message:       mcmAdapterNotImplementedMessage,
		Conditions:    nil,
	}
}

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
	return m.machine.Labels[capiNodeGroupLabelKey]
}

func (m *capiMachine) GetMachineRef() *deckhousev1alpha2.MachineRef {
	return &deckhousev1alpha2.MachineRef{
		Kind:       "Machine",
		APIVersion: capi.GroupVersion.String(),
		Name:       m.machine.Name,
		Namespace:  MachineNamespace,
	}
}

func (m *capiMachine) GetStatus() MachineStatus {
	phase := m.calculatePhase()
	state := m.calculateState()

	condition := deckhousev1alpha2.InstanceCondition{
		Type:     machineReadyConditionType,
		Status:   state.conditionStatus,
		Reason:   state.reason,
		Message:  state.message,
		Severity: state.severity,
	}
	if state.sourceCondition != nil {
		condition.LastTransitionTime = state.sourceCondition.LastTransitionTime
		condition.ObservedGeneration = state.sourceCondition.ObservedGeneration
	}

	return MachineStatus{
		Phase:         phase,
		MachineStatus: state.statusString,
		Message:       state.message,
		Conditions:    []deckhousev1alpha2.InstanceCondition{condition},
	}
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

type machineState struct {
	statusString    string
	conditionStatus metav1.ConditionStatus
	reason          string
	message         string
	severity        string
	sourceCondition *metav1.Condition
}

type machineStatePriority int

const (
	machineStatePriorityNone machineStatePriority = iota
	machineStatePriorityReady
	machineStatePriorityDeleting
	machineStatePriorityInfraWait
	machineStatePriorityInfraProblem
)

type capiConditionRefs struct {
	infra    *metav1.Condition
	deleting *metav1.Condition
	ready    *metav1.Condition
}

func (m *capiMachine) calculateState() machineState {
	refs := indexMachineConditions(m.machine.Status.Conditions)
	switch m.detectMachineStatePriority(refs) {
	case machineStatePriorityInfraWait:
		return m.stateFromInfrastructureWait(refs.infra)
	case machineStatePriorityInfraProblem:
		return m.stateFromInfrastructureProblem(refs.infra)
	case machineStatePriorityDeleting:
		return m.stateFromDeleting(refs.deleting)
	case machineStatePriorityReady:
		return m.stateFromReady(refs.ready)
	}

	return defaultMachineState()
}

func indexMachineConditions(conditions []metav1.Condition) capiConditionRefs {
	var refs capiConditionRefs
	for i := range conditions {
		c := &conditions[i]
		switch c.Type {
		case capi.InfrastructureReadyCondition:
			refs.infra = c
		case capi.DeletingCondition:
			refs.deleting = c
		case capi.ReadyCondition:
			refs.ready = c
		}
	}
	return refs
}

func (m *capiMachine) detectMachineStatePriority(refs capiConditionRefs) machineStatePriority {
	if refs.infra != nil && refs.infra.Status == metav1.ConditionFalse {
		if m.isExpectedInfrastructureWait(refs.infra) {
			return machineStatePriorityInfraWait
		}
		return machineStatePriorityInfraProblem
	}
	if refs.deleting != nil && refs.deleting.Status == metav1.ConditionTrue {
		return machineStatePriorityDeleting
	}
	if refs.ready != nil {
		return machineStatePriorityReady
	}
	return machineStatePriorityNone
}

func (m *capiMachine) stateFromInfrastructureWait(infra *metav1.Condition) machineState {
	reason := infra.Reason
	if reason == "" {
		reason = reasonWaitingForInfra
	}
	return machineState{
		statusString:    MachineStatusProgressing,
		conditionStatus: metav1.ConditionFalse,
		reason:          reason,
		message:         waitingForInfrastructureMessage,
		severity:        string(capi.ConditionSeverityInfo),
		sourceCondition: infra,
	}
}

func (m *capiMachine) stateFromInfrastructureProblem(infra *metav1.Condition) machineState {
	return machineState{
		statusString:    MachineStatusProgressing,
		conditionStatus: metav1.ConditionFalse,
		reason:          infra.Reason,
		message:         conditionMessageOrReason(infra),
		severity:        string(capi.ConditionSeverityWarning),
		sourceCondition: infra,
	}
}

func (m *capiMachine) stateFromDeleting(deleting *metav1.Condition) machineState {
	if isDrainBlockedDeletingCondition(deleting) {
		return machineState{
			statusString:    MachineStatusBlocked,
			conditionStatus: metav1.ConditionFalse,
			reason:          deleting.Reason,
			message:         deleting.Message,
			severity:        string(capi.ConditionSeverityWarning),
			sourceCondition: deleting,
		}
	}

	return machineState{
		statusString:    MachineStatusProgressing,
		conditionStatus: metav1.ConditionFalse,
		reason:          deleting.Reason,
		message:         conditionMessageOrReason(deleting),
		severity:        string(capi.ConditionSeverityInfo),
		sourceCondition: deleting,
	}
}

func (m *capiMachine) stateFromReady(ready *metav1.Condition) machineState {
	if ready.Status == metav1.ConditionTrue {
		return machineState{
			statusString:    MachineStatusReady,
			conditionStatus: metav1.ConditionTrue,
			reason:          reasonReady,
			message:         ready.Message,
			sourceCondition: ready,
		}
	}

	return machineState{
		statusString:    MachineStatusProgressing,
		conditionStatus: metav1.ConditionFalse,
		reason:          ready.Reason,
		message:         conditionMessageOrReason(ready),
		severity:        string(capi.ConditionSeverityInfo),
		sourceCondition: ready,
	}
}

func defaultMachineState() machineState {
	return machineState{
		statusString:    MachineStatusProgressing,
		conditionStatus: metav1.ConditionUnknown,
		reason:          reasonWaitingForInfra,
		message:         waitingForInfrastructureMessage,
		severity:        string(capi.ConditionSeverityInfo),
	}
}

func conditionMessageOrReason(c *metav1.Condition) string {
	if c == nil {
		return ""
	}
	if c.Message != "" {
		return c.Message
	}
	return c.Reason
}

func isDrainBlockedDeletingCondition(c *metav1.Condition) bool {
	if c.Reason != capi.MachineDeletingDrainingNodeReason {
		return false
	}
	return c.Message != ""
}

func (m *capiMachine) isExpectedInfrastructureWait(c *metav1.Condition) bool {
	if c.Reason == reasonWaitingForInfra {
		return true
	}
	if c.Reason != reasonNotReady {
		return false
	}

	switch capi.MachinePhase(m.machine.Status.Phase) {
	case capi.MachinePhasePending, capi.MachinePhaseProvisioning:
		return true
	default:
		return false
	}
}
