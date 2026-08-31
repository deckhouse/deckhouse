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

package utils

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
)

// EditionLicensing projects the licensing block of a module package version
// onto the edition evaluator's shape. A missing version, a draft without
// metadata or an absent licensing block yield the zero Licensing, which the
// evaluator answers false for - the same answer an unknown module gets.
func EditionLicensing(mpv *v1alpha1.ModulePackageVersion) d8edition.Licensing {
	if mpv == nil || mpv.Status.PackageMetadata == nil || mpv.Status.PackageMetadata.Licensing == nil {
		return d8edition.Licensing{}
	}

	source := mpv.Status.PackageMetadata.Licensing.Editions
	editions := make(map[string]d8edition.EditionLicense, len(source))

	for name, license := range source {
		editions[name] = d8edition.EditionLicense{
			Available:        license.Available,
			EnabledInBundles: license.EnabledInBundles,
		}
	}

	return d8edition.Licensing{Editions: editions}
}
