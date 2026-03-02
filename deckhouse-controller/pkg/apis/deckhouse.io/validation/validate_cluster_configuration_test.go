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

package validation

import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		expectError bool
		expected    string
	}{
		{
			name:        "valid version",
			version:     "1.23.4",
			expectError: false,
			expected:    "1.23.4",
		},
		{
			name:        "version with whitespace",
			version:     "  1.23.4  \n",
			expectError: false,
			expected:    "1.23.4",
		},
		{
			name:        "version with tabs and newlines",
			version:     "\t1.23.4\r\n",
			expectError: false,
			expected:    "1.23.4",
		},
		{
			name:        "empty string",
			version:     "",
			expectError: true,
		},
		{
			name:        "whitespace only",
			version:     "   \n\t  ",
			expectError: true,
		},
		{
			name:        "invalid version",
			version:     "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseVersion(tt.version)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.String() != tt.expected {
					t.Errorf("expected version %s, got %s", tt.expected, result.String())
				}
			}
		})
	}
}

func TestValidateKubernetesVersionDowngrade(t *testing.T) {
	tests := []struct {
		name        string
		oldVersion  string
		newVersion  string
		secretData  map[string][]byte
		expectValid bool
		expectError bool
	}{
		{
			name:        "same version",
			oldVersion:  "1.23.4",
			newVersion:  "1.23.4",
			secretData:  map[string][]byte{},
			expectValid: true,
			expectError: false,
		},
		{
			name:        "upgrade within minor version",
			oldVersion:  "1.23.4",
			newVersion:  "1.23.5",
			secretData:  map[string][]byte{},
			expectValid: true,
			expectError: false,
		},
		{
			name:        "upgrade across minor versions",
			oldVersion:  "1.23.4",
			newVersion:  "1.24.0",
			secretData:  map[string][]byte{},
			expectValid: true,
			expectError: false,
		},
		{
			name:        "downgrade within 1 minor version",
			oldVersion:  "1.24.0",
			newVersion:  "1.23.5",
			secretData:  map[string][]byte{},
			expectValid: true,
			expectError: false,
		},
		{
			name:        "downgrade more than 1 minor version",
			oldVersion:  "1.25.0",
			newVersion:  "1.23.0",
			secretData:  map[string][]byte{},
			expectValid: false,
			expectError: false,
		},
		{
			name:        "downgrade across major versions",
			oldVersion:  "2.0.0",
			newVersion:  "1.30.0",
			secretData:  map[string][]byte{},
			expectValid: false,
			expectError: false,
		},
		{
			name:       "old version Automatic with maxUsed version",
			oldVersion: "Automatic",
			newVersion: "1.23.0",
			secretData: map[string][]byte{
				"maxUsedControlPlaneKubernetesVersion": []byte("1.24.0"),
			},
			expectValid: true,
			expectError: false,
		},
		{
			name:       "old version Automatic downgrade",
			oldVersion: "Automatic",
			newVersion: "1.22.0",
			secretData: map[string][]byte{
				"maxUsedControlPlaneKubernetesVersion": []byte("1.24.0"),
			},
			expectValid: false,
			expectError: false,
		},
		{
			name:       "new version Automatic with deckhouse default",
			oldVersion: "1.23.0",
			newVersion: "Automatic",
			secretData: map[string][]byte{
				"deckhouseDefaultKubernetesVersion": []byte("1.24.0"),
			},
			expectValid: true,
			expectError: false,
		},
		{
			name:       "new version Automatic downgrade",
			oldVersion: "1.25.0",
			newVersion: "Automatic",
			secretData: map[string][]byte{
				"deckhouseDefaultKubernetesVersion": []byte("1.24.0"),
			},
			expectValid: false,
			expectError: false,
		},
		{
			name:       "both versions Automatic",
			oldVersion: "Automatic",
			newVersion: "Automatic",
			secretData: map[string][]byte{
				"maxUsedControlPlaneKubernetesVersion": []byte("1.24.0"),
				"deckhouseDefaultKubernetesVersion":    []byte("1.24.0"),
			},
			expectValid: true,
			expectError: false,
		},
		{
			name:       "both versions Automatic with different values",
			oldVersion: "Automatic",
			newVersion: "Automatic",
			secretData: map[string][]byte{
				"maxUsedControlPlaneKubernetesVersion": []byte("1.25.0"),
				"deckhouseDefaultKubernetesVersion":    []byte("1.24.0"),
			},
			expectValid: true, // both are "Automatic" so considered same, regardless of resolved versions
			expectError: false,
		},
		{
			name:       "old version Automatic missing maxUsed version",
			oldVersion: "Automatic",
			newVersion: "1.23.0",
			secretData: map[string][]byte{
				// missing maxUsedControlPlaneKubernetesVersion
			},
			expectValid: true, // should allow when maxUsed version is missing
			expectError: false,
		},
		{
			name:       "new version Automatic missing deckhouse default",
			oldVersion: "1.23.0",
			newVersion: "Automatic",
			secretData: map[string][]byte{
				// missing deckhouseDefaultKubernetesVersion
			},
			expectValid: true, // should allow when deckhouse default is missing
			expectError: false,
		},
		{
			name:       "old version Automatic with invalid maxUsed version",
			oldVersion: "Automatic",
			newVersion: "1.23.0",
			secretData: map[string][]byte{
				"maxUsedControlPlaneKubernetesVersion": []byte("invalid"),
			},
			expectValid: false,
			expectError: true,
		},
		{
			name:       "new version Automatic with invalid deckhouse default",
			oldVersion: "1.23.0",
			newVersion: "Automatic",
			secretData: map[string][]byte{
				"deckhouseDefaultKubernetesVersion": []byte("invalid"),
			},
			expectValid: false,
			expectError: true,
		},
		{
			name:       "old version Automatic with whitespace in maxUsed version",
			oldVersion: "Automatic",
			newVersion: "1.23.0",
			secretData: map[string][]byte{
				"maxUsedControlPlaneKubernetesVersion": []byte("  1.24.0  \n"),
			},
			expectValid: true,
			expectError: false,
		},
		{
			name:       "new version Automatic with whitespace in deckhouse default",
			oldVersion: "1.23.0",
			newVersion: "Automatic",
			secretData: map[string][]byte{
				"deckhouseDefaultKubernetesVersion": []byte("\t1.24.0\r\n"),
			},
			expectValid: true,
			expectError: false,
		},
		{
			name:       "old version Automatic with empty maxUsed version",
			oldVersion: "Automatic",
			newVersion: "1.23.0",
			secretData: map[string][]byte{
				"maxUsedControlPlaneKubernetesVersion": []byte(""),
			},
			expectValid: false,
			expectError: true,
		},
		{
			name:       "new version Automatic with empty deckhouse default",
			oldVersion: "1.23.0",
			newVersion: "Automatic",
			secretData: map[string][]byte{
				"deckhouseDefaultKubernetesVersion": []byte("   "),
			},
			expectValid: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := &v1.Secret{
				Data: tt.secretData,
			}

			result, err := validateKubernetesVersionDowngrade(tt.oldVersion, tt.newVersion, secret)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.Valid != tt.expectValid {
					t.Errorf("expected valid=%v, got valid=%v, message=%v", tt.expectValid, result.Valid, result.Message)
				}
			}
		})
	}
}

