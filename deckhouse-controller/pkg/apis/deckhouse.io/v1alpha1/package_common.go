/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"maps"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/openapi"
)

// This file holds the package vocabulary shared by applications and modules. A type belongs
// here only if both flavours use it — ApplicationPackage and ModulePackage, or their version
// counterparts; anything specific to one kind stays in that kind's file. The metadata keys
// below are the exception: the whole packages.deckhouse.io label and annotation vocabulary is
// declared here whichever kind writes it, so that one key never grows two names.

// Labels of the packages.deckhouse.io vocabulary. The same key means the same thing wherever
// it is set — on a package version, on a PackageRepositoryOperation, or on a resource a
// package chart rendered — so each key has exactly one constant.
const (
	// PackageLabelPackage names the package: the package a version belongs to, or the
	// package whose chart rendered the resource.
	PackageLabelPackage = "packages.deckhouse.io/package"

	// PackageLabelRepository names the PackageRepository the object came from.
	PackageLabelRepository = "packages.deckhouse.io/repository"

	// PackageLabelInstance names the application instance that owns the resource;
	// modules render no instance, so only application charts carry it.
	PackageLabelInstance = "packages.deckhouse.io/instance"

	// PackageLabelDraft marks a package version whose metadata has not been loaded yet;
	// the version controller drops the label once loading succeeds.
	PackageLabelDraft = "packages.deckhouse.io/draft"

	// PackageLabelExistInRegistry records whether the version's bundle image is still
	// present in the registry.
	PackageLabelExistInRegistry = "packages.deckhouse.io/exist-in-registry"

	// PackageLabelLegacy marks a module package version restored from a legacy module
	// release rather than scanned out of a repository.
	PackageLabelLegacy = "packages.deckhouse.io/legacy"

	// PackageLabelOperationType mirrors a PackageRepositoryOperation's spec.type as a
	// selectable label; its values are the PackageRepositoryOperationType constants.
	PackageLabelOperationType = "packages.deckhouse.io/operation-type"

	// PackageLabelOperationTrigger records what started a PackageRepositoryOperation:
	// PackageOperationTriggerAuto for the repository's own scan interval,
	// PackageOperationTriggerManual for an operation a user created.
	PackageLabelOperationTrigger = "packages.deckhouse.io/operation-trigger"

	PackageOperationTriggerManual = "manual"
	PackageOperationTriggerAuto   = "auto"
)

// Annotations of the packages.deckhouse.io vocabulary.
const (
	// PackageAnnotationManagedBy marks a Helm release as owned by the package release
	// layer; PackageAnnotationManagedByValue is the only value it carries. Releases are
	// looked up by this pair on cleanup.
	PackageAnnotationManagedBy      = "packages.deckhouse.io/managed-by"
	PackageAnnotationManagedByValue = "deckhouse"

	// PackageAnnotationRegistrySpecChanged is stamped on every Application and Module of
	// a repository whose registry spec changed, to make their controllers re-read the
	// registry; each controller removes it once it has.
	PackageAnnotationRegistrySpecChanged = "packages.deckhouse.io/registry-spec-changed"

	// PackageAnnotationRegistrySpecChecksum holds the checksum of the registry spec a
	// PackageRepository was last reconciled with; a mismatch is what fans
	// PackageAnnotationRegistrySpecChanged out.
	PackageAnnotationRegistrySpecChecksum = "packages.deckhouse.io/registry-spec-checksum"

	// PackageAnnotationEndpointDescription marks an Ingress in the application chart as
	// an application endpoint and holds its description; the hosts and paths of that
	// Ingress are reflected in status.urls.
	PackageAnnotationEndpointDescription = "packages.deckhouse.io/application-endpoint-description"
)

// PackageReleaseChannels maps repository name -> release channel name -> version.
type PackageReleaseChannels map[string]map[string]string

// SetChannels replaces the channels repoName offers the package on, and reports whether that changed
// anything. Nil channels mean the scan could not read them and leave the row untouched; an empty set
// drops the row rather than storing an empty one.
func (c *PackageReleaseChannels) SetChannels(repoName string, channels map[string]string) bool {
	if channels == nil {
		return false
	}

	if len(channels) == 0 {
		return c.RemoveChannels(repoName)
	}

	if maps.Equal((*c)[repoName], channels) {
		return false
	}

	if *c == nil {
		*c = make(PackageReleaseChannels, 1)
	}
	(*c)[repoName] = maps.Clone(channels)

	return true
}

