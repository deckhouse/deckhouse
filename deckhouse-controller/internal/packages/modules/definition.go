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
}

// Requirements specifies dependencies required by the module.
type Requirements struct {
	Kubernetes *semver.Constraints `json:"kubernetes" yaml:"kubernetes"`
	Deckhouse  *semver.Constraints `json:"deckhouse" yaml:"deckhouse"`
	Modules    ModulesRequirements `json:"modules" yaml:"modules"`
}

// ModulesRequirements groups module dependencies by how they affect module startup.
type ModulesRequirements struct {
	// Mandatory lists modules that MUST be present (and satisfy constraint, if any)
	// for the module to start. The map value is nil when no version constraint applies.
	Mandatory map[string]*semver.Constraints `json:"mandatory" yaml:"mandatory"`
	// Conditional lists modules that are not required to be present, but if installed
	// must satisfy the version constraint. The map value is nil when no version constraint applies.
	Conditional map[string]*semver.Constraints `json:"conditional" yaml:"conditional"`
}

// DisableOptions configures application disablement behavior.
type DisableOptions struct {
	Confirmation bool   `json:"confirmation" yaml:"confirmation"` // Whether confirmation is required to disable
	Message      string `json:"message" yaml:"message"`           // Message to display when disabling
}

// Constraints projects the module definition onto the scheduler input shape,
// flattening mandatory and conditional module requirements into a single dependency map.
// Mandatory entries win over conditional entries when both reference the same module.
func (d Definition) Constraints() schedule.Constraints {
	deps := make(map[string]schedule.Dependency, len(d.Requirements.Modules.Mandatory)+len(d.Requirements.Modules.Conditional))
	for name, constraint := range d.Requirements.Modules.Conditional {
		deps[name] = schedule.Dependency{
			Constraint: constraint,
			Optional:   true,
		}
	}
	for name, constraint := range d.Requirements.Modules.Mandatory {
		deps[name] = schedule.Dependency{
			Constraint: constraint,
			Optional:   false,
		}
	}

	order := schedule.Order(d.Weight)
	if order == 0 {
		order = schedule.FunctionalOrder
	}

	return schedule.Constraints{
		Order:        order,
		Kubernetes:   d.Requirements.Kubernetes,
		Deckhouse:    d.Requirements.Deckhouse,
		Dependencies: deps,
	}
}
