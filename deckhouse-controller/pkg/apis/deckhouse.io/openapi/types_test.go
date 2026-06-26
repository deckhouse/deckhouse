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
	"encoding/json"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func jsonPtr(raw string) *apiextensionsv1.JSON {
	if raw == "" {
		return nil
	}
	return &apiextensionsv1.JSON{Raw: []byte(raw)}
}

func float64Ptr(v float64) *float64 { return &v }
func int64Ptr(v int64) *int64       { return &v }
func stringPtr(v string) *string    { return &v }

// TestMarshalRoundtrip_shallowSchema verifies a simple object schema roundtrips
// through JSON.
func TestMarshalRoundtrip_shallowSchema(t *testing.T) {
	original := &OpenAPIV3Schema{
		Type: "object",
		Properties: map[string]OpenAPIV3Schema{
			"name":    {Type: "string"},
			"enabled": {Type: "boolean", Default: jsonPtr("true")},
		},
		Required: []string{"name"},
	}

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored OpenAPIV3Schema
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.Type != "object" {
		t.Errorf("type: got %q, want object", restored.Type)
	}
	if len(restored.Properties) != 2 {
		t.Fatalf("properties count: got %d, want 2", len(restored.Properties))
	}
	if prop, ok := restored.Properties["name"]; !ok || prop.Type != "string" {
		t.Errorf("properties.name: got %+v", prop)
	}
	if prop, ok := restored.Properties["enabled"]; !ok || prop.Type != "boolean" || string(prop.Default.Raw) != "true" {
		t.Errorf("properties.enabled: got %+v", prop)
	}
	if len(restored.Required) != 1 || restored.Required[0] != "name" {
		t.Errorf("required: got %v", restored.Required)
	}
}

// TestMarshalRoundtrip_allStandardFields exercises every standard JSON Schema
// field we forked into OpenAPIV3Schema.
func TestMarshalRoundtrip_allStandardFields(t *testing.T) {
	original := &OpenAPIV3Schema{
		ID:               "https://example.com/schema",
		Schema:           "http://json-schema.org/draft-04/schema#",
		Ref:              stringPtr("#/definitions/foo"),
		Description:      "a test schema",
		Type:             "object",
		Format:           "email",
		Title:            "Test Schema",
		Default:          jsonPtr(`{"foo":"bar"}`),
		Maximum:          float64Ptr(100),
		ExclusiveMaximum: true,
		Minimum:          float64Ptr(1),
		ExclusiveMinimum: true,
		MaxLength:        int64Ptr(255),
		MinLength:        int64Ptr(1),
		Pattern:          "^[a-z]+$",
		MaxItems:         int64Ptr(10),
		MinItems:         int64Ptr(1),
		UniqueItems:      true,
		MultipleOf:       float64Ptr(2),
		Enum:             []apiextensionsv1.JSON{{Raw: []byte(`"a"`)}, {Raw: []byte(`"b"`)}},
		MaxProperties:    int64Ptr(20),
		MinProperties:    int64Ptr(1),
		Required:         []string{"field"},
		Items: &OpenAPIV3SchemaOrArray{
			Schema: &OpenAPIV3Schema{Type: "string"},
		},
		AdditionalProperties: &OpenAPIV3SchemaOrBool{Allows: false},
		AdditionalItems:      &OpenAPIV3SchemaOrBool{Schema: &OpenAPIV3Schema{Type: "integer"}},
		ExternalDocs: &ExternalDocumentation{
			Description: "docs",
			URL:         "https://example.com",
		},
		Example:  jsonPtr(`"hello"`),
		Nullable: true,
	}

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored OpenAPIV3Schema
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.ID != "https://example.com/schema" {
		t.Errorf("id mismatch")
	}
	if string(restored.Schema) != "http://json-schema.org/draft-04/schema#" {
		t.Errorf("$schema mismatch: %s", restored.Schema)
	}
	if restored.Ref == nil || *restored.Ref != "#/definitions/foo" {
		t.Errorf("$ref mismatch")
	}
	if restored.Format != "email" {
		t.Errorf("format mismatch")
	}
	if restored.Maximum == nil || *restored.Maximum != 100 {
		t.Errorf("maximum mismatch")
	}
	if !restored.ExclusiveMaximum {
		t.Errorf("exclusiveMaximum mismatch")
	}
	if restored.Items == nil || restored.Items.Schema == nil || restored.Items.Schema.Type != "string" {
		t.Errorf("items mismatch")
	}
	if restored.AdditionalProperties == nil || restored.AdditionalProperties.Schema != nil || restored.AdditionalProperties.Allows {
		t.Errorf("additionalProperties mismatch: allows=%v, schema=%v", restored.AdditionalProperties.Allows, restored.AdditionalProperties.Schema)
	}
	if restored.AdditionalItems == nil || restored.AdditionalItems.Schema == nil || restored.AdditionalItems.Schema.Type != "integer" {
		t.Errorf("additionalItems mismatch")
	}
	if restored.ExternalDocs == nil || restored.ExternalDocs.URL != "https://example.com" {
		t.Errorf("externalDocs mismatch")
	}
	if restored.Example == nil || string(restored.Example.Raw) != `"hello"` {
		t.Errorf("example mismatch: %v", restored.Example)
	}
	if !restored.Nullable {
		t.Errorf("nullable mismatch")
	}
	if restored.Default == nil || string(restored.Default.Raw) != `{"foo":"bar"}` {
		t.Errorf("default mismatch: %v", restored.Default)
	}
	if len(restored.Enum) != 2 {
		t.Errorf("enum len: got %d", len(restored.Enum))
	}
}

