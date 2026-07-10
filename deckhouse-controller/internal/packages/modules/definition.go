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
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
)

// Definition represents module metadata.
type Definition struct {
	Name     string `json:"name" yaml:"name"`
	Version  string `json:"version" yaml:"version"`
	Stage    string `json:"stage" yaml:"stage"`
	Critical bool   `json:"critical,omitempty" yaml:"critical,omitempty"`
	Weight   uint32 `json:"weight,omitempty" yaml:"weight,omitempty"`

	Requirements   Requirements      `json:"requirements" yaml:"requirements"`
	Licensing      edition.Licensing `json:"licensing" yaml:"licensing"`
	DisableOptions DisableOptions    `json:"disableOptions" yaml:"disableOptions"`
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
	// AnyOf lists groups of alternative dependencies; at least one member of each
	// group must be present (and satisfy its constraint, if any) for the module to
	// start. AnyOf groups are checker-only — they add no edges to the dependency
	// graph, so fallback chains across packages do not produce cycles.
	AnyOf []ModuleGroup `json:"anyOf,omitempty" yaml:"anyOf,omitempty"`
	// NoneOf lists groups of forbidden dependencies; no member of any group may be
	// present for the module to start. A nil constraint on a member forbids any
	// installed version; a non-nil constraint narrows the forbidden range so
	// versions outside it remain acceptable. Checker-only — adds no graph edges.
	NoneOf []ModuleGroup `json:"noneOf,omitempty" yaml:"noneOf,omitempty"`
}

// ModuleGroup is a named group of module dependencies shared by the AnyOf and
// NoneOf buckets; the containing field decides whether members are alternatives
// (at least one must be installed) or forbidden (none may be installed). Members
// maps each member's module name to its semver constraint (nil meaning "any
// version"). Name is the stable identifier used by the scheduler in diagnostics.
type ModuleGroup struct {
	Name    string                         `json:"name" yaml:"name"`
	Members map[string]*semver.Constraints `json:"members" yaml:"members"`
}

// DisableOptions configures module disablement behavior.
type DisableOptions struct {
	Confirmation bool            `json:"confirmation" yaml:"confirmation"`
	Messages     DisableMessages `json:"messages" yaml:"messages"`
}

// DisableMessages holds localized disable confirmation messages for the module.
type DisableMessages struct {
	Ru string `json:"ru,omitempty" yaml:"ru,omitempty"`
	En string `json:"en,omitempty" yaml:"en,omitempty"`
}

// Constraints projects the module definition onto the scheduler input shape,
// flattening mandatory and conditional module requirements into a single dependency
// map and projecting AnyOf groups onto schedule.AnyOfGroup. Mandatory entries win
// over conditional entries when both reference the same module.
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

	anyOf := make([]schedule.AnyOfGroup, 0, len(d.Requirements.Modules.AnyOf))
	for _, g := range d.Requirements.Modules.AnyOf {
		anyOf = append(anyOf, schedule.AnyOfGroup{
			Name:    g.Name,
			Members: g.Members,
		})
	}

	noneOf := make([]schedule.NoneOfGroup, 0, len(d.Requirements.Modules.NoneOf))
	for _, g := range d.Requirements.Modules.NoneOf {
		noneOf = append(noneOf, schedule.NoneOfGroup{
			Name:    g.Name,
			Members: g.Members,
		})
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
		AnyOf:        anyOf,
		NoneOf:       noneOf,
		Subscriptions: map[string]struct{}{
			"global": {},
		},
		Licensing: d.Licensing,
		// Modules are disabled by default; a higher-precedence intent rule
		// (bundle membership, user config) turns them on.
		Floor: rule.Static(rule.Disable),
	}
}
