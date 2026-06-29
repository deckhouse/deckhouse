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

package markers

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	ctmarkers "sigs.k8s.io/controller-tools/pkg/markers"
)

const (
	deckhouseDescriptionRuMarker                              = "deckhouse:ru:description"
	deckhouseExampleMarker                                    = "deckhouse:example"
	deckhouseDisableAdditionalPropertiesMarker                = "deckhouse:DisableAdditionalProperties"
	deckhouseXDocSearchMarker                                 = "deckhouse:XDocSearch"
	deckhouseXDocExampleMarker                                = "deckhouse:XDocExample"
	deckhouseXRulesMarker                                     = "deckhouse:XRules"
	deckhouseXConfigVersionMarker                             = "deckhouse:XConfigVersion"
	deckhouseValidationAdditionalPropertiesItemsPatternMarker = "deckhouse:validation:AdditionalProperties:items:Pattern"
)

const (
	XDocExampleExtensionKey    = "x-doc-example"
	XDocSearchExtensionKey     = "x-doc-search"
	XRulesExtensionKey         = "x-rules"
	XConfigVersionExtensionKey = "x-config-version"
)

// SchemaMarker is implemented by every parsed deckhouse marker value type.
// Use value receivers so both T and *T satisfy the interface after parsing.
type SchemaMarker interface {
	ApplyToSchema(schema *openapi3.Schema) error
}

// MergeableSchemaMarker collapses every occurrence of the same marker into a single
// resulting marker during the MarkerValues normalization phase (before applying to the
// schema). After normalization there is exactly one element of this group left in
// MarkerValues, and its ApplyToSchema is invoked exactly once.
type MergeableSchemaMarker interface {
	SchemaMarker
	MergeFrom(occurrences []any) (SchemaMarker, error)
}

type deckhouseDescriptionRuType struct {
	Value string `marker:"value,optional"`
}

type deckhouseExampleType struct {
	Value any `marker:",optional"`
}

type deckhouseDisableAdditionalPropertiesType struct {
	Value bool `marker:",optional"`
}

type deckhouseXDocSearchType struct {
	Value []string `marker:",optional"`
}

type deckhouseXDocExampleType struct {
	Value string `marker:"value,optional"`
}

type deckhouseXRulesType struct {
	Value []string `marker:",optional"`
}

type deckhouseXConfigVersionType struct {
	Value int `marker:",optional"`
}

type deckhouseValidationAdditionalPropertiesItemsPatternType struct {
	Value string `marker:",optional"`
}

type markerRegistry map[string]SchemaMarker

func BuildDeckhouseOpenAPIMarkerRegistry() (*ctmarkers.Registry, error) {
	reg := &ctmarkers.Registry{}
	if err := registerMarkers(reg,
		markerRegistry{
			deckhouseExampleMarker:                                    deckhouseExampleType{},
			deckhouseDisableAdditionalPropertiesMarker:                deckhouseDisableAdditionalPropertiesType{},
			deckhouseXDocSearchMarker:                                 deckhouseXDocSearchType{},
			deckhouseXDocExampleMarker:                                deckhouseXDocExampleType{},
			deckhouseXRulesMarker:                                     deckhouseXRulesType{},
			deckhouseXConfigVersionMarker:                             deckhouseXConfigVersionType{},
			deckhouseValidationAdditionalPropertiesItemsPatternMarker: deckhouseValidationAdditionalPropertiesItemsPatternType{},
		},
	); err != nil {
		return nil, err
	}

	return reg, nil
}

func BuildDeckhouseDescriptionRuOpenAPIMarkerRegistry() (*ctmarkers.Registry, error) {
	reg := &ctmarkers.Registry{}
	if err := registerMarkers(reg, markerRegistry{
		deckhouseDescriptionRuMarker: deckhouseDescriptionRuType{},
	}); err != nil {
		return nil, err
	}

	return reg, nil
}

func registerMarkers(reg *ctmarkers.Registry, registry markerRegistry) error {
	for k, v := range registry {
		for _, describe := range []ctmarkers.TargetType{
			ctmarkers.DescribesField,
			ctmarkers.DescribesType,
		} {
			fieldDef, err := ctmarkers.MakeAnyTypeDefinition(k, describe, v)
			if err != nil {
				return fmt.Errorf("make definition for '%s' marker: %w", k, err)
			}
			if err := reg.Register(fieldDef); err != nil {
				return fmt.Errorf("register '%s' marker: %w", k, err)
			}
		}
	}
	return nil
}

func appendString(target, src string) string {
	return fmt.Sprintf("%s%s\n", target, src)
}

// ApplyToSchema assigns Description in full. This method is invoked exactly once,
// after the MarkerValues normalization phase (see MergeableSchemaMarker), so it is
// an assign rather than an append.
func (m deckhouseDescriptionRuType) ApplyToSchema(schema *openapi3.Schema) error {
	schema.Description = m.Value
	return nil
}

// MergeFrom collapses every occurrence of deckhouseDescriptionRuType into a single
// resulting marker by concatenating their Value via appendString. Returns an error if
// occurrences is empty or contains an element of a foreign type.
func (m deckhouseDescriptionRuType) MergeFrom(occurrences []any) (SchemaMarker, error) {
	if len(occurrences) == 0 {
		return nil, fmt.Errorf("merge: empty occurrences")
	}
	var value string
	for i, raw := range occurrences {
		typed, ok := raw.(deckhouseDescriptionRuType)
		if !ok {
			return nil, fmt.Errorf("merge: occurrence[%d] has type %T, want deckhouseDescriptionRuType", i, raw)
		}
		value = appendString(value, typed.Value)
	}
	return deckhouseDescriptionRuType{Value: value}, nil
}

func (m deckhouseExampleType) ApplyToSchema(schema *openapi3.Schema) error {
	schema.Example = m.Value
	return nil
}

func (m deckhouseDisableAdditionalPropertiesType) ApplyToSchema(schema *openapi3.Schema) error {
	schema.AdditionalProperties.Has = openapi3.Ptr(!m.Value)
	return nil
}

func (m deckhouseXDocSearchType) ApplyToSchema(schema *openapi3.Schema) error {
	schema.Extensions[XDocSearchExtensionKey] = m.Value
	return nil
}

func (m deckhouseXDocExampleType) ApplyToSchema(schema *openapi3.Schema) error {
	schema.Extensions[XDocExampleExtensionKey] = m.Value
	return nil
}

func (m deckhouseXRulesType) ApplyToSchema(schema *openapi3.Schema) error {
	schema.Extensions[XRulesExtensionKey] = m.Value
	return nil
}

func (m deckhouseXConfigVersionType) ApplyToSchema(schema *openapi3.Schema) error {
	schema.Extensions[XConfigVersionExtensionKey] = m.Value
	return nil
}

func (m deckhouseValidationAdditionalPropertiesItemsPatternType) ApplyToSchema(schema *openapi3.Schema) error {
	if !schema.Type.Is(openapi3.TypeObject) {
		return fmt.Errorf("validation:AdditionalProperties markers can only be applied to types or maps")
	}

	if schema.AdditionalProperties.Schema == nil ||
		schema.AdditionalProperties.Schema.Value == nil ||
		schema.AdditionalProperties.Schema.Value.Items == nil ||
		schema.AdditionalProperties.Schema.Value.Items.Value == nil {
		return fmt.Errorf("validation:AdditionalProperties:items:Pattern requires a map[string][]string field type")
	}

	schema.AdditionalProperties.Schema.Value.Items.Value.Pattern = m.Value
	return nil
}
