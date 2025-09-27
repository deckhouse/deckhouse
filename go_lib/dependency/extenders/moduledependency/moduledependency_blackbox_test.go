/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package moduledependency_test

import (
	"fmt"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/moduledependency"
)

// TestInstance checks that the extender is a singleton
func TestInstance(t *testing.T) {
	instance1 := moduledependency.Instance()
	instance2 := moduledependency.Instance()

	assert.Equal(t, instance1, instance2, "Instance() should return the same instance")
	assert.NotNil(t, instance1, "Instance() should not return nil")
}

// TestName checks the extender's name
func TestName(t *testing.T) {
	extender := moduledependency.Instance()

	assert.Equal(t, moduledependency.Name, extender.Name())
}

// TestIsTerminator checks the terminator status
func TestIsTerminator(t *testing.T) {
	extender := moduledependency.Instance()

	assert.True(t, extender.IsTerminator(), "Module dependency extender should be a terminator")
}

// TestVersionHandling tests successful version handling with prerelease and metadata
func TestVersionHandling(t *testing.T) {
	extender := moduledependency.Instance()

	// Set up version helper that includes prerelease and metadata
	extender.SetModulesVersionHelper(func(_ string) (string, error) {
		return "1.0.0", nil
	})

	// Test version stripping works correctly for constraint validation
	version, _ := semver.NewVersion("1.0.0")
	err := extender.ValidateRelease("moduleWithPrerelease", "v1.0.0", version, map[string]string{
		"prereleaseModule": ">= 1.0.0", // Should match 1.0.0-beta+build after stripping
	})

	assert.NoError(t, err, "ValidateRelease should successfully handle prerelease and metadata stripping")
}

func TestVersionEmptyHandling(t *testing.T) {
	extender := moduledependency.Instance()

	// Set up version helper that includes prerelease and metadata
	extender.SetModulesVersionHelper(func(_ string) (string, error) {
		return "", nil
	})

	err := extender.AddConstraint("moduleA", map[string]string{
		"moduleB": ">= 1.0.0",
	})
	require.NoError(t, err, "AddConstraint should succeed")

	// Test version stripping works correctly for constraint validation
	version, _ := semver.NewVersion("1.0.0")
	err = extender.ValidateRelease("moduleA", "v1.0.0", version, map[string]string{
		"moduleB": ">= 1.0.0",
	})

	assert.Error(t, err, "ValidateRelease should fail to handle not deployed module")
}

// TestAddConstraintAndGetTopologicalHints tests adding constraints and getting hints
func TestAddConstraintAndGetTopologicalHints(t *testing.T) {
	extender := moduledependency.Instance()

	// Set up version helper for testing
	extender.SetModulesVersionHelper(func(_ string) (string, error) {
		return "1.0.0", nil
	})

	// Test adding a constraint
	err := extender.AddConstraint("moduleA", map[string]string{
		"moduleB": ">= 1.0.0",
		"moduleC": ">= 2.0.0",
	})
	require.NoError(t, err, "AddConstraint should succeed")

	// Test getting topological hints
	hints := extender.GetTopologicalHints("moduleA")
	assert.ElementsMatch(t, []string{"moduleB", "moduleC"}, hints)

	// Test adding a constraint with self-reference
	err = extender.AddConstraint("moduleD", map[string]string{
		"moduleD": ">= 1.0.0", // Self-reference
		"moduleE": ">= 2.0.0",
	})
	require.NoError(t, err, "AddConstraint should succeed even with self-reference")

	hints = extender.GetTopologicalHints("moduleD")
	assert.ElementsMatch(t, []string{"moduleE"}, hints, "Self-reference should be excluded from hints")

	// Test adding an invalid constraint
	err = extender.AddConstraint("moduleF", map[string]string{
		"moduleG": "invalid constraint",
	})
	assert.Error(t, err, "AddConstraint should fail with an invalid constraint")
}

