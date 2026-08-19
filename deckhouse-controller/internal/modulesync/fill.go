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
	// a module of unknown origin keeps the spec another writer gave it - only disposable decides its fate
	if origin.Known() {
		moduleV2.Spec.PackageRepositoryName = origin.RepositoryName
		moduleV2.Spec.PackageVersion = origin.PackageVersion
	}

	// the annotation, not the spec, routes a module to the filesystem, so it is reconciled both ways
	if origin.Embedded {
		markAnnotation(moduleV2, v1alpha2.ModuleAnnotationEmbedded)
	} else {
		delete(moduleV2.Annotations, v1alpha2.ModuleAnnotationEmbedded)
	}

	// the dev annotation is only ever set, as it always has been
	if origin.Dev {
		markAnnotation(moduleV2, v1alpha2.ModuleAnnotationDev)
	}

	// the config block dies with the ModuleConfig deprecation, together with liveModuleConfigs
	if conf == nil {
		return
	}

	moduleV2.Spec.Settings = conf.Spec.Settings
	moduleV2.Spec.SettingsVersion = conf.Spec.Version
	moduleV2.Spec.Maintenance = conf.Spec.Maintenance
	moduleV2.Spec.Enabled = conf.Spec.Enabled
}

// markAnnotation sets the marker key to "true", allocating the map when the
// module carries no annotations.
func markAnnotation(moduleV2 *v1alpha2.Module, key string) {
	if moduleV2.Annotations == nil {
		moduleV2.Annotations = make(map[string]string)
	}

	moduleV2.Annotations[key] = "true"
}
