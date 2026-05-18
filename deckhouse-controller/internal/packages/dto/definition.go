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
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
)

const (
	// DefinitionFile is the filename for package metadata.
	DefinitionFile = "package.yaml"

	// TypeModule identifies a Module-kind package in TypeMeta.
	TypeModule = "Module"
	// TypeApplication identifies an Application-kind package in TypeMeta.
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
	Subscribe      Subscribe      `yaml:"subscribe,omitempty" json:"subscribe,omitempty"`
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

// DisableOptions configures package disablement behavior.
type DisableOptions struct {
	Confirmation bool   `json:"confirmation" yaml:"confirmation"` // Whether confirmation is required to disable
	Message      string `json:"message" yaml:"message"`           // Message to display when disabling
}

// Subscribe declares reactive bindings for a package. APIs lists Kubernetes
// API groups whose changes the package wants to be notified about (handled by
// the runtime informer layer, not the scheduler). Values declares a values
// dependency on another package: when that package's values change at Path,
// this package is rerun. Values.Module becomes a scheduler subscription edge.
type Subscribe struct {
	APIs   []string          `yaml:"apis" json:"apis"`
	Values []SubscribeValues `yaml:"values" json:"values"`
}

// SubscribeValues identifies a values path on a specific package whose changes
// should trigger this package to rerun. Module is the package name; Path is a
// dotted path into its values document.
type SubscribeValues struct {
	Module string `yaml:"module" json:"module"`
	Path   string `yaml:"path" json:"path"`
}

// Requirements specifies dependencies required by this package.
type Requirements struct {
	Kubernetes VersionConstraint  `yaml:"kubernetes" json:"kubernetes"`
	Deckhouse  VersionConstraint  `yaml:"deckhouse" json:"deckhouse"`
	Modules    ModulesRequirement `yaml:"modules" json:"modules"`
}

// VersionConstraint wraps a semver range expression that gates a requirement.
// An empty Constraint disables the check.
type VersionConstraint struct {
	Constraint string `yaml:"constraint,omitempty" json:"constraint,omitempty"`
}

// ModulesRequirement groups module dependencies by enforcement semantics.
type ModulesRequirement struct {
	// Mandatory modules must be installed and version-satisfying; otherwise the package does not start.
	Mandatory []ModuleDependency `yaml:"mandatory,omitempty" json:"mandatory,omitempty"`
	// Conditional modules are checked only when installed; absent modules are silently skipped.
	Conditional []ModuleDependency `yaml:"conditional,omitempty" json:"conditional,omitempty"`
	// AnyOf expresses "satisfy at least one" requirements: each group passes when ≥1 of its modules is installed and version-satisfying.
	AnyOf []ModuleGroup `yaml:"anyOf,omitempty" json:"anyOf,omitempty"`
}

// ModuleDependency declares a semver constraint on a specific module by name.
type ModuleDependency struct {
	Name       string `yaml:"name" json:"name"`
	Constraint string `yaml:"constraint,omitempty" json:"constraint,omitempty"`
}

// ModuleGroup expresses a "satisfy at least one" constraint over a set of modules.
// The group passes when ≥1 module in Modules is installed and version-satisfying.
// Name identifies the group and is surfaced in scheduler failure messages.
// Description is optional human-facing documentation; it is not consumed by the
// scheduler but is preserved through the CRD for UI/docs use.
type ModuleGroup struct {
	Name        string             `yaml:"name" json:"name"`
	Description string             `yaml:"description,omitempty" json:"description,omitempty"`
	Modules     []ModuleDependency `yaml:"modules" json:"modules"`
}

// Convert converts application definition to application domain model.
func (d *ApplicationDefinition) Convert() (apps.Definition, error) {
	kubernetesConstraint, err := parseConstraint(d.Requirements.Kubernetes.Constraint)
	if err != nil {
		return apps.Definition{}, fmt.Errorf("parse kubernetes requirement: %w", err)
	}

	deckhouseConstraint, err := parseConstraint(d.Requirements.Deckhouse.Constraint)
	if err != nil {
		return apps.Definition{}, fmt.Errorf("parse deckhouse requirement: %w", err)
	}

	deps, err := resolveModuleDeps(d.Requirements.Modules)
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
			Modules:    deps,
		},
		Subscribe: apps.Subscribe{
			APIs:   d.Subscribe.APIs,
			Values: appsSubscribeValues(d.Subscribe.Values),
		},
	}, nil
}

// Convert converts module definition to module domain model.
func (d *ModuleDefinition) Convert() (modules.Definition, error) {
	kubernetesConstraint, err := parseConstraint(d.Requirements.Kubernetes.Constraint)
	if err != nil {
		return modules.Definition{}, fmt.Errorf("parse kubernetes requirement: %w", err)
	}

	deckhouseConstraint, err := parseConstraint(d.Requirements.Deckhouse.Constraint)
	if err != nil {
		return modules.Definition{}, fmt.Errorf("parse deckhouse requirement: %w", err)
	}

	deps, err := resolveModuleDeps(d.Requirements.Modules)
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
			Modules:    deps,
		},
		Subscribe: modules.Subscribe{
			APIs:   d.Subscribe.APIs,
			Values: modulesSubscribeValues(d.Subscribe.Values),
		},
	}, nil
}

