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

package anyof

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/dependency"
)

// Reason constants for checker results.
// Must match Kubernetes condition reason pattern: ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$
const (
	reasonAnyOfUnmet = "AnyOfDependenciesUnmet"
)

// Checker validates "satisfy at least one" (anyOf) module groups against the
// current cluster state. Each group passes when ≥1 of its members is installed
// at a constraint-satisfying version; the Checker fails on the first group
// that has no satisfied member.
//
// Graph subscriptions (followee/follower edge wiring) are intentionally NOT
// performed here — the scheduler manages graph topology elsewhere. This
// package is purely an enable/disable predicate.
type Checker struct {
	getter dependency.Getter
	groups []Group
}

// Group is a "satisfy at least one" set of module dependencies. The group
// passes as soon as one member is installed at a constraint-satisfying
// version. Keys are module names; nil constraints mean "any installed version
// of that module satisfies".
type Group struct {
	Name    string
	Modules map[string]*semver.Constraints
}

// NewChecker builds a Checker over the given anyOf groups, all sharing a
// single Getter so the predicate observes a consistent view of installed
// module versions.
func NewChecker(getter dependency.Getter, groups []Group) *Checker {
	return &Checker{
		getter: getter,
		groups: groups,
	}
}

// Check returns the first group that has no satisfied member. An empty groups
// slice is a no-op that returns Enabled: true.
func (c *Checker) Check() checker.Result {
	for _, group := range c.groups {
		var satisfied bool
		for name, constraint := range group.Modules {
			version := removePrereleaseAndMetadata(c.getter(name))
			if version == nil {
				continue
			}

			if constraint == nil || constraint.Check(version) {
				satisfied = true
				break
			}
		}

		if !satisfied {
			return checker.Result{
				Reason:  reasonAnyOfUnmet,
				Message: fmt.Sprintf("anyOf group '%s' unmet", group.Name),
			}
		}
	}

	return checker.Result{Enabled: true}
}

// removePrereleaseAndMetadata returns a version without prerelease and metadata parts
func removePrereleaseAndMetadata(version *semver.Version) *semver.Version {
	if version == nil {
		return nil
	}

	if len(version.Prerelease()) > 0 {
		clearVersion, err := version.SetPrerelease("")
		if err != nil {
			return version
		}
		version = &clearVersion
	}

	if len(version.Metadata()) > 0 {
		clearVersion, err := version.SetMetadata("")
		if err != nil {
			return version
		}
		version = &clearVersion
	}

	return version
}
