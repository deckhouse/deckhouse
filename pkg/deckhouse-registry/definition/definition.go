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

// Package definition maps the two manifests a release image can carry.
//
// module.yaml is the legacy manifest, published by module releases under
// modules/<module>/release. package.yaml is the v2 manifest, published by
// package releases under packages/<package>/version and shared by modules and
// applications alike.
//
// They are not two spellings of one schema. Requirements in particular differ:
// module.yaml states them as bare version ranges and a flat module map, while
// package.yaml wraps each in a constraint object and splits module
// dependencies into mandatory/conditional/anyOf/noneOf buckets. Both shapes are
// kept as written, so nothing is lost in translation.
//
// These are transport DTOs — they decode the file and nothing more. Validating
// requirement buckets, resolving semver constraints and projecting onto cluster
// resources stay with the consumer (see the deckhouse-controller package
// loader, which owns those rules).
package definition

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// File names of the two manifests, at the release image root.
const (
	// ModuleFile is the legacy manifest, on a module release image.
	ModuleFile = "module.yaml"
	// PackageFile is the v2 manifest, on a package release image.
	PackageFile = "package.yaml"
)

// Package types, as written in package.yaml's type field.
const (
	TypeModule      = "Module"
	TypeApplication = "Application"
)

// Module maps module.yaml, the legacy manifest.
type Module struct {
	Name           string   `yaml:"name" json:"name"`
	Critical       bool     `yaml:"critical,omitempty" json:"critical,omitempty"`
	Weight         uint32   `yaml:"weight,omitempty" json:"weight,omitempty"`
	Tags           []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Subsystems     []string `yaml:"subsystems,omitempty" json:"subsystems,omitempty"`
	Namespace      string   `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Stage          string   `yaml:"stage,omitempty" json:"stage,omitempty"`
	ExclusiveGroup string   `yaml:"exclusiveGroup,omitempty" json:"exclusiveGroup,omitempty"`

	Descriptions  *Descriptions       `yaml:"descriptions,omitempty" json:"descriptions,omitempty"`
	Requirements  *ModuleRequirements `yaml:"requirements,omitempty" json:"requirements,omitempty"`
	Accessibility *Accessibility      `yaml:"accessibility,omitempty" json:"accessibility,omitempty"`

	DisableOptions *ModuleDisableOptions `yaml:"disable,omitempty" json:"disable,omitempty"`

	// Update lists version transitions that may skip intermediate releases.
	Update *Update `yaml:"update,omitempty" json:"update,omitempty"`
}

// ModuleRequirements are module.yaml's requirements: bare version ranges plus a
// flat map of required modules. package.yaml states the same thing differently
// — see Requirements.
type ModuleRequirements struct {
	// Deckhouse is the required platform version range.
	Deckhouse string `yaml:"deckhouse,omitempty" json:"deckhouse,omitempty"`
	// Kubernetes is the required Kubernetes version range.
	Kubernetes string `yaml:"kubernetes,omitempty" json:"kubernetes,omitempty"`
	// Bootstrapped is the required cluster installation status, for built-in
	// modules only.
	Bootstrapped string `yaml:"bootstrapped,omitempty" json:"bootstrapped,omitempty"`
	// ParentModules maps the name of each required module to its version range.
	ParentModules map[string]string `yaml:"modules,omitempty" json:"modules,omitempty"`
}

// ModuleDisableOptions is module.yaml's disable section.
type ModuleDisableOptions struct {
	Confirmation bool `yaml:"confirmation" json:"confirmation,omitempty"`
	// Message is superseded by Messages and kept for older manifests.
	//
	// Deprecated: use Messages.
	Message  string          `yaml:"message,omitempty" json:"message,omitempty"`
	Messages DisableMessages `yaml:"messages,omitempty" json:"messages,omitempty"`
}

// Accessibility declares, per edition, whether the module is available and
// which bundles enable it by default. The "_default" key applies to editions
// with no entry of their own.
type Accessibility struct {
	Editions map[string]EditionAccess `yaml:"editions" json:"editions"`
}

// EditionAccess is one edition's entry in Accessibility or Licensing.
type EditionAccess struct {
	Available        bool     `yaml:"available" json:"available"`
	EnabledInBundles []string `yaml:"enabledInBundles" json:"enabledInBundles"`
}

// Update lists allowed version transitions for the release the manifest
// describes, letting an upgrade skip intermediate versions.
type Update struct {
	Versions []UpdateVersion `yaml:"versions,omitempty" json:"versions,omitempty"`
}

// UpdateVersion is one allowed transition. Both bounds accept major.minor or
// major.minor.patch; To points at the version this manifest describes.
type UpdateVersion struct {
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to" json:"to"`
}

