// Copyright 2026 Flant JSC
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

package openapi

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// SchemaURL represents a schema url.
type SchemaURL string

// OpenAPIV3Schema is a JSON-Schema following Specification Draft 4 (http://json-schema.org/).
// It is a forked subset of apiextensionsv1.JSONSchemaProps that drops all x-kubernetes-*
// extensions and adds Deckhouse-specific x-deckhouse-* extensions as typed fields.
type OpenAPIV3Schema struct {
	ID          string    `json:"id,omitempty"`
	Schema      SchemaURL `json:"$schema,omitempty"`
	Ref         *string   `json:"$ref,omitempty"`
	Description string    `json:"description,omitempty"`
	Type        string    `json:"type,omitempty"`
	Format      string    `json:"format,omitempty"`

	Title string `json:"title,omitempty"`
	// default is a default value for undefined object fields.
	Default *apiextensionsv1.JSON `json:"default,omitempty"`

	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMaximum bool     `json:"exclusiveMaximum,omitempty"`
	Minimum          *float64 `json:"minimum,omitempty"`
	ExclusiveMinimum bool     `json:"exclusiveMinimum,omitempty"`

	MaxLength *int64 `json:"maxLength,omitempty"`
	MinLength *int64 `json:"minLength,omitempty"`
	Pattern   string `json:"pattern,omitempty"`

	MaxItems    *int64 `json:"maxItems,omitempty"`
	MinItems    *int64 `json:"minItems,omitempty"`
	UniqueItems bool   `json:"uniqueItems,omitempty"`

	MultipleOf *float64 `json:"multipleOf,omitempty"`

	// +listType=atomic
	Enum []apiextensionsv1.JSON `json:"enum,omitempty"`

	MaxProperties *int64 `json:"maxProperties,omitempty"`
	MinProperties *int64 `json:"minProperties,omitempty"`

	// +listType=atomic
	Required []string                 `json:"required,omitempty"`
	Items    *OpenAPIV3SchemaOrArray  `json:"items,omitempty"`

	// +listType=atomic
	AllOf []OpenAPIV3Schema `json:"allOf,omitempty"`
	// +listType=atomic
	OneOf []OpenAPIV3Schema `json:"oneOf,omitempty"`
	// +listType=atomic
	AnyOf                []OpenAPIV3Schema                    `json:"anyOf,omitempty"`
	Not                  *OpenAPIV3Schema                     `json:"not,omitempty"`
	Properties           map[string]OpenAPIV3Schema           `json:"properties,omitempty"`
	AdditionalProperties *OpenAPIV3SchemaOrBool               `json:"additionalProperties,omitempty"`
	PatternProperties    map[string]OpenAPIV3Schema           `json:"patternProperties,omitempty"`
	Dependencies         SchemaDependencies                   `json:"dependencies,omitempty"`
	AdditionalItems      *OpenAPIV3SchemaOrBool               `json:"additionalItems,omitempty"`
	Definitions          SchemaDefinitions                    `json:"definitions,omitempty"`
	ExternalDocs         *ExternalDocumentation               `json:"externalDocs,omitempty"`
	Example              *apiextensionsv1.JSON                `json:"example,omitempty"`
	Nullable             bool                                 `json:"nullable,omitempty"`

	// x-deckhouse-grantable-resource binds a string settings field to a grantable
	// cluster resource (multitenancy-manager AvailableClusterResource).
	// +optional
	XGrant string `json:"x-deckhouse-grantable-resource,omitempty"`

	// x-deckhouse-validations describes a list of validation rules written in the CEL expression language.
	// +optional
	// +patchMergeKey=rule
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=rule
	XValidations []ValidationRule `json:"x-deckhouse-validations,omitempty"`

	// x-deckhouse-ui-advanced marks a settings field as "advanced", hiding it behind
	// a toggle in the web console UI.
	// +optional
	XUIAdvanced bool `json:"x-deckhouse-ui-advanced,omitempty"`

	// x-required-for-helm declares field names that the Helm chart rendering
	// step treats as required even when they are not marked required in the
	// OpenAPI schema. This extension is consumed by the schema transformer
	// and promoted into the standard required array for Helm schemas.
	// +optional
	// +listType=atomic
	XRequiredForHelm []string `json:"x-required-for-helm,omitempty"`
}

// OpenAPIV3SchemaOrArray represents a value that can either be an OpenAPIV3Schema
// or an array of OpenAPIV3Schema.
type OpenAPIV3SchemaOrArray struct {
	Schema      *OpenAPIV3Schema   `json:"-"`
	JSONSchemas []OpenAPIV3Schema  `json:"-"`
}

// OpenAPIV3SchemaOrBool represents an OpenAPIV3Schema or a boolean value.
// Defaults to true for the boolean property.
type OpenAPIV3SchemaOrBool struct {
	Allows bool             `json:"-"`
	Schema *OpenAPIV3Schema `json:"-"`
}

// OpenAPIV3SchemaOrStringArray represents an OpenAPIV3Schema or a string array.
type OpenAPIV3SchemaOrStringArray struct {
	Schema   *OpenAPIV3Schema `json:"-"`
	Property []string         `json:"-"`
}

// SchemaDependencies represent a dependencies property.
type SchemaDependencies map[string]OpenAPIV3SchemaOrStringArray

// SchemaDefinitions contains the models explicitly defined in this spec.
type SchemaDefinitions map[string]OpenAPIV3Schema

// ExternalDocumentation allows referencing an external resource for extended documentation.
type ExternalDocumentation struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
}

// ValidationRule describes a validation rule written in the CEL expression language.
type ValidationRule struct {
	// Rule represents the expression which will be evaluated by CEL.
	// The `self` variable in the CEL expression is bound to the scoped value.
	Rule string `json:"rule"`

	// Message represents the message displayed when validation fails.
	Message string `json:"message,omitempty"`

	// MessageExpression declares a CEL expression that evaluates to the
	// validation failure message that is returned when this rule fails.
	// +optional
	MessageExpression string `json:"messageExpression,omitempty"`

	// Reason provides a machine-readable validation failure reason.
	// +optional
	Reason *string `json:"reason,omitempty"`

	// FieldPath represents the field path returned when the validation fails.
	// +optional
	FieldPath string `json:"fieldPath,omitempty"`
}