func TestValidateKubernetesVersionDowngradeIntegration(t *testing.T) {
	t.Run("complex scenario with real secret data", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				"maxUsedControlPlaneKubernetesVersion": []byte("1.27.5"),
				"deckhouseDefaultKubernetesVersion":    []byte("1.28.0"),
			},
		}

		// Test various transitions
		testCases := []struct {
			oldVersion string
			newVersion string
			expected   bool
		}{
			{"1.26.0", "1.27.0", true},       // upgrade
			{"1.27.0", "1.26.0", true},       // downgrade within 1 minor
			{"1.28.0", "1.26.0", false},      // downgrade more than 1 minor
			{"Automatic", "1.26.0", true},    // old automatic, new specific (1.27.5 > 1.26.0)
			{"Automatic", "1.28.0", true},    // old automatic, new specific (1.27.5 < 1.28.0)
			{"Automatic", "1.29.0", true},    // old automatic, new specific (upgrade)
			{"1.26.0", "Automatic", true},    // old specific, new automatic (1.26.0 < 1.28.0)
			{"1.29.0", "Automatic", false},   // old specific, new automatic (1.29.0 > 1.28.0 - downgrade)
			{"Automatic", "Automatic", true}, // both automatic, same resolved versions
		}

		for _, tc := range testCases {
			result, err := validateKubernetesVersionDowngrade(tc.oldVersion, tc.newVersion, secret)
			if err != nil {
				t.Errorf("unexpected error for %s -> %s: %v", tc.oldVersion, tc.newVersion, err)
				continue
			}
			if result.Valid != tc.expected {
				t.Errorf("for %s -> %s: expected valid=%v, got valid=%v, message=%v",
					tc.oldVersion, tc.newVersion, tc.expected, result.Valid, result.Message)
			}
		}
	})
}
