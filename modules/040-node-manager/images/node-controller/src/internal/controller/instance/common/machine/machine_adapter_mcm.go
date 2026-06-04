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

package machine

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

const (
	mcmConditionSeverityInfo    = "Info"
	mcmConditionSeverityWarning = "Warning"
	mcmReasonReady              = "Ready"
	mcmReasonNotReady           = "NotReady"
	mcmReasonDeleting           = "Deleting"
	mcmReasonDeleteFailed       = "DeleteFailed"
	mcmReasonUnknown            = "Unknown"
)

type mcmMachine struct {
	machine *mcmv1alpha1.Machine
}

var _ Machine = (*mcmMachine)(nil)

func (m *mcmMachine) GetName() string {
	return m.machine.GetName()
}

func (m *mcmMachine) GetNodeName() string {
	return m.machine.Status.Node
}

func (m *mcmMachine) GetNodeGroup() string {
	return m.machine.Spec.NodeTemplateSpec.Labels[nodecommon.NodeGroupLabel]
}

func (m *mcmMachine) GetMachineRef() *deckhousev1alpha2.MachineRef {
	return newMachineRef(mcmv1alpha1.SchemeGroupVersion.String(), m.machine.Name)
}

func (m *mcmMachine) GetStatus() MachineStatus {
	phase := m.calculatePhase()
	state := m.calculateState(phase)
	condition := buildMachineReadyCondition(state)

	return MachineStatus{
		Phase:                 phase,
		Status:                state.status,
		MachineReadyCondition: &condition,
	}
}

func (m *mcmMachine) EnsureDeleted(ctx context.Context, c client.Client) (DeletionResult, error) {
	if !m.machine.DeletionTimestamp.IsZero() {
		return DeletionResult{Gone: false}, nil
	}

	if err := c.Delete(ctx, m.machine); err != nil {
		if apierrors.IsNotFound(err) {
			return DeletionResult{Gone: true}, nil
		}
		return DeletionResult{}, fmt.Errorf("delete mcm machine %q: %w", m.machine.Name, err)
	}

	return DeletionResult{Gone: false}, nil
}

func (m *mcmMachine) calculatePhase() deckhousev1alpha2.InstancePhase {
	switch m.machine.Status.CurrentStatus.Phase {
	case mcmv1alpha1.MachinePending, mcmv1alpha1.MachineCreating, mcmv1alpha1.MachineAvailable:
		return deckhousev1alpha2.InstancePhasePending
	case mcmv1alpha1.MachineRunning:
		return deckhousev1alpha2.InstancePhaseRunning
	case mcmv1alpha1.MachineTerminating:
		return deckhousev1alpha2.InstancePhaseTerminating
	case mcmv1alpha1.MachineUnknown, mcmv1alpha1.MachineFailed, mcmv1alpha1.MachineCrashLoopBackOff:
		return deckhousev1alpha2.InstancePhaseUnknown
	default:
		return deckhousev1alpha2.InstancePhaseUnknown
	}
}

