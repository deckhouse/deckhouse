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

package validate

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMergeWithDefaults_DeclaredNextToAdditionalProperties pins that the declared properties get
// their defaults and keep the project values whatever additionalProperties says, and that
// additionalProperties only decides whether the project's undeclared keys are kept.
func TestMergeWithDefaults_DeclaredNextToAdditionalProperties(t *testing.T) {
	t.Parallel()

	declared := map[string]any{
		"withDefault":  map[string]any{"type": "boolean", "default": true},
		"setByProject": map[string]any{"type": "string", "default": "from-schema"},
	}
	project := map[string]any{"setByProject": "from-project", "free": "value"}

	tests := []struct {
		name     string
		schema   map[string]any
		params   map[string]any
		expected map[string]any
	}{
		{
			name:     "no additionalProperties",
			schema:   map[string]any{"type": "object", "properties": declared},
			params:   project,
			expected: map[string]any{"withDefault": true, "setByProject": "from-project"},
		},
		{
			name:     "additionalProperties true",
			schema:   map[string]any{"type": "object", "properties": declared, "additionalProperties": true},
			params:   project,
			expected: map[string]any{"withDefault": true, "setByProject": "from-project", "free": "value"},
		},
		{
			name: "additionalProperties schema",
			schema: map[string]any{
				"type":                 "object",
				"properties":           declared,
				"additionalProperties": map[string]any{"type": "string"},
			},
			params:   project,
			expected: map[string]any{"withDefault": true, "setByProject": "from-project", "free": "value"},
		},
		{
			name:     "additionalProperties false",
			schema:   map[string]any{"type": "object", "properties": declared, "additionalProperties": false},
			params:   project,
			expected: map[string]any{"withDefault": true, "setByProject": "from-project"},
		},
		{
			name: "free-form map without declared properties",
			schema: map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
			},
			params:   map[string]any{"team": "a"},
			expected: map[string]any{"team": "a"},
		},
		{
			name: "nested object with additionalProperties",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"nested": map[string]any{
						"type":                 "object",
						"properties":           declared,
						"additionalProperties": true,
					},
				},
			},
			params: map[string]any{"nested": project},
			expected: map[string]any{
				"nested": map[string]any{"withDefault": true, "setByProject": "from-project", "free": "value"},
			},
		},
		{
			name: "declared object next to additionalProperties",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"nested": map[string]any{"type": "object", "properties": declared},
				},
				"additionalProperties": true,
			},
			params: map[string]any{"nested": map[string]any{"setByProject": "from-project"}, "free": "value"},
			expected: map[string]any{
				"nested": map[string]any{"withDefault": true, "setByProject": "from-project"},
				"free":   "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			schema, err := LoadSchema(tt.schema)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, MergeDefaults(schema, tt.params))
		})
	}
}

// TestParamPath_DeclaredNextToAdditionalProperties pins that a fromParam reference to a declared
// parameter is type-checked whatever additionalProperties says, on the root and in a nested object,
// and that additionalProperties and x-kubernetes-preserve-unknown-fields only decide whether a key
// the schema does not declare is accepted. Every reference is bound to a boolean field.
func TestParamPath_DeclaredNextToAdditionalProperties(t *testing.T) {
	t.Parallel()

	declared := map[string]any{
		"strParam":  map[string]any{"type": "string"},
		"boolParam": map[string]any{"type": "boolean"},
	}
	allowAll := map[string]any{"additionalProperties": true}
	allowStrings := map[string]any{"additionalProperties": map[string]any{"type": "string"}}
	allowNone := map[string]any{"additionalProperties": false}
	preserveUnknown := map[string]any{"x-kubernetes-preserve-unknown-fields": true}

	// object returns an object schema with the given properties and the extra keywords.
	object := func(properties, keywords map[string]any) map[string]any {
		schema := map[string]any{"type": "object", "properties": properties}
		maps.Copy(schema, keywords)
		return schema
	}
	nested := func(keywords map[string]any) map[string]any {
		return object(map[string]any{"nested": object(declared, keywords)}, nil)
	}

	const wrongType = "of type 'string', but the field requires type 'boolean'"

	tests := []struct {
		name        string
		schema      map[string]any
		path        string
		expectedErr string
	}{
		{
			name:        "declared wrong type without additionalProperties",
			schema:      object(declared, nil),
			path:        "strParam",
			expectedErr: wrongType,
		},
		{
			name:        "declared wrong type next to additionalProperties true",
			schema:      object(declared, allowAll),
			path:        "strParam",
			expectedErr: wrongType,
		},
		{
			name:        "declared wrong type next to an additionalProperties schema",
			schema:      object(declared, allowStrings),
			path:        "strParam",
			expectedErr: wrongType,
		},
		{
			name:        "declared wrong type next to x-kubernetes-preserve-unknown-fields",
			schema:      object(declared, preserveUnknown),
			path:        "strParam",
			expectedErr: wrongType,
		},
		{
			name:   "declared right type without additionalProperties",
			schema: object(declared, nil),
			path:   "boolParam",
		},
		{
			name:   "declared right type next to additionalProperties true",
			schema: object(declared, allowAll),
			path:   "boolParam",
		},
		{
			name:        "undeclared key without additionalProperties",
			schema:      object(declared, nil),
			path:        "free",
			expectedErr: "'free' is not declared in spec.parametersSchema.properties",
		},
		{
			name:        "undeclared key next to additionalProperties false",
			schema:      object(declared, allowNone),
			path:        "free",
			expectedErr: "'free' is not declared in spec.parametersSchema.properties",
		},
		{
			name:   "undeclared key next to additionalProperties true",
			schema: object(declared, allowAll),
			path:   "free",
		},
		{
			name:   "undeclared key next to an additionalProperties schema",
			schema: object(declared, allowStrings),
			path:   "free",
		},
		{
			name:   "undeclared key next to x-kubernetes-preserve-unknown-fields",
			schema: object(declared, preserveUnknown),
			path:   "free",
		},
		{
			name:   "path under an undeclared key next to additionalProperties true",
			schema: object(declared, allowAll),
			path:   "free.deeper",
		},
		{
			name:        "declared nested wrong type without additionalProperties",
			schema:      nested(nil),
			path:        "nested.strParam",
			expectedErr: wrongType,
		},
		{
			name:        "declared nested wrong type in an object with additionalProperties true",
			schema:      nested(allowAll),
			path:        "nested.strParam",
			expectedErr: wrongType,
		},
		{
			name:        "declared nested wrong type under a root with additionalProperties true",
			schema:      object(map[string]any{"nested": object(declared, nil)}, allowAll),
			path:        "nested.strParam",
			expectedErr: wrongType,
		},
		{
			name:   "declared nested right type in an object with additionalProperties true",
			schema: nested(allowAll),
			path:   "nested.boolParam",
		},
		{
			name:        "undeclared nested key without additionalProperties",
			schema:      nested(nil),
			path:        "nested.free",
			expectedErr: "'free' is not declared in property 'nested'",
		},
		{
			name:   "undeclared nested key in an object with additionalProperties true",
			schema: nested(allowAll),
			path:   "nested.free",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			schema, err := LoadSchema(tt.schema)
			require.NoError(t, err)

			err = ParamPath(schema, tt.path, "boolean")
			if tt.expectedErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.expectedErr)
		})
	}
}
