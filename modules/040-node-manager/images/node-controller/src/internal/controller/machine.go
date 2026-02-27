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
	"strings"

	capi "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MachineStatusProgressing = "Progressing"
	MachineStatusReady       = "Ready"
	MachineStatusBlocked     = "Blocked"
)

const (
	mcmNodeGroupLabelKey = "node.deckhouse.io/group"
)

const (
	machineReadyConditionType       = "MachineReady"
	waitingForInfrastructureMessage = "Waiting for infrastructure"
	reasonWaitingForInfra           = "WaitingForInfrastructure"
	reasonNotReady                  = "NotReady"
	reasonReady                     = "Ready"
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

func newMachineRef(apiVersion, name string) *deckhousev1alpha2.MachineRef {
	return &deckhousev1alpha2.MachineRef{
		Kind:       "Machine",
		APIVersion: apiVersion,
		Name:       name,
		Namespace:  MachineNamespace,
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
	return newMachineRef(mcmv1alpha1.SchemeGroupVersion.String(), m.machine.Name)
}

func (m *mcmMachine) GetStatus() MachineStatus {
	return MachineStatus{
		Phase:         deckhousev1alpha2.InstancePhaseUnknown,
		MachineStatus: MachineStatusProgressing,
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

	return MachineStatus{
		Phase:         phase,
		MachineStatus: state.statusString,
		Conditions:    []deckhousev1alpha2.InstanceCondition{buildMachineReadyCondition(state)},
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

type capiConditionRefs struct {
	infra    *metav1.Condition
	deleting *metav1.Condition
	ready    *metav1.Condition
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

func calculateCAPIState(conditions []metav1.Condition, phase capi.MachinePhase) machineState {
	refs := indexMachineConditions(conditions)

	if refs.infra != nil && refs.infra.Status == metav1.ConditionFalse {
		return stateFromInfra(phase, refs.infra)
	}
	if refs.deleting != nil && refs.deleting.Status == metav1.ConditionTrue {
		return stateFromDeleting(refs.deleting)
	}
	if phase == capi.MachinePhaseRunning {
		return machineState{
			statusString:    MachineStatusReady,
			conditionStatus: metav1.ConditionTrue,
			reason:          reasonReady,
		}
	}
	if refs.ready != nil {
		return stateFromReady(refs.ready)
	}
	return defaultMachineState()
}

func buildMachineReadyCondition(state machineState) deckhousev1alpha2.InstanceCondition {
	cond := deckhousev1alpha2.InstanceCondition{
		Type:               machineReadyConditionType,
		Status:             state.conditionStatus,
		Reason:             state.reason,
		Message:            normalizeMessage(state.message),
		Severity:           state.severity,
		LastTransitionTime: metav1.Now(),
	}
	if state.sourceCondition != nil {
		cond.LastTransitionTime = state.sourceCondition.LastTransitionTime
		cond.ObservedGeneration = state.sourceCondition.ObservedGeneration
	}
	return cond
}

func stateFromInfra(phase capi.MachinePhase, infra *metav1.Condition) machineState {
	if isExpectedInfrastructureWait(phase, infra) {
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
	return machineState{
		statusString:    MachineStatusProgressing,
		conditionStatus: metav1.ConditionFalse,
		reason:          infra.Reason,
		message:         conditionMessageOrReason(infra),
		severity:        string(capi.ConditionSeverityWarning),
		sourceCondition: infra,
	}
}

func stateFromDeleting(deleting *metav1.Condition) machineState {
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

func stateFromReady(ready *metav1.Condition) machineState {
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

func isExpectedInfrastructureWait(phase capi.MachinePhase, c *metav1.Condition) bool {
	if c.Reason == reasonWaitingForInfra {
		return true
	}
	if c.Reason != reasonNotReady {
		return false
	}
	switch phase {
	case capi.MachinePhasePending, capi.MachinePhaseProvisioning:
		return true
	default:
		return false
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
	return c.Reason == capi.MachineDeletingDrainingNodeReason && c.Message != ""
}

func normalizeMessage(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
