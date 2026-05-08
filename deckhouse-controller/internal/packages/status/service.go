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

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/werf/nelm/pkg/legacy/progrep"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ConditionRequirementsMet indicates package requirements validation passed
	ConditionRequirementsMet ConditionType = "RequirementsMet"
	// ConditionReadyOnFilesystem indicates package was successfully mounted and accessible
	ConditionReadyOnFilesystem ConditionType = "ReadyOnFilesystem"
	// ConditionLoaded indicates package is loaded in runtime
	ConditionLoaded ConditionType = "Loaded"
	// ConditionHooksProcessed indicates all package hooks executed successfully
	ConditionHooksProcessed ConditionType = "HooksProcessed"
	// ConditionManifestsApplied indicates Helm release was successfully applied
	ConditionManifestsApplied ConditionType = "ManifestsApplied"
	// ConditionScaled checks the cluster resources are ready
	ConditionScaled ConditionType = "Scaled"
	// ConditionConfigured checks the settings passed openAPI validation
	ConditionConfigured ConditionType = "Configured"
	// ConditionPending indicates that the package wait converge
	ConditionPending ConditionType = "Pending"

	// ConditionReasonApplyingManifests indicates that nelm is applying manifests to the cluster
	ConditionReasonApplyingManifests ConditionReason = "ApplyingManifests"
)

// Error wraps an error with associated status conditions
// Used to propagate both error details and status updates through the call stack
type Error struct {
	err     error
	reason  ConditionReason
	message string
}

func NewError(reason ConditionReason, err error) *Error {
	return &Error{
		err:     err,
		reason:  reason,
		message: err.Error(),
	}
}

func (e *Error) Error() string {
	return e.err.Error()
}

type ConditionType string

type ConditionReason string

// Service tracks package statuses and notifies listeners of changes
type Service struct {
	mu       sync.Mutex
	statuses map[string]*Status // keyed by "namespace.name"

	ch chan string // notification channel for status changes
}

// Status represents the current state of a package
type Status struct {
	Version    string            `json:"version"`
	Conditions []Condition       `json:"conditions"`
	Tracking   Tracking          `json:"tracking"`
	Settings   addonutils.Values `json:"settings,omitempty"`
}

type Tracking struct {
	Completed int                 `json:"completed"`
	Remaining int                 `json:"remaining"`
	Report    progrep.StageReport `json:"report"`
}

// Condition represents a single status condition for a package
type Condition struct {
	Type    ConditionType          `json:"type"`
	Status  metav1.ConditionStatus `json:"status"` // true = condition met, false = condition failed
	Reason  ConditionReason        `json:"reason,omitempty"`
	Message string                 `json:"message,omitempty"`
}

func NewService() *Service {
	return &Service{
		ch:       make(chan string, 10000),
		statuses: make(map[string]*Status),
	}
}

// GetCh returns a read-only channel that receives package names when their status changes
func (s *Service) GetCh() <-chan string {
	return s.ch
}

// GetStatus retrieves a copy of the current status for a package by name ("namespace.name")
// Returns a copy to prevent race conditions with concurrent modifications
func (s *Service) GetStatus(name string) Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	status, ok := s.statuses[name]
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
		Settings:   status.Settings,
	}
}

// DeleteStatus removes a package status from tracking
// Should be called when a package is deleted to prevent memory leaks
func (s *Service) DeleteStatus(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.statuses, name)
}

// SetConditionTrue marks a condition as successful and notifies listeners if changed
func (s *Service) SetConditionTrue(name string, condition ConditionType) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.statuses[name]; !ok {
		return
	}

	// Notify only if the condition actually changed
	if s.statuses[name].setCondition(Condition{Type: condition, Status: metav1.ConditionTrue}) {
		s.ch <- name
	}
}

// SetConditionFalse marks a condition as successful and notifies listeners if changed
func (s *Service) SetConditionFalse(name string, condition ConditionType, reason, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.statuses[name]; !ok {
		return
	}

	// Notify only if the condition actually changed
	notify := s.statuses[name].setCondition(Condition{
		Type:    condition,
		Status:  metav1.ConditionFalse,
		Reason:  ConditionReason(reason),
		Message: message,
	})

	if notify {
		s.ch <- name
	}
}

// UpdateVersion sets the current version of package
func (s *Service) UpdateVersion(name string, version string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status, ok := s.statuses[name]
	if !ok {
		return
	}

	status.Version = version
	status.setCondition(Condition{Type: ConditionLoaded, Status: metav1.ConditionTrue})
	status.setCondition(Condition{Type: ConditionPending, Status: metav1.ConditionTrue, Message: "waiting for processing"})

	s.ch <- name
}

// UpdateTracking updates the nelm progress report for a package and notifies listeners.
// If the package is not tracked by the service, the update is silently ignored.
func (s *Service) UpdateTracking(name string, report progrep.ProgressReport) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status, ok := s.statuses[name]
	if !ok {
		return
	}

	s.statuses[name].setCondition(Condition{
		Type:   ConditionManifestsApplied,
		Status: metav1.ConditionFalse,
		Reason: ConditionReasonApplyingManifests})

	for i := len(report.StageReports) - 1; i >= 0; i-- {
		r := report.StageReports[i]
		if len(r.Operations) == 0 {
			continue
		}

		completed := 0
		remaining := 0
		for _, op := range r.Operations {
			if op.Status == progrep.OperationStatusCompleted {
				completed++
			} else {
				remaining++
			}
		}

		status.Tracking = Tracking{Completed: completed, Remaining: remaining, Report: r}
		break
	}

	s.ch <- name
}

// UpdateSettings stores the effective settings of a package.
// Does not notify — the caller pairs this with SetConditionTrue which notifies.
func (s *Service) UpdateSettings(name string, settings addonutils.Values) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status, ok := s.statuses[name]
	if !ok {
		return
	}

	status.Settings = settings
	status.setCondition(Condition{Type: ConditionConfigured, Status: metav1.ConditionTrue})
}

// HandleError processes an error and extracts status conditions from it
// Notifies listeners if any conditions changed
func (s *Service) HandleError(name string, cond ConditionType, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	statusErr := new(Error)
	if !errors.As(err, &statusErr) {
		return
	}

	if _, ok := s.statuses[name]; !ok {
		return
	}

	notify := s.statuses[name].setCondition(Condition{
		Type:    cond,
		Status:  metav1.ConditionFalse,
		Reason:  statusErr.reason,
		Message: statusErr.message,
	})

	if notify {
		s.ch <- name
	}
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
	}

	// Condition doesn't exist, add it
	if !found {
		s.Conditions = append(s.Conditions, condition)
		notify = true
	}

	return notify
}

// ClearStatus creates a new status or resets conditions
func (s *Service) ClearStatus(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.statuses[name] = &Status{
		Conditions: []Condition{
			{Type: ConditionRequirementsMet, Status: metav1.ConditionUnknown},
			{Type: ConditionReadyOnFilesystem, Status: metav1.ConditionUnknown},
			{Type: ConditionLoaded, Status: metav1.ConditionUnknown},
			{Type: ConditionHooksProcessed, Status: metav1.ConditionUnknown},
			{Type: ConditionManifestsApplied, Status: metav1.ConditionUnknown},
			{Type: ConditionScaled, Status: metav1.ConditionUnknown},
			{Type: ConditionConfigured, Status: metav1.ConditionUnknown},
			{Type: ConditionPending, Status: metav1.ConditionUnknown},
		},
	}
}
