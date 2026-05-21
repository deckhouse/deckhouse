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
}

// ModuleDependency is a single named module dependency with an optional semver constraint.
type ModuleDependency struct {
	Name       string `yaml:"name" json:"name"`
	Constraint string `yaml:"constraint,omitempty" json:"constraint,omitempty"`
}

// DisableOptions configures package disablement behavior.
type DisableOptions struct {
	Confirmation bool   `json:"confirmation" yaml:"confirmation"` // Whether confirmation is required to disable
	Message      string `json:"message" yaml:"message"`           // Message to display when disabling
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

	return apps.Definition{
		Name:    d.Name,
		Version: d.Version,
		Stage:   d.Stage,
		DisableOptions: apps.DisableOptions{
			Confirmation: d.DisableOptions.Confirmation,
			Message:      d.DisableOptions.Message,
		},
		Requirements: apps.Requirements{
			Kubernetes: kubernetesConstraint,
			Deckhouse:  deckhouseConstraint,
			Modules: apps.ModulesRequirements{
				Mandatory:   mandatory,
				Conditional: conditional,
			},
		},
	}, nil
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

	return modules.Definition{
		Name:     d.Name,
		Version:  d.Version,
		Critical: d.Critical,
		Weight:   uint32(d.Weight),
		Stage:    d.Stage,
		DisableOptions: modules.DisableOptions{
			Confirmation: d.DisableOptions.Confirmation,
			Message:      d.DisableOptions.Message,
		},
		Requirements: modules.Requirements{
			Kubernetes: kubernetesConstraint,
			Deckhouse:  deckhouseConstraint,
			Modules: modules.ModulesRequirements{
				Mandatory:   mandatory,
				Conditional: conditional,
			},
		},
	}, nil
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