// TestMarshalRoundtrip_xDeckhouseExtensions verifies all three x-deckhouse-*
// extensions survive JSON roundtrip.
func TestMarshalRoundtrip_xDeckhouseExtensions(t *testing.T) {
	original := &OpenAPIV3Schema{
		Type: "object",
		Properties: map[string]OpenAPIV3Schema{
			"storageClass": {
				Type:   "string",
				XGrant: "storageclasses",
			},
			"replicas": {
				Type:        "integer",
				Default:     jsonPtr("1"),
				XUIAdvanced: true,
			},
		},
		XValidations: []ValidationRule{
			{Rule: "self.storageClass != ''", Message: "storageClass must be set"},
		},
	}

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored OpenAPIV3Schema
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	sc, ok := restored.Properties["storageClass"]
	if !ok {
		t.Fatal("missing storageClass property")
	}
	if sc.XGrant != "storageclasses" {
		t.Errorf("x-deckhouse-grantable-resource: got %q, want storageclasses", sc.XGrant)
	}

	rep, ok := restored.Properties["replicas"]
	if !ok {
		t.Fatal("missing replicas property")
	}
	if !rep.XUIAdvanced {
		t.Errorf("x-deckhouse-ui-advanced: got false, want true")
	}
	if len(restored.XValidations) != 1 || restored.XValidations[0].Rule != "self.storageClass != ''" {
		t.Errorf("x-deckhouse-validations: got %+v", restored.XValidations)
	}
}

// TestOrArray_singleSchema verifies marshalling an OpenAPIV3SchemaOrArray that
// holds a single schema (object form).
func TestOrArray_singleSchema(t *testing.T) {
	val := OpenAPIV3SchemaOrArray{
		Schema: &OpenAPIV3Schema{Type: "string"},
	}
	raw, err := json.Marshal(val)
	if err != nil {
		t.Fatal(err)
	}

	var restored OpenAPIV3SchemaOrArray
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Schema == nil || restored.Schema.Type != "string" {
		t.Errorf("single schema not restored: %+v", restored)
	}
	if len(restored.JSONSchemas) != 0 {
		t.Errorf("unexpected JSONSchemas: %+v", restored.JSONSchemas)
	}
}

// TestOrArray_multiSchema verifies an OpenAPIV3SchemaOrArray that holds
// multiple schemas (array form).
func TestOrArray_multiSchema(t *testing.T) {
	val := OpenAPIV3SchemaOrArray{
		JSONSchemas: []OpenAPIV3Schema{
			{Type: "string"},
			{Type: "integer"},
		},
	}
	raw, err := json.Marshal(val)
	if err != nil {
		t.Fatal(err)
	}

	var restored OpenAPIV3SchemaOrArray
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Schema != nil {
		t.Errorf("unexpected single schema: %+v", restored.Schema)
	}
	if len(restored.JSONSchemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(restored.JSONSchemas))
	}
	if restored.JSONSchemas[0].Type != "string" || restored.JSONSchemas[1].Type != "integer" {
		t.Errorf("schema types wrong: %+v", restored.JSONSchemas)
	}
}

// TestOrBool_schema verifies marshalling AdditionalProperties/AdditionalItems
// as a schema object.
func TestOrBool_schema(t *testing.T) {
	val := OpenAPIV3SchemaOrBool{
		Allows: true,
		Schema: &OpenAPIV3Schema{Type: "string"},
	}
	raw, err := json.Marshal(val)
	if err != nil {
		t.Fatal(err)
	}

	var restored OpenAPIV3SchemaOrBool
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Schema == nil || restored.Schema.Type != "string" {
		t.Errorf("schema not restored: %+v", restored)
	}
	if !restored.Allows {
		t.Errorf("allows should be true when schema is present")
	}
}