// TestDeleteConstraint tests removing constraints
func TestDeleteConstraint(t *testing.T) {
	extender := moduledependency.Instance()

	// Add a constraint
	err := extender.AddConstraint("moduleToDelete", map[string]string{
		"dependency": ">= 1.0.0",
	})
	require.NoError(t, err)

	// Verify constraint exists
	hints := extender.GetTopologicalHints("moduleToDelete")
	assert.NotEmpty(t, hints)

	// Delete the constraint
	extender.DeleteConstraint("moduleToDelete")

	// Verify constraint is gone
	hints = extender.GetTopologicalHints("moduleToDelete")
	assert.Empty(t, hints, "Hints should be empty after DeleteConstraint")
}

// TestValidateReleaseWithLoopDetection tests loop detection in dependency constraints
func TestValidateReleaseWithLoopDetection(t *testing.T) {
	extender := moduledependency.Instance()

	// Set up version helper
	extender.SetModulesVersionHelper(func(_ string) (string, error) {
		return "1.0.0", nil
	})

	// Add a constraint for moduleX that depends on moduleY
	err := extender.AddConstraint("moduleX", map[string]string{
		"moduleY": ">= 1.0.0",
	})
	require.NoError(t, err)

	// Try to add moduleY with a dependency on moduleX - should detect loop
	version, _ := semver.NewVersion("1.0.0")
	err = extender.ValidateRelease("moduleY", "v1.0.0", version, map[string]string{
		"moduleX": ">= 1.0.0",
	})

	assert.Error(t, err, "ValidateRelease should detect a dependency loop")
	assert.Contains(t, err.Error(), "forms a dependency loop", "Error should mention the loop")
}

// TestValidateReleaseWithInvalidConstraint tests validation with an invalid constraint
func TestValidateReleaseWithInvalidConstraint(t *testing.T) {
	extender := moduledependency.Instance()

	version, _ := semver.NewVersion("1.0.0")
	err := extender.ValidateRelease("moduleWithInvalidConstraint", "v1.0.0", version, map[string]string{
		"dependency": "invalid constraint",
	})

	assert.Error(t, err, "ValidateRelease should fail with an invalid constraint")
	assert.Contains(t, err.Error(), "could not validate", "Error should mention validation failure")
}

// TestValidateReleaseWithMissingDependency tests validation with a missing dependency
func TestValidateReleaseWithMissingDependency(t *testing.T) {
	extender := moduledependency.Instance()

	// Set up version helper that returns not found error
	extender.SetModulesVersionHelper(func(moduleName string) (string, error) {
		return "", apierrors.NewNotFound(schema.GroupResource{Group: "modules", Resource: "module"}, moduleName)
	})

	version, _ := semver.NewVersion("1.0.0")
	err := extender.ValidateRelease("moduleWithMissingDep", "v1.0.0", version, map[string]string{
		"missingDep": ">= 1.0.0",
	})

	assert.Error(t, err, "ValidateRelease should report missing dependency")
	assert.Contains(t, err.Error(), "could not get", "Error should mention the get failure")
}

// TestValidateReleaseWithUnparsableVersion tests validation with an unparsable parent version
func TestValidateReleaseWithUnparsableVersion(t *testing.T) {
	extender := moduledependency.Instance()

	// Set up version helper that returns unparsable version
	extender.SetModulesVersionHelper(func(_ string) (string, error) {
		return "not.a.valid.version", nil
	})

	version, _ := semver.NewVersion("1.0.0")
	err := extender.ValidateRelease("moduleWithUnparsableDepVersion", "v1.0.0", version, map[string]string{
		"unparsableDep": ">= 1.0.0",
	})

	assert.Error(t, err, "ValidateRelease should report unparsable version")
	assert.Contains(t, err.Error(), "unparsable version", "Error should mention version parsing problem")
}

// TestValidateReleaseWithVersionConstraintFailure tests validation with a version constraint failure
func TestValidateReleaseWithVersionConstraintFailure(t *testing.T) {
	extender := moduledependency.Instance()

	// Set up version helper that returns a version that doesn't meet constraint
	extender.SetModulesVersionHelper(func(_ string) (string, error) {
		return "1.0.0", nil
	})

	version, _ := semver.NewVersion("1.0.0")
	err := extender.ValidateRelease("moduleWithConstraintViolation", "v1.0.0", version, map[string]string{
		"constrainedDep": ">= 2.0.0", // Requires 2.0.0 or higher
	})

	assert.Error(t, err, "ValidateRelease should report constraint violation")
	assert.Contains(t, err.Error(), "does not meet the version constraint", "Error should mention constraint violation")
}

