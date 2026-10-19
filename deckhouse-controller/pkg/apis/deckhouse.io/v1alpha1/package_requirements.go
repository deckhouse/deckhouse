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

package v1alpha1

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
)

// ToScheduleConstraints parses the module version's requirements and gives them the module's
// order (Weight for a critical module, else FunctionalOrder, as modules.Definition derives it): the
// module's node gets the same order, so the bootstrap rule, which applies at FunctionalOrder only,
// gates exactly the functional modules. nil metadata yields no requirements at FunctionalOrder.
func (m *ModulePackageVersionStatusMetadata) ToScheduleConstraints() (schedule.Constraints, error) {
	if m == nil {
		return (*PackageRequirements)(nil).ToScheduleConstraints()
	}

	constraints, err := m.Requirements.ToScheduleConstraints()
	if err != nil {
		return schedule.Constraints{}, err
	}

	if m.Critical && m.Weight != 0 {
		constraints.Order = schedule.Order(m.Weight)
	}

	return constraints, nil
}

// ToScheduleConstraints parses the requirements into scheduler constraints at the functional
// order; nil requirements yield none. The package controllers and the admission webhooks share it,
// so a version the webhook admits is gated by the same constraints.
func (reqs *PackageRequirements) ToScheduleConstraints() (schedule.Constraints, error) {
	constraints := schedule.Constraints{
		Order: schedule.FunctionalOrder,
	}

	if reqs == nil {
		return constraints, nil
	}

	// Parse the minimum Kubernetes version constraint (e.g. ">= 1.28").
	kubernetesConstraint, err := parsePackageConstraint(reqs.Kubernetes)
	if err != nil {
		return schedule.Constraints{}, fmt.Errorf("parse kubernetes requirement: %w", err)
	}

	constraints.Kubernetes = kubernetesConstraint

	// Parse the minimum Deckhouse version constraint (e.g. ">= 1.60").
	deckhouseConstraint, err := parsePackageConstraint(reqs.Deckhouse)
	if err != nil {
		return schedule.Constraints{}, fmt.Errorf("parse deckhouse requirement: %w", err)
	}

	constraints.Deckhouse = deckhouseConstraint

	// Parse module dependency constraints. Mandatory entries must be present;
	// conditional entries (formerly the "!optional" suffix) are skippable;
	// anyOf groups require ≥1 installed member that satisfies its constraint;
	// noneOf groups require zero installed members that match their constraints.
	// A name listed in both mandatory and conditional is rejected — silently
	// letting conditional overwrite mandatory would weaken the requirement
	// without telling the user.
	modules := make(map[string]schedule.Dependency)
	var anyOfGroups []schedule.AnyOfGroup
	var noneOfGroups []schedule.NoneOfGroup
	if reqs.Modules != nil {
		for _, dep := range reqs.Modules.Mandatory {
			constraint, err := parsePackageDependencyConstraint(dep.Constraint)
			if err != nil {
				return schedule.Constraints{}, fmt.Errorf("parse mandatory module requirement '%s': %w", dep.Name, err)
			}

			modules[dep.Name] = schedule.Dependency{
				Constraint: constraint,
				Optional:   false,
			}
		}
		for _, dep := range reqs.Modules.Conditional {
			if _, ok := modules[dep.Name]; ok {
				return schedule.Constraints{}, fmt.Errorf("parse conditional module requirement '%s': also listed as mandatory", dep.Name)
			}

			if len(dep.Constraint) == 0 {
				return schedule.Constraints{}, fmt.Errorf("parse conditional module requirement '%s': constraint is required", dep.Name)
			}

			constraint, err := parsePackageDependencyConstraint(dep.Constraint)
			if err != nil {
				return schedule.Constraints{}, fmt.Errorf("parse conditional module requirement '%s': %w", dep.Name, err)
			}

			modules[dep.Name] = schedule.Dependency{
				Constraint: constraint,
				Optional:   true,
			}
		}

		anyOfGroups = make([]schedule.AnyOfGroup, 0, len(reqs.Modules.AnyOf))
		seenAnyOfNames := make(map[string]struct{}, len(reqs.Modules.AnyOf))
		for i, group := range reqs.Modules.AnyOf {
			if len(group.Name) == 0 {
				return schedule.Constraints{}, fmt.Errorf("parse anyOf group [%d]: name is required", i)
			}

			if _, dup := seenAnyOfNames[group.Name]; dup {
				return schedule.Constraints{}, fmt.Errorf("parse anyOf group '%s': duplicate group name", group.Name)
			}

			seenAnyOfNames[group.Name] = struct{}{}

			if len(group.Modules) == 0 {
				return schedule.Constraints{}, fmt.Errorf("parse anyOf group '%s': at least one member is required", group.Name)
			}

			members := make(map[string]*semver.Constraints, len(group.Modules))
			for _, m := range group.Modules {
				if len(m.Name) == 0 {
					return schedule.Constraints{}, fmt.Errorf("parse anyOf group '%s': member name is required", group.Name)
				}

				if _, dup := members[m.Name]; dup {
					return schedule.Constraints{}, fmt.Errorf("parse anyOf group '%s': duplicate member '%s'", group.Name, m.Name)
				}

				if existing, clash := modules[m.Name]; clash {
					bucket := "mandatory"
					if existing.Optional {
						bucket = "conditional"
					}

					return schedule.Constraints{}, fmt.Errorf("parse anyOf group '%s' member '%s': also listed as %s", group.Name, m.Name, bucket)
				}

				constraint, err := parsePackageDependencyConstraint(m.Constraint)
				if err != nil {
					return schedule.Constraints{}, fmt.Errorf("parse anyOf group '%s' member '%s': %w", group.Name, m.Name, err)
				}

				members[m.Name] = constraint
			}

			anyOfGroups = append(anyOfGroups, schedule.AnyOfGroup{
				Name:    group.Name,
				Members: members,
			})
		}

		noneOfGroups = make([]schedule.NoneOfGroup, 0, len(reqs.Modules.NoneOf))
		seenNoneOfNames := make(map[string]struct{}, len(reqs.Modules.NoneOf))
		for i, group := range reqs.Modules.NoneOf {
			if len(group.Name) == 0 {
				return schedule.Constraints{}, fmt.Errorf("parse noneOf group [%d]: name is required", i)
			}

			if _, dup := seenNoneOfNames[group.Name]; dup {
				return schedule.Constraints{}, fmt.Errorf("parse noneOf group '%s': duplicate group name", group.Name)
			}

			seenNoneOfNames[group.Name] = struct{}{}

			if len(group.Modules) == 0 {
				return schedule.Constraints{}, fmt.Errorf("parse noneOf group '%s': at least one member is required", group.Name)
			}

			members := make(map[string]*semver.Constraints, len(group.Modules))
			for _, m := range group.Modules {
				if len(m.Name) == 0 {
					return schedule.Constraints{}, fmt.Errorf("parse noneOf group '%s': member name is required", group.Name)
				}

				if _, dup := members[m.Name]; dup {
					return schedule.Constraints{}, fmt.Errorf("parse noneOf group '%s': duplicate member '%s'", group.Name, m.Name)
				}

				if existing, clash := modules[m.Name]; clash {
					bucket := "mandatory"
					if existing.Optional {
						bucket = "conditional"
					}

					return schedule.Constraints{}, fmt.Errorf("parse noneOf group '%s' member '%s': also listed as %s", group.Name, m.Name, bucket)
				}

				for _, ag := range anyOfGroups {
					if _, clash := ag.Members[m.Name]; clash {
						return schedule.Constraints{}, fmt.Errorf("parse noneOf group '%s' member '%s': also listed in anyOf group '%s'", group.Name, m.Name, ag.Name)
					}
				}

				constraint, err := parsePackageDependencyConstraint(m.Constraint)
				if err != nil {
					return schedule.Constraints{}, fmt.Errorf("parse noneOf group '%s' member '%s': %w", group.Name, m.Name, err)
				}

				members[m.Name] = constraint
			}

			noneOfGroups = append(noneOfGroups, schedule.NoneOfGroup{
				Name:    group.Name,
				Members: members,
			})
		}
	}

	constraints.Dependencies = modules
	constraints.AnyOf = anyOfGroups
	constraints.NoneOf = noneOfGroups

	return constraints, nil
}

// parsePackageConstraint parses the optional semver expression on a PackageConstraint,
// returning a nil *semver.Constraints when the wrapper is nil or its constraint is empty.
func parsePackageConstraint(c *VersionConstraint) (*semver.Constraints, error) {
	if c == nil {
		return nil, nil
	}

	return parsePackageDependencyConstraint(c.Constraint)
}

// parsePackageDependencyConstraint parses a raw semver expression, treating an empty
// string as "no constraint" rather than an error.
func parsePackageDependencyConstraint(raw string) (*semver.Constraints, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	return semver.NewConstraint(raw)
}
