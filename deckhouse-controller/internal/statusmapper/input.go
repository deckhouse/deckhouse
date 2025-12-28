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

package statusmapper

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// Input contains all data needed for condition evaluation.
type Input struct {
	// InternalConditions from the operator (keyed by condition name)
	InternalConditions map[status.ConditionName]status.Condition

	// ExternalConditions from Application.Status.Conditions
	ExternalConditions map[status.ConditionName]status.Condition

	// App provides access to spec/status for complex predicates
	App *v1alpha1.Application

	// VersionChanged indicates spec.version != status.currentVersion
	VersionChanged bool

	// IsInitialInstall indicates Installed condition was never True
	IsInitialInstall bool
}

// NewInput builds input from Application and internal conditions.
func NewInput(app *v1alpha1.Application, internalConds []status.Condition) *Input {
	input := &Input{
		InternalConditions: make(map[status.ConditionName]status.Condition),
		ExternalConditions: make(map[status.ConditionName]status.Condition),
		App:                app,
	}

	for _, c := range internalConds {
		input.InternalConditions[c.Name] = c
	}

	for _, c := range app.Status.Conditions {
		input.ExternalConditions[status.ConditionName(c.Type)] = status.Condition{
			Name:    status.ConditionName(c.Type),
			Status:  metav1.ConditionStatus(c.Status),
			Reason:  status.ConditionReason(c.Reason),
			Message: c.Message,
		}
	}

	input.IsInitialInstall = input.ExternalConditions[status.ConditionInstalled].Status != metav1.ConditionTrue
	input.VersionChanged = app.Status.CurrentVersion != nil &&
		app.Status.CurrentVersion.Current != "" &&
		app.Spec.Version != app.Status.CurrentVersion.Current

	return input
}
