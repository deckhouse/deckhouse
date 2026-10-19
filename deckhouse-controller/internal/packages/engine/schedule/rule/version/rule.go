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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
)

// Getter returns the current version (of Kubernetes, of Deckhouse).
type Getter func() (*semver.Version, error)

// Rule is a version gate: Undefined or Forbid.
type Rule struct {
	versionGetter Getter
	constraints   *semver.Constraints
	subject       string // subject names the version in messages ("Kubernetes").
	reason        string
}

// NewRule creates a version rule. subject repeats reason in a form fit for the message, which is
// all the user gets on the admission path.
func NewRule(getter Getter, constraints *semver.Constraints, subject, reason string) *Rule {
	return &Rule{
		versionGetter: getter,
		constraints:   constraints,
		subject:       subject,
		reason:        reason,
	}
}

// Decide forbids when the version cannot be read or does not satisfy the constraints.
func (r *Rule) Decide() rule.Decision {
	version, err := r.versionGetter()
	if err != nil {
		return rule.Decision{
			Kind:    rule.Forbid,
			Reason:  "VersionLookupFailed",
			Message: fmt.Sprintf("cannot determine the current %s version: %s", r.subject, err.Error()),
		}
	}

	// semver's errors are discarded: "1.34.9 is less than v1.35" names neither the subject nor the
	// constraint.
	if ok, _ := r.constraints.Validate(version); !ok {
		return rule.Decision{
			Kind:    rule.Forbid,
			Reason:  r.reason,
			Message: fmt.Sprintf("requires %s version '%s', current version is '%s'", r.subject, r.constraints, version),
		}
	}

	return rule.Decision{Kind: rule.Undefined}
}
