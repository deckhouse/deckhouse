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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
)

// Reason constants for checker results.
// Must match Kubernetes condition reason pattern: ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$
const (
	ReasonDependencyNotEnabled      = "DependencyNotEnabled"
	ReasonDependencyVersionMismatch = "DependencyVersionMismatch"
)

type Checker struct {
	getter       Getter
	dependencies map[string]Dependency
}

type Dependency struct {
	Constraint *semver.Constraints
	// if node not present, just skip check
	Optional bool
}

type Getter func(version string) *semver.Version

func NewChecker(getter Getter, dependencies map[string]Dependency) *Checker {
	return &Checker{
		getter:       getter,
		dependencies: dependencies,
	}
}

func (c *Checker) Check() checker.Result {
	for name, dep := range c.dependencies {
		version := c.getter(name)
		if version == nil {
			if dep.Optional {
				continue // Optional dependency - skip validation
			}

			return checker.Result{
				Reason:  ReasonDependencyNotEnabled,
				Message: fmt.Sprintf("dependency '%s' not enabled", name),
			}
		}

		if dep.Constraint != nil && !dep.Constraint.Check(version) {
			return checker.Result{
				Reason:  ReasonDependencyVersionMismatch,
				Message: fmt.Sprintf("dependency '%s' unmet requirements", name),
			}
		}
	}

	return checker.Result{Enabled: true}
}
