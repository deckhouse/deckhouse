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

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:metadata:labels="heritage=deckhouse"
// +kubebuilder:metadata:labels="app.kubernetes.io/name=deckhouse"
// +kubebuilder:metadata:labels="app.kubernetes.io/part-of=deckhouse"
// +crd-enricher:crd:preserveUnknownFields=false
// +crd-enricher:crd:minimal=true

// It displays module settings.
// Defines a list of module settings versions.
type ModuleSettingsDefinition struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the module settings.
	Spec ModuleSettingsDefinitionSpec `json:"spec"`
}

type ModuleSettingsDefinitionSpec struct {
	// List of module settings versions. Each version includes a name and a schema.
	Versions []ModuleSettingsDefinitionVersion `json:"versions,omitempty"`
}

type ModuleSettingsDefinitionVersion struct {
	// Module settings version.
	Name string `json:"name"`
	// Settings schema for the given module version.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Schema *apiextensionsv1.CustomResourceValidation `json:"schema,omitempty"`
	// List of conversion rules for this version.
	Conversions []ModuleSettingsConversion `json:"conversions,omitempty"`
}

// A single conversion rule with expressions and descriptions.
type ModuleSettingsConversion struct {
	// Array of jq expressions to transform settings.
	Expr []string `json:"expr,omitempty"`
	// Localized descriptions of the conversion.
	Descriptions *ModuleSettingsConversionDescriptions `json:"descriptions,omitempty"`
}

type ModuleSettingsConversionDescriptions struct {
	// Russian description of the conversion.
	Ru string `json:"ru,omitempty"`
	// English description of the conversion.
	En string `json:"en,omitempty"`
}

// SetVersion adds or updates a version in the ModuleSettingsSpec.
func (s *ModuleSettingsDefinition) SetVersion(rawSchema []byte, conversions []ModuleSettingsConversion) error {
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

// +kubebuilder:object:root=true

// ModuleSettingsDefinitionList is a list of ModuleSettings resources
type ModuleSettingsDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ModuleSettingsDefinition `json:"items"`
}
