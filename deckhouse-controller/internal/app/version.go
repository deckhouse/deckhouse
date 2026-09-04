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

package app

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// devPackageVersion is what a "dev" binary's packages carry, matching the package runtime.
const devPackageVersion = "v2.0.0"

// EmbeddedPackageVersion reduces a Deckhouse version to the one every package the image ships
// carries: major.minor.patch, so a single version name spans every build of a release. A "dev"
// binary counts as v2.0.0; a version that is not semver is passed through unchanged, which keeps
// every caller naming the same version even when the result is no legal object name.
func EmbeddedPackageVersion(deckhouseVersion string) string {
	if deckhouseVersion == "dev" {
		return devPackageVersion
	}

	parsed, err := semver.NewVersion(deckhouseVersion)
	if err != nil {
		return deckhouseVersion
	}

	return fmt.Sprintf("v%d.%d.%d", parsed.Major(), parsed.Minor(), parsed.Patch())
}
