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
	"context"
	"log/slog"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// ConditionTypeInstalled indicates the application completed its initial installation successfully
	// Once true, this condition stays true (sticky) - it never reverts to false
	ConditionTypeInstalled = "Installed"

	// ConditionTypeReady indicates the application is currently operational and healthy
	// Relies on the ReadyInRuntime internal condition
	ConditionTypeReady = "Ready"

	// ConditionTypePartiallyDegraded indicates the application is not fully operational
	// Inverse of Ready - true when ReadyInRuntime is false
	ConditionTypePartiallyDegraded = "PartiallyDegraded"

	// ConditionTypeUpdateInstalled indicates whether an update to a new version succeeded or failed
	// Only set when version changes after initial installation. True if update succeeds, false if it fails
	ConditionTypeUpdateInstalled = "UpdateInstalled"

	// ConditionTypeManaged indicates the application is under active operator management
	// True when ReadyInRuntime is true, meaning the operator is successfully managing the package
	ConditionTypeManaged = "Managed"

	// ConditionTypeConfigurationApplied indicates Helm configuration was successfully applied
	// Relies on the HelmApplied internal condition
	ConditionTypeConfigurationApplied = "ConfigurationApplied"
)

type Service struct {
	client client.Client
	getter getter

	logger *log.Logger
}

type getter func(name string) status.Status

func NewService(client client.Client, getter getter, logger *log.Logger) *Service {
	return &Service{
		client: client,
		getter: getter,
		logger: logger.Named("status-service"),
	}
}

// Start begins the status service event loop in a goroutine
// It listens for package status change events and updates Application resources accordingly
func (s *Service) Start(ctx context.Context, ch <-chan string) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-ch:
				s.handleEvent(ctx, event)
			}
		}
	}()
}

// handleEvent processes a status change event for a package
// Event format is "namespace.name" identifying the Application resource
func (s *Service) handleEvent(ctx context.Context, ev string) {
	logger := s.logger.With(slog.String("name", ev))

	// Parse event name: "namespace.name"
	splits := strings.Split(ev, ".")
	if len(splits) != 2 {
		logger.Warn("invalid event format, expected 'namespace.name'")
		return
	}

	// Fetch the Application resource
	app := new(v1alpha1.Application)
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: splits[0], Name: splits[1]}, app); err != nil {
		logger.Warn("failed to get application", log.Err(err))
		return
	}

	// Get the package status from the operator
	packageStatus := s.getter(ev)

	// Update the Application status with new conditions
	original := app.DeepCopy()
	s.applyInternalConditions(app, packageStatus.Conditions)

	if app.Status.CurrentVersion == nil {
		app.Status.CurrentVersion = new(v1alpha1.ApplicationStatusVersion)
	}

	app.Status.CurrentVersion.Current = packageStatus.Version

	if err := s.client.Status().Patch(ctx, app, client.MergeFrom(original)); err != nil {
		logger.Warn("failed to patch application status", log.Err(err))
	}
}

// applyInternalConditions updates the Application's internal conditions from the operator
// and then computes the public Installed and Ready conditions
func (s *Service) applyInternalConditions(app *v1alpha1.Application, conds []status.Condition) {
	// Build a map of previous conditions to preserve LastTransitionTime
	prev := make(map[string]v1alpha1.ApplicationInternalStatusCondition)
	for _, cond := range app.Status.InternalConditions {
		prev[cond.Type] = cond
	}

	now := metav1.Now()
	applied := make([]v1alpha1.ApplicationInternalStatusCondition, 0, len(conds))

	// Convert operator conditions to Application internal conditions
	for _, c := range conds {
		cond := v1alpha1.ApplicationInternalStatusCondition{
			Type:               string(c.Name),
			Status:             corev1.ConditionStatus(c.Status),
			Reason:             string(c.Reason),
			Message:            c.Message,
			LastTransitionTime: now,
			LastProbeTime:      now,
		}

		// Preserve LastTransitionTime if status hasn't changed
		if p, ok := prev[cond.Type]; ok && p.Status == cond.Status {
			cond.LastTransitionTime = p.LastTransitionTime
		}

		applied = append(applied, cond)
	}

	app.Status.InternalConditions = applied

	// Compute public conditions from internal conditions
	s.computeConditions(app)
}