// RemoveChannels drops the channels repoName offered, and reports whether there were any.
func (c *PackageReleaseChannels) RemoveChannels(repoName string) bool {
	if _, ok := (*c)[repoName]; !ok {
		return false
	}

	delete(*c, repoName)

	if len(*c) == 0 {
		*c = nil
	}

	return true
}

// PackageSchema is an OpenAPI v3 schema wrapper that preserves all custom x-*
// extensions (e.g. x-deckhouse-grantable-resource) as typed fields on the
// embedded OpenAPIV3Schema. The serialised JSON shape is
// {"openAPIV3Schema": <schema-object>}, identical to
// apiextensionsv1.CustomResourceValidation but with all Deckhouse extensions
// retained.
type PackageSchema struct {
	// +optional
	OpenAPIV3Schema *openapi.OpenAPIV3Schema `json:"openAPIV3Schema,omitempty"`
}

// PackageVersionStatusSchemas holds the settings and values schemas of a package version,
// shared by applications and modules.
type PackageVersionStatusSchemas struct {
	// SettingsSchema is the OpenAPI v3 schema used to validate the user-supplied
	// settings of the package. Stored as an opaque object because its contents
	// form a recursive JSON schema that cannot be expressed structurally in a
	// CRD; the controller validates this subtree in Go when loading package
	// metadata.
	// +optional
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	SettingsSchema *PackageSchema `json:"settingsSchema,omitempty"`

	// ValuesSchema is the OpenAPI v3 schema used to validate the effective
	// values (defaults merged with settings) passed to the package's hooks and
	// charts. Stored as an opaque object because its contents form a recursive
	// JSON schema that cannot be expressed structurally in a CRD; the
	// controller validates this subtree in Go when loading package metadata.
	// +optional
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Type=object
	ValuesSchema *PackageSchema `json:"valuesSchema,omitempty"`
}

// PackageDisableOptions describes the package's disable protection surfaced to the UI.
type PackageDisableOptions struct {
	// Whether confirmation is required to disable the package.
	// +optional
	Confirmation bool `json:"confirmation,omitempty"`

	// Localized disable confirmation messages.
	// +optional
	Messages *PackageDisableMessages `json:"messages,omitempty"`
}

// PackageDisableMessages holds localized disable confirmation messages for the package.
type PackageDisableMessages struct {
	// Russian disable confirmation message.
	// +optional
	Ru string `json:"ru,omitempty"`

	// English disable confirmation message.
	// +optional
	En string `json:"en,omitempty"`
}

// PackageRequirements describes the platform and module dependencies of a package,
// surfaced as part of the package version status.
type PackageRequirements struct {
	// Required Deckhouse Platform version.
	// +optional
	Deckhouse *VersionConstraint `json:"deckhouse,omitempty"`

	// Required Kubernetes version.
	// +optional
	Kubernetes *VersionConstraint `json:"kubernetes,omitempty"`

	// Required modules, partitioned into mandatory, conditional, and anyOf dependencies.
	// +optional
	Modules *PackageModulesRequirements `json:"modules,omitempty"`
}

// VersionConstraint wraps a single semver constraint expression (e.g. ">= 1.26").
type VersionConstraint struct {
	// Semver constraint expression.
	// +optional
	Constraint string `json:"constraint,omitempty"`
}

