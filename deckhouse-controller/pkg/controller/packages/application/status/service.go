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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/condmapper"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Service processes status events and updates Application conditions.
type Service struct {
	client client.Client
	getter getter
	mapper condmapper.Mapper
	logger *log.Logger
}

type getter func(name string) status.Status

// NewService creates a new status service with default condition specs.
func NewService(client client.Client, getter getter, logger *log.Logger) *Service {
	return &Service{
		client: client,
		getter: getter,
		mapper: buildMapper(),
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

	original := app.DeepCopy()

	// Get the package status from the operator and compute conditions
	s.computeAndApplyConditions(ev, app)

	if err := s.client.Status().Patch(ctx, app, client.MergeFrom(original)); err != nil {
		logger.Warn("failed to patch application status", log.Err(err))
	}
}

func (s *Service) computeAndApplyConditions(ev string, app *v1alpha1.Application) {
	packageStatus := s.getter(ev)

	if app.Status.CurrentVersion == nil {
		app.Status.CurrentVersion = new(v1alpha1.ApplicationStatusVersion)
	}

	versionChanged := app.Status.CurrentVersion.Version != "" && app.Status.CurrentVersion.Version != packageStatus.Version
	mapperStatus := s.buildMapperStatus(versionChanged, app.Status.Conditions, packageStatus.Conditions)

	now := metav1.Now()
	mappedConditions := s.mapper.Map(mapperStatus)
	setMapperConditions(app, now, mappedConditions...)
	setInternalConditions(app, now, packageStatus.Conditions...)

	// We can lose versionChanged=true during different events processing.
	//
	// So we need to commit version when ReadyInCluster (internal condition) is True.
	// ReadyInCluster is the last condition in the chain, so when it's True,
	// all other conditions (Downloaded, ReadyOnFilesystem, ReadyInRuntime) are also True.
	//
	// And this means we can commit the resulted version.
	if internalConditionIsTrue(packageStatus.Conditions, status.ConditionReadyInCluster) {
		app.Status.CurrentVersion.Version = packageStatus.Version
	}
}

// buildMapperStatus creates mapper input from Application and internal conditions.
func (s *Service) buildMapperStatus(versionChanged bool, external []v1alpha1.ApplicationStatusCondition, internal []status.Condition) condmapper.State {
	mapperStatus := condmapper.State{
		External: make(map[string]metav1.Condition, len(external)),
		Internal: make(map[string]metav1.Condition, len(internal)),
	}

	for _, cond := range internal {
		mapperStatus.Internal[string(cond.Type)] = metav1.Condition{
			Type:    string(cond.Type),
			Status:  cond.Status,
			Reason:  string(cond.Reason),
			Message: cond.Message,
		}
	}

	for _, cond := range external {
		mapperStatus.External[cond.Type] = metav1.Condition{
			Type:    cond.Type,
			Status:  metav1.ConditionStatus(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		}
	}

	mapperStatus.VersionChanged = versionChanged

	return mapperStatus
}

// setMapperConditions creates or updates conditions, preserving LastTransitionTime if unchanged.
func setMapperConditions(app *v1alpha1.Application, now metav1.Time, conds ...metav1.Condition) {
	for _, cond := range conds {
		newCond := v1alpha1.ApplicationStatusCondition{
			Type:               cond.Type,
			Status:             corev1.ConditionStatus(cond.Status),
			Reason:             cond.Reason,
			Message:            cond.Message,
			LastTransitionTime: now,
			LastProbeTime:      now,
		}

		found := false
		for i, oldCond := range app.Status.Conditions {
			if oldCond.Type == cond.Type {
				if oldCond.Status == corev1.ConditionStatus(cond.Status) {
					newCond.LastTransitionTime = oldCond.LastTransitionTime
				}
				app.Status.Conditions[i] = newCond
				found = true
				break
			}
		}

		if !found {
			app.Status.Conditions = append(app.Status.Conditions, newCond)
		}
	}
}

// setInternalConditions creates or updates internal conditions, preserving LastTransitionTime if unchanged.
func setInternalConditions(app *v1alpha1.Application, now metav1.Time, conds ...status.Condition) {
	for _, cond := range conds {
		newCond := v1alpha1.ApplicationStatusInternalCondition{
			Type:               string(cond.Type),
			Status:             corev1.ConditionStatus(cond.Status),
			Reason:             string(cond.Reason),
			Message:            cond.Message,
			LastTransitionTime: now,
			LastProbeTime:      now,
		}

		found := false
		for i, oldCond := range app.Status.InternalConditions {
			if oldCond.Type == string(cond.Type) {
				if oldCond.Status == corev1.ConditionStatus(cond.Status) {
					newCond.LastTransitionTime = oldCond.LastTransitionTime
				}
				app.Status.InternalConditions[i] = newCond
				found = true
				break
			}
		}

		if !found {
			app.Status.InternalConditions = append(app.Status.InternalConditions, newCond)
		}
	}
}

// internalConditionIsTrue checks if an internal condition with the given name has status True.
func internalConditionIsTrue(conditions []status.Condition, condName status.ConditionType) bool {
	for _, cond := range conditions {
		if cond.Type == condName && cond.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}
