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

package dto

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
)

const (
	// DefinitionFile is the filename for package metadata.
	DefinitionFile = "package.yaml"

	// TypeModule is the TypeMeta.Type value identifying a package as a module.
	TypeModule = "Module"
	// TypeApplication is the TypeMeta.Type value identifying a package as an application.
	TypeApplication = "Application"
)

// TypeMeta represents apiVersion/type header for determining the package type.
type TypeMeta struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Type       string `yaml:"type" json:"type"`
}

// Definition represents common package metadata loaded from package.yaml.
type Definition struct {
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
	Stage   string `yaml:"stage" json:"stage"`

	Descriptions   Descriptions   `yaml:"descriptions" json:"descriptions"`
	Requirements   Requirements   `yaml:"requirements" json:"requirements"`
	DisableOptions DisableOptions `yaml:"disable" json:"disable"`
}

// ApplicationDefinition extends Definition for application packages.
type ApplicationDefinition struct {
	TypeMeta   `yaml:",inline"`
	Definition `yaml:",inline"`
}

// ModuleDefinition extends Definition with module-specific fields.
type ModuleDefinition struct {
	TypeMeta   `yaml:",inline"`
	Definition `yaml:",inline"`

	Weight   int  `yaml:"weight" json:"weight"`
	Critical bool `yaml:"critical,omitempty" json:"critical,omitempty"`
}

// Descriptions holds localized description text for the package.
type Descriptions struct {
	Ru string `json:"ru,omitempty" yaml:"ru,omitempty"`
	En string `json:"en,omitempty" yaml:"en,omitempty"`
}

// Requirements specifies dependencies required by this package.
type Requirements struct {
	Kubernetes VersionConstraint   `yaml:"kubernetes" json:"kubernetes"`
	Deckhouse  VersionConstraint   `yaml:"deckhouse" json:"deckhouse"`
	Modules    ModulesRequirements `yaml:"modules" json:"modules"`
}

// VersionConstraint wraps a semver constraint expression for a single platform requirement.
// An empty VersionConstraint means "no version constraint".
type VersionConstraint struct {
	Constraint string `yaml:"constraint,omitempty" json:"constraint,omitempty"`
}

// ModulesRequirements groups module dependencies by how they affect package startup.
type ModulesRequirements struct {
	// Mandatory lists modules that MUST be present (and satisfy constraint, if any)
	// for the package to start.
	Mandatory []ModuleDependency `yaml:"mandatory,omitempty" json:"mandatory,omitempty"`
	// Conditional lists modules that are not required to be present, but if installed
	// must satisfy the version constraint for the package to work correctly.
	Conditional []ModuleDependency `yaml:"conditional,omitempty" json:"conditional,omitempty"`
	// AnyOf lists groups of alternative dependencies; at least one member of each
	// group must be installed (and satisfy its constraint, if any) for the package
	// to start. Each group must declare a stable Name used in scheduler diagnostics.
	AnyOf []ModuleGroup `yaml:"anyOf,omitempty" json:"anyOf,omitempty"`
	// NoneOf lists groups of forbidden dependencies; no member of any group may be
	// installed for the package to start. A member's constraint, if non-empty,
	// narrows the forbidden version range — an empty constraint forbids the
	// module entirely. Each group must declare a stable Name used in scheduler
	// diagnostics.
	NoneOf []ModuleGroup `yaml:"noneOf,omitempty" json:"noneOf,omitempty"`
}

// ModuleDependency is a single named module dependency with an optional semver constraint.
type ModuleDependency struct {
	Name       string `yaml:"name" json:"name"`
	Constraint string `yaml:"constraint,omitempty" json:"constraint,omitempty"`
}

// ModuleGroup is a named group of module dependencies. Group semantics depend on
// the containing bucket (AnyOf: at least one member must be installed; NoneOf:
// no member may be installed). Name is required and surfaces in scheduler error
// messages; Description is optional and flows through to the CR status for
// kubectl visibility but is not used in any scheduling decision.
type ModuleGroup struct {
	Name        string             `yaml:"name" json:"name"`
	Description string             `yaml:"description,omitempty" json:"description,omitempty"`
	Modules     []ModuleDependency `yaml:"modules" json:"modules"`
}

// DisableOptions configures package disablement behavior.
type DisableOptions struct {
	Confirmation bool            `json:"confirmation" yaml:"confirmation"`             // Whether confirmation is required to disable
	Messages     DisableMessages `json:"messages,omitempty" yaml:"messages,omitempty"` // Localized messages to display when disabling
}

// DisableMessages holds localized disable confirmation text for the package.
type DisableMessages struct {
	Ru string `json:"ru,omitempty" yaml:"ru,omitempty"`
	En string `json:"en,omitempty" yaml:"en,omitempty"`
}

