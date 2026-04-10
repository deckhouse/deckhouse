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

const (
	CPOConditionReady    = "Ready"
	CPOConditionFailed   = "Failed"
	CPOConditionApproved = "Approved"
)

func (op *ControlPlaneOperation) IsCompleted() bool {
	return meta.IsStatusConditionTrue(op.Status.Conditions, CPOConditionReady)
}

func (op *ControlPlaneOperation) IsFailed() bool {
	return meta.IsStatusConditionTrue(op.Status.Conditions, CPOConditionFailed)
}

func (op *ControlPlaneOperation) IsCancelled() bool {
	for _, cond := range op.Status.Conditions {
		if cond.Type == CPOConditionReady && cond.Reason == "Cancelled" {
			return true
		}
	}
	return false
}

// IsTerminal reports whether the operation reached a not retryable state.
func (op *ControlPlaneOperation) IsTerminal() bool {
	return op.IsCompleted() || op.IsFailed() || op.IsCancelled()
}

func (op *ControlPlaneOperation) IsCommandCompleted(name CommandName) bool {
	return meta.IsStatusConditionTrue(op.Status.Conditions, string(name))
}

func (op *ControlPlaneOperation) FailureMessage() string {
	for _, cond := range op.Status.Conditions {
		if cond.Type == CPOConditionFailed && cond.Status == metav1.ConditionTrue {
			return cond.Message
		}
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
func (s *OperationState) IsCancelled() bool { return s.op.IsCancelled() }
func (s *OperationState) IsCommandCompleted(name CommandName) bool {
	return s.op.IsCommandCompleted(name)
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

func (s *OperationState) MarkCommandInProgress(name CommandName) {
	s.SetCondition(metav1.Condition{
		Type:   string(name),
		Status: metav1.ConditionFalse,
		Reason: "InProgress",
	})
}

func (s *OperationState) MarkCommandCompleted(name CommandName) {
	s.SetCondition(metav1.Condition{
		Type:   string(name),
		Status: metav1.ConditionTrue,
		Reason: "Completed",
	})
}

func (s *OperationState) MarkCommandFailed(name CommandName, message string) {
	s.SetCondition(metav1.Condition{
		Type:    string(name),
		Status:  metav1.ConditionFalse,
		Reason:  "Failed",
		Message: message,
	})
}

func (s *OperationState) SetReadyReason(reason, message string) {
	s.SetCondition(metav1.Condition{
		Type:    CPOConditionReady,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: message,
	})
}

func (s *OperationState) MarkSucceeded() {
	s.SetCondition(metav1.Condition{
		Type:    CPOConditionReady,
		Status:  metav1.ConditionTrue,
		Reason:  "OperationSucceeded",
		Message: "operation completed",
	})
	s.SetCondition(metav1.Condition{
		Type:   CPOConditionFailed,
		Status: metav1.ConditionFalse,
		Reason: "NoFailure",
	})
}

func (s *OperationState) SetObservedState(state map[OperationComponent]ObservedComponentState) {
	s.op.Status.ObservedState = state
}