// TestOrBool_false verifies marshalling AdditionalProperties as false.
func TestOrBool_false(t *testing.T) {
	val := OpenAPIV3SchemaOrBool{Allows: false}
	raw, err := json.Marshal(val)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "false" {
		t.Errorf("Expected false, got %s", raw)
	}

	var restored OpenAPIV3SchemaOrBool
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Schema != nil {
		t.Errorf("unexpected schema: %+v", restored)
	}
	if restored.Allows {
		t.Errorf("allows should be false")
	}
}

// TestOrBool_true verifies marshalling AdditionalProperties as true.
func TestOrBool_true(t *testing.T) {
	val := OpenAPIV3SchemaOrBool{Allows: true}
	raw, err := json.Marshal(val)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "true" {
		t.Errorf("Expected true, got %s", raw)
	}

	var restored OpenAPIV3SchemaOrBool
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Schema != nil {
		t.Errorf("unexpected schema: %+v", restored)
	}
	if !restored.Allows {
		t.Errorf("allows should be true")
	}
}

// TestOrStringArray_schema verifies marshalling Dependencies as a schema.
func TestOrStringArray_schema(t *testing.T) {
	val := OpenAPIV3SchemaOrStringArray{
		Schema: &OpenAPIV3Schema{Type: "object"},
	}
	raw, err := json.Marshal(val)
	if err != nil {
		t.Fatal(err)
	}

	var restored OpenAPIV3SchemaOrStringArray
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Schema == nil || restored.Schema.Type != "object" {
		t.Errorf("schema not restored: %+v", restored)
	}
	if len(restored.Property) != 0 {
		t.Errorf("unexpected property list")
	}
}

// TestOrStringArray_properties verifies marshalling Dependencies as a string list.
func TestOrStringArray_properties(t *testing.T) {
	val := OpenAPIV3SchemaOrStringArray{
		Property: []string{"fieldA", "fieldB"},
	}
	raw, err := json.Marshal(val)
	if err != nil {
		t.Fatal(err)
	}

	var restored OpenAPIV3SchemaOrStringArray
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Schema != nil {
		t.Errorf("unexpected schema: %+v", restored)
	}
	if len(restored.Property) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(restored.Property))
	}
	if restored.Property[0] != "fieldA" || restored.Property[1] != "fieldB" {
		t.Errorf("properties wrong: %v", restored.Property)
	}
}

// realModuleSchema returns a schema that mirrors a realistic Deckhouse Application
// package definition: nested objects, arrays with items, oneOf variant fields,
// pattern-validated strings, enums, mixed x-deckhouse extensions, and
// dependencies between fields.
func realModuleSchema() *OpenAPIV3Schema {
	return &OpenAPIV3Schema{
		Type:        "object",
		Description: "Application settings for the demo module",
		Required:    []string{"storageClass", "replicas"},
		Properties: map[string]OpenAPIV3Schema{
			"storageClass": {
				Type:        "string",
				Description: "Storage class for persistent volumes",
				XGrant:      "storageclasses",
			},
			"replicas": {
				Type:        "integer",
				Default:     jsonPtr("1"),
				Minimum:     float64Ptr(1),
				Maximum:     float64Ptr(10),
				XUIAdvanced: true,
				XValidations: []ValidationRule{
					{
						Rule:      "self >= 1 && self <= 10",
						Message:   "replicas must be between 1 and 10",
						Reason:    stringPtr("FieldValueInvalid"),
						FieldPath: ".replicas",
					},
				},
			},
			"mode": {
				Type: "string",
				Enum: []apiextensionsv1.JSON{
					{Raw: []byte(`"production"`)},
					{Raw: []byte(`"staging"`)},
					{Raw: []byte(`"development"`)},
				},
				Default: jsonPtr(`"production"`),
			},
			"ingress": {
				Type: "object",
				Properties: map[string]OpenAPIV3Schema{
					"enabled": {
						Type:    "boolean",
						Default: jsonPtr("false"),
					},
					"hosts": {
						Type: "array",
						Items: &OpenAPIV3SchemaOrArray{
							Schema: &OpenAPIV3Schema{
								Type:   "string",
								Format: "hostname",
							},
						},
					},
				},
			},
			"resources": {
				Type: "object",
				OneOf: []OpenAPIV3Schema{
					{
						Type: "object",
						Properties: map[string]OpenAPIV3Schema{
							"requests": {Type: "object"},
							"limits":   {Type: "object"},
						},
					},
					{
						Type: "null",
					},
				},
			},
			"labels": {
				Type: "object",
				AdditionalProperties: &OpenAPIV3SchemaOrBool{
					Schema: &OpenAPIV3Schema{Type: "string"},
				},
			},
			"env": {
				Type: "array",
				Items: &OpenAPIV3SchemaOrArray{
					JSONSchemas: []OpenAPIV3Schema{
						{
							Type:     "object",
							Required: []string{"name"},
							Properties: map[string]OpenAPIV3Schema{
								"name":  {Type: "string"},
								"value": {Type: "string"},
							},
						},
					},
				},
			},
			"logLevel": {
				Type:    "string",
				Pattern: "^(debug|info|warn|error)$",
				Default: jsonPtr(`"info"`),
			},
		},
		XValidations: []ValidationRule{
			{
				Rule:    "has(self.storageClass) && self.storageClass != ''",
				Message: "storageClass is required",
			},
		},
	}
}

