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
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/statusmapper"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/packages/application/status/specs"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Service processes status events and updates Application conditions.
type Service struct {
	client client.Client
	getter getter
	mapper *statusmapper.Mapper
	logger *log.Logger
}

type getter func(name string) status.Status

// NewService creates a new status service with default condition specs.
func NewService(client client.Client, getter getter, logger *log.Logger) *Service {
	return &Service{
		client: client,
		getter: getter,
		mapper: statusmapper.New(specs.DefaultSpecs()),
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

// applyInternalConditions updates internal conditions and computes external conditions.
func (s *Service) applyInternalConditions(app *v1alpha1.Application, internalConds []status.Condition) {
	// Preserve LastTransitionTime for unchanged conditions
	prev := make(map[string]v1alpha1.ApplicationInternalStatusCondition)
	for _, cond := range app.Status.InternalConditions {
		prev[cond.Type] = cond
	}

	now := metav1.Now()
	applied := make([]v1alpha1.ApplicationInternalStatusCondition, 0, len(internalConds))

	for _, c := range internalConds {
		cond := v1alpha1.ApplicationInternalStatusCondition{
			Type:               string(c.Name),
			Status:             corev1.ConditionStatus(c.Status),
			Reason:             string(c.Reason),
			Message:            c.Message,
			LastTransitionTime: now,
			LastProbeTime:      now,
		}

		if p, ok := prev[cond.Type]; ok && p.Status == cond.Status {
			cond.LastTransitionTime = p.LastTransitionTime
		}

		applied = append(applied, cond)
	}

	app.Status.InternalConditions = applied

	// Compute external conditions and update Application status
	s.computeExternalConditions(app, internalConds)
}

// computeExternalConditions uses the mapper to compute external conditions from internal.
func (s *Service) computeExternalConditions(app *v1alpha1.Application, internalConds []status.Condition) {
	// Build input for the mapper
	input := s.buildInput(app, internalConds)
	externalConds := s.mapper.Map(input)

	now := metav1.Now()
	for _, cond := range externalConds {
		s.setCondition(app, string(cond.Name), cond.Status, string(cond.Reason), cond.Message, now)
	}
}

// buildInput creates mapper input from Application and internal conditions.
func (s *Service) buildInput(app *v1alpha1.Application, internalConds []status.Condition) *statusmapper.Input {
	internalMap := make(map[status.ConditionName]status.Condition, len(internalConds))
	for _, c := range internalConds {
		internalMap[c.Name] = c
	}

	externalMap := make(map[status.ConditionName]status.Condition, len(app.Status.Conditions))
	for _, c := range app.Status.Conditions {
		externalMap[status.ConditionName(c.Type)] = status.Condition{
			Name:    status.ConditionName(c.Type),
			Status:  metav1.ConditionStatus(c.Status),
			Reason:  status.ConditionReason(c.Reason),
			Message: c.Message,
		}
	}

	isInitialInstall := externalMap[status.ConditionInstalled].Status != metav1.ConditionTrue
	versionChanged := app.Status.CurrentVersion != nil &&
		app.Status.CurrentVersion.Current != "" &&
		app.Spec.Version != app.Status.CurrentVersion.Current

	return &statusmapper.Input{
		InternalConditions: internalMap,
		ExternalConditions: externalMap,
		IsInitialInstall:   isInitialInstall,
		VersionChanged:     versionChanged,
	}
}

// setCondition creates or updates a condition, preserving LastTransitionTime if unchanged.
func (s *Service) setCondition(app *v1alpha1.Application, condType string, condStatus metav1.ConditionStatus, reason, message string, now metav1.Time) {
	newCond := v1alpha1.ApplicationStatusCondition{
		Type:               condType,
		Status:             corev1.ConditionStatus(condStatus),
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		LastProbeTime:      now,
	}

	for i, cond := range app.Status.Conditions {
		if cond.Type == condType {
			if cond.Status == corev1.ConditionStatus(condStatus) {
				newCond.LastTransitionTime = cond.LastTransitionTime
			}
			app.Status.Conditions[i] = newCond
			return
		}
	}

	app.Status.Conditions = append(app.Status.Conditions, newCond)
}
