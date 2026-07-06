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

package version

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
)

// Getter retrieves the current version from the system.
// Examples:
//   - Kubernetes version from API server
//   - Deckhouse version from deployment
//   - Module version from registry
type Getter func() (*semver.Version, error)

// Rule validates version constraints using semantic versioning. It is a pure
// gate: it either has no opinion (Undefined) or hard-vetoes (Forbid) — it never
// turns a package on, so a satisfied version check cannot override an intent rule.
type Rule struct {
	versionGetter Getter              // Function to get current version
	constraints   *semver.Constraints // Required version constraint (e.g., ">=1.21, <2.0")
	reason        string
}

// NewRule creates a new version rule with the given getter and constraints.
//
// Example constraints:
//   - ">=1.21"           - Minimum version 1.21
//   - ">=1.21, <2.0"     - Range from 1.21 to 2.0
//   - "~1.21"            - Patch releases of 1.21
//   - "^1.21"            - Minor releases of 1.x
func NewRule(getter Getter, constraints *semver.Constraints, reason string) *Rule {
	return &Rule{
		versionGetter: getter,
		constraints:   constraints,
		reason:        reason,
	}
}

// Decide retrieves the current version and validates it against constraints.
// Returns Forbid if:
//   - Version getter fails (network error, API error, etc.)
//   - Version doesn't satisfy constraints
//
// Otherwise it returns Undefined (no opinion): the gate is satisfied.
func (r *Rule) Decide() rule.Decision {
	version, err := r.versionGetter()
	if err != nil {
		return rule.Decision{
			Kind:    rule.Forbid,
			Reason:  "VersionLookupFailed",
			Message: fmt.Sprintf("get version: %s", err.Error()),
		}
	}

	// Validate returns (bool, []error) - we only use the errors
	if _, errs := r.constraints.Validate(version); len(errs) != 0 {
		return rule.Decision{
			Kind:    rule.Forbid,
			Reason:  r.reason,
			Message: fmt.Errorf("check version error: %w", errs[0]).Error(),
		}
	}

	return rule.Decision{Kind: rule.Undefined}
}