// TestRealModuleSchema_roundtrip ensures the full complex schema survives
// JSON marshal → unmarshal.
func TestRealModuleSchema_roundtrip(t *testing.T) {
	original := realModuleSchema()

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored OpenAPIV3Schema
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.Type != "object" {
		t.Errorf("type mismatch")
	}
	if len(restored.Properties) != 8 {
		t.Errorf("expected 8 properties, got %d", len(restored.Properties))
	}

	sc, ok := restored.Properties["storageClass"]
	if !ok || sc.XGrant != "storageclasses" {
		t.Errorf("storageClass grant lost")
	}

	replicas, ok := restored.Properties["replicas"]
	if !ok || len(replicas.XValidations) != 1 || !replicas.XUIAdvanced {
		t.Errorf("replicas extensions lost")
	}

	mode, ok := restored.Properties["mode"]
	if !ok || len(mode.Enum) != 3 {
		t.Errorf("mode enum lost")
	}

	ingress, ok := restored.Properties["ingress"]
	if !ok || len(ingress.Properties) != 2 {
		t.Errorf("ingress properties lost")
	}
	hosts, ok := ingress.Properties["hosts"]
	if !ok || hosts.Items == nil || hosts.Items.Schema == nil || hosts.Items.Schema.Format != "hostname" {
		t.Errorf("ingress.hosts items lost")
	}

	resources, ok := restored.Properties["resources"]
	if !ok || len(resources.OneOf) != 2 {
		t.Errorf("resources oneOf lost")
	}

	labels, ok := restored.Properties["labels"]
	if !ok || labels.AdditionalProperties == nil || labels.AdditionalProperties.Schema == nil {
		t.Errorf("labels additionalProperties lost")
	}

	env, ok := restored.Properties["env"]
	if !ok || env.Items == nil || len(env.Items.JSONSchemas) != 1 {
		t.Errorf("env items (multi schema) lost")
	}

	if len(restored.XValidations) != 1 {
		t.Errorf("root-level x-deckhouse-validations lost")
	}
}

// TestRealModuleSchema_deepCopy verifies DeepCopy produces an independent copy.
func TestRealModuleSchema_deepCopy(t *testing.T) {
	original := realModuleSchema()
	copied := original.DeepCopy()

	if copied == original {
		t.Fatal("DeepCopy returned same pointer")
	}

	sc := copied.Properties["storageClass"]
	sc.XGrant = "mutated"
	copied.Properties["storageClass"] = sc

	if original.Properties["storageClass"].XGrant != "storageclasses" {
		t.Errorf("DeepCopy was shallow: original mutated by copy change")
	}
}

// TestDeepCopy_nilSafety verifies DeepCopy on nil receivers.
func TestDeepCopy_nilSafety(t *testing.T) {
	var s *OpenAPIV3Schema
	if s.DeepCopy() != nil {
		t.Error("DeepCopy on nil should return nil")
	}

	var or *OpenAPIV3SchemaOrArray
	if or.DeepCopy() != nil {
		t.Error("DeepCopy on nil OrArray should return nil")
	}

	var ob *OpenAPIV3SchemaOrBool
	if ob.DeepCopy() != nil {
		t.Error("DeepCopy on nil OrBool should return nil")
	}

	var osa *OpenAPIV3SchemaOrStringArray
	if osa.DeepCopy() != nil {
		t.Error("DeepCopy on nil OrStringArray should return nil")
	}

	var vr *ValidationRule
	if vr.DeepCopy() != nil {
		t.Error("DeepCopy on nil ValidationRule should return nil")
	}
}
