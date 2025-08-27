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

package moduleloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestLoadConversions(t *testing.T) {
	tests := []struct {
		name        string
		modulePath  string
		expectError bool
		expectEmpty bool
		expectCount int
	}{
		{
			name:        "user-authn module with conversions",
			modulePath:  "./testdata/modules/user-authn",
			expectError: false,
			expectEmpty: false,
			expectCount: 1, // v2.yaml should have 1 conversion object
		},
		{
			name:        "istio module with multiple conversions",
			modulePath:  "./testdata/modules/istio",
			expectError: false,
			expectEmpty: false,
			expectCount: 2, // v2.yaml + v3.yaml should have 2 conversion objects
		},
		{
			name:        "module without conversions",
			modulePath:  "./testdata/modules/simple-module",
			expectError: false,
			expectEmpty: true,
			expectCount: 0,
		},
		{
			name:        "empty conversions path",
			modulePath:  "",
			expectError: true,
			expectEmpty: true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &Loader{}
			var conversionsDir string
			if tt.modulePath == "" {
				conversionsDir = ""
			} else {
				conversionsDir = filepath.Join(tt.modulePath, "openapi", "conversions")
			}
			conversions, err := loader.loadConversions(conversionsDir)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
				return
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expectEmpty && len(conversions) != 0 {
				t.Errorf("Expected empty conversions but got %d", len(conversions))
				return
			}

			if !tt.expectEmpty && len(conversions) == 0 {
				t.Errorf("Expected non-empty conversions but got empty")
				return
			}

			if tt.expectCount > 0 && len(conversions) != tt.expectCount {
				t.Errorf("Expected %d conversions but got %d", tt.expectCount, len(conversions))
			}

			// Log the conversions for debugging
			t.Logf("Module: %s, Conversions count: %d", tt.name, len(conversions))
			for i, conversion := range conversions {
				t.Logf("  Conversion %d:", i+1)
				t.Logf("    Expressions (%d): %v", len(conversion.Expr), conversion.Expr)
				if conversion.Descriptions != nil {
					t.Logf("    Descriptions: RU=%q, EN=%q", conversion.Descriptions.Ru, conversion.Descriptions.En)
				}
			}
		})
	}
}

func TestSetVersionWithConversions(t *testing.T) {
	tests := []struct {
		name              string
		modulePath        string
		configYAML        string
		expectConversions bool
	}{
		{
			name:       "user-authn with conversions",
			modulePath: "./testdata/modules/user-authn",
			configYAML: `x-config-version: 2
type: object
properties:
  publishAPI:
    type: object`,
			expectConversions: true,
		},
		{
			name:       "module without conversions",
			modulePath: "./testdata/modules/simple-module",
			configYAML: `x-config-version: 1
type: object`,
			expectConversions: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &Loader{}
			settings := &v1alpha1.ModuleSettingsDefinition{}

			// Load conversions using the loader
			var conversions []v1alpha1.ModuleSettingsConversion
			conversionsDir := filepath.Join(tt.modulePath, "openapi", "conversions")
			// Check if conversions directory exists (like in processModuleDefinition)
			if _, err := os.Stat(conversionsDir); err == nil {
				conversions, err = loader.loadConversions(conversionsDir)
				if err != nil {
					t.Errorf("Unexpected error loading conversions: %v", err)
					return
				}
			} else if !os.IsNotExist(err) {
				t.Errorf("Unexpected error checking conversions directory: %v", err)
				return
			}
			// If directory doesn't exist, conversions remains empty slice (nil)

			err := settings.SetVersion([]byte(tt.configYAML), conversions)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(settings.Spec.Versions) != 1 {
				t.Errorf("Expected 1 version but got %d", len(settings.Spec.Versions))
				return
			}

			version := settings.Spec.Versions[0]
			hasConversions := len(version.Conversions) > 0

			if tt.expectConversions && !hasConversions {
				t.Errorf("Expected conversions but got none")
			}

			if !tt.expectConversions && hasConversions {
				t.Errorf("Expected no conversions but got %d", len(version.Conversions))
			}

			// Log results for debugging
			t.Logf("Module: %s, Version: %s, Conversions count: %d",
				tt.name, version.Name, len(version.Conversions))
			for i, conversion := range version.Conversions {
				t.Logf("  Conversion %d:", i+1)
				t.Logf("    Expressions (%d): %v", len(conversion.Expr), conversion.Expr)
				if conversion.Descriptions != nil {
					t.Logf("    Descriptions: RU=%q, EN=%q", conversion.Descriptions.Ru, conversion.Descriptions.En)
				}
			}
		})
	}
}
