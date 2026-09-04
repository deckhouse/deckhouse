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

package pkgsync

import (
	"slices"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// The old module stack names a ModuleSource, the package system names a PackageRepository.
// This file is the only place that maps the two onto each other.

// Names of the repositories the module packages come from during the migration
// off the module sources.
const (
	// moduleSourceNameDeckhouse is the built-in module source shipped with the platform.
	moduleSourceNameDeckhouse = "deckhouse"

	// moduleSourceNameFlant is the module source present on the clusters
	// managed by Flant.
	moduleSourceNameFlant = "flant"

	// repositoryNameDeckhouseModules serves the modules of the "deckhouse"
	// ModuleSource. The plain "deckhouse" name belongs to the application-packages
	// repository, while the module source points at <registry>/modules.
	repositoryNameDeckhouseModules = "deckhouse-modules"

	// repositoryNameEmbedded stands for the Deckhouse image itself and
	// resolves to no PackageRepository object.
	repositoryNameEmbedded = "embedded"
)

// RepositoryNameForSource maps a ModuleSource name to the name of the
// PackageRepository serving the same registry path.
func RepositoryNameForSource(moduleSourceName string) string {
	if moduleSourceName == moduleSourceNameDeckhouse {
		return repositoryNameDeckhouseModules
	}

	return moduleSourceName
}

// SourceNameForRepository maps a PackageRepository name back to the ModuleSource
// serving the same registry path. The embedded repository stands for the image
// and names no source.
func SourceNameForRepository(repositoryName string) string {
	switch repositoryName {
	case repositoryNameDeckhouseModules:
		return moduleSourceNameDeckhouse
	case repositoryNameEmbedded:
		return ""
	}

	return repositoryName
}

// ConfiguredModuleSource returns the source the operator selected in the module config
// (.spec.source), or an empty string without a config or a selection. "Embedded" is
// the sentinel for the built-in copy, not a real ModuleSource, so it counts as no
// selection.
func ConfiguredModuleSource(config *v1alpha1.ModuleConfig) string {
	if config == nil || config.Spec.Source == v1alpha1.ModuleSourceEmbedded {
		return ""
	}

	return config.Spec.Source
}

// ConfiguredRepository names the repository the operator selected in the module config
// through .spec.source, or an empty string without a config or a selection. "Embedded"
// is the sentinel for the built-in copy, not a source, so it counts as no selection.
func ConfiguredRepository(config *v1alpha1.ModuleConfig) string {
	if config == nil || config.Spec.Source == v1alpha1.ModuleSourceEmbedded {
		return ""
	}

	return RepositoryNameForSource(config.Spec.Source)
}

// ModuleSourceNamesForRepositories names the module sources behind the repositories, sorted
// and without duplicates. The embedded repository names no module source and is left out.
func ModuleSourceNamesForRepositories(repositories []string) []string {
	moduleSourceNames := make([]string, 0, len(repositories))

	for _, repository := range repositories {
		moduleSourceName := SourceNameForRepository(repository)
		if moduleSourceName == "" || slices.Contains(moduleSourceNames, moduleSourceName) {
			continue
		}

		moduleSourceNames = append(moduleSourceNames, moduleSourceName)
	}

	slices.Sort(moduleSourceNames)

	return moduleSourceNames
}