func (m *mcmMachine) calculateState(phase deckhousev1alpha2.InstancePhase) machineState {
	lastOp := m.machine.Status.LastOperation
	sourceTransitionTime := buildMCMSourceCondition(lastOp.LastUpdateTime, m.machine.Status.CurrentStatus.LastUpdateTime)
	reason := mcmReasonFromLastOperation(lastOp, phase)
	message := normalizeMessage(lastOp.Description)

	if phase == deckhousev1alpha2.InstancePhaseTerminating {
		switch {
		case isMCMDrainBlocked(message):
			return mcmBlockedMachineState(reason, message, sourceTransitionTime)
		case lastOp.State == mcmv1alpha1.MachineStateFailed:
			return mcmErrorMachineState(reason, message, sourceTransitionTime)
		default:
			return mcmProgressingMachineState(reason, message, sourceTransitionTime)
		}
	}

	switch lastOp.State {
	case mcmv1alpha1.MachineStateSuccessful:
		if phase == deckhousev1alpha2.InstancePhaseRunning {
			return mcmReadyMachineState(message, sourceTransitionTime)
		}
		return mcmProgressingMachineState(reason, message, sourceTransitionTime)
	case mcmv1alpha1.MachineStateFailed:
		if isMCMDrainBlocked(message) {
			return mcmBlockedMachineState(reason, message, sourceTransitionTime)
		}
		return mcmErrorMachineState(reason, message, sourceTransitionTime)
	case mcmv1alpha1.MachineStateProcessing:
		return mcmProgressingMachineState(reason, message, sourceTransitionTime)
	default:
		if phase == deckhousev1alpha2.InstancePhaseRunning {
			return mcmReadyMachineState("", sourceTransitionTime)
		}

		return machineState{
			status:               StatusProgressing,
			conditionStatus:      metav1.ConditionUnknown,
			reason:               mcmReasonUnknown,
			sourceTransitionTime: sourceTransitionTime,
			severity:             mcmConditionSeverityInfo,
		}
	}
}

func mcmReadyMachineState(message string, transitionTime *metav1.Time) machineState {
	return machineState{
		status:               StatusReady,
		conditionStatus:      metav1.ConditionTrue,
		reason:               mcmReasonReady,
		message:              message,
		sourceTransitionTime: transitionTime,
	}
}

func mcmProgressingMachineState(reason, message string, transitionTime *metav1.Time) machineState {
	return machineState{
		status:               StatusProgressing,
		conditionStatus:      metav1.ConditionFalse,
		reason:               reason,
		message:              message,
		severity:             mcmConditionSeverityInfo,
		sourceTransitionTime: transitionTime,
	}
}

func mcmErrorMachineState(reason, message string, transitionTime *metav1.Time) machineState {
	return machineState{
		status:               StatusError,
		conditionStatus:      metav1.ConditionFalse,
		reason:               reason,
		message:              message,
		severity:             mcmConditionSeverityWarning,
		sourceTransitionTime: transitionTime,
	}
}

func mcmBlockedMachineState(reason, message string, transitionTime *metav1.Time) machineState {
	return machineState{
		status:               StatusBlocked,
		conditionStatus:      metav1.ConditionFalse,
		reason:               reason,
		message:              message,
		severity:             mcmConditionSeverityWarning,
		sourceTransitionTime: transitionTime,
	}
}

func mcmReasonFromLastOperation(lastOp mcmv1alpha1.LastOperation, phase deckhousev1alpha2.InstancePhase) string {
	if phase == deckhousev1alpha2.InstancePhaseTerminating {
		if lastOp.State == mcmv1alpha1.MachineStateFailed {
			return mcmReasonDeleteFailed
		}
		return mcmReasonDeleting
	}

	if phase == deckhousev1alpha2.InstancePhaseRunning && lastOp.State == mcmv1alpha1.MachineStateSuccessful {
		return mcmReasonReady
	}

	switch lastOp.State {
	case mcmv1alpha1.MachineStateSuccessful, mcmv1alpha1.MachineStateProcessing, mcmv1alpha1.MachineStateFailed:
		return mcmReasonNotReady
	default:
		return mcmReasonUnknown
	}
}

func buildMCMSourceCondition(lastOpUpdate, currentStatusUpdate metav1.Time) *metav1.Time {
	lastTransitionTime := lastOpUpdate
	if lastTransitionTime.IsZero() {
		lastTransitionTime = currentStatusUpdate
	}
	if lastTransitionTime.IsZero() {
		return nil
	}

	return &lastTransitionTime
}

func isMCMDrainBlocked(message string) bool {
	if message == "" {
		return false
	}

	msgLower := strings.ToLower(message)
	return strings.Contains(msgLower, "drain failed") ||
		strings.Contains(msgLower, "cannot evict") ||
		strings.Contains(msgLower, "disruption budget")
}
