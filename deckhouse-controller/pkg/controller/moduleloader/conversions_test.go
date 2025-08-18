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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"testing"

	"sigs.k8s.io/yaml"

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
			modulePath:  filepath.Join("./testdata/modules/user-authn"),
			expectError: false,
			expectEmpty: false,
			expectCount: 1, // v2.yaml should have 1 conversion
		},
		{
			name:        "istio module with multiple conversions",
			modulePath:  filepath.Join("./testdata/modules/istio"),
			expectError: false,
			expectEmpty: false,
			expectCount: 3, // v2.yaml + v3.yaml should have 3 conversions total
		},
		{
			name:        "module without conversions",
			modulePath:  filepath.Join("./testdata/modules/simple-module"),
			expectError: false,
			expectEmpty: true,
			expectCount: 0,
		},
		{
			name:        "non-existent module path",
			modulePath:  "/non/existent/path",
			expectError: false,
			expectEmpty: true,
			expectCount: 0,
		},
		{
			name:        "empty module path",
			modulePath:  "",
			expectError: false,
			expectEmpty: true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conversions, err := loadConversions(tt.modulePath)

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
				t.Logf("  Conversion %d: %s", i+1, conversion)
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
			modulePath: filepath.Join("./testdata/modules/user-authn"),
			configYAML: `x-config-version: 2
type: object
properties:
  publishAPI:
    type: object`,
			expectConversions: true,
		},
		{
			name:       "module without conversions",
			modulePath: filepath.Join("./testdata/modules/simple-module"),
			configYAML: `x-config-version: 1
type: object`,
			expectConversions: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &v1alpha1.ModuleSettingsDefinition{}

			err := settings.SetVersion([]byte(tt.configYAML), tt.modulePath)
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
				t.Logf("  Conversion %d: %s", i+1, conversion)
			}
		})
	}
}

// LoadConversions loads all conversion rules from the module's conversions directory
func loadConversions(modulePath string) ([]string, error) {
	if modulePath == "" {
		return nil, nil
	}

	conversionsDir := filepath.Join(modulePath, "openapi", "conversions")

	// Check if conversions directory exists
	if _, err := os.Stat(conversionsDir); os.IsNotExist(err) {
		return nil, nil // No conversions directory, return empty slice
	} else if err != nil {
		return nil, fmt.Errorf("check conversions directory: %w", err)
	}

	// Read all files from conversions directory
	files, err := os.ReadDir(conversionsDir)
	if err != nil {
		return nil, fmt.Errorf("read conversions directory: %w", err)
	}

	// Regex to match version files like v1.yaml, v2.yaml, etc.
	versionFileRe := regexp.MustCompile(`^v(\d+)\.yaml$`)

	var allConversions []string
	versionNumbers := make([]int, 0, len(files))

	// Process each version file
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		matches := versionFileRe.FindStringSubmatch(file.Name())
		if matches == nil {
			continue // Skip non-version files
		}

		versionNum, err := strconv.Atoi(matches[1])
		if err != nil {
			continue // Skip files with invalid version numbers
		}

		versionNumbers = append(versionNumbers, versionNum)

		// Read and parse the conversion file
		filePath := filepath.Join(conversionsDir, file.Name())
		conversions, err := readConversionFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read conversion file %s: %w", file.Name(), err)
		}

		allConversions = append(allConversions, conversions...)
	}

	// Sort version numbers to ensure consistent ordering
	sort.Ints(versionNumbers)

	return allConversions, nil
}

// readConversionFile reads a single conversion file and extracts the conversions array
func readConversionFile(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Conversions []string `yaml:"conversions"`
	}

	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal conversion file: %w", err)
	}

	return parsed.Conversions, nil
}
