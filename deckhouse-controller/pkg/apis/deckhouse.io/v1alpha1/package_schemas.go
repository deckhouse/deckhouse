/*
Copyright 2026 Flant JSC

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

package v1alpha1

import (
	"cmp"
	"fmt"
	"path"
	"slices"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/openapi"
)

// SettingsConversionsDir is the package directory, relative to the package root,
// holding the settings conversion files.
const SettingsConversionsDir = "openapi/conversions"

// MaxSettingsConversions bounds the number of settings conversion files a reader of
// an untrusted package accepts.
const MaxSettingsConversions = 100

// IsSettingsConversionFile reports whether a package file path, relative to the
// package root, is a settings conversion file.
func IsSettingsConversionFile(name string) bool {
	return path.Dir(name) == SettingsConversionsDir && path.Ext(name) == ".yaml"
}

// ParsePackageSchemas parses the raw openapi files of a package, and its raw
// settings conversion files (openapi/conversions/v<N>.yaml), into the status
// schemas, so every writer of the schemas stores one shape. A package without
// any schema or conversion file yields nil.
func ParsePackageSchemas(settings, values []byte, conversions [][]byte) (*PackageVersionStatusSchemas, error) {
	settingsSchema, err := parsePackageSchema(settings)
	if err != nil {
		return nil, fmt.Errorf("settings schema: %w", err)
	}

	valuesSchema, err := parsePackageSchema(values)
	if err != nil {
		return nil, fmt.Errorf("values schema: %w", err)
	}

	settingsConversions, err := parseSettingsConversions(conversions)
	if err != nil {
		return nil, fmt.Errorf("settings conversions: %w", err)
	}

	if settingsSchema == nil && valuesSchema == nil && settingsConversions == nil {
		return nil, nil
	}

	return &PackageVersionStatusSchemas{
		SettingsSchema:      settingsSchema,
		ValuesSchema:        valuesSchema,
		SettingsConversions: settingsConversions,
	}, nil
}

// parseSettingsConversions parses the raw conversion files into a chain ordered by
// version; each holds expressions and the versions run from 2 without a gap or a
// duplicate. None yields nil.
func parseSettingsConversions(raws [][]byte) ([]PackageSettingsConversion, error) {
	if len(raws) == 0 {
		return nil, nil
	}

	conversions := make([]PackageSettingsConversion, 0, len(raws))
	for _, raw := range raws {
		var file struct {
			Version     int                                    `json:"version"`
			Conversions []string                               `json:"conversions"`
			Description *PackageSettingsConversionDescriptions `json:"description,omitempty"`
		}

		if err := yaml.Unmarshal(raw, &file); err != nil {
			return nil, fmt.Errorf("unmarshal conversion: %w", err)
		}

		if len(file.Conversions) == 0 {
			return nil, fmt.Errorf("conversion version %d: no conversions", file.Version)
		}

		conversions = append(conversions, PackageSettingsConversion{
			Version:      file.Version,
			Expr:         file.Conversions,
			Descriptions: file.Description,
		})
	}

	slices.SortFunc(conversions, func(a, b PackageSettingsConversion) int {
		return cmp.Compare(a.Version, b.Version)
	})

	for i, conversion := range conversions {
		if expected := i + 2; conversion.Version != expected {
			return nil, fmt.Errorf("conversion version %d: expected %d, the chain starts at 2 without gaps or duplicates", conversion.Version, expected)
		}
	}

	return conversions, nil
}

// parsePackageSchema parses one raw openapi file; the x-config-version marker
// is not part of the schema and is stripped.
func parsePackageSchema(raw []byte) (*PackageSchema, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var wrapper struct {
		Version string `json:"x-config-version"`
		openapi.OpenAPIV3Schema
	}

	if err := yaml.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal schema: %w", err)
	}

	return &PackageSchema{OpenAPIV3Schema: &wrapper.OpenAPIV3Schema}, nil
}