// PackageModulesRequirements groups module dependencies by how they affect startup.
type PackageModulesRequirements struct {
	// Mandatory dependencies — must be present (and satisfy the constraint, if any)
	// for the package to start.
	// +optional
	Mandatory []PackageModuleDependency `json:"mandatory,omitempty"`

	// Conditional dependencies — not required to be present, but if installed must
	// satisfy the constraint for the package to function correctly. Replaces the
	// legacy "!optional" suffix from the v1 requirements format.
	// +optional
	Conditional []PackageModuleDependency `json:"conditional,omitempty"`

	// AnyOf groups of alternative dependencies — at least one member of each group
	// must be installed (and satisfy its constraint, if any) for the package to
	// start. Groups are checker-only and add no edges to the dependency graph.
	// +optional
	AnyOf []PackageModuleGroup `json:"anyOf,omitempty"`

	// NoneOf groups of forbidden dependencies — no member of any group may be
	// installed for the package to start. A member with no constraint is forbidden
	// at any version; a member with a constraint is forbidden only at versions
	// matching that constraint. Groups are checker-only and add no edges to the
	// dependency graph.
	// +optional
	NoneOf []PackageModuleGroup `json:"noneOf,omitempty"`
}

// PackageModuleDependency is a single named module dependency with an optional
// semver constraint. An empty constraint means "any version".
type PackageModuleDependency struct {
	// Module name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Semver constraint expression.
	// +optional
	Constraint string `json:"constraint,omitempty"`
}

// PackageModuleGroup is a group of alternative module dependencies. At least one
// member must be installed (and satisfy its constraint, if any) for the package
// to start. The Name is required and surfaces in scheduler diagnostics; the
// Description is optional human-facing documentation.
type PackageModuleGroup struct {
	// Stable identifier used by the scheduler in diagnostics.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Human-readable description of the group's purpose.
	// +optional
	Description string `json:"description,omitempty"`

	// Alternative module dependencies in this group.
	Modules []PackageModuleDependency `json:"modules"`
}

// PackageDescription holds the localized descriptions of a package.
type PackageDescription struct {
	// Russian description of the package.
	// +optional
	Ru string `json:"ru,omitempty"`

	// English description of the package.
	// +optional
	En string `json:"en,omitempty"`
}

// PackageLicensing holds per-edition licensing of a package.
type PackageLicensing struct {
	// Licensing information for different package editions.
	// +optional
	Editions map[string]PackageEditionLicense `json:"editions,omitempty"`
}

// PackageEditionLicense is a single edition's licensing, shared by applications
// and modules: whether the edition is available and which bundles enable the
// package by default. Bundle membership is meaningful for modules; applications
// have no bundles and typically leave EnabledInBundles empty.
type PackageEditionLicense struct {
	// Whether this edition is available for use.
	// +optional
	Available bool `json:"available,omitempty"`

	// Bundles that enable the module by default in this edition.
	// +optional
	EnabledInBundles []string `json:"enabledInBundles,omitempty"`
}

// PackageChangelog lists what changed in a package version.
type PackageChangelog struct {
	// List of new features in this version.
	// +optional
	Features []string `json:"features,omitempty"`

	// List of bug fixes in this version.
	// +optional
	Fixes []string `json:"fixes,omitempty"`
}

// PackageVersionCompatibilityRules bounds which versions may upgrade or downgrade to this one.
type PackageVersionCompatibilityRules struct {
	// Compatibility rules for upgrading to this version.
	// +optional
	Upgrade *PackageVersionCompatibilityRule `json:"upgrade,omitempty"`

	// Compatibility rules for downgrading from this version.
	// +optional
	Downgrade *PackageVersionCompatibilityRule `json:"downgrade,omitempty"`
}

// PackageVersionCompatibilityRule is one direction's version range and skip allowances.
type PackageVersionCompatibilityRule struct {
	// Starting version range for compatibility.
	// +optional
	From string `json:"from,omitempty"`

	// Ending version range for compatibility.
	// +optional
	To string `json:"to,omitempty"`

	// How many patch versions can be skipped.
	// +optional
	// +kubebuilder:validation:Minimum=0
	AllowSkipPatches int32 `json:"allowSkipPatches,omitempty"`

	// How many minor versions can be skipped.
	// +optional
	// +kubebuilder:validation:Minimum=0
	AllowSkipMinor int32 `json:"allowSkipMinor,omitempty"`

	// How many major versions can be skipped.
	// +optional
	// +kubebuilder:validation:Minimum=0
	AllowSkipMajor int32 `json:"allowSkipMajor,omitempty"`

	// Maximum number of versions that can be rolled back.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxRollback int32 `json:"maxRollback,omitempty"`
}
