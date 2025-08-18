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

package v1alpha1

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

const (
	ModuleSettingsDefinitionResource = "modulesettingsdefinitions"
	ModuleSettingsDefinitionKind     = "ModuleSettingsDefinition"
)

var (
	// ModuleSettingsDefinitionGVR GroupVersionResource
	ModuleSettingsDefinitionGVR = schema.GroupVersionResource{
		Group:    SchemeGroupVersion.Group,
		Version:  SchemeGroupVersion.Version,
		Resource: ModuleSettingsDefinitionResource,
	}
	ModuleSettingsDefinitionGVK = schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    ModuleSettingsDefinitionKind,
	}
)

var _ runtime.Object = (*ModuleConfig)(nil)

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleSettingsDefinitionList is a list of ModuleSettings resources
type ModuleSettingsDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleSettingsDefinition `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ModuleSettingsDefinition is a configuration for module or for global config values.
type ModuleSettingsDefinition struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleSettingsDefinitionSpec `json:"spec"`
}

type ModuleSettingsDefinitionSpec struct {
	Versions []ModuleSettingsDefinitionVersion `json:"versions"`
}

type ModuleSettingsDefinitionVersion struct {
	Name        string                                    `json:"name"`
	Schema      *apiextensionsv1.CustomResourceValidation `json:"schema,omitempty"`
	Conversions []string                                  `json:"conversions,omitempty"`
}

// SetVersion adds or updates a version in the ModuleSettingsSpec.
func (s *ModuleSettingsDefinition) SetVersion(rawSchema []byte, modulePath string) error {
	if rawSchema == nil {
		return nil
	}

	type schemaVersion struct {
		Version string `json:"x-config-version"`
		apiextensionsv1.JSONSchemaProps
	}

	jsonSchema := &schemaVersion{
		Version: "1",
	}
	if err := yaml.Unmarshal(rawSchema, jsonSchema); err != nil {
		return fmt.Errorf("invalid JSON schema: %w", err)
	}

	// Load conversions from the module path
	conversions, err := loadConversions(modulePath)
	if err != nil {
		return fmt.Errorf("load conversions: %w", err)
	}

	version := ModuleSettingsDefinitionVersion{
		Name:        jsonSchema.Version,
		Schema:      &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: &jsonSchema.JSONSchemaProps},
		Conversions: conversions,
	}

	for i, v := range s.Spec.Versions {
		if v.Name == jsonSchema.Version {
			s.Spec.Versions[i] = version
			return nil
		}
	}

	s.Spec.Versions = append(s.Spec.Versions, version)
	return nil
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