// Package maps package.yaml, the v2 manifest.
//
// One schema covers both package types, discriminated by Type. Weight and
// Critical apply to modules only and are absent from an application manifest.
type Package struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Type       string `yaml:"type" json:"type"`

	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
	Stage   string `yaml:"stage" json:"stage"`

	Descriptions   Descriptions   `yaml:"descriptions" json:"descriptions"`
	Requirements   Requirements   `yaml:"requirements" json:"requirements"`
	Licensing      Licensing      `yaml:"licensing" json:"licensing"`
	DisableOptions DisableOptions `yaml:"disable" json:"disable"`

	// Weight is the module priority. Modules only.
	Weight int `yaml:"weight,omitempty" json:"weight,omitempty"`
	// Critical marks a module the platform cannot run without. Modules only.
	Critical bool `yaml:"critical,omitempty" json:"critical,omitempty"`
}

// IsModule reports whether the manifest describes a module.
func (p *Package) IsModule() bool {
	return p.Type == TypeModule
}

// IsApplication reports whether the manifest describes an application.
func (p *Package) IsApplication() bool {
	return p.Type == TypeApplication
}

// Requirements are package.yaml's requirements. Each platform requirement is a
// constraint object rather than a bare string, and module dependencies are
// split into buckets by how they affect startup.
type Requirements struct {
	Kubernetes VersionConstraint   `yaml:"kubernetes" json:"kubernetes"`
	Deckhouse  VersionConstraint   `yaml:"deckhouse" json:"deckhouse"`
	Modules    ModulesRequirements `yaml:"modules" json:"modules"`
}

// VersionConstraint is a semver constraint expression. Empty means no
// constraint.
type VersionConstraint struct {
	Constraint string `yaml:"constraint,omitempty" json:"constraint,omitempty"`
}

// ModulesRequirements groups module dependencies by how they affect startup.
type ModulesRequirements struct {
	// Mandatory lists modules that must be present, and satisfy their
	// constraint if one is given, for the package to start.
	Mandatory []ModuleDependency `yaml:"mandatory,omitempty" json:"mandatory,omitempty"`
	// Conditional lists modules that need not be present, but must satisfy
	// their constraint if they are.
	Conditional []ModuleDependency `yaml:"conditional,omitempty" json:"conditional,omitempty"`
	// AnyOf lists groups of alternatives; at least one member of each group
	// must be installed for the package to start.
	AnyOf []ModuleGroup `yaml:"anyOf,omitempty" json:"anyOf,omitempty"`
	// NoneOf lists groups of forbidden modules; no member of any group may be
	// installed. A member constraint narrows the forbidden range — an empty one
	// forbids the module outright.
	NoneOf []ModuleGroup `yaml:"noneOf,omitempty" json:"noneOf,omitempty"`
}

// ModuleDependency is a named module dependency with an optional semver
// constraint.
type ModuleDependency struct {
	Name       string `yaml:"name" json:"name"`
	Constraint string `yaml:"constraint,omitempty" json:"constraint,omitempty"`
}

// ModuleGroup is a named group of module dependencies. Its meaning comes from
// the bucket holding it — see ModulesRequirements. Name is required and appears
// in scheduler diagnostics; Description is optional prose.
type ModuleGroup struct {
	Name        string             `yaml:"name" json:"name"`
	Description string             `yaml:"description,omitempty" json:"description,omitempty"`
	Modules     []ModuleDependency `yaml:"modules" json:"modules"`
}

// Licensing declares, per edition, whether the package is available and which
// bundles enable it by default. It is package.yaml's counterpart to
// module.yaml's Accessibility.
type Licensing struct {
	Editions map[string]EditionAccess `yaml:"editions" json:"editions"`
}

// DisableOptions is package.yaml's disable section.
type DisableOptions struct {
	// Confirmation requires the user to confirm before the package is disabled.
	Confirmation bool            `yaml:"confirmation" json:"confirmation"`
	Messages     DisableMessages `yaml:"messages,omitempty" json:"messages,omitempty"`
}

// DisableMessages is localized text shown when disabling.
type DisableMessages struct {
	Ru string `yaml:"ru,omitempty" json:"ru,omitempty"`
	En string `yaml:"en,omitempty" json:"en,omitempty"`
}

// Descriptions is localized description text.
type Descriptions struct {
	Ru string `yaml:"ru,omitempty" json:"ru,omitempty"`
	En string `yaml:"en,omitempty" json:"en,omitempty"`
}

// ParseModule decodes a module.yaml.
func ParseModule(raw []byte) (*Module, error) {
	module := new(Module)
	if err := yaml.Unmarshal(raw, module); err != nil {
		return nil, fmt.Errorf("decode %s: %w", ModuleFile, err)
	}

	return module, nil
}

// ParsePackage decodes a package.yaml.
func ParsePackage(raw []byte) (*Package, error) {
	pkg := new(Package)
	if err := yaml.Unmarshal(raw, pkg); err != nil {
		return nil, fmt.Errorf("decode %s: %w", PackageFile, err)
	}

	return pkg, nil
}