// Convert converts application definition to application domain model.
func (d *ApplicationDefinition) Convert() (apps.Definition, error) {
	kubernetesConstraint, err := parseOptionalConstraint(d.Requirements.Kubernetes.Constraint)
	if err != nil {
		return apps.Definition{}, fmt.Errorf("parse kubernetes requirement: %w", err)
	}

	deckhouseConstraint, err := parseOptionalConstraint(d.Requirements.Deckhouse.Constraint)
	if err != nil {
		return apps.Definition{}, fmt.Errorf("parse deckhouse requirement: %w", err)
	}

	mandatory, err := buildDependencyMap(d.Requirements.Modules.Mandatory, "mandatory")
	if err != nil {
		return apps.Definition{}, err
	}

	conditional, err := buildDependencyMap(d.Requirements.Modules.Conditional, "conditional")
	if err != nil {
		return apps.Definition{}, err
	}

	anyOf, err := buildModuleGroups(d.Requirements.Modules.AnyOf, "anyOf")
	if err != nil {
		return apps.Definition{}, err
	}

	noneOf, err := buildModuleGroups(d.Requirements.Modules.NoneOf, "noneOf")
	if err != nil {
		return apps.Definition{}, err
	}

	if err := validateBucketCollisions(mandatory, conditional, anyOf, noneOf); err != nil {
		return apps.Definition{}, err
	}

	return apps.Definition{
		Name:    d.Name,
		Version: d.Version,
		Stage:   d.Stage,
		Requirements: apps.Requirements{
			Kubernetes: kubernetesConstraint,
			Deckhouse:  deckhouseConstraint,
			Modules: apps.ModulesRequirements{
				Mandatory:   mandatory,
				Conditional: conditional,
				AnyOf:       toAppGroups(anyOf),
				NoneOf:      toAppGroups(noneOf),
			},
		},
	}, nil
}

// toAppGroups widens the parser-internal parsedGroup into apps.ModuleGroup,
// dropping the Description (carried only on the dto.ModuleGroup → CR path,
// not needed by the scheduler-facing domain type).
func toAppGroups(groups []parsedGroup) []apps.ModuleGroup {
	out := make([]apps.ModuleGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, apps.ModuleGroup{Name: g.Name, Members: g.Members})
	}

	return out
}

// Convert converts module definition to module domain model.
func (d *ModuleDefinition) Convert() (modules.Definition, error) {
	kubernetesConstraint, err := parseOptionalConstraint(d.Requirements.Kubernetes.Constraint)
	if err != nil {
		return modules.Definition{}, fmt.Errorf("parse kubernetes requirement: %w", err)
	}

	deckhouseConstraint, err := parseOptionalConstraint(d.Requirements.Deckhouse.Constraint)
	if err != nil {
		return modules.Definition{}, fmt.Errorf("parse deckhouse requirement: %w", err)
	}

	mandatory, err := buildDependencyMap(d.Requirements.Modules.Mandatory, "mandatory")
	if err != nil {
		return modules.Definition{}, err
	}

	conditional, err := buildDependencyMap(d.Requirements.Modules.Conditional, "conditional")
	if err != nil {
		return modules.Definition{}, err
	}

	anyOf, err := buildModuleGroups(d.Requirements.Modules.AnyOf, "anyOf")
	if err != nil {
		return modules.Definition{}, err
	}

	noneOf, err := buildModuleGroups(d.Requirements.Modules.NoneOf, "noneOf")
	if err != nil {
		return modules.Definition{}, err
	}

	if err := validateBucketCollisions(mandatory, conditional, anyOf, noneOf); err != nil {
		return modules.Definition{}, err
	}

	return modules.Definition{
		Name:     d.Name,
		Version:  d.Version,
		Critical: d.Critical,
		Weight:   uint32(d.Weight),
		Stage:    d.Stage,
		Requirements: modules.Requirements{
			Kubernetes: kubernetesConstraint,
			Deckhouse:  deckhouseConstraint,
			Modules: modules.ModulesRequirements{
				Mandatory:   mandatory,
				Conditional: conditional,
				AnyOf:       toModuleGroups(anyOf),
				NoneOf:      toModuleGroups(noneOf),
			},
		},
	}, nil
}

// toModuleGroups widens the parser-internal parsedGroup into modules.ModuleGroup,
// dropping the Description (carried only on the dto.ModuleGroup → CR path, not
// needed by the scheduler-facing domain type).
func toModuleGroups(groups []parsedGroup) []modules.ModuleGroup {
	out := make([]modules.ModuleGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, modules.ModuleGroup{Name: g.Name, Members: g.Members})
	}

	return out
}

// buildDependencyMap turns a list of ModuleDependency into a name → constraint map,
// parsing each entry's semver constraint. The kind argument ("mandatory" or
// "conditional") is woven into error messages so failures point at the right section.
// Mandatory entries may omit the constraint (interpreted as "any version, must be
// installed"); conditional entries must declare one, since "if installed, no version
// requirement" is a no-op and almost always indicates a malformed package.
func buildDependencyMap(deps []ModuleDependency, kind string) (map[string]*semver.Constraints, error) {
	out := make(map[string]*semver.Constraints, len(deps))
	for _, dep := range deps {
		if kind == "conditional" && len(dep.Constraint) == 0 {
			return nil, fmt.Errorf("parse conditional module requirement '%s': constraint is required", dep.Name)
		}

		constraint, err := parseOptionalConstraint(dep.Constraint)
		if err != nil {
			return nil, fmt.Errorf("parse %s module requirement '%s': %w", kind, dep.Name, err)
		}

		out[dep.Name] = constraint
	}

	return out, nil
}

