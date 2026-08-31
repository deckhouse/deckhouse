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

package crdenricher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnquotedMappingWarning pins the shape of value that silently stops being
// text: prose with a colon and a space in it, which YAML reads as a mapping.
func TestUnquotedMappingWarning(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		warn bool
	}{
		{
			name: "prose with a colon parses as a mapping",
			raw:  "Condition type. `Ready` is `False` while deleting (`reason: Deleting`).",
			warn: true,
		},
		{
			name: "the same prose quoted stays text",
			raw:  "\"Condition type. `Ready` is `False` while deleting (`reason: Deleting`).\"",
			warn: false,
		},
		{
			name: "a colon without a following space is not a mapping",
			raw:  "https://example.io/docs",
			warn: false,
		},
		{
			name: "a deliberate flow mapping is left alone",
			raw:  "{field: value}",
			warn: false,
		},
		{
			name: "plain prose",
			raw:  "Current status of the condition.",
			warn: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := decodeValue(tc.raw)
			if err != nil {
				t.Fatalf("decodeValue(%q): %v", tc.raw, err)
			}
			got := unquotedMappingWarning("raw:description", tc.raw, decoded)
			if tc.warn && got == "" {
				t.Errorf("expected a warning for %q, got none (decoded as %T)", tc.raw, decoded)
			}
			if !tc.warn && got != "" {
				t.Errorf("unexpected warning for %q: %s", tc.raw, got)
			}
		})
	}
}

// TestUnquotedMappingWarningReachesTheCaller runs the enricher over a fixture
// whose marker value is prose with a colon in it, and checks the warning comes
// back out: the marker is accepted, the schema node is not the string the author
// wrote, and without the warning nothing says so.
func TestUnquotedMappingWarningReachesTheCaller(t *testing.T) {
	crdDir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "crd", "quoting.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(crdDir, "quoting.yaml"), src, 0o644); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}

	_, warnings, err := RunWithWarnings(Options{Paths: []string{testFixturePaths}, CRDDir: crdDir})
	if err != nil {
		t.Fatalf("RunWithWarnings: %v", err)
	}

	var found string
	for _, w := range warnings {
		if strings.Contains(w, "parsed as a mapping") {
			found = w
			break
		}
	}
	if found == "" {
		t.Fatalf("no warning about the unquoted value, got %v", warnings)
	}
	if !strings.Contains(found, "raw:description") {
		t.Errorf("warning does not name the marker: %s", found)
	}
}
