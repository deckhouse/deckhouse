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

package edition

import "slices"

// defaultEdition is the fallback key in Licensing.Editions, consulted when the
// active edition has no explicit entry.
const defaultEdition = "_default"

// Licensing is a package's per-edition availability and bundle membership.
// The edition gate and the bundle floor resolve their decisions against it via
// Edition's IsAvailable/IsEnabled helpers, so the edition-vs-defaultEdition
// resolution lives here as the single source of truth.
type Licensing struct {
	// Editions maps an edition name to its license. The special key
	// defaultEdition supplies the fallback used when an edition has no entry.
	Editions map[string]EditionLicense
}

// EditionLicense is a single edition's license: whether the package ships in
// that edition and which bundles enable it by default.
type EditionLicense struct {
	Available        bool
	EnabledInBundles []string
}

// IsAvailable reports whether the package is available in this edition. The
// edition's own entry wins, then the defaultEdition entry; with neither present
// the package is treated as available, so it is banned only when its licensing
// explicitly marks it unavailable.
func (e *Edition) IsAvailable(license Licensing) bool {
	if entry, ok := license.Editions[e.Name]; ok {
		return entry.Available
	}

	if entry, ok := license.Editions[defaultEdition]; ok {
		return entry.Available
	}

	return true
}

// IsEnabled reports whether the active bundle (e.Bundle) enables the package in
// this edition. The bundle counts when EITHER the edition-specific licensing OR
// the defaultEdition entry lists it (union), so either source can opt the active
// bundle in.
func (e *Edition) IsEnabled(license Licensing) bool {
	return slices.Contains(license.Editions[e.Name].EnabledInBundles, e.Bundle) ||
		slices.Contains(license.Editions[defaultEdition].EnabledInBundles, e.Bundle)
}
