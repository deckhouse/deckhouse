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
	"log/slog"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type ModuleDependencyGetter func(moduleName string) (*ModuleInfo, error)

type ModuleInfo struct {
	IsModuleEnabled *bool
	Version         *semver.Version
}

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
	getter              ModuleDependencyGetter // Retrieves dependency information
	modulesDependencies map[string]Dependency  // Map of dependency name -> requirements
	logger              *log.Logger
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
func NewChecker(getter ModuleDependencyGetter, modulesDependencies map[string]Dependency, logger *log.Logger) *Checker {
	return &Checker{
		getter:              getter,
		modulesDependencies: modulesDependencies,
		logger:              logger.Named("dependency-checker").With(slog.Int("dependencies count", len(modulesDependencies))),
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
	for name, dep := range c.modulesDependencies {
		c.logger.Debug("check dependency",
			slog.String("name", name),
			slog.Bool("optional", dep.Optional))

		moduleInfo, err := c.getter(name)
		if err != nil {
			return checker.Result{
				Enabled: false,
				Reason:  err.Error(),
				Message: err.Error(),
			}
		}

		// Dependency not enabled
		if moduleInfo.IsModuleEnabled == nil || !*moduleInfo.IsModuleEnabled {
			if dep.Optional {
				continue // Optional dependency - skip validation
			}

			suffix := "not enabled"
			if moduleInfo.IsModuleEnabled == nil {
				suffix = "not found"
			}
			msg := fmt.Sprintf("dependency '%s' %s", name, suffix)

			// Required dependency is missing - fail
			return checker.Result{
				Enabled: false,
				Message: msg,
				Reason:  msg,
			}
		}

		version := removePrereleaseAndMetadata(moduleInfo.Version, c.logger)

		c.logger.Debug("semver validate",
			slog.String("constraint", dep.Constraint.String()),
			slog.String("version", version.String()))

		// Validate version constraint
		// semver.Constraints.Validate returns (bool, []error)
		// We only care about the errors - if any exist, constraint is not satisfied
		if _, errs := dep.Constraint.Validate(version); len(errs) != 0 {
			return checker.Result{
				Enabled: false,
				Reason:  fmt.Sprintf("dependency %s error: %s", name, errs[0].Error()), // Return first validation error
				Message: errs[0].Error(),
			}
		}
	}

	// All dependencies satisfied
	return checker.Result{
		Enabled: true,
	}
}

// removePrereleaseAndMetadata returns a version without prerelease and metadata parts
func removePrereleaseAndMetadata(version *semver.Version, logger *log.Logger) *semver.Version {
	if len(version.Prerelease()) > 0 {
		woPrerelease, err := version.SetPrerelease("")
		if err != nil {
			logger.Warn("could not remove prerelease")
			return version
		}
		version = &woPrerelease
	}

	if len(version.Metadata()) > 0 {
		woMetadata, err := version.SetMetadata("")
		if err != nil {
			logger.Warn("could not remove metadata")
			return version
		}
		version = &woMetadata
	}

	return version
}
