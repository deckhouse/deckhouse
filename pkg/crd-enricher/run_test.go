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
	"testing"

	"sigs.k8s.io/yaml"
)

const testFixturePaths = "./testdata/api/v1alpha1"

// runOnFixture copies the CRD fixture into a fresh temp directory (Run rewrites
// files in place) and runs the enricher over it with the given options, then
// returns the openAPIV3Schema of the single version.
func runOnFixture(t *testing.T, generateExamples bool) map[string]any {
	t.Helper()

	crdDir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "crd", "foo.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	dst := filepath.Join(crdDir, "foo.yaml")
	if err := os.WriteFile(dst, src, 0o644); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}

	if _, err := Run(Options{
		Paths:            []string{testFixturePaths},
		CRDDir:           crdDir,
		GenerateExamples: generateExamples,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read enriched fixture: %v", err)
	}
	var crd map[string]any
	if err := yaml.Unmarshal(out, &crd); err != nil {
		t.Fatalf("parse enriched fixture: %v", err)
	}

	version := childMap(crd, "spec")["versions"].([]any)[0].(map[string]any)
	return childMap(childMap(version, "schema"), "openAPIV3Schema")
}

// TestRunExamplesDisabledByDefault asserts that without the flag the root gets no
// synthesized example, while an explicit examples marker is still applied.
func TestRunExamplesDisabledByDefault(t *testing.T) {
	root := runOnFixture(t, false)

	if _, ok := root["x-doc-examples"]; ok {
		t.Errorf("root x-doc-examples must not be synthesized by default: %#v", root["x-doc-examples"])
	}

	channel := childMap(childMap(childMap(root, "properties"), "spec"), "properties")["channel"].(map[string]any)
	ex, ok := channel["x-doc-examples"].([]any)
	if !ok || len(ex) != 1 || ex[0] != "stable" {
		t.Errorf("explicit examples marker must survive with the flag off: %#v", channel["x-doc-examples"])
	}
}

// TestRunExamplesEnabled asserts that with the flag the root receives a
// synthesized example aggregating the spec fields.
func TestRunExamplesEnabled(t *testing.T) {
	root := runOnFixture(t, true)

	examples, ok := root["x-doc-examples"].([]any)
	if !ok || len(examples) != 1 {
		t.Fatalf("root x-doc-examples = %#v, want one synthesized example", root["x-doc-examples"])
	}
	got := examples[0].(map[string]any)

	if got["apiVersion"] != "example.io/v1alpha1" {
		t.Errorf("apiVersion = %#v", got["apiVersion"])
	}
	if got["kind"] != "Foo" {
		t.Errorf("kind = %#v", got["kind"])
	}
	spec, _ := got["spec"].(map[string]any)
	if spec["name"] != exampleStringValue {
		t.Errorf("spec.name example = %#v, want %q", spec["name"], exampleStringValue)
	}
	if spec["channel"] != "stable" {
		t.Errorf("spec.channel example = %#v, want the explicit marker value", spec["channel"])
	}
}
