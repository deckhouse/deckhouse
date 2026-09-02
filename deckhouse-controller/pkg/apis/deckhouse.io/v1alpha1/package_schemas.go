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
	"fmt"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/openapi"
)

// ParsePackageSchemas parses the raw openapi files of a package into the
// status schemas, so every writer of the schemas stores one shape. A package
// without any schema file yields nil.
func ParsePackageSchemas(settings, values []byte) (*PackageVersionStatusSchemas, error) {
	settingsSchema, err := parsePackageSchema(settings)
	if err != nil {
		return nil, fmt.Errorf("settings schema: %w", err)
	}

	valuesSchema, err := parsePackageSchema(values)
	if err != nil {
		return nil, fmt.Errorf("values schema: %w", err)
	}

	if settingsSchema == nil && valuesSchema == nil {
		return nil, nil
	}

	return &PackageVersionStatusSchemas{
		SettingsSchema: settingsSchema,
		ValuesSchema:   valuesSchema,
	}, nil
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
