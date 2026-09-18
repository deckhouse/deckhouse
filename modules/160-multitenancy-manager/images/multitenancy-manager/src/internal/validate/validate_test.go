/*
Copyright 2024 Flant JSC

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
	"testing"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
)

func TestMergeWithDefaults_AdditionalProperties(t *testing.T) {
	schema := &spec.Schema{}
	schema.Properties = map[string]spec.Schema{
		"declared": {SchemaProps: spec.SchemaProps{Default: "from-schema"}},
	}
	schema.AdditionalProperties = &spec.SchemaOrBool{Allows: true}

	out := MergeDefaults(schema, map[string]any{"free": "value", "declared": "kept"})

	// only the non-property key survives; the declared property and its schema default are dropped.
	assert.Equal(t, map[string]any{"free": "value"}, out)
}

// TestMergeWithDefaults_ObjectDefaultPreserved pins the behaviour the schema-based ProjectTemplate
// render relies on: an object-typed property whose default is the whole object is kept verbatim (the
// merge only recurses into sub-properties when there is no default). Structured templates carry their
// fixed values (namespaceMetadata, dedicatedNodes, allowedUIDs) as such object defaults.
func TestMergeWithDefaults_ObjectDefaultPreserved(t *testing.T) {
	schema := &spec.Schema{}
	schema.Properties = map[string]spec.Schema{
		"namespace": {SchemaProps: spec.SchemaProps{
			Type:    spec.StringOrArray{"object"},
			Default: map[string]any{"labels": map[string]any{"team": "x"}},
			Properties: map[string]spec.Schema{
				"labels": {SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"object"}}},
			},
		}},
	}

	out := MergeDefaults(schema, map[string]any{})
	assert.Equal(t, map[string]any{"labels": map[string]any{"team": "x"}}, out["namespace"])
}
