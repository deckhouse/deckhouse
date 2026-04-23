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

package v1alpha1

import (
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (op *ControlPlaneOperation) IsCompleted() bool {
	cond := op.GetCondition(CPOConditionCompleted)
	return cond != nil && cond.Status == metav1.ConditionTrue && cond.Reason == CPOReasonOperationCompleted
}

func (op *ControlPlaneOperation) IsFailed() bool {
	cond := op.GetCondition(CPOConditionCompleted)
	return cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == CPOReasonOperationFailed
}

func (op *ControlPlaneOperation) IsAbandoned() bool {
	cond := op.GetCondition(CPOConditionCompleted)
	return cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == CPOReasonOperationAbandoned
}

// IsTerminal reports whether the operation reached a not retryable state.
func (op *ControlPlaneOperation) IsTerminal() bool {
	return op.IsCompleted() || op.IsFailed() || op.IsAbandoned()
}

func (op *ControlPlaneOperation) IsStepCompleted(name StepName) bool {
	cond := op.GetCondition(StepConditionType(name))
	return cond != nil && cond.Status == metav1.ConditionTrue && cond.Reason == CPOReasonStepCompleted
}

func (op *ControlPlaneOperation) IsStepInProgress(name StepName) bool {
	cond := op.GetCondition(StepConditionType(name))
	return cond != nil && cond.Status == metav1.ConditionFalse && cond.Reason == CPOReasonStepInProgress
}

func (op *ControlPlaneOperation) FailureMessage() string {
	cond := op.GetCondition(CPOConditionCompleted)
	if cond != nil && cond.Reason == CPOReasonOperationFailed {
		return cond.Message
	}
	return ""
}

// OperationState wraps a ControlPlaneOperation for tracking status changes for patching
type OperationState struct {
	op       *ControlPlaneOperation
	original *ControlPlaneOperation
}

func NewOperationState(op *ControlPlaneOperation) *OperationState {
	return &OperationState{op: op, original: op.DeepCopy()}
}

func (s *OperationState) IsCompleted() bool { return s.op.IsCompleted() }
func (s *OperationState) IsFailed() bool    { return s.op.IsFailed() }
func (s *OperationState) IsTerminal() bool  { return s.op.IsTerminal() }
func (s *OperationState) IsAbandoned() bool { return s.op.IsAbandoned() }
func (s *OperationState) IsStepCompleted(name StepName) bool {
	return s.op.IsStepCompleted(name)
}
func (s *OperationState) IsStepInProgress(name StepName) bool {
	return s.op.IsStepInProgress(name)
}
func (s *OperationState) FailureMessage() string { return s.op.FailureMessage() }

func (s *OperationState) Raw() *ControlPlaneOperation      { return s.op }
func (s *OperationState) Original() *ControlPlaneOperation { return s.original }

func (s *OperationState) HasStatusChanges() bool {
	return !reflect.DeepEqual(s.original.Status, s.op.Status)
}

// ResetBaseline resets the original status to the current status.
func (s *OperationState) ResetBaseline() {
	s.original = s.op.DeepCopy()
}

// Write accessors
func (s *OperationState) SetCondition(cond metav1.Condition) {
	meta.SetStatusCondition(&s.op.Status.Conditions, cond)
}

// EnsureInitialConditions populates missing operation and step conditions with Unknown state.
// Existing conditions are not modified.
func (s *OperationState) EnsureInitialConditions() {
	if s.op.GetCondition(CPOConditionCompleted) == nil {
		s.SetCondition(metav1.Condition{
			Type:   CPOConditionCompleted,
			Status: metav1.ConditionUnknown,
			Reason: CPOReasonOperationUnknown,
		})
	}

	for _, name := range s.op.Spec.Steps {
		conditionType := StepConditionType(name)
		if s.op.GetCondition(conditionType) != nil {
			continue
		}
		s.SetCondition(metav1.Condition{
			Type:   conditionType,
			Status: metav1.ConditionFalse,
			Reason: CPOReasonStepUnknown,
		})
	}
}

func (s *OperationState) MarkStepInProgress(name StepName) {
	s.MarkStepInProgressWithMessage(name, "")
}

func (s *OperationState) MarkStepInProgressWithMessage(name StepName, message string) {
	s.SetCondition(metav1.Condition{
		Type:    StepConditionType(name),
		Status:  metav1.ConditionFalse,
		Reason:  CPOReasonStepInProgress,
		Message: message,
	})
}

func (s *OperationState) MarkStepCompleted(name StepName) {
	s.MarkStepCompletedWithMessage(name, "")
}

func (s *OperationState) MarkStepCompletedWithMessage(name StepName, message string) {
	s.SetCondition(metav1.Condition{
		Type:    StepConditionType(name),
		Status:  metav1.ConditionTrue,
		Reason:  CPOReasonStepCompleted,
		Message: message,
	})
}

func (s *OperationState) MarkStepFailed(name StepName, message string) {
	s.SetCondition(metav1.Condition{
		Type:    StepConditionType(name),
		Status:  metav1.ConditionFalse,
		Reason:  CPOReasonStepFailed,
		Message: message,
	})
}

func (s *OperationState) setOperationCondition(reason, message string) {
	s.SetCondition(metav1.Condition{
		Type:    CPOConditionCompleted,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: message,
	})
}

func (s *OperationState) MarkOperationInProgress(message string) {
	s.setOperationCondition(CPOReasonOperationInProgress, message)
}

func (s *OperationState) MarkOperationAbandoned(message string) {
	s.markCurrentInProgressStepAbandoned()
	s.setOperationCondition(CPOReasonOperationAbandoned, message)
}

func (s *OperationState) MarkOperationFailed(message string) {
	s.setOperationCondition(CPOReasonOperationFailed, message)
}

func (s *OperationState) MarkOperationCompleted() {
	s.SetCondition(metav1.Condition{
		Type:    CPOConditionCompleted,
		Status:  metav1.ConditionTrue,
		Reason:  CPOReasonOperationCompleted,
		Message: "operation completed",
	})
}

func (s *OperationState) SetObservedState(state *ObservedComponentState) {
	s.op.Status.ObservedState = state
}

func (s *OperationState) markCurrentInProgressStepAbandoned() {
	for _, name := range s.op.Spec.Steps {
		cond := s.op.GetCondition(StepConditionType(name))
		if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != CPOReasonStepInProgress {
			continue
		}
		s.SetCondition(metav1.Condition{
			Type:    cond.Type,
			Status:  metav1.ConditionFalse,
			Reason:  CPOReasonStepAbandoned,
			Message: cond.Message,
		})
		return
	}
}
