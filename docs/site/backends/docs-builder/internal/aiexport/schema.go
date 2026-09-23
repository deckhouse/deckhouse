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

package aiexport

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
)

// The corpus embeds its own JSON Schema (see Corpus.Schema) so the file
// describes itself. The schema is derived here from the Go structs — the same
// structs that produce the data — so the two cannot drift: change a field and
// schema_test.go fails until the copy the Jekyll site embeds is regenerated.
const schemaDialect = "https://json-schema.org/draft/2020-12/schema"

var rawMessageType = reflect.TypeOf(json.RawMessage(nil))

// buildCorpusSchema returns the JSON Schema of a Corpus as a plain map, so that
// json.Marshal emits it deterministically (map keys are sorted).
func buildCorpusSchema() map[string]any {
	schema := schemaForType(reflect.TypeOf(Corpus{}))
	schema["$schema"] = schemaDialect
	schema["title"] = "Deckhouse documentation AI corpus"

	return schema
}

// corpusSchemaJSON is the compact schema embedded into every corpus.
func corpusSchemaJSON() (json.RawMessage, error) {
	return json.Marshal(buildCorpusSchema())
}

func schemaForType(t reflect.Type) map[string]any {
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice:
		// The embedded schema is an opaque object: describing it in full would
		// only recurse into the JSON Schema meta-schema.
		if t == rawMessageType {
			return map[string]any{"type": "object"}
		}

		return map[string]any{"type": "array", "items": schemaForType(t.Elem())}
	case reflect.Struct:
		return schemaForStruct(t)
	default:
		return map[string]any{}
	}
}

func schemaForStruct(t reflect.Type) map[string]any {
	properties := map[string]any{}
	required := make([]string, 0, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		name, optional, ok := jsonField(field)
		if !ok {
			continue
		}

		property := schemaForType(field.Type)
		if desc := field.Tag.Get("desc"); desc != "" {
			property["description"] = desc
		}
		properties[name] = property

		if !optional {
			required = append(required, name)
		}
	}

	sort.Strings(required)

	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// jsonField returns the JSON name of a struct field, whether it is optional
// (`omitempty`), and whether it is serialized at all.
func jsonField(field reflect.StructField) (string, bool, bool) {
	if field.PkgPath != "" {
		return "", false, false // unexported
	}

	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, false
	}

	parts := strings.Split(tag, ",")

	name := parts[0]
	if name == "" {
		name = field.Name
	}

	optional := false
	for _, option := range parts[1:] {
		if option == "omitempty" {
			optional = true
		}
	}

	return name, optional, true
}