func (s *Service) computeConditions(app *v1alpha1.Application) {
	now := metav1.Now()

	// Build a map of internal conditions for easier access
	internalConds := make(map[string]v1alpha1.ApplicationInternalStatusCondition)
	for _, cond := range app.Status.InternalConditions {
		internalConds[cond.Type] = cond
	}

	// Check if Installed was ever set to true (sticky flag for initial installation)
	installedPreviously := s.getConditionStatus(app, ConditionTypeInstalled) == corev1.ConditionTrue

	// Check if version changed (indicates update scenario)
	versionChanged := app.Status.CurrentVersion != nil &&
		app.Status.CurrentVersion.Current != "" &&
		app.Spec.Version != app.Status.CurrentVersion.Current

	// Find any failed condition
	failedCond := s.findFailedCondition(internalConds)
	allConditionsMet := failedCond == nil

	// Compute Installed condition
	// Once true, stays true forever (sticky)
	if !installedPreviously {
		// Initial installation - set based on conditions
		if allConditionsMet {
			s.setCondition(app, ConditionTypeInstalled, corev1.ConditionTrue, "", "", now)
		} else {
			s.setCondition(app, ConditionTypeInstalled, corev1.ConditionFalse, failedCond.Reason, failedCond.Message, now)
		}
	}
	// If already installed, condition stays true (don't modify it)

	// Compute UpdateInstalled condition (only relevant when version changes after initial install)
	if installedPreviously && versionChanged {
		if allConditionsMet {
			s.setCondition(app, ConditionTypeUpdateInstalled, corev1.ConditionTrue, "", "", now)
		} else {
			s.setCondition(app, ConditionTypeUpdateInstalled, corev1.ConditionFalse, failedCond.Reason, failedCond.Message, now)
		}
	}

	// Compute Ready, PartiallyDegraded, and Managed conditions (all depend on ReadyInRuntime)
	// Only set detailed message on Ready condition to avoid duplication
	if readyInRuntime, ok := internalConds[string(status.ConditionReadyInRuntime)]; ok && readyInRuntime.Status == corev1.ConditionTrue {
		s.setCondition(app, ConditionTypeReady, corev1.ConditionTrue, "", "", now)
		s.setCondition(app, ConditionTypePartiallyDegraded, corev1.ConditionFalse, "", "", now)
		s.setCondition(app, ConditionTypeManaged, corev1.ConditionTrue, "", "", now)
	} else if ok {
		// Only set reason/message on Ready to avoid duplication
		s.setCondition(app, ConditionTypeReady, corev1.ConditionFalse, readyInRuntime.Reason, readyInRuntime.Message, now)
		s.setCondition(app, ConditionTypePartiallyDegraded, corev1.ConditionTrue, "", "", now)
		s.setCondition(app, ConditionTypeManaged, corev1.ConditionFalse, "", "", now)
	} else {
		s.setCondition(app, ConditionTypeReady, corev1.ConditionFalse, "", "", now)
		s.setCondition(app, ConditionTypePartiallyDegraded, corev1.ConditionTrue, "", "", now)
		s.setCondition(app, ConditionTypeManaged, corev1.ConditionFalse, "", "", now)
	}

	// Compute ConfigurationApplied condition (depends on HelmApplied)
	if helmApplied, ok := internalConds[string(status.ConditionHelmApplied)]; ok && helmApplied.Status == corev1.ConditionTrue {
		s.setCondition(app, ConditionTypeConfigurationApplied, corev1.ConditionTrue, "", "", now)
	} else if ok {
		s.setCondition(app, ConditionTypeConfigurationApplied, corev1.ConditionFalse, helmApplied.Reason, helmApplied.Message, now)
	} else {
		s.setCondition(app, ConditionTypeConfigurationApplied, corev1.ConditionFalse, "", "", now)
	}
}

// findFailedCondition returns the first condition that is not true, prioritizing critical conditions
func (s *Service) findFailedCondition(internalConds map[string]v1alpha1.ApplicationInternalStatusCondition) *v1alpha1.ApplicationInternalStatusCondition {
	// Check critical conditions first in order of execution
	criticalConditions := []status.ConditionName{
		status.ConditionDownloaded,
		status.ConditionReadyOnFilesystem,
		status.ConditionRequirementsMet,
		status.ConditionReadyInRuntime,
	}

	for _, condName := range criticalConditions {
		if cond, exists := internalConds[string(condName)]; exists && cond.Status != corev1.ConditionTrue {
			return &cond
		}
	}

	// Check remaining conditions (HooksProcessed, HelmApplied)
	for _, cond := range internalConds {
		if cond.Status != corev1.ConditionTrue {
			return &cond
		}
	}

	return nil
}

// getConditionStatus retrieves the current status of a condition
func (s *Service) getConditionStatus(app *v1alpha1.Application, condType string) corev1.ConditionStatus {
	for _, cond := range app.Status.Conditions {
		if cond.Type == condType {
			return cond.Status
		}
	}
	return corev1.ConditionUnknown
}

// setCondition creates or updates a condition in the application status
func (s *Service) setCondition(app *v1alpha1.Application, condType string, status corev1.ConditionStatus, reason, message string, now metav1.Time) {
	newCond := v1alpha1.ApplicationStatusCondition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		LastProbeTime:      now,
	}

	// Find and update existing condition, preserving LastTransitionTime if status unchanged
	for i, cond := range app.Status.Conditions {
		if cond.Type == condType {
			if cond.Status == status {
				newCond.LastTransitionTime = cond.LastTransitionTime
			}
			app.Status.Conditions[i] = newCond
			return
		}
	}

	// Condition doesn't exist, append it
	app.Status.Conditions = append(app.Status.Conditions, newCond)
}
