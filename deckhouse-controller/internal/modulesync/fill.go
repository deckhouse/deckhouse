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

package modulesync

// This file is the field mapping - what a v1alpha2 Module must contain for a
// given origin and config. Pure functions, no cluster access.

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

// fillModuleV2 writes the origin, its annotations and the config's settings onto the module.
func fillModuleV2(moduleV2 *v1alpha2.Module, origin Origin, conf *v1alpha1.ModuleConfig) {
	// a module of unknown origin keeps the spec another writer gave it
	if origin.Known() {
		moduleV2.Spec.PackageRepositoryName = origin.RepositoryName
		moduleV2.Spec.PackageVersion = origin.PackageVersion
	}

	// the embedded annotation, not the spec, routes a module to the filesystem,
	// so a known origin reconciles it both ways; an unknown origin (a pure
	// config mirror) owns no annotations and leaves them be
	if origin.Known() {
		if origin.Embedded {
			markAnnotation(moduleV2, v1alpha2.ModuleAnnotationEmbedded)
		} else {
			delete(moduleV2.Annotations, v1alpha2.ModuleAnnotationEmbedded)
		}

		// the dev annotation is only ever set, as it always has been
		if origin.Dev {
			markAnnotation(moduleV2, v1alpha2.ModuleAnnotationDev)
		}
	}

	// the config block dies with the ModuleConfig deprecation, together with liveModuleConfigs
	if conf == nil {
		return
	}

	// spec.packageVersion is required by the Module schema, so config fields
	// cannot materialize the spec on their own: the API server would reject
	// such a write. They are filled once a version is there.
	if !origin.Known() && moduleV2.Spec.PackageVersion == "" {
		return
	}

	moduleV2.Spec.Settings = conf.Spec.Settings
	moduleV2.Spec.SettingsVersion = conf.Spec.Version
	moduleV2.Spec.Maintenance = conf.Spec.Maintenance
	moduleV2.Spec.Enabled = conf.Spec.Enabled
	moduleV2.Spec.UpdatePolicy = conf.Spec.UpdatePolicy

	// the config names the repository the module must come from, and it wins
	// over the origin: the origin only reports where the package came from
	// last. An embedded module keeps "embedded" - it ships in the image, and
	// no repository serves it.
	if conf.Spec.Source != "" && !moduleV2.IsEmbedded() {
		moduleV2.Spec.PackageRepositoryName = conf.Spec.Source
	}
}

// markAnnotation sets the marker key to "true", allocating the map when the
// module carries no annotations.
func markAnnotation(moduleV2 *v1alpha2.Module, key string) {
	if moduleV2.Annotations == nil {
		moduleV2.Annotations = make(map[string]string)
	}

	moduleV2.Annotations[key] = "true"
}
