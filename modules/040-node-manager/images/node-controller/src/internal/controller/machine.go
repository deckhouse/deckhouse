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
	mcmNodeGroupLabelKey      = "node.deckhouse.io/group"
	capiNodeGroupLabelKey     = "node-group"
	capiFallbackNodeGroupKey  = "node.deckhouse.io/group"
	machineReadyConditionType = "MachineReady"

	MachineStatusProgressing = "Progressing"
	MachineStatusReady       = "Ready"
	MachineStatusBlocked     = "Blocked"
	MachineStatusRebooting   = "Rebooting"
	MachineStatusError       = "Error"

	mcmAdapterNotImplementedMessage = "MCM machine adapter is not implemented yet"
)

type MachineFactory interface {
	NewMachine(obj client.Object) (Machine, error)
}

type Machine interface {
	GetName() string
	GetNodeName() string
	GetStatus() MachineStatus
	GetNodeGroup() string
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

func (m *mcmMachine) GetStatus() MachineStatus {
	return MachineStatus{
		Phase:         deckhousev1alpha2.InstancePhaseUnknown,
		MachineStatus: MachineStatusProgressing,
		Message:       mcmAdapterNotImplementedMessage,
		Conditions:    nil,
	}
}

func (m *mcmMachine) GetNodeGroup() string {
	return m.machine.Spec.NodeTemplateSpec.Labels[mcmNodeGroupLabelKey]
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

func (m *capiMachine) GetStatus() MachineStatus {
	phase := m.calculatePhase()
	relevantConditions := m.filterConditions()
	instanceConditions := m.convertConditions(relevantConditions)
	statusStr, message := m.calculateMachineStatusAndMessage(relevantConditions)

	return MachineStatus{
		Phase:         phase,
		MachineStatus: statusStr,
		Message:       message,
		Conditions:    instanceConditions,
	}
}

func (m *capiMachine) GetNodeGroup() string {
	if m.machine.Labels == nil {
		return ""
	}
	if ng := m.machine.Labels[capiNodeGroupLabelKey]; ng != "" {
		return ng
	}
	return m.machine.Labels[capiFallbackNodeGroupKey]
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
	case capi.MachinePhaseFailed, capi.MachinePhaseUnknown:
		return deckhousev1alpha2.InstancePhaseUnknown
	default:
		return deckhousev1alpha2.InstancePhaseUnknown
	}
}

func (m *capiMachine) calculateMachineStatusAndMessage(conditions []metav1.Condition) (string, string) {
	deleting := findCondition(conditions, capi.DeletingCondition)
	if deleting != nil && deleting.Status == metav1.ConditionTrue {
		if isDrainBlocked(deleting.Message) {
			return MachineStatusBlocked, deleting.Message
		}
		return MachineStatusProgressing, deleting.Message
	}

	infra := findCondition(conditions, capi.InfrastructureReadyCondition)
	if infra != nil {
		if infra.Status == metav1.ConditionFalse {
			if infra.Reason != "WaitingForInfrastructure" {
				return MachineStatusError, conditionMessageOrReason(infra)
			}
		}
	}

	ready := findCondition(conditions, capi.ReadyCondition)
	if ready != nil {
		if ready.Status == metav1.ConditionTrue {
			return MachineStatusReady, ""
		}

		if isRebooting(ready.Reason, ready.Message) {
			return MachineStatusRebooting, ready.Message
		}

		if isDrainBlockedCondition(*ready) {
			return MachineStatusBlocked, ready.Message
		}
	}

	msg := ""
	if ready != nil {
		msg = ready.Message
	}
	if msg == "" && infra != nil {
		msg = infra.Message
	}
	return MachineStatusProgressing, msg
}

func (m *capiMachine) filterConditions() []metav1.Condition {
	var result []metav1.Condition
	for _, c := range m.machine.Status.Conditions {
		switch c.Type {
		case capi.InfrastructureReadyCondition, capi.ReadyCondition, capi.DeletingCondition:
			result = append(result, c)
		}
	}
	return result
}

func (m *capiMachine) convertConditions(conditions []metav1.Condition) []deckhousev1alpha2.InstanceCondition {
	c := aggregateMachineReadyCondition(conditions)
	if c == nil {
		return nil
	}
	return []deckhousev1alpha2.InstanceCondition{*c}
}

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

func isDrainBlocked(message string) bool {
	msg := strings.ToLower(message)
	return strings.Contains(msg, "cannot evict") ||
		strings.Contains(msg, "disruption budget") ||
		strings.Contains(msg, "pdb")
}

func isDrainBlockedCondition(c metav1.Condition) bool {
	_ = c
	// TODO
	return false
}

func isRebooting(reason, message string) bool {
	_, _ = reason, message
	// TODO
	return false
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

func aggregateMachineReadyCondition(conditions []metav1.Condition) *deckhousev1alpha2.InstanceCondition {
	deleting := findCondition(conditions, capi.DeletingCondition)
	if deleting != nil && deleting.Status == metav1.ConditionTrue {
		return machineReadyConditionFrom(
			deleting,
			metav1.ConditionFalse,
			string(capi.ConditionSeverityWarning),
			drainMessage(deleting.Message),
		)
	}

	infra := findCondition(conditions, capi.InfrastructureReadyCondition)
	ready := findCondition(conditions, capi.ReadyCondition)

	if infra != nil && infra.Status == metav1.ConditionFalse && infra.Reason != "WaitingForInfrastructure" {
		return machineReadyConditionFrom(infra, infra.Status, string(capi.ConditionSeverityWarning), infra.Message)
	}

	if ready != nil {
		severity := ""
		if ready.Status == metav1.ConditionFalse && isDrainBlocked(ready.Message) {
			severity = string(capi.ConditionSeverityWarning)
		}
		return machineReadyConditionFrom(ready, ready.Status, severity, ready.Message)
	}

	if infra != nil {
		severity := ""
		if infra.Status == metav1.ConditionFalse && infra.Reason == "WaitingForInfrastructure" {
			severity = string(capi.ConditionSeverityInfo)
		}
		return machineReadyConditionFrom(infra, infra.Status, severity, infra.Message)
	}

	return nil
}

func machineReadyConditionFrom(
	src *metav1.Condition,
	status metav1.ConditionStatus,
	severity string,
	message string,
) *deckhousev1alpha2.InstanceCondition {
	if src == nil {
		return nil
	}

	return &deckhousev1alpha2.InstanceCondition{
		Type:               machineReadyConditionType,
		Status:             status,
		Reason:             src.Reason,
		Severity:           severity,
		Message:            message,
		LastTransitionTime: src.LastTransitionTime,
		ObservedGeneration: src.ObservedGeneration,
	}
}

func drainMessage(message string) string {
	if !isDrainBlocked(message) {
		return message
	}

	for _, line := range strings.Split(message, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if line == "" {
			continue
		}
		if strings.Contains(strings.ToLower(line), "cannot evict") || strings.Contains(strings.ToLower(line), "disruption budget") {
			return line
		}
	}

	return message
}
