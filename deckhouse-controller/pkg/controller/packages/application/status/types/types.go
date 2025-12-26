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

package types

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// =============================================================================
// External Condition Types
// =============================================================================

type ExternalConditionType string

const (
	ConditionInstalled            ExternalConditionType = "Installed"
	ConditionReady                ExternalConditionType = "Ready"
	ConditionPartiallyDegraded    ExternalConditionType = "PartiallyDegraded"
	ConditionUpdateInstalled      ExternalConditionType = "UpdateInstalled"
	ConditionManaged              ExternalConditionType = "Managed"
	ConditionConfigurationApplied ExternalConditionType = "ConfigurationApplied"
)

// AllExternalConditions is the canonical list of all external condition types.
// Used by exhaustiveness tests to verify all types have corresponding specs.
var AllExternalConditions = []ExternalConditionType{
	ConditionInstalled,
	ConditionReady,
	ConditionPartiallyDegraded,
	ConditionUpdateInstalled,
	ConditionManaged,
	ConditionConfigurationApplied,
}

// =============================================================================
// Internal Condition Names (from operator)
// =============================================================================

// InternalConditionName is an alias for status.ConditionName from the operator.
// Use status.ConditionXxx constants directly — they are the single source of truth.
type InternalConditionName = status.ConditionName

// =============================================================================
// Condition Data Types
// =============================================================================

// InternalCondition represents an internal condition state.
type InternalCondition struct {
	Name    string
	Status  corev1.ConditionStatus
	Reason  string
	Message string
}

// ExternalCondition represents a computed external condition.
type ExternalCondition struct {
	Type    ExternalConditionType
	Status  corev1.ConditionStatus
	Reason  string
	Message string
}

// =============================================================================
// Mapping Internal->External Conditions Evaluation Context
// =============================================================================

// MappingInput contains all data needed for condition evaluation.
type MappingInput struct {
	// InternalConditions from the operator (keyed by condition name)
	InternalConditions map[string]InternalCondition

	// CurrentConditions from Application.Status.Conditions
	CurrentConditions map[ExternalConditionType]ExternalCondition

	// App provides access to spec/status for complex predicates
	App *v1alpha1.Application

	// VersionChanged indicates spec.version != status.currentVersion
	VersionChanged bool

	// IsInitialInstall indicates Installed condition was never True
	IsInitialInstall bool
}

// NewMappingInput builds input from Application and internal conditions.
// This is more like a snapshot of the current state.
func NewMappingInput(app *v1alpha1.Application, internalConds []InternalCondition) *MappingInput {
	input := &MappingInput{
		InternalConditions: make(map[string]InternalCondition),
		CurrentConditions:  make(map[ExternalConditionType]ExternalCondition),
		App:                app,
	}

	for _, c := range internalConds {
		input.InternalConditions[c.Name] = c
	}

	for _, c := range app.Status.Conditions {
		input.CurrentConditions[ExternalConditionType(c.Type)] = ExternalCondition{
			Type:    ExternalConditionType(c.Type),
			Status:  c.Status,
			Reason:  c.Reason,
			Message: c.Message,
		}
	}

	input.IsInitialInstall = input.CurrentConditions[ConditionInstalled].Status != corev1.ConditionTrue
	input.VersionChanged = app.Status.CurrentVersion != nil &&
		app.Status.CurrentVersion.Current != "" &&
		app.Spec.Version != app.Status.CurrentVersion.Current

	return input
}
