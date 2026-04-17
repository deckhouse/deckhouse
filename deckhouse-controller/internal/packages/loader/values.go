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

package loader

import (
	"fmt"
	"os"
	"path/filepath"

	addonapp "github.com/flant/addon-operator/pkg/app"
	addonutils "github.com/flant/addon-operator/pkg/utils"
)

const (
	// openAPIDir is the subdirectory containing OpenAPI schema files.
	openAPIDir = "openapi"
	// settingsFile is the OpenAPI schema for user-configurable values.
	settingsFile = "settings.yaml"
	// configValuesFile is the legacy name for settingsFile, kept for backward compatibility.
	configValuesFile = "config-values.yaml"
	// valuesFile is the OpenAPI schema for all values including internal ones.
	valuesFile = "values.yaml"
)

// loadValues loads all values-related files for a package.
// It loads:
//  1. Static values from values.yaml
//  2. Config schema from openapi/settings.yaml (with openapi/config-values.yaml fallback)
//  3. Values schema from openapi/values.yaml
//
// The static values are scoped to the package name if they contain a matching key.
// For example, if name="myapp" and values.yaml contains a "myapp" key, only that
// section is returned.
//
// Returns:
//   - static: Parsed static values (maybe scoped to package name)
//   - config: Raw OpenAPI config schema bytes
//   - values: Raw OpenAPI values schema bytes
//   - error: Any error encountered during loading
func loadValues(name, path string) (addonutils.Values, []byte, []byte, error) {
	// Convert package name to values key format (e.g., "my-app" → "myApp")
	valuesPackageName := addonutils.ModuleNameToValuesKey(name)

	// Load static values from values.yaml
	static, err := addonutils.LoadValuesFileFromDir(path, addonapp.StrictModeEnabled)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load values file from dir '%s': %w", path, err)
	}

	// If values are scoped under package name, extract only that section
	// Example: values.yaml contains {"myApp": {...}} → return just {...}
	if static.HasKey(valuesPackageName) {
		static = static.GetKeySection(valuesPackageName)
	}

	// Load OpenAPI schemas (settings.yaml and values.yaml)
	// Returns raw YAML bytes for schema validation
	config, values, err := loadPackageSchemas(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read openapi files: %w", err)
	}

	return static, config, values, nil
}

// readOptionalFile reads the file at path. If the file does not exist, it
// returns (nil, nil) instead of an error. Any other read error is wrapped.
func readOptionalFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read file '%s': %w", path, err)
	}
	return data, nil
}

// readSettingsSchema returns the OpenAPI schema for user-configurable values.
// It prefers openapi/settings.yaml and silently falls back to the legacy
// openapi/config-values.yaml when settings.yaml is absent.
func readSettingsSchema(schemasDir string) ([]byte, error) {
	data, err := readOptionalFile(filepath.Join(schemasDir, settingsFile))
	if err != nil || data != nil {
		return data, err
	}
	return readOptionalFile(filepath.Join(schemasDir, configValuesFile))
}

// loadPackageSchemas reads settings.yaml (or legacy config-values.yaml) and
// values.yaml from the specified directory. Package schemas:
//
//	/modules/XXX-module-name/openapi/settings.yaml      (preferred)
//	/modules/XXX-module-name/openapi/config-values.yaml (legacy fallback)
//	/modules/XXX-module-name/openapi/values.yaml
func loadPackageSchemas(packageDir string) ([]byte, []byte, error) {
	schemasDir := filepath.Join(packageDir, openAPIDir)

	configValues, err := readSettingsSchema(schemasDir)
	if err != nil {
		return nil, nil, err
	}

	values, err := readOptionalFile(filepath.Join(schemasDir, valuesFile))
	if err != nil {
		return nil, nil, err
	}

	return configValues, values, nil
}
