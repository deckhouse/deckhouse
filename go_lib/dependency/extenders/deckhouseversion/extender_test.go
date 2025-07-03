/*
Copyright 2025 Flant JSC

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

package deckhouseversion

import (
	"fmt"
	"os"
	"testing"

	scherror "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// Helper function to create a test logger
func testLogger() *log.Logger {
	return log.NewNop()
}

// TestNewExtender tests all initialization paths of NewExtender
func TestNewExtender(t *testing.T) {
	tests := []struct {
		name             string
		envVar           string
		deckhouseVersion string
		expectError      bool
		expectedVersion  string
		setupEnv         func()
		cleanupEnv       func()
	}{
		{
			name:             "valid env var overrides version",
			envVar:           "1.2.3",
			deckhouseVersion: "2.3.4",
			expectedVersion:  "1.2.3",
			setupEnv:         func() { os.Setenv("TEST_EXTENDER_DECKHOUSE_VERSION", "1.2.3") },
			cleanupEnv:       func() { os.Unsetenv("TEST_EXTENDER_DECKHOUSE_VERSION") },
		},
		{
			name:             "invalid env var is ignored",
			envVar:           "not-a-version",
			deckhouseVersion: "2.3.4",
			expectedVersion:  "2.3.4",
			setupEnv:         func() { os.Setenv("TEST_EXTENDER_DECKHOUSE_VERSION", "not-a-version") },
			cleanupEnv:       func() { os.Unsetenv("TEST_EXTENDER_DECKHOUSE_VERSION") },
		},
		{
			name:             "dev version uses default",
			deckhouseVersion: "dev",
			expectedVersion:  "v2.0.0", // Default version from versionmatcher
		},
		{
			name:             "valid version string",
			deckhouseVersion: "3.4.5",
			expectedVersion:  "3.4.5",
		},
		{
			name:             "invalid version string",
			deckhouseVersion: "not-a-version",
			expectError:      true,
			expectedVersion:  "v2.0.0", // Default version
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.setupEnv != nil {
				tt.setupEnv()
			}

			// Cleanup
			defer func() {
				if tt.cleanupEnv != nil {
					tt.cleanupEnv()
				}
			}()

			// Create extender
			extender := NewExtender(tt.deckhouseVersion, testLogger())

			// Check results
			if tt.expectError {
				assert.NotNil(t, extender.err)
			} else {
				assert.Nil(t, extender.err)
			}

			// Check version
			actualVersion := extender.versionMatcher.GetBaseVersion().Original()
			assert.Equal(t, tt.expectedVersion, actualVersion)
		})
	}
}

// TestAddConstraint tests adding constraints to the extender
func TestAddConstraint(t *testing.T) {
	tests := []struct {
		name        string
		constraint  string
		expectError bool
	}{
		{
			name:        "valid constraint",
			constraint:  ">=1.0.0",
			expectError: false,
		},
		{
			name:        "invalid constraint",
			constraint:  "invalid-constraint",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create extender
			extender := NewExtender("1.0.0", testLogger())

			// Add constraint
			err := extender.AddConstraint("test-module", tt.constraint)

			// Check result
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify constraint was added
				assert.True(t, extender.versionMatcher.Has("test-module"))
			}
		})
	}
}

// TestDeleteConstraint tests removing constraints from the extender
func TestDeleteConstraint(t *testing.T) {
	// Create extender
	extender := NewExtender("1.0.0", testLogger())

	// Add a constraint
	moduleName := "test-module"
	err := extender.AddConstraint(moduleName, ">=1.0.0")
	require.NoError(t, err)
	assert.True(t, extender.versionMatcher.Has(moduleName))

	// Delete the constraint
	extender.DeleteConstraint(moduleName)

	// Verify it was removed
	assert.False(t, extender.versionMatcher.Has(moduleName))
}

// TestName tests the Name method
func TestName(t *testing.T) {
	extender := NewExtender("1.0.0", testLogger())
	assert.Equal(t, Name, extender.Name())
}

// TestIsTerminator tests the IsTerminator method
func TestIsTerminator(t *testing.T) {
	extender := NewExtender("1.0.0", testLogger())
	assert.True(t, extender.IsTerminator())
}

// TestFilter tests the Filter method with different scenarios
func TestFilter(t *testing.T) {
	tests := []struct {
		name           string
		moduleName     string
		baseVersion    string
		constraint     string
		addConstraint  bool
		setErr         bool
		expectedResult *bool
		expectError    bool
		isPermanentErr bool
	}{
		{
			name:           "module without constraint",
			moduleName:     "no-constraint-module",
			baseVersion:    "1.0.0",
			expectedResult: nil,
			expectError:    false,
		},
		{
			name:           "satisfied constraint",
			moduleName:     "satisfied-module",
			baseVersion:    "2.0.0",
			constraint:     ">=1.0.0",
			addConstraint:  true,
			expectedResult: ptr.To(true),
			expectError:    false,
		},
		{
			name:           "unsatisfied constraint",
			moduleName:     "unsatisfied-module",
			baseVersion:    "1.0.0",
			constraint:     ">=2.0.0",
			addConstraint:  true,
			expectedResult: ptr.To(false),
			expectError:    true,
		},
		{
			name:           "extender has error",
			moduleName:     "error-module",
			baseVersion:    "1.0.0",
			constraint:     ">=1.0.0",
			addConstraint:  true,
			setErr:         true,
			expectError:    true,
			isPermanentErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create extender
			extender := NewExtender(tt.baseVersion, testLogger())

			// Setup test conditions
			if tt.addConstraint {
				err := extender.AddConstraint(tt.moduleName, tt.constraint)
				require.NoError(t, err)
			}

			if tt.setErr {
				extender.err = fmt.Errorf("test error")
			}

			// Call Filter
			result, err := extender.Filter(tt.moduleName, nil)

			// Check results
			if tt.expectError {
				assert.Error(t, err)
				if tt.isPermanentErr {
					assert.IsType(t, &scherror.PermanentError{}, err)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// TestValidateBaseVersion tests the ValidateBaseVersion method
func TestValidateBaseVersion(t *testing.T) {
	tests := []struct {
		name                  string
		moduleConstraints     map[string]string
		baseVersionToValidate string
		expectError           bool
	}{
		{
			name: "all constraints satisfied",
			moduleConstraints: map[string]string{
				"module1": ">=1.0.0",
				"module2": ">=1.0.0",
			},
			baseVersionToValidate: "2.0.0",
			expectError:           false,
		},
		{
			name: "one constraint not satisfied",
			moduleConstraints: map[string]string{
				"module1": ">=1.0.0",
				"module2": ">=3.0.0",
			},
			baseVersionToValidate: "2.0.0",
			expectError:           true,
		},
		{
			name: "invalid version to validate",
			moduleConstraints: map[string]string{
				"module1": ">=1.0.0",
			},
			baseVersionToValidate: "not-a-version",
			expectError:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create extender
			extender := NewExtender("1.0.0", testLogger())

			// Add constraints
			for module, constraint := range tt.moduleConstraints {
				err := extender.AddConstraint(module, constraint)
				require.NoError(t, err)
			}

			// Validate base version
			moduleName, err := extender.ValidateBaseVersion(tt.baseVersionToValidate)

			// Check results
			if tt.expectError {
				assert.Error(t, err)
				if moduleName != "" {
					// If a module name is returned, it means a specific module's constraint wasn't met
					assert.Contains(t, tt.moduleConstraints, moduleName)
				}
			} else {
				assert.NoError(t, err)
				assert.Empty(t, moduleName)
			}
		})
	}
}

// TestValidateRelease tests the ValidateRelease method
func TestValidateRelease(t *testing.T) {
	tests := []struct {
		name        string
		releaseName string
		constraint  string
		baseVersion string
		setErr      bool
		expectError bool
	}{
		{
			name:        "satisfied constraint",
			releaseName: "release1",
			constraint:  ">=1.0.0",
			baseVersion: "2.0.0",
			expectError: false,
		},
		{
			name:        "unsatisfied constraint",
			releaseName: "release2",
			constraint:  ">=3.0.0",
			baseVersion: "2.0.0",
			expectError: true,
		},
		{
			name:        "extender has error",
			releaseName: "release3",
			constraint:  ">=1.0.0",
			baseVersion: "2.0.0",
			setErr:      true,
			expectError: true,
		},
		{
			name:        "invalid constraint",
			releaseName: "release4",
			constraint:  "invalid-constraint",
			baseVersion: "2.0.0",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create extender
			extender := NewExtender(tt.baseVersion, testLogger())

			if tt.setErr {
				extender.err = fmt.Errorf("test error")
			}

			// Call ValidateRelease
			err := extender.ValidateRelease(tt.releaseName, tt.constraint)

			// Check results
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEndToEnd tests the full flow of the extender from creation to validation
func TestEndToEnd(t *testing.T) {
	// Create extender with version 2.0.0
	extender := NewExtender("2.0.0", testLogger())
	require.Nil(t, extender.err)

	// Add constraints for two modules
	err := extender.AddConstraint("module1", ">=1.0.0")
	require.NoError(t, err)

	err = extender.AddConstraint("module2", ">=2.0.0")
	require.NoError(t, err)

	// Filter modules
	// module1 should pass
	result1, err := extender.Filter("module1", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result1)
	assert.True(t, *result1)

	// module2 should pass
	result2, err := extender.Filter("module2", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result2)
	assert.True(t, *result2)

	// Add a module with an unsatisfied constraint
	err = extender.AddConstraint("module3", ">=3.0.0")
	require.NoError(t, err)

	// module3 should fail
	result3, err := extender.Filter("module3", nil)
	assert.Error(t, err)
	assert.NotNil(t, result3)
	assert.False(t, *result3)

	// Validate that version 3.0.0 would satisfy all constraints
	moduleName, err := extender.ValidateBaseVersion("3.0.0")
	assert.NoError(t, err)
	assert.Empty(t, moduleName)

	// Validate that version 1.0.0 would not satisfy module2 and module3
	moduleName, err = extender.ValidateBaseVersion("1.0.0")
	assert.Error(t, err)
	assert.NotEmpty(t, moduleName)

	// Delete a module constraint
	extender.DeleteConstraint("module3")
	assert.False(t, extender.versionMatcher.Has("module3"))

	// Now 1.0.0 should only fail for module2
	moduleName, err = extender.ValidateBaseVersion("1.0.0")
	assert.Error(t, err)
	assert.Equal(t, "module2", moduleName)

	// Test a release validation
	err = extender.ValidateRelease("test-release", "<=2.5.0")
	assert.NoError(t, err)

	err = extender.ValidateRelease("test-release2", ">=3.0.0")
	assert.Error(t, err)
}
