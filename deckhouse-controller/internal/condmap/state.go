// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package condmap

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// State is the snapshot fed into a single mapping run.
// Internal holds conditions set by operator tasks (inputs to mapping).
// External holds user-facing conditions from the previous run; rules read it
// to implement sticky and guard behavior (e.g. Installed stays True once set).
type State struct {
	Updating bool                        // true when a version change is in progress
	Internal map[string]metav1.Condition // task-set conditions, keyed by condition type
	External map[string]metav1.Condition // previously computed user-facing conditions, read-only during mapping
}

// ExtEqual reports whether the external condition has the given status.
// Returns false if the condition is absent.
func (s *State) ExtEqual(cond string, status metav1.ConditionStatus) bool {
	if _, ok := s.External[cond]; ok {
		return s.External[cond].Status == status
	}

	return false
}

// GetExtReason returns the Reason and Message of an external condition.
// Returns ("", "") if the condition is absent.
func (s *State) GetExtReason(cond string) (string, string) {
	c, ok := s.External[cond]
	if !ok {
		return "", ""
	}

	return c.Reason, c.Message
}

func (s *State) HasExt(cond string) bool {
	_, ok := s.External[cond]
	return ok
}

// IntEqual reports whether the internal condition has the given status.
// Returns false if the condition is absent.
func (s *State) IntEqual(cond string, status metav1.ConditionStatus) bool {
	if _, ok := s.Internal[cond]; ok {
		return s.Internal[cond].Status == status
	}

	return false
}

func (s *State) HasInt(cond string) bool {
	_, ok := s.Internal[cond]
	return ok
}

// IsUpdating reports whether this mapping run observes a version change.
func (s *State) IsUpdating() bool {
	return s.Updating
}

// GetIntReason returns the Reason and Message of an internal condition.
// Returns ("", "") if the condition is absent.
func (s *State) GetIntReason(cond string) (string, string) {
	c, ok := s.Internal[cond]
	if !ok {
		return "", ""
	}

	return c.Reason, c.Message
}

// ConditionByInt builds an external condition that passes through reason and
// message from an internal condition. The result has empty reason if the
// internal condition has none — callers that need fallback handling should use
// their own helper.
func (s *State) ConditionByInt(cond string, condStatus metav1.ConditionStatus, internalCond string) metav1.Condition {
	reason, message := s.GetIntReason(internalCond)

	return metav1.Condition{
		Type:    cond,
		Status:  condStatus,
		Reason:  reason,
		Message: message,
	}
}

// GetIntStatus returns the status of an internal condition.
// Returns ("", false) if the condition is absent.
func (s *State) GetIntStatus(cond string) (metav1.ConditionStatus, bool) {
	c, ok := s.Internal[cond]
	if !ok {
		return "", false
	}

	return c.Status, true
}

// AllIntEqual reports whether every listed internal condition has the given
// status. It returns false if any condition is absent.
func (s *State) AllIntEqual(status metav1.ConditionStatus, conditions ...string) bool {
	for _, cond := range conditions {
		if !s.IntEqual(cond, status) {
			return false
		}
	}

	return true
}
