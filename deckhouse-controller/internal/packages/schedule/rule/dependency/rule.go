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

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
)

// Reason constants for rule decisions.
// Must match Kubernetes condition reason pattern: ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$
const (
	reasonDependencyNotEnabled      = "DependencyNotEnabled"
	reasonDependencyVersionMismatch = "DependencyVersionMismatch"
)

// Rule is a gate that vetoes a package whose declared dependencies are absent
// or installed at an incompatible version. It only ever returns Undefined or
// Forbid — it never turns a package on.
type Rule struct {
	getter       Getter
	dependencies map[string]Dependency
}

// Dependency describes a requirement on another module's installed version.
type Dependency struct {
	Constraint *semver.Constraints
	// if node not present, just skip check
	Optional bool
}

// Getter returns the installed version of a module, or nil when it is absent.
type Getter func(module string) *semver.Version

// NewRule constructs a dependency rule resolving versions through the getter.
func NewRule(getter Getter, dependencies map[string]Dependency) *Rule {
	return &Rule{
		getter:       getter,
		dependencies: dependencies,
	}
}

// Decide vetoes (Forbid) on the first dependency that is missing (and not
// optional) or violates its constraint; otherwise it returns Undefined.
func (r *Rule) Decide() rule.Decision {
	for name, dep := range r.dependencies {
		version := removePrereleaseAndMetadata(r.getter(name))
		if version == nil {
			if dep.Optional {
				continue // Optional dependency - skip validation
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