// parseOptionalConstraint parses a semver constraint expression, returning a nil
// pointer when the expression is empty (meaning "no version constraint").
func parseOptionalConstraint(raw string) (*semver.Constraints, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	return semver.NewConstraint(raw)
}

// parsedGroup is the parser-internal projection of a ModuleGroup with member
// constraints already resolved to *semver.Constraints. Domain layers (apps,
// modules) widen this into their own ModuleGroup type carrying the same shape.
type parsedGroup struct {
	Name    string
	Members map[string]*semver.Constraints
}

// buildModuleGroups validates and parses a list of dto.ModuleGroup for the
// given bucket name (woven into error messages so failures point at the right
// section: "anyOf" or "noneOf"). Each group must declare a non-empty Name
// (used by the scheduler in diagnostics), a unique Name across the bucket, and
// at least one member. Member names must be unique within a group; member
// constraints, when present, must be valid semver. Empty member constraints
// are allowed and mean "any version" — interpreted by the bucket's checker
// (anyOf: "any installed version of this alternative is acceptable"; noneOf:
// "this module is forbidden at any version").
func buildModuleGroups(groups []ModuleGroup, bucket string) ([]parsedGroup, error) {
	if len(groups) == 0 {
		return nil, nil
	}

	seenGroupNames := make(map[string]struct{}, len(groups))
	out := make([]parsedGroup, 0, len(groups))

	for i, g := range groups {
		if len(g.Name) == 0 {
			return nil, fmt.Errorf("parse %s group [%d]: name is required", bucket, i)
		}

		if _, dup := seenGroupNames[g.Name]; dup {
			return nil, fmt.Errorf("parse %s group '%s': duplicate group name", bucket, g.Name)
		}

		seenGroupNames[g.Name] = struct{}{}

		if len(g.Modules) == 0 {
			return nil, fmt.Errorf("parse %s group '%s': at least one member is required", bucket, g.Name)
		}

		members := make(map[string]*semver.Constraints, len(g.Modules))
		for _, m := range g.Modules {
			if len(m.Name) == 0 {
				return nil, fmt.Errorf("parse %s group '%s': member name is required", bucket, g.Name)
			}

			if _, dup := members[m.Name]; dup {
				return nil, fmt.Errorf("parse %s group '%s': duplicate member '%s'", bucket, g.Name, m.Name)
			}

			constraint, err := parseOptionalConstraint(m.Constraint)
			if err != nil {
				return nil, fmt.Errorf("parse %s group '%s' member '%s': %w", bucket, g.Name, m.Name, err)
			}

			members[m.Name] = constraint
		}

		out = append(out, parsedGroup{Name: g.Name, Members: members})
	}

	return out, nil
}

// validateBucketCollisions rejects module names that appear in more than one
// bucket. Mandatory and conditional are mutually exclusive (a module cannot be
// both required and skippable). AnyOf members cannot also appear in mandatory
// or conditional, since the unconditional bucket subsumes the alternative —
// the anyOf member would be dead code, almost always indicating a copy-paste
// mistake. NoneOf members cannot appear in any other bucket: "must be
// installed" and "must not be installed" are flatly contradictory; "needed as
// fallback" and "forbidden" are also contradictory.
//
// Within a single bucket, the same name across distinct groups is allowed
// (multi-coverage for anyOf; redundant-but-not-wrong for noneOf).
func validateBucketCollisions(mandatory, conditional map[string]*semver.Constraints, anyOf, noneOf []parsedGroup) error {
	for name := range conditional {
		if _, clash := mandatory[name]; clash {
			return fmt.Errorf("module '%s' appears in both mandatory and conditional", name)
		}
	}

	for _, g := range anyOf {
		for memberName := range g.Members {
			if _, clash := mandatory[memberName]; clash {
				return fmt.Errorf("module '%s' appears in both mandatory and anyOf group '%s'", memberName, g.Name)
			}

			if _, clash := conditional[memberName]; clash {
				return fmt.Errorf("module '%s' appears in both conditional and anyOf group '%s'", memberName, g.Name)
			}
		}
	}

	for _, g := range noneOf {
		for memberName := range g.Members {
			if _, clash := mandatory[memberName]; clash {
				return fmt.Errorf("module '%s' appears in both mandatory and noneOf group '%s'", memberName, g.Name)
			}

			if _, clash := conditional[memberName]; clash {
				return fmt.Errorf("module '%s' appears in both conditional and noneOf group '%s'", memberName, g.Name)
			}

			for _, ag := range anyOf {
				if _, clash := ag.Members[memberName]; clash {
					return fmt.Errorf("module '%s' appears in both anyOf group '%s' and noneOf group '%s'", memberName, ag.Name, g.Name)
				}
			}
		}
	}

	return nil
}