// TestValidateReleaseWithBreakingCurrentConstraint tests validation when the new version breaks existing constraints
func TestValidateReleaseWithBreakingCurrentConstraint(t *testing.T) {
	extender := moduledependency.Instance()

	// Add a constraint for an existing module
	err := extender.AddConstraint("dependentModule", map[string]string{
		"targetModule": "~1.0.0", // Only allows 1.0.x versions
	})
	require.NoError(t, err)

	// Try to validate a release of targetModule with version 2.0.0
	version, _ := semver.NewVersion("2.0.0")
	err = extender.ValidateRelease("targetModule", "v2.0.0", version, map[string]string{})

	assert.Error(t, err, "ValidateRelease should detect breaking existing constraints")
	assert.Contains(t, err.Error(), "does not meet the version constraint", "Error should mention constraint violation")
}

// TestFilter tests the Filter function with various scenarios
func TestFilter(t *testing.T) {
	tests := []struct {
		name            string
		moduleName      string
		moduleVersion   string
		dependencies    map[string]string
		enabledModules  []string
		versionHelper   func(moduleName string) (string, error)
		expectedAllow   bool
		expectedIsError bool
	}{
		{
			name:            "No constraints",
			moduleName:      "moduleWithoutConstraints",
			expectedAllow:   false,
			expectedIsError: false,
		},
		{
			name:       "All dependencies satisfied",
			moduleName: "moduleWithSatisfiedDeps",
			dependencies: map[string]string{
				"dep1": ">= 1.0.0",
				"dep2": ">= 1.0.0",
			},
			enabledModules: []string{"dep1", "dep2"},
			versionHelper: func(_ string) (string, error) {
				return "1.0.0", nil
			},
			expectedAllow:   true,
			expectedIsError: false,
		},
		{
			name:       "Missing dependency",
			moduleName: "moduleWithMissingDep",
			dependencies: map[string]string{
				"missingDep": ">= 1.0.0",
			},
			enabledModules: []string{},
			versionHelper: func(moduleName string) (string, error) {
				return "", apierrors.NewNotFound(schema.GroupResource{Group: "modules", Resource: "module"}, moduleName)
			},
			expectedAllow:   false,
			expectedIsError: true,
		},
		{
			name:       "Disabled dependency",
			moduleName: "moduleWithDisabledDep",
			dependencies: map[string]string{
				"disabledDep": ">= 1.0.0",
			},
			enabledModules: []string{}, // Empty list means disabledDep is disabled
			versionHelper: func(_ string) (string, error) {
				return "1.0.0", nil
			},
			expectedAllow:   false,
			expectedIsError: true,
		},
		{
			name:       "Version helper error",
			moduleName: "moduleWithHelperError",
			dependencies: map[string]string{
				"errorDep": ">= 1.0.0",
			},
			enabledModules: []string{"errorDep"},
			versionHelper: func(_ string) (string, error) {
				return "", fmt.Errorf("internal error")
			},
			expectedAllow:   false,
			expectedIsError: true,
		},
		{
			name:       "Unparsable version",
			moduleName: "moduleWithUnparsableVersion",
			dependencies: map[string]string{
				"badVersionDep": ">= 1.0.0",
			},
			enabledModules: []string{"badVersionDep"},
			versionHelper: func(_ string) (string, error) {
				return "not.a.version", nil
			},
			expectedAllow:   false,
			expectedIsError: true,
		},
		{
			name:       "Constraint not satisfied",
			moduleName: "moduleWithUnsatisfiedConstraint",
			dependencies: map[string]string{
				"constraintDep": ">= 2.0.0",
			},
			enabledModules: []string{"constraintDep"},
			versionHelper: func(_ string) (string, error) {
				return "1.0.0", nil
			},
			expectedAllow:   false,
			expectedIsError: true,
		},
		{
			name:       "Optional constraint",
			moduleName: "moduleWithOptionalConstraint",
			dependencies: map[string]string{
				"constraintDep": ">= 1.0.0 !optional",
			},
			enabledModules: []string{},
			versionHelper: func(_ string) (string, error) {
				return "", nil
			},
			expectedAllow:   true,
			expectedIsError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extender := moduledependency.Instance()

			// Set up helpers
			extender.SetModulesVersionHelper(tt.versionHelper)
			extender.SetModulesStateHelper(func() []string {
				return tt.enabledModules
			})

			// Add constraint if it exists
			if tt.dependencies != nil {
				err := extender.AddConstraint(tt.moduleName, tt.dependencies)
				require.NoError(t, err)
			} else {
				extender.DeleteConstraint(tt.moduleName) // Ensure no constraint exists
			}

			// Test Filter function
			allowed, err := extender.Filter(tt.moduleName, nil)

			if tt.dependencies == nil {
				assert.Nil(t, allowed, "Filter should return nil when no constraints exist")
				assert.Nil(t, err, "Filter should return nil error when no constraints exist")
			} else {
				if tt.expectedIsError {
					assert.NotNil(t, err, "Filter should return an error")
				} else {
					assert.Nil(t, err, "Filter should not return an error")
				}

				if allowed != nil {
					assert.Equal(t, tt.expectedAllow, *allowed, "Filter should return expected allowed value")
				} else {
					assert.Error(t, err, "Filter should return an error when allowed is nil")
				}
			}
		})
	}
}

