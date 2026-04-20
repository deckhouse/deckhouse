// Copyright 2025 Flant JSC
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

package status

import (
	"errors"
	"sync"

	"github.com/werf/nelm/pkg/legacy/progrep"
)

type ConditionType string

const (
	// ConditionReadyOnFilesystem indicates package was successfully mounted and accessible
	ConditionReadyOnFilesystem ConditionType = "ReadyOnFilesystem"
	// ConditionReadyInRuntime indicates package is fully loaded and operational in runtime
	ConditionReadyInRuntime ConditionType = "ReadyInRuntime"
	// ConditionReadyInCluster checks the resources are ready
	ConditionReadyInCluster ConditionType = "ReadyInCluster"
	// ConditionHelmApplied indicates Helm release was successfully applied
	ConditionHelmApplied ConditionType = "HelmApplied"
	// ConditionHooksReady indicates all package hooks executed successfully
	ConditionHooksReady ConditionType = "HooksReady"
	// ConditionConfigured checks the settings passed openAPI validation and applied to the package
	ConditionConfigured ConditionType = "Configured"
)

type ConditionReason string

const (
	// ConditionReasonApplyingManifests indicates that nelm is applying manifests to the cluster
	ConditionReasonApplyingManifests ConditionReason = "ApplyingManifests"
	ConditionReasonPending           ConditionReason = "Pending"
)

type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

// Error wraps an error with associated status conditions
// Used to propagate both error details and status updates through the call stack
type Error struct {
	Err    error
	Reason ConditionReason
}

func NewError(reason ConditionReason, err error) error {
	return &Error{
		Err:    err,
		Reason: reason,
	}
}

func (e *Error) Error() string {
	return e.Err.Error()
}

// Registry tracks package statuses and notifies listeners of changes
type Registry struct {
	mu       sync.Mutex
	statuses map[string]*Status

	ch chan string // notification channel for status changes
}

// Status represents the current state of a package
type Status struct {
	Version    string      `json:"version"`
	Conditions []Condition `json:"conditions"`
	Tracking   Tracking    `json:"tracking"`
}

type Tracking struct {
	Completed int                 `json:"completed"`
	Remaining int                 `json:"remaining"`
	Report    progrep.StageReport `json:"report"`
}

// Condition represents a single status condition for a package
type Condition struct {
	Type    ConditionType   `json:"type"`
	Status  ConditionStatus `json:"status"` // true = condition met, false = condition failed
	Reason  ConditionReason `json:"reason,omitempty"`
	Message string          `json:"message,omitempty"`
}

func NewRegistry() *Registry {
	return &Registry{
		ch:       make(chan string, 10000),
		statuses: make(map[string]*Status),
	}
}

// GetCh returns a read-only channel that receives package names when their status changes
func (r *Registry) GetCh() <-chan string {
	return r.ch
}

// GetStatus retrieves a copy of the current status for a package by name ("namespace.name")
// Returns a copy to prevent race conditions with concurrent modifications
func (r *Registry) GetStatus(name string) Status {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, ok := r.statuses[name]
	if !ok {
		return Status{}
	}

	// Return a deep copy to prevent race conditions
	condsCopy := make([]Condition, len(status.Conditions))
	copy(condsCopy, status.Conditions)

	return Status{
		Version:    status.Version,
		Conditions: condsCopy,
		Tracking:   status.Tracking,
	}
}

// SetVersion sets the current version of package
func (r *Registry) SetVersion(name string, version string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, ok := r.statuses[name]
	if !ok {
		return
	}

	status.Version = version
	// Notify only if the condition actually changed
	notify := r.statuses[name].setCondition(Condition{
		Type:    ConditionReadyInRuntime,
		Status:  ConditionFalse,
		Reason:  ConditionReasonPending,
		Message: "",
	})
	if notify {
		r.ch <- name
	}
}

// SetConditionTrue marks a condition as successful and notifies listeners if changed
func (r *Registry) SetConditionTrue(name string, condition ConditionType) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.statuses[name]; !ok {
		return
	}

	// Notify only if the condition actually changed
	notify := r.statuses[name].setCondition(Condition{Type: condition, Status: ConditionTrue})
	if notify {
		r.ch <- name
	}
}

