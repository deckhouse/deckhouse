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

func TestParseMarkerLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want marker
		ok   bool
	}{
		{"plain comment", "ScanInterval holds the value", marker{}, false},
		{"kubebuilder", "+kubebuilder:object:root=true", marker{name: "kubebuilder:object:root", rawValue: "true", hasValue: true}, true},
		{"legacy form ignored", "+x-doc-default=3m", marker{name: "x-doc-default", rawValue: "3m", hasValue: true}, true},
		{"flag", "+crd-enricher:deckhouse:documentation:deprecated", marker{name: "deprecated", enricher: true}, true},
		{"scalar", "+crd-enricher:deckhouse:documentation:default=3m", marker{name: "default", rawValue: "3m", hasValue: true, enricher: true}, true},
		{"empty value", "+crd-enricher:deckhouse:documentation:default=", marker{name: "default", rawValue: "", hasValue: true, enricher: true}, true},
		{"value with equals", "+crd-enricher:raw:pattern=a=b", marker{name: "raw:pattern", rawValue: "a=b", hasValue: true, enricher: true}, true},
		{"whitespace", "  +crd-enricher:deckhouse:documentation:default = 3m  ", marker{name: "default", rawValue: "3m", hasValue: true, enricher: true}, true},
		{"examples", "+crd-enricher:deckhouse:documentation:examples=5m", marker{name: "examples", rawValue: "5m", hasValue: true, enricher: true}, true},
		{"crd subkey", "+crd-enricher:deckhouse:crd:minimal=true", marker{name: "crd:minimal", rawValue: "true", hasValue: true, enricher: true}, true},
		{"crd subkey flag", "+crd-enricher:deckhouse:crd:minimal", marker{name: "crd:minimal", enricher: true}, true},
		{"raw", "+crd-enricher:raw:pattern=^a$", marker{name: "raw:pattern", rawValue: "^a$", hasValue: true, enricher: true}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseMarkerLine(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Fatalf("marker = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestParseJSONTag(t *testing.T) {
	cases := []struct {
		tag    string
		name   string
		inline bool
		skip   bool
	}{
		{`json:"version,omitempty"`, "version", false, false},
		{`json:"registry"`, "registry", false, false},
		{`json:",inline"`, "", true, false},
		{`json:"-"`, "", false, true},
		{`json:"-,omitempty"`, "", false, false},
		{``, "", false, false},
	}

	for _, tc := range cases {
		name, inline, skip := parseJSONTag(tc.tag)
		if name != tc.name || inline != tc.inline || skip != tc.skip {
			t.Errorf("parseJSONTag(%q) = (%q,%v,%v), want (%q,%v,%v)",
				tc.tag, name, inline, skip, tc.name, tc.inline, tc.skip)
		}
	}
}

func TestApplyMarkersScalarAndFlag(t *testing.T) {
	e := &Enricher{}
	schema := map[string]any{"type": "string"}

	e.applyMarkers(schema, []marker{
		{name: "default", rawValue: "3m", hasValue: true, enricher: true},
		{name: "deprecated", enricher: true},
		{name: "kubebuilder:validation:Required"}, // not an enricher marker, ignored
	})

	if got := schema["x-doc-default"]; got != "3m" {
		t.Errorf("x-doc-default = %#v, want %q", got, "3m")
	}
	if got := schema["x-doc-deprecated"]; got != true {
		t.Errorf("x-doc-deprecated = %#v, want true", got)
	}
	if _, ok := schema["kubebuilder:validation:Required"]; ok {
		t.Errorf("non-enricher marker leaked into schema")
	}
}

func TestApplyMarkersExamplesAccumulate(t *testing.T) {
	e := &Enricher{}
	schema := map[string]any{"type": "string"}

	e.applyMarkers(schema, []marker{
		{name: "examples", rawValue: "5m", hasValue: true, enricher: true},
		{name: "examples", rawValue: "1h", hasValue: true, enricher: true},
		{name: "examples", rawValue: "[10m, 20m]", hasValue: true, enricher: true},
	})

	want := []any{"5m", "1h", "10m", "20m"}
	if got := schema["x-doc-examples"]; !reflect.DeepEqual(got, want) {
		t.Errorf("x-doc-examples = %#v, want %#v", got, want)
	}
}

func TestApplyMarkersExampleObject(t *testing.T) {
	e := &Enricher{}
	schema := map[string]any{"type": "object"}

	e.applyMarkers(schema, []marker{
		{name: "examples", rawValue: "{kind: ModuleSource, spec: {registry: {repo: example.io}}}", hasValue: true, enricher: true},
	})

	want := []any{
		map[string]any{
			"kind": "ModuleSource",
			"spec": map[string]any{
				"registry": map[string]any{"repo": "example.io"},
			},
		},
	}
	if got := schema["x-doc-examples"]; !reflect.DeepEqual(got, want) {
		t.Errorf("x-doc-examples = %#v, want %#v", got, want)
	}
}

func TestApplyMarkersRawKey(t *testing.T) {
	e := &Enricher{}
	schema := map[string]any{"type": "string"}

	e.applyMarkers(schema, []marker{
		{name: "raw:pattern", rawValue: `^(\d+h)?(\d+m)?(\d+s)?$`, hasValue: true, enricher: true},
	})

	if got := schema["pattern"]; got != `^(\d+h)?(\d+m)?(\d+s)?$` {
		t.Errorf("pattern = %#v, want the regex", got)
	}
	if _, ok := schema["raw:pattern"]; ok {
		t.Errorf("raw marker name leaked into schema")
	}
}

func TestApplyMarkersRawNestedKey(t *testing.T) {
	e := &Enricher{}
	schema := map[string]any{
		"type":        "array",
		"description": "field description",
		"items": map[string]any{
			"type":        "object",
			"description": "shared type description",
			"properties": map[string]any{
				"reason": map[string]any{"type": "string", "description": "shared reason"},
			},
		},
	}

	e.applyMarkers(schema, []marker{
		{name: "raw:items.description", rawValue: "custom item description", hasValue: true, enricher: true},
		{name: "raw:items.properties.reason.description", rawValue: "custom reason", hasValue: true, enricher: true},
	})

	items := schema["items"].(map[string]any)
	if got := items["description"]; got != "custom item description" {
		t.Errorf("items.description = %#v, want override", got)
	}
	reason := items["properties"].(map[string]any)["reason"].(map[string]any)
	if got := reason["description"]; got != "custom reason" {
		t.Errorf("items.properties.reason.description = %#v, want override", got)
	}
	if len(e.warnings) != 0 {
		t.Errorf("unexpected warnings: %v", e.warnings)
	}

	// A path that does not resolve must warn instead of growing the schema.
	e2 := &Enricher{}
	s2 := map[string]any{"type": "string"}
	e2.applyMarkers(s2, []marker{{name: "raw:items.description", rawValue: "x", hasValue: true, enricher: true}})
	if _, ok := s2["items"]; ok {
		t.Errorf("nonexistent path should not be created")
	}
	if len(e2.warnings) == 0 {
		t.Errorf("expected a warning for unresolved path")
	}
}

func TestApplyCRDMarkers(t *testing.T) {
	e := &Enricher{}
	crd := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{"controller-gen.kubebuilder.io/version": "v0.19.0"},
		},
		"spec": map[string]any{
			"names": map[string]any{"kind": "Foo", "listKind": "FooList"},
			"versions": []any{
				map[string]any{
					"name": "v1",
					"schema": map[string]any{
						"openAPIV3Schema": map[string]any{
							"properties": map[string]any{
								"apiVersion": map[string]any{"type": "string"},
								"kind":       map[string]any{"type": "string"},
								"metadata":   map[string]any{"type": "object"},
								"spec": map[string]any{
									"properties": map[string]any{
										"weight": map[string]any{"type": "integer", "format": "int32"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	e.applyCRDMarkers(crd, []marker{
		{name: "crd:preserveUnknownFields", rawValue: "false", hasValue: true, enricher: true},
		{name: "crd:minimal", rawValue: "true", hasValue: true, enricher: true},
		{name: "crd:stripFormat", rawValue: "true", hasValue: true, enricher: true},
	})

	if !e.curatedStyle {
		t.Error("curatedStyle not set")
	}

	metadata := childMap(crd, "metadata")
	if _, ok := metadata["annotations"]; ok {
		t.Error("generator annotation not stripped")
	}

	spec := childMap(crd, "spec")
	if spec["preserveUnknownFields"] != false {
		t.Errorf("preserveUnknownFields = %#v, want false", spec["preserveUnknownFields"])
	}
	if _, ok := childMap(spec, "names")["listKind"]; ok {
		t.Error("listKind not stripped")
	}

	version := spec["versions"].([]any)[0].(map[string]any)
	props := childMap(childMap(childMap(version, "schema"), "openAPIV3Schema"), "properties")
	for _, k := range []string{"apiVersion", "kind", "metadata"} {
		if _, ok := props[k]; ok {
			t.Errorf("root property %q not stripped", k)
		}
	}
	weight := childMap(childMap(props, "spec"), "properties")["weight"].(map[string]any)
	if _, ok := weight["format"]; ok {
		t.Error("schema-level format not stripped")
	}
}

func TestChildMap(t *testing.T) {
	node := map[string]any{
		"properties": map[string]any{
			"repo": map[string]any{"type": "string"},
		},
		"type": "object",
	}

	props := childMap(node, "properties")
	if props == nil {
		t.Fatal("properties not found")
	}
	if childMap(props, "repo") == nil {
		t.Error("repo not found")
	}
	if childMap(node, "type") != nil {
		t.Error("scalar value should not be returned as a map")
	}
	if childMap(node, "missing") != nil {
		t.Error("missing key should return nil")
	}
}