// appsSubscribeValues translates dto SubscribeValues into the apps domain.
func appsSubscribeValues(in []SubscribeValues) []apps.SubscribeValues {
	if len(in) == 0 {
		return nil
	}

	out := make([]apps.SubscribeValues, len(in))
	for i, v := range in {
		out[i] = apps.SubscribeValues{Module: v.Module, Path: v.Path}
	}

	return out
}

// modulesSubscribeValues translates dto SubscribeValues into the modules domain.
func modulesSubscribeValues(in []SubscribeValues) []modules.SubscribeValues {
	if len(in) == 0 {
		return nil
	}

	out := make([]modules.SubscribeValues, len(in))
	for i, v := range in {
		out[i] = modules.SubscribeValues{Module: v.Module, Path: v.Path}
	}

	return out
}

// resolveModuleDeps parses the YAML-shaped module requirements into the
// scheduler's three-bucket Dependencies shape. Empty names, duplicate names
// within a bucket, names that appear in more than one of mandatory /
// conditional / an anyOf group, empty anyOf groups, and unparseable semver
// constraints are all rejected so misconfigured manifests fail loudly at load.
func resolveModuleDeps(req ModulesRequirement) (schedule.Dependencies, error) {
	deps := schedule.Dependencies{
		Mandatory:   make(map[string]*semver.Constraints, len(req.Mandatory)),
		Conditional: make(map[string]*semver.Constraints, len(req.Conditional)),
		AnyOf:       make([]schedule.AnyOfGroup, 0, len(req.AnyOf)),
	}

	// Track names already claimed by any bucket so we can reject collisions
	// (e.g. the same module appearing in both mandatory and an anyOf group).
	seen := make(map[string]struct{}, len(req.Mandatory)+len(req.Conditional))

	for i, m := range req.Mandatory {
		if m.Name == "" {
			return schedule.Dependencies{}, fmt.Errorf("mandatory module #%d: name is empty", i)
		}

		if _, dup := seen[m.Name]; dup {
			return schedule.Dependencies{}, fmt.Errorf("module %q declared more than once", m.Name)
		}

		c, err := parseConstraint(m.Constraint)
		if err != nil {
			return schedule.Dependencies{}, fmt.Errorf("parse mandatory module %q requirement: %w", m.Name, err)
		}

		deps.Mandatory[m.Name] = c
		seen[m.Name] = struct{}{}
	}

	for i, m := range req.Conditional {
		if m.Name == "" {
			return schedule.Dependencies{}, fmt.Errorf("conditional module #%d: name is empty", i)
		}

		if _, dup := seen[m.Name]; dup {
			return schedule.Dependencies{}, fmt.Errorf("module %q declared more than once", m.Name)
		}

		c, err := parseConstraint(m.Constraint)
		if err != nil {
			return schedule.Dependencies{}, fmt.Errorf("parse conditional module %q requirement: %w", m.Name, err)
		}

		deps.Conditional[m.Name] = c
		seen[m.Name] = struct{}{}
	}

	for gi, g := range req.AnyOf {
		if g.Name == "" {
			return schedule.Dependencies{}, fmt.Errorf("anyOf group #%d: name is empty", gi)
		}

		if len(g.Modules) == 0 {
			return schedule.Dependencies{}, fmt.Errorf("anyOf group %q: no modules", g.Name)
		}

		group := schedule.AnyOfGroup{
			Name:    g.Name,
			Modules: make(map[string]*semver.Constraints, len(g.Modules)),
		}

		for mi, m := range g.Modules {
			if m.Name == "" {
				return schedule.Dependencies{}, fmt.Errorf("anyOf group %q module #%d: name is empty", g.Name, mi)
			}

			if _, dup := group.Modules[m.Name]; dup {
				return schedule.Dependencies{}, fmt.Errorf("anyOf group %q: module %q declared more than once", g.Name, m.Name)
			}

			if _, dup := seen[m.Name]; dup {
				return schedule.Dependencies{}, fmt.Errorf("anyOf group %q: module %q already declared in mandatory/conditional", g.Name, m.Name)
			}

			c, err := parseConstraint(m.Constraint)
			if err != nil {
				return schedule.Dependencies{}, fmt.Errorf("parse anyOf group %q module %q requirement: %w", g.Name, m.Name, err)
			}

			group.Modules[m.Name] = c
			seen[m.Name] = struct{}{}
		}

		deps.AnyOf = append(deps.AnyOf, group)
	}

	return deps, nil
}

// parseConstraint compiles a semver range expression, returning a nil
// Constraints when the input is empty (meaning "no version gate").
func parseConstraint(s string) (*semver.Constraints, error) {
	if s == "" {
		return nil, nil
	}

	return semver.NewConstraint(s)
}