// SetConditionFalse marks a condition status as false and notifies listeners if changed
func (r *Registry) SetConditionFalse(name string, condition ConditionType, reason, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.statuses[name]; !ok {
		return
	}

	// Notify only if the condition actually changed
	notify := r.statuses[name].setCondition(Condition{
		Type:    condition,
		Status:  ConditionFalse,
		Reason:  ConditionReason(reason),
		Message: message})

	if notify {
		r.ch <- name
	}
}

// UpdateTracking updates the nelm progress report for a package and notifies listeners.
// If the package is not tracked by the service, the update is silently ignored.
func (r *Registry) UpdateTracking(name string, report progrep.ProgressReport) {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, ok := r.statuses[name]
	if !ok {
		return
	}

	r.statuses[name].setCondition(Condition{
		Type:   ConditionHelmApplied,
		Status: ConditionFalse,
		Reason: ConditionReasonApplyingManifests})
	r.statuses[name].setCondition(Condition{
		Type:   ConditionReadyInCluster,
		Status: ConditionFalse,
		Reason: ConditionReasonApplyingManifests})

	for i := len(report.StageReports) - 1; i >= 0; i-- {
		stage := report.StageReports[i]
		if len(stage.Operations) == 0 {
			continue
		}

		completed := 0
		remaining := 0
		for _, op := range stage.Operations {
			if op.Status == progrep.OperationStatusCompleted {
				completed++
			} else {
				remaining++
			}
		}

		status.Tracking = Tracking{Completed: completed, Remaining: remaining, Report: stage}
		break
	}

	r.ch <- name
}

// ClearStatus creates a new Status with all known conditions initialized to unknown
func (r *Registry) ClearStatus(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.statuses[name] = &Status{
		Conditions: []Condition{
			{Type: ConditionReadyOnFilesystem, Status: ConditionUnknown},
			{Type: ConditionReadyInRuntime, Status: ConditionUnknown},
			{Type: ConditionReadyInCluster, Status: ConditionUnknown},
			{Type: ConditionHooksReady, Status: ConditionUnknown},
			{Type: ConditionHelmApplied, Status: ConditionUnknown},
			{Type: ConditionConfigured, Status: ConditionUnknown},
		},
	}
}

// Delete removes a package status from tracking
// Should be called when a package is deleted to prevent memory leaks
func (r *Registry) Delete(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.statuses, name)
}

// HandleError processes an error and extracts status conditions from it
// Notifies listeners if any conditions changed
func (r *Registry) HandleError(name string, cond ConditionType, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.statuses[name]; !ok {
		return
	}

	reason := extractReason(err)
	if reason == "" {
		return
	}

	notify := r.statuses[name].setCondition(Condition{
		Type:    cond,
		Status:  ConditionFalse,
		Reason:  reason,
		Message: err.Error(),
	})

	if notify {
		r.ch <- name
	}
}

// extractCondition recursively extracts all conditions from wrapped status errors
func extractReason(err error) ConditionReason {
	statusErr := new(Error)
	if !errors.As(err, &statusErr) {
		return ""
	}

	return statusErr.Reason
}

// setCondition updates or adds a condition, returning true if anything changed
func (s *Status) setCondition(condition Condition) bool {
	var notify bool
	var found bool

	// Try to find and update existing condition
	for i := range s.Conditions {
		if s.Conditions[i].Type != condition.Type {
			continue
		}

		found = true

		// Track if any field changed
		if s.Conditions[i].Status != condition.Status {
			s.Conditions[i].Status = condition.Status
			notify = true
		}

		if s.Conditions[i].Reason != condition.Reason {
			s.Conditions[i].Reason = condition.Reason
			notify = true
		}

		if s.Conditions[i].Message != condition.Message {
			s.Conditions[i].Message = condition.Message
			notify = true
		}

		break
	}

	// Condition doesn't exist, add it
	if !found {
		s.Conditions = append(s.Conditions, condition)
		notify = true
	}

	return notify
}
