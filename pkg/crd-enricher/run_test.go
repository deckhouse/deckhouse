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
	goyaml "sigs.k8s.io/yaml/goyaml.v3"
)

const testFixturePaths = "./testdata/api/v1alpha1"

// runOnFixture copies the CRD fixture into a fresh temp directory (Run rewrites
// files in place) and runs the enricher over it with the given options, then
// returns the enriched file bytes.
func runOnFixture(t *testing.T, generateExamples bool) []byte {
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
	return out
}

// fixtureSchema parses the enriched fixture and returns the openAPIV3Schema of
// the single version.
func fixtureSchema(t *testing.T, out []byte) map[string]any {
	t.Helper()
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
	root := fixtureSchema(t, runOnFixture(t, false))

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
	root := fixtureSchema(t, runOnFixture(t, true))

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

// TestRunExamplesPreserveKeyOrder asserts that an object example keeps its
// authored key order ("repo" before "dockerCfg") in the rendered YAML, even
// though the schema properties for the same object are sorted alphabetically.
func TestRunExamplesPreserveKeyOrder(t *testing.T) {
	out := runOnFixture(t, false)

	var doc goyaml.Node
	if err := goyaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("parse enriched fixture as nodes: %v", err)
	}

	orders := exampleKeyOrders(&doc)
	found := false
	for _, keys := range orders {
		if len(keys) == 2 && keys[0] == "repo" && keys[1] == "dockerCfg" {
			found = true
		}
	}
	if !found {
		t.Errorf("no x-doc-examples object rendered with authored order [repo dockerCfg]; got orders %v", orders)
	}

	// Guard the premise: the schema property node for the same object is sorted,
	// so the example order is genuinely being preserved against the default.
	if props := findMappingKeys(&doc, "registry", "properties"); props != nil {
		if len(props) == 2 && (props[0] != "dockerCfg" || props[1] != "repo") {
			t.Errorf("registry properties expected sorted [dockerCfg repo], got %v", props)
		}
	}
}

// exampleKeyOrders walks the node tree and returns the key order of every
// mapping that is the first element of an "x-doc-examples" sequence.
func exampleKeyOrders(n *goyaml.Node) [][]string {
	var out [][]string
	var walk func(*goyaml.Node)
	walk = func(node *goyaml.Node) {
		switch node.Kind {
		case goyaml.MappingNode:
			for i := 0; i+1 < len(node.Content); i += 2 {
				key, val := node.Content[i], node.Content[i+1]
				if key.Value == "x-doc-examples" && val.Kind == goyaml.SequenceNode &&
					len(val.Content) > 0 && val.Content[0].Kind == goyaml.MappingNode {
					out = append(out, mappingKeys(val.Content[0]))
				}
				walk(val)
			}
		default:
			for _, c := range node.Content {
				walk(c)
			}
		}
	}
	walk(n)
	return out
}

// findMappingKeys walks to the mapping stored under parentKey.childKey and
// returns its keys in order, or nil when the path is absent.
func findMappingKeys(n *goyaml.Node, parentKey, childKey string) []string {
	var found []string
	var walk func(*goyaml.Node)
	walk = func(node *goyaml.Node) {
		if node.Kind == goyaml.MappingNode {
			for i := 0; i+1 < len(node.Content); i += 2 {
				if node.Content[i].Value == parentKey {
					if child := childMappingNode(node.Content[i+1], childKey); child != nil {
						found = mappingKeys(child)
					}
				}
			}
		}
		for _, c := range node.Content {
			walk(c)
		}
	}
	walk(n)
	return found
}

// childMappingNode returns the mapping stored under key within a mapping node.
func childMappingNode(node *goyaml.Node, key string) *goyaml.Node {
	if node.Kind != goyaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key && node.Content[i+1].Kind == goyaml.MappingNode {
			return node.Content[i+1]
		}
	}
	return nil
}

// mappingKeys returns the keys of a mapping node in order.
func mappingKeys(node *goyaml.Node) []string {
	keys := make([]string, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		keys = append(keys, node.Content[i].Value)
	}
	return keys
}
