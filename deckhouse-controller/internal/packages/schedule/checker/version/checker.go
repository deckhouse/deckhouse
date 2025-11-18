// Copyright 2025 Flant JSC
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

package version

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
)

// Getter retrieves the current version from the system.
// Examples:
//   - Kubernetes version from API server
//   - Deckhouse version from deployment
//   - Module version from registry
type Getter func() (*semver.Version, error)

// Checker validates version constraints using semantic versioning.
// Always acts as a blocker - packages cannot be enabled if version requirements aren't met.
type Checker struct {
	versionGetter Getter              // Function to get current version
	constraints   *semver.Constraints // Required version constraint (e.g., ">=1.21, <2.0")
	reason        string
}

// NewChecker creates a new version checker with the given getter and constraints.
//
// Example constraints:
//   - ">=1.21"           - Minimum version 1.21
//   - ">=1.21, <2.0"     - Range from 1.21 to 2.0
//   - "~1.21"            - Patch releases of 1.21
//   - "^1.21"            - Minor releases of 1.x
func NewChecker(getter Getter, constraints *semver.Constraints, reason string) *Checker {
	return &Checker{
		versionGetter: getter,
		constraints:   constraints,
		reason:        reason,
	}
}

// Check retrieves the current version and validates it against constraints.
// Returns disabled if:
//   - Version getter fails (network error, API error, etc.)
//   - Version doesn't satisfy constraints
func (c *Checker) Check() checker.Result {
	version, err := c.versionGetter()
	if err != nil {
		return checker.Result{
			Enabled: false,
			Reason:  fmt.Sprintf("get version: %s", err.Error()),
		}
	}

	// Validate returns (bool, []error) - we only use the errors
	if _, errs := c.constraints.Validate(version); len(errs) != 0 {
		return checker.Result{
			Enabled: false,
			Reason:  fmt.Sprintf("%s: %s", c.reason, errs[0].Error()), // Return first validation error with reason
		}
	}

	return checker.Result{
		Enabled: true,
	}
}
