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
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var updateSchema = flag.Bool("update", false, "rewrite the checked-in corpus schema from the Go structs")

// jekyllSchemaPath is the copy the Jekyll site embeds into its own corpus
// (`site.data['corpus_schema']`). It is the only reason a second copy of the
// schema exists at all — the two Jekyll builds cannot reach into this Go tree —
// so this test keeps it byte-for-meaning equal to what these structs generate.
const jekyllSchemaPath = "../../../../../documentation/_data/corpus_schema.json"

func TestCorpusSchema(t *testing.T) {
	generated := buildCorpusSchema()

	if *updateSchema {
		pretty, err := json.MarshalIndent(generated, "", "  ")
		if err != nil {
			t.Fatalf("marshal schema: %v", err)
		}

		if err := os.WriteFile(filepath.Clean(jekyllSchemaPath), append(pretty, '\n'), 0o644); err != nil {
			t.Fatalf("write %s: %v", jekyllSchemaPath, err)
		}

		t.Logf("wrote %s", jekyllSchemaPath)

		return
	}

	raw, err := os.ReadFile(filepath.Clean(jekyllSchemaPath))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Skipf("Jekyll schema copy %s is absent; a full checkout is needed. Run with -update to create it.", jekyllSchemaPath)
		}

		t.Fatalf("read %s: %v", jekyllSchemaPath, err)
	}

	var committed any
	if err := json.Unmarshal(raw, &committed); err != nil {
		t.Fatalf("parse %s: %v", jekyllSchemaPath, err)
	}

	// Compare as decoded JSON, so the formatting of the checked-in file (indented
	// for review) does not matter against the compact form the exporter embeds.
	roundtrip, err := json.Marshal(generated)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}

	var current any
	if err := json.Unmarshal(roundtrip, &current); err != nil {
		t.Fatalf("re-parse generated schema: %v", err)
	}

	if !reflect.DeepEqual(current, committed) {
		t.Fatalf("corpus schema drifted from the Go structs; regenerate the Jekyll copy with:\n"+
			"  go test ./internal/aiexport -run TestCorpusSchema -update\n(%s)", jekyllSchemaPath)
	}
}

// assertCorpusMatchesSchema checks that the corpus embeds the generated schema
// and that its contents validate against it. It implements only the slice of
// JSON Schema the generator emits — types, object properties/required, closed
// objects and typed arrays — which is enough to catch a field that drifted out
// of the omitempty contract shared with the Jekyll generator.
func assertCorpusMatchesSchema(t *testing.T, corpusJSON string) {
	t.Helper()

	var value any
	if err := json.Unmarshal([]byte(corpusJSON), &value); err != nil {
		t.Fatalf("parse corpus: %v", err)
	}

	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("corpus is not an object")
	}

	// The embedded schema must be the one the structs generate.
	embedded, err := json.Marshal(object["schema"])
	if err != nil {
		t.Fatalf("re-marshal embedded schema: %v", err)
	}
	generated, err := json.Marshal(buildCorpusSchema())
	if err != nil {
		t.Fatalf("marshal generated schema: %v", err)
	}
	if !equalJSON(t, embedded, generated) {
		t.Fatalf("embedded schema differs from the generated one")
	}

	if err := validateValue(value, buildCorpusSchema(), "$"); err != nil {
		t.Fatalf("corpus does not validate against its schema: %v", err)
	}
}

func equalJSON(t *testing.T, a, b []byte) bool {
	t.Helper()

	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("parse a: %v", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("parse b: %v", err)
	}

	return reflect.DeepEqual(av, bv)
}

func validateValue(value any, schema map[string]any, path string) error {
	switch schema["type"] {
	case "object":
		return validateObject(value, schema, path)
	case "array":
		items, _ := value.([]any)
		if value != nil && !isSlice(value) {
			return fmt.Errorf("%s: want array, got %T", path, value)
		}
		itemSchema, _ := schema["items"].(map[string]any)
		for i, item := range items {
			if err := validateValue(item, itemSchema, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: want string, got %T", path, value)
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("%s: want number, got %T", path, value)
		}
	case "integer":
		f, ok := value.(float64)
		if !ok || f != math.Trunc(f) {
			return fmt.Errorf("%s: want integer, got %v", path, value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: want boolean, got %T", path, value)
		}
	}

	return nil
}

func validateObject(value any, schema map[string]any, path string) error {
	object, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: want object, got %T", path, value)
	}

	properties, _ := schema["properties"].(map[string]any)

	if required, ok := schema["required"].([]string); ok {
		for _, name := range required {
			if _, present := object[name]; !present {
				return fmt.Errorf("%s: missing required %q", path, name)
			}
		}
	}

	// additionalProperties defaults to true (JSON Schema): only a present,
	// explicit `false` closes the object. `{"type":"object"}` — the embedded
	// schema field — has none, so it accepts any content.
	if ap, present := schema["additionalProperties"]; present {
		if closed, _ := ap.(bool); !closed {
			keys := make([]string, 0, len(object))
			for key := range object {
				if _, allowed := properties[key]; !allowed {
					keys = append(keys, key)
				}
			}
			if len(keys) > 0 {
				sort.Strings(keys)
				return fmt.Errorf("%s: unexpected properties %s", path, strings.Join(keys, ", "))
			}
		}
	}

	for key, sub := range object {
		propSchema, ok := properties[key].(map[string]any)
		if !ok {
			continue
		}
		if err := validateValue(sub, propSchema, path+"."+key); err != nil {
			return err
		}
	}

	return nil
}

func isSlice(value any) bool {
	_, ok := value.([]any)
	return ok
}
