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

package modules

import (
	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
)

// Definition represents module metadata.
type Definition struct {
	Name     string `json:"name" yaml:"name"`
	Version  string `json:"version" yaml:"version"`
	Stage    string `json:"stage" yaml:"stage"`
	Critical bool   `json:"critical,omitempty" yaml:"critical,omitempty"`
	Weight   uint32 `json:"weight,omitempty" yaml:"weight,omitempty"`

	Requirements   Requirements   `json:"requirements" yaml:"requirements"`
	DisableOptions DisableOptions `json:"disableOptions" yaml:"disableOptions"`
	Subscribe      Subscribe      `json:"subscribe,omitempty" yaml:"subscribe,omitempty"`
}

// Subscribe declares reactive bindings for a module. APIs lists Kubernetes
// API groups whose changes the runtime should observe; the scheduler does
// not consume these. Values is a list of values-path bindings on other
// packages — each Module becomes a scheduler subscription edge through
// Module.GetSubscriptions.
type Subscribe struct {
	APIs   []string          `json:"apis,omitempty" yaml:"apis,omitempty"`
	Values []SubscribeValues `json:"values,omitempty" yaml:"values,omitempty"`
}

// SubscribeValues identifies a values path on another package whose changes
// should rerun this module. Module is the target package name; Path is the
// dotted path into its values document.
type SubscribeValues struct {
	Module string `json:"module" yaml:"module"`
	Path   string `json:"path" yaml:"path"`
}

// Requirements specifies dependencies required by the module.
type Requirements struct {
	Kubernetes *semver.Constraints   `json:"kubernetes" yaml:"kubernetes"`
	Deckhouse  *semver.Constraints   `json:"deckhouse" yaml:"deckhouse"`
	Modules    schedule.Dependencies `json:"modules" yaml:"modules"`
}

// DisableOptions configures application disablement behavior.
type DisableOptions struct {
	Confirmation bool   `json:"confirmation" yaml:"confirmation"` // Whether confirmation is required to disable
	Message      string `json:"message" yaml:"message"`           // Message to display when disabling
}

// Constraints projects the module definition into the scheduler's Constraints
// shape. Weight maps to the scheduler Order; modules without a weight fall
// back to the functional tier.
func (d Definition) Constraints() schedule.Constraints {
	order := schedule.Order(d.Weight)
	if order == 0 {
		order = schedule.FunctionalOrder
	}

	return schedule.Constraints{
		Order:        order,
		Kubernetes:   d.Requirements.Kubernetes,
		Deckhouse:    d.Requirements.Deckhouse,
		Dependencies: d.Requirements.Modules,
	}
}
