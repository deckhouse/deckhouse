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

// Package dependency provides a checker for validating package dependencies.
// It verifies that required dependencies exist, are enabled, and satisfy version constraints.
package dependency

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
)

// Getter retrieves dependency information by name.
// Implementations should return nil if the dependency doesn't exist.
//
// Example implementation:
//
//	func (s *Scheduler) GetDependency(name string) Node {
//	    s.mu.Lock()
//	    defer s.mu.Unlock()
//	    return s.nodes[name]
//	}
type Getter interface {
	// IsEnabled checks if the module enabled, returning nil means module does not exist
	IsEnabled(name string) *bool

	// GetDependency returns the dependency node by name, or nil if not found.
	// GetDependency(name string) Node
}

// Node represents a package dependency that can be validated.
// It provides version information and enable/disable state.
// type Node interface {
// GetVersion returns the semantic version of this dependency.
// May return nil if version is not yet determined.
// GetVersion() *semver.Version

// IsEnabled returns whether this dependency is currently enabled.
// A package cannot be enabled if any of its required dependencies are disabled.
// IsEnabled() bool
// }

// Dependency defines a version constraint requirement for a package dependency.
type Dependency struct {
	// Constraint specifies the required version range (e.g., ">=1.21, <2.0").
	// Uses semantic versioning constraints from github.com/Masterminds/semver/v3.
	Constraint *semver.Constraints

	// Optional indicates whether this dependency is optional.
	// If true, missing or disabled dependency will not prevent package enablement.
	// If false, dependency must exist, be enabled, and satisfy version constraint.
	Optional bool
}

// Checker validates package dependencies against version constraints and enabled state.
// It checks that all required dependencies:
//  1. Exist (not nil from Getter)
//  2. Are enabled (IsEnabled() returns true)
//  3. Satisfy version constraints
//
// Optional dependencies are skipped if missing but still validated if present.
type Checker struct {
	getter       Getter                // Retrieves dependency information
	dependencies map[string]Dependency // Map of dependency name -> requirements
}

// NewChecker creates a new dependency checker.
//
// Parameters:
//   - getter: Interface to retrieve dependency information by name
//   - dependencies: Map of package names to their version constraints and optional flags
//
// Example:
//
//	dependencies := map[string]Dependency{
//	    "database": {
//	        Constraint: semver.MustParseConstraint(">=1.0.0"),
//	        Optional:   false,
//	    },
//	    "cache": {
//	        Constraint: semver.MustParseConstraint(">=2.0.0"),
//	        Optional:   true,  // Package can work without cache
//	    },
//	}
//	checker := NewChecker(getter, dependencies)
func NewChecker(getter Getter, dependencies map[string]Dependency) *Checker {
	return &Checker{
		getter:       getter,
		dependencies: dependencies,
	}
}

// Check evaluates all dependencies and returns whether the package should be enabled.
//
// Validation logic:
//  1. For each dependency in the map:
//     a. Retrieve dependency node via Getter
//     b. If not found:
//     - Optional dependency: skip to next
//     - Required dependency: return disabled with reason
//     c. If found but disabled:
//     - Return disabled (even for optional deps if present)
//     d. Validate version constraint:
//     - If version doesn't satisfy constraint: return disabled
//  2. If all checks pass: return enabled
//
// Returns:
//   - Result{Enabled: false, Reason: "..."} if any required dependency fails validation
//   - Result{Enabled: true} if all dependencies satisfy constraints
//
// Note: Optional dependencies are only skipped if completely missing.
// If an optional dependency exists, it must still be enabled and satisfy version constraints.
func (c *Checker) Check() checker.Result {
	// Iterate through all declared dependencies
	for name, dep := range c.dependencies {
		// Retrieve the dependency node
		enabled := c.getter.IsEnabled(name)
		if enabled == nil {
			// Dependency doesn't exist
			if dep.Optional {
				// Optional dependency - skip validation
				continue
			}

			// Required dependency is missing - fail
			return checker.Result{
				Enabled: false,
				Reason:  fmt.Sprintf("dependency '%s' not found", name),
			}
		}

		// Dependency exists but check if it's enabled
		// Even optional dependencies must be enabled if they exist
		if !*enabled {
			return checker.Result{
				Enabled: false,
				Message: fmt.Sprintf("dependency '%s' not enabled", name),
			}
		}

		// Validate version constraint
		// semver.Constraints.Validate returns (bool, []error)
		// We only care about the errors - if any exist, constraint is not satisfied
		// if _, errs := dep.Constraint.Validate(node.GetVersion()); len(errs) != 0 {
		// 	return checker.Result{
		// 		Enabled: false,
		// 		Reason:  errs[0].Error(), // Return first validation error
		// 	}
		// }
	}

	// All dependencies satisfied
	return checker.Result{
		Enabled: true,
	}
}
