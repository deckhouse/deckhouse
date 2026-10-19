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

package dependency

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
)

// Reasons; they must match the Kubernetes condition reason pattern.
const (
	reasonDependencyNotEnabled      = "DependencyNotEnabled"
	reasonDependencyVersionMismatch = "DependencyVersionMismatch"
)

// Rule is a gate that forbids a package whose dependencies are absent or of an incompatible
// version.
type Rule struct {
	getter       Getter
	dependencies map[string]Dependency
	// names are the dependencies sorted, so the first unmet one, and the reason, is stable.
	names []string
}

// Dependency is a requirement on another module's version; Optional skips an absent one.
type Dependency struct {
	Constraint *semver.Constraints
	Optional   bool
}

// Getter returns the version of an enabled module, or nil.
type Getter func(module string) *semver.Version

// NewRule constructs a dependency rule.
func NewRule(getter Getter, dependencies map[string]Dependency) *Rule {
	return &Rule{
		getter:       getter,
		dependencies: dependencies,
		names:        slices.Sorted(maps.Keys(dependencies)),
	}
}

// Decide forbids on the first dependency, by name, that is missing (and not optional) or violates
// its constraint.
func (r *Rule) Decide() rule.Decision {
	for _, name := range r.names {
		dep := r.dependencies[name]
		version := removePrereleaseAndMetadata(r.getter(name))
		if version == nil {
			if dep.Optional {
				continue
			}

			return rule.Decision{
				Kind:    rule.Forbid,
				Reason:  reasonDependencyNotEnabled,
				Message: fmt.Sprintf("dependency '%s' not enabled", name),
			}
		}

		if dep.Constraint != nil && !dep.Constraint.Check(version) {
			return rule.Decision{
				Kind:    rule.Forbid,
				Reason:  reasonDependencyVersionMismatch,
				Message: fmt.Sprintf("dependency '%s' unmet requirements", name),
			}
		}
	}

	return rule.Decision{Kind: rule.Undefined}
}

// Satisfies reports whether version meets constraint as the rule compares them: prerelease and
// metadata are dropped first. A nil constraint accepts any version.
func Satisfies(constraint *semver.Constraints, version *semver.Version) bool {
	if constraint == nil {
		return true
	}

	return constraint.Check(removePrereleaseAndMetadata(version))
}

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
