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

// Origin is where a module's package comes from.
type Origin struct {
	// RepositoryName is the PackageRepository resource name, or the
	// "embedded" sentinel for modules shipped in the image.
	RepositoryName string
	// PackageVersion is what spec.packageVersion gets: a release version,
	// the Deckhouse version, or a pull override image tag.
	PackageVersion string
	Dev            bool
	Embedded       bool
}

// Known reports whether any source claims the module; the zero Origin means none does.
func (o Origin) Known() bool {
	return o != Origin{}
}

// mergeOrigins folds the per-source origins into one per module. The argument
// order sets the precedence: the first source claiming a module keeps it, so
// a lower-precedence source never overwrites a higher one.
func mergeOrigins(sources ...map[string]Origin) map[string]Origin {
	total := 0
	for _, source := range sources {
		total += len(source)
	}

	origins := make(map[string]Origin, total)

	for _, source := range sources {
		for name, origin := range source {
			if _, ok := origins[name]; !ok {
				origins[name] = origin
			}
		}
	}

	return origins
}
