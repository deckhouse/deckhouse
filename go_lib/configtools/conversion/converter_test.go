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

package conversion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConvertToLatest(t *testing.T) {
	t.Run("should convert from 1 to latest version", func(t *testing.T) {
		err := TestConvert(
			`
simpleField: simpleValue
objectField:
  arrayFieldInObjectValue:
    - value1
    - value2
    - value3
  mapFieldInObjectValue:
    key1: val1
    key2: val2
  simpleFieldInObjectValue: 2
arrayField:
  - val1
  - val2
  - val3
mapField:
  simpleFieldInObjectValue: 2
  arrayFieldInMapValue:
    - v1
    - v2
    - v3
  mapFieldInObjectValue:
    k1: 1
    k2: 2
objectFirst:
  check: true
objectForDelete:
  key: val
`,
			`
simpleField: value
objectField:
  arrayFieldInObjectValue:
    - value1
    - value2
    - value3
  mapFieldInObjectValue:
    key1: val2
  simpleFieldInObjectValue: 2
mapField:
  arrayFieldInMapValue:
    - v1
    - v2
    - v3
    - v4
  mapFieldInObjectValue:
    k1: 1
    k2: 2
    k3: 3
newField: value
objectFirst:
  objectSecond:
    objectThird:
      check: true
arrayField:
  - v1
  - v2
  - v3
`,
			"testdata",
			1,
			0)
		if err != nil {
			t.Error(err)
		}
	})
}

func TestNewConverter(t *testing.T) {
	t.Run("should convert along an in-memory chain", func(t *testing.T) {
		converter, err := NewConverter(map[int][]string{
			2: {".b = .a", "del(.a)"},
			3: {".c = .b", "del(.b)"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if converter.LatestVersion() != 3 {
			t.Fatalf("expected latest 3, got %d", converter.LatestVersion())
		}
		version, converted, err := converter.ConvertToLatest(1, map[string]interface{}{"a": "value"})
		if err != nil {
			t.Fatal(err)
		}
		if version != 3 || len(converted) != 1 || converted["c"] != "value" {
			t.Fatalf("expected {c: value} at 3, got %v at %d", converted, version)
		}
	})

	t.Run("should have latest 1 without conversions", func(t *testing.T) {
		converter, err := NewConverter(nil)
		if err != nil {
			t.Fatal(err)
		}
		if converter.LatestVersion() != 1 {
			t.Fatalf("expected latest 1, got %d", converter.LatestVersion())
		}
	})

	t.Run("should reject a gap in the chain", func(t *testing.T) {
		if _, err := NewConverter(map[int][]string{2: {"."}, 4: {"."}}); err == nil {
			t.Fatal("expected an error for the missing version 3")
		}
	})

	t.Run("should reject a version below 2", func(t *testing.T) {
		if _, err := NewConverter(map[int][]string{1: {"."}}); err == nil {
			t.Fatal("expected an error for version 1")
		}
	})

	t.Run("should reject an invalid expression", func(t *testing.T) {
		if _, err := NewConverter(map[int][]string{2: {".a = "}}); err == nil {
			t.Fatal("expected an error for the invalid expression")
		}
	})
}

func TestNewConverterFromDirVersionFiles(t *testing.T) {
	for _, filename := range []string{"v2.yaml", "v2.yml"} {
		t.Run(filename, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				filename: `version: 2
conversions:
  - .newField = .oldField | del(.oldField)
`,
				"testcases.yaml": "testcases: []\n",
				"README.yaml":    "not: [valid YAML",
			}
			for name, content := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(filepath.Join(dir, "v3.yaml"), 0755); err != nil {
				t.Fatal(err)
			}

			converter, err := NewConverterFromDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if converter.IsKnownVersion(0) {
				t.Fatal("testcases.yaml was loaded as a conversion")
			}
			version, settings, err := converter.ConvertToLatest(1, map[string]interface{}{"oldField": "value"})
			if err != nil {
				t.Fatal(err)
			}
			if version != 2 || len(settings) != 1 || settings["newField"] != "value" {
				t.Fatalf("expected {newField: value} at version 2, got %v at version %d", settings, version)
			}
		})
	}
}
