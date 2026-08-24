// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package metadata projects parsed package definitions onto the version
// metadata the catalog CRs carry. It is the single mapping between the
// package.yaml world (internal/packages/dto) and the ModulePackageVersion
// status shapes, so every writer of that status - the version controller
// filling drafts from the registry and the startup sync filling embedded
// versions from disk - produces the same metadata for the same definition.
package metadata

import (
	"slices"
	"strings"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/dto"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// FromPackageDefinition projects a parsed v2 package.yaml onto the MPV status
// metadata. Mirrors the APV controller: only fields present on
// dto.ModuleDefinition are surfaced (stage, descriptions, disable options,
// licensing, requirements). Remaining module-only status fields (category,
// version-compatibility) are intentionally not populated here - extend
// dto.ModuleDefinition if you need to surface them.
func FromPackageDefinition(pd *dto.ModuleDefinition) *v1alpha1.ModulePackageVersionStatusMetadata {
	return &v1alpha1.ModulePackageVersionStatusMetadata{
		Stage: pd.Stage,
		Description: &v1alpha1.PackageDescription{
			Ru: pd.Descriptions.Ru,
			En: pd.Descriptions.En,
		},
		Weight:         int32(pd.Weight),
		Critical:       pd.Critical,
		ExclusiveGroup: pd.ExclusiveGroup,
		DisableOptions: disableOptionsToCR(pd.DisableOptions),
		Licensing:      licensingToCR(pd.Licensing),
		Requirements:   requirementsToCR(pd.Requirements),
	}
}

// disableOptionsToCR projects parsed disable protection onto the CR shape, returning nil
// when no disable protection is configured so the field omits cleanly.
func disableOptionsToCR(opts dto.DisableOptions) *v1alpha1.PackageDisableOptions {
	messages := disableMessagesToCR(opts)
	if !opts.Confirmation && messages == nil {
		return nil
	}

	return &v1alpha1.PackageDisableOptions{
		Confirmation: opts.Confirmation,
		Messages:     messages,
	}
}

// disableMessagesToCR projects the localized confirmation messages, returning nil when
// neither translation is set.
func disableMessagesToCR(opts dto.DisableOptions) *v1alpha1.PackageDisableMessages {
	if opts.Messages.Ru == "" && opts.Messages.En == "" {
		return nil
	}

	return &v1alpha1.PackageDisableMessages{
		Ru: opts.Messages.Ru,
		En: opts.Messages.En,
	}
}

// requirementsToCR projects parsed package requirements onto the v1alpha1
// PackageRequirements CR shape. Returns nil when no requirements are configured
// so the status field omits cleanly via omitempty.
func requirementsToCR(r dto.Requirements) *v1alpha1.PackageRequirements {
	kubernetes := versionConstraintToCR(r.Kubernetes.Constraint)
	deckhouse := versionConstraintToCR(r.Deckhouse.Constraint)
	modulesCR := moduleRequirementsToCR(r.Modules)

	if kubernetes == nil && deckhouse == nil && modulesCR == nil {
		return nil
	}

	return &v1alpha1.PackageRequirements{
		Kubernetes: kubernetes,
		Deckhouse:  deckhouse,
		Modules:    modulesCR,
	}
}

// licensingToCR projects dto.Licensing onto the v1alpha1 PackageLicensing CR shape.
func licensingToCR(l dto.Licensing) *v1alpha1.PackageLicensing {
	if len(l.Editions) == 0 {
		return nil
	}

	editions := make(map[string]v1alpha1.PackageEditionLicense, len(l.Editions))
	for name, e := range l.Editions {
		editions[name] = v1alpha1.PackageEditionLicense{
			Available:        e.Available,
			EnabledInBundles: slices.Clone(e.EnabledInBundles),
		}
	}

	return &v1alpha1.PackageLicensing{Editions: editions}
}

// legacyOptionalSuffix marks a legacy module.yaml parentModules dependency as
// conditional (skippable if the parent module is absent). See
// go_lib/dependency/extenders/moduledependency for the original parser.
const legacyOptionalSuffix = "!optional"

// LegacyRequirementsToCR projects a legacy v1alpha1.ModuleRequirements (flat strings
// plus a name to constraint map) onto the new PackageRequirements CR shape. A constraint
// ending in "!optional" maps to a conditional dependency; the suffix is stripped from
// the surfaced constraint string.
func LegacyRequirementsToCR(req *v1alpha1.ModuleRequirements) *v1alpha1.PackageRequirements {
	kubernetes := versionConstraintToCR(req.Kubernetes)
	deckhouse := versionConstraintToCR(req.Deckhouse)

	var moduleReqs *v1alpha1.PackageModulesRequirements
	if len(req.ParentModules) > 0 {
		var (
			mandatory   []v1alpha1.PackageModuleDependency
			conditional []v1alpha1.PackageModuleDependency
		)

		for name, constraint := range req.ParentModules {
			raw, optional := strings.CutSuffix(constraint, legacyOptionalSuffix)
			dep := v1alpha1.PackageModuleDependency{
				Name:       name,
				Constraint: strings.TrimSpace(raw),
			}

			if optional {
				conditional = append(conditional, dep)
			} else {
				mandatory = append(mandatory, dep)
			}
		}

		if len(mandatory) > 0 || len(conditional) > 0 {
			moduleReqs = &v1alpha1.PackageModulesRequirements{
				Mandatory:   mandatory,
				Conditional: conditional,
			}
		}
	}

	if kubernetes == nil && deckhouse == nil && moduleReqs == nil {
		return nil
	}

	return &v1alpha1.PackageRequirements{
		Kubernetes: kubernetes,
		Deckhouse:  deckhouse,
		Modules:    moduleReqs,
	}
}

// versionConstraintToCR wraps a raw semver constraint string into the v1alpha1
// VersionConstraint CR shape, returning nil when the string is empty.
func versionConstraintToCR(raw string) *v1alpha1.VersionConstraint {
	if len(raw) == 0 {
		return nil
	}

	return &v1alpha1.VersionConstraint{Constraint: raw}
}

// moduleRequirementsToCR projects dto.ModulesRequirements onto the v1alpha1
// PackageModulesRequirements CR shape, returning nil when mandatory, conditional,
// anyOf, and noneOf are all empty.
func moduleRequirementsToCR(mr dto.ModulesRequirements) *v1alpha1.PackageModulesRequirements {
	if len(mr.Mandatory) == 0 && len(mr.Conditional) == 0 && len(mr.AnyOf) == 0 && len(mr.NoneOf) == 0 {
		return nil
	}

	return &v1alpha1.PackageModulesRequirements{
		Mandatory:   moduleDependenciesToCR(mr.Mandatory),
		Conditional: moduleDependenciesToCR(mr.Conditional),
		AnyOf:       moduleGroupsToCR(mr.AnyOf),
		NoneOf:      moduleGroupsToCR(mr.NoneOf),
	}
}

// moduleDependenciesToCR projects a slice of dto.ModuleDependency onto the
// v1alpha1 PackageModuleDependency CR slice. Returns nil for empty input so
// the parent CR omitempty fields render cleanly.
func moduleDependenciesToCR(deps []dto.ModuleDependency) []v1alpha1.PackageModuleDependency {
	if len(deps) == 0 {
		return nil
	}

	out := make([]v1alpha1.PackageModuleDependency, 0, len(deps))
	for _, dep := range deps {
		out = append(out, v1alpha1.PackageModuleDependency{
			Name:       dep.Name,
			Constraint: dep.Constraint,
		})
	}

	return out
}

// moduleGroupsToCR projects a slice of dto.ModuleGroup onto the v1alpha1
// PackageModuleGroup CR slice. Used for both anyOf and noneOf - the shape is
// identical at the CR layer; the bucket semantics live on the field they're
// attached to. Returns nil for empty input so the parent CR omitempty field
// renders cleanly. The legacy module.yaml path does not carry anyOf or noneOf
// groups and never reaches this function - only the v2 package.yaml path
// (FromPackageDefinition) emits group metadata.
func moduleGroupsToCR(groups []dto.ModuleGroup) []v1alpha1.PackageModuleGroup {
	if len(groups) == 0 {
		return nil
	}

	out := make([]v1alpha1.PackageModuleGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, v1alpha1.PackageModuleGroup{
			Name:        g.Name,
			Description: g.Description,
			Modules:     moduleDependenciesToCR(g.Modules),
		})
	}

	return out
}
