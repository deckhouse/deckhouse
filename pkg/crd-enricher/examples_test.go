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
	"reflect"
	"testing"
)

func TestLeafExample(t *testing.T) {
	cases := []struct {
		name string
		node map[string]any
		want any
		ok   bool
	}{
		{"default wins", map[string]any{"type": "string", "default": "HTTPS", "enum": []any{"HTTP", "HTTPS"}}, "HTTPS", true},
		{"x-doc-default", map[string]any{"type": "string", "x-doc-default": "3m"}, "3m", true},
		{"enum first", map[string]any{"type": "string", "enum": []any{"Active", "Terminating"}}, "Active", true},
		{"string placeholder", map[string]any{"type": "string"}, exampleStringValue, true},
		{"date-time", map[string]any{"type": "string", "format": "date-time"}, exampleDateTimeValue, true},
		{"integer", map[string]any{"type": "integer"}, 0, true},
		{"number", map[string]any{"type": "number"}, 0, true},
		{"boolean", map[string]any{"type": "boolean"}, false, true},
		{"free-form", map[string]any{}, nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := leafExample(tc.node)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("value = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestComputeExampleComposite(t *testing.T) {
	e := &Enricher{}

	node := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"repo":   map[string]any{"type": "string", "x-doc-examples": []any{"registry.example.io/x"}},
			"scheme": map[string]any{"type": "string", "default": "HTTPS"},
			"tags": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"labels": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
			},
		},
	}

	got, ok := e.computeExample(node)
	if !ok {
		t.Fatal("expected an example")
	}
	want := map[string]any{
		"repo":   "registry.example.io/x",
		"scheme": "HTTPS",
		"tags":   []any{exampleStringValue},
		"labels": map[string]any{examplePlaceholderKey: exampleStringValue},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("example = %#v, want %#v", got, want)
	}
}

func TestComputeExampleExplicitWins(t *testing.T) {
	e := &Enricher{}
	node := map[string]any{
		"type":           "object",
		"x-doc-examples": []any{map[string]any{"hand": "written"}},
		"properties": map[string]any{
			"ignored": map[string]any{"type": "string"},
		},
	}
	got, _ := e.computeExample(node)
	if !reflect.DeepEqual(got, map[string]any{"hand": "written"}) {
		t.Fatalf("explicit example not preferred: %#v", got)
	}
}

func TestGenerateExamplesRoot(t *testing.T) {
	e := &Enricher{}
	spec := map[string]any{"group": "deckhouse.io"}
	names := map[string]any{"kind": "ModuleSource"}
	root := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"apiVersion": map[string]any{"type": "string"},
			"kind":       map[string]any{"type": "string"},
			"metadata":   map[string]any{"type": "object"},
			"spec": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repo":   map[string]any{"type": "string", "x-doc-examples": []any{"registry.example.io/x"}},
					"scheme": map[string]any{"type": "string", "default": "HTTPS"},
				},
			},
			"status": map[string]any{
				"type":       "object",
				"properties": map[string]any{"phase": map[string]any{"type": "string"}},
			},
		},
	}

	e.generateExamples(spec, names, "v1alpha1", root)

	examples, ok := root["x-doc-examples"].([]any)
	if !ok || len(examples) != 1 {
		t.Fatalf("root x-doc-examples = %#v", root["x-doc-examples"])
	}
	got := examples[0].(map[string]any)

	if got["apiVersion"] != "deckhouse.io/v1alpha1" {
		t.Errorf("apiVersion = %#v", got["apiVersion"])
	}
	if got["kind"] != "ModuleSource" {
		t.Errorf("kind = %#v", got["kind"])
	}
	if md, _ := got["metadata"].(map[string]any); md["name"] != exampleMetadataName {
		t.Errorf("metadata = %#v", got["metadata"])
	}
	specEx, _ := got["spec"].(map[string]any)
	if specEx["repo"] != "registry.example.io/x" || specEx["scheme"] != "HTTPS" {
		t.Errorf("spec example = %#v", specEx)
	}
	if _, ok := got["status"]; ok {
		t.Errorf("status must be omitted from the root example: %#v", got)
	}
}

func TestGenerateExamplesExplicitRootWins(t *testing.T) {
	e := &Enricher{}
	spec := map[string]any{"group": "deckhouse.io"}
	names := map[string]any{"kind": "Foo"}
	hand := []any{map[string]any{"apiVersion": "deckhouse.io/v1", "kind": "Foo"}}
	root := map[string]any{
		"type":           "object",
		"x-doc-examples": hand,
		"properties": map[string]any{
			"spec": map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "string"}}},
		},
	}

	e.generateExamples(spec, names, "v1", root)

	if !reflect.DeepEqual(root["x-doc-examples"], hand) {
		t.Fatalf("explicit root example overwritten: %#v", root["x-doc-examples"])
	}
}

func TestGenerateExamplesTreeScope(t *testing.T) {
	e := &Enricher{exampleScope: exampleScopeTree}
	spec := map[string]any{"group": "deckhouse.io"}
	names := map[string]any{"kind": "Foo"}
	registry := map[string]any{
		"type":       "object",
		"properties": map[string]any{"repo": map[string]any{"type": "string"}},
	}
	specNode := map[string]any{
		"type":       "object",
		"properties": map[string]any{"registry": registry},
	}
	root := map[string]any{
		"type":       "object",
		"properties": map[string]any{"spec": specNode},
	}

	e.generateExamples(spec, names, "v1", root)

	if _, ok := registry["x-doc-examples"]; !ok {
		t.Error("nested registry node did not receive a composite example")
	}
	if _, ok := specNode["x-doc-examples"]; !ok {
		t.Error("nested spec node did not receive a composite example")
	}
	if _, ok := root["x-doc-examples"]; !ok {
		t.Error("root did not receive a synthesized example")
	}
}