// TestVersionHandlingWithPrereleaseAndMetadata tests the handling of prerelease and metadata in versions
func TestVersionHandlingWithPrereleaseAndMetadata(t *testing.T) {
	extender := moduledependency.Instance()

	// Set up version helper that includes prerelease and metadata
	extender.SetModulesVersionHelper(func(_ string) (string, error) {
		return "1.0.0-beta+build", nil
	})

	version, _ := semver.NewVersion("1.0.0")
	err := extender.ValidateRelease("moduleWithPrerelease", "v1.0.0", version, map[string]string{
		"prereleaseModule": "= 1.0.0", // Should match 1.0.0-beta+build after stripping
	})

	assert.NoError(t, err, "ValidateRelease should handle prerelease and metadata stripping")

	// Test with a version that can't be stripped properly
	invalidVersion, _ := semver.NewVersion("0.0.0+metadata-only")
	err = extender.ValidateRelease("invalidVersionModule", "v0.0.0", invalidVersion, map[string]string{})

	assert.NoError(t, err, "ValidateRelease should handle metadata-only versions")
}

// TestMultipleErrorFormatting tests the formatting of multiple errors
func TestMultipleErrorFormatting(t *testing.T) {
	extender := moduledependency.Instance()

	// Set up version helper that returns different errors for different modules
	extender.SetModulesVersionHelper(func(moduleName string) (string, error) {
		switch moduleName {
		case "notFoundModule":
			return "", apierrors.NewNotFound(schema.GroupResource{}, moduleName)
		case "badVersionModule":
			return "invalid.version", nil
		case "validModule":
			return "1.0.0", nil
		default:
			return "", fmt.Errorf("unknown module")
		}
	})

	version, _ := semver.NewVersion("1.0.0")
	err := extender.ValidateRelease("moduleWithMultipleErrors", "v1.0.0", version, map[string]string{
		"notFoundModule":   ">= 1.0.0",
		"badVersionModule": ">= 1.0.0",
		"validModule":      ">= 2.0.0", // Will fail the constraint check
	})

	assert.Error(t, err)

	// Check that the error contains multiple lines with sorted error messages
	multiError, ok := err.(*multierror.Error)
	if assert.True(t, ok, "Error should be a multierror.Error") {
		assert.GreaterOrEqual(t, len(multiError.Errors), 3, "Should have at least 3 errors")

		errString := err.Error()
		assert.Contains(t, errString, "errors occurred", "Error should mention multiple errors")
		assert.Contains(t, errString, "could not get", "Error should mention missing module")
		assert.Contains(t, errString, "unparsable version", "Error should mention bad version")
		assert.Contains(t, errString, "does not meet the version constraint", "Error should mention constraint violation")
	}
}
