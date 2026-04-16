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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateRFC1123SubdomainName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "valid", in: "abc.def-123"},
		{name: "invalid uppercase", in: "ABC", wantErr: true},
		{name: "invalid underscore", in: "a_b", wantErr: true},
		{name: "invalid empty", in: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRFC1123SubdomainName(tt.in)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q", tt.in)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.in, err)
			}
		})
	}
}

func TestNormalizeRFC1123SubdomainName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "already valid", in: "abc.def-123", want: "abc.def-123"},
		{name: "mixed chars normalized", in: "My.Case_Name", want: "my.case-name"},
		{name: "collapses separators", in: "..A__B..", want: "a-b"},
		{name: "empty after normalization", in: "!!!", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeRFC1123SubdomainName(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got name %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected normalized name: got %q want %q", got, tt.want)
			}
			if !rfc1123SubdomainRe.MatchString(got) {
				t.Fatalf("name %q must match RFC1123 regex", got)
			}
		})
	}
}

func TestEnsureObjectMetadataNameRFC1123StrictOrFallback_UsesFallbackAndNormalizes(t *testing.T) {
	doc := map[string]interface{}{"kind": "Pod"}

	if err := ensureObjectMetadataNameRFC1123StrictOrFallback(doc, "Case_01", "pod object"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	meta := doc["metadata"].(map[string]interface{})
	if got := meta["name"]; got != "case-01" {
		t.Fatalf("unexpected metadata.name: got %v want %q", got, "case-01")
	}
}

func TestEnsureObjectMetadataNameRFC1123StrictOrFallback_InvalidProvidedNameFails(t *testing.T) {
	doc := map[string]interface{}{
		"kind": "Pod",
		"metadata": map[string]interface{}{
			"name": "Bad_Name",
		},
	}

	err := ensureObjectMetadataNameRFC1123StrictOrFallback(doc, "case-01", "pod object")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApplyNamedException_ValidNamePreservedInMetadataAndLabel(t *testing.T) {
	spec := &matrixSpec{
		NamedExceptions: map[string]namedExceptionDef{
			"my.exception-1": {
				Base:  "securityPolicyException",
				Merge: map[string]interface{}{},
			},
		},
	}
	c := &matrixCase{
		Exception: "my.exception-1",
		Object: map[string]interface{}{
			"base": "pod",
		},
	}

	if err := applyNamedException(spec, c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	inv0 := c.Inventory[0].(map[string]interface{})
	merge := inv0["merge"].(map[string]interface{})
	meta := merge["metadata"].(map[string]interface{})
	if got := meta["name"]; got != "my.exception-1" {
		t.Fatalf("unexpected exception metadata.name: got %v want %q", got, "my.exception-1")
	}

	obj := c.Object.(map[string]interface{})
	objMerge := obj["merge"].(map[string]interface{})
	objMeta := objMerge["metadata"].(map[string]interface{})
	labels := objMeta["labels"].(map[string]interface{})
	if got := labels[spePodLabelKey]; got != "my.exception-1" {
		t.Fatalf("unexpected pod label value: got %v want %q", got, "my.exception-1")
	}
}

func TestApplyNamedException_InvalidNameFails(t *testing.T) {
	spec := &matrixSpec{
		NamedExceptions: map[string]namedExceptionDef{
			"Bad_Name": {Base: "securityPolicyException", Merge: map[string]interface{}{}},
		},
	}
	c := &matrixCase{Exception: "Bad_Name", Object: map[string]interface{}{"base": "pod"}}
	if err := applyNamedException(spec, c); err == nil {
		t.Fatal("expected RFC1123 validation error")
	}
}

func TestResolveMatrixInventoryItem_InvalidExplicitMetadataNameFails(t *testing.T) {
	outDir := t.TempDir()
	samplesDir := filepath.Join(outDir, "test_samples")
	if err := os.MkdirAll(samplesDir, 0o755); err != nil {
		t.Fatalf("mkdir samples dir: %v", err)
	}

	bases := map[string]matrixBase{
		"ns": {
			Document: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]interface{}{
					"name": "Name_SPACE",
				},
			},
		},
	}

	seq := &genSeqCounters{}
	_, err := resolveMatrixInventoryItem(map[string]interface{}{"base": "ns"}, bases, samplesDir, outDir, "Case_One", seq, nil)
	if err == nil {
		t.Fatal("expected error for invalid explicit metadata.name")
	}
}

func TestResolveMatrixInventoryItem_FallbackNameIsNormalized(t *testing.T) {
	outDir := t.TempDir()
	samplesDir := filepath.Join(outDir, "test_samples")
	if err := os.MkdirAll(samplesDir, 0o755); err != nil {
		t.Fatalf("mkdir samples dir: %v", err)
	}

	bases := map[string]matrixBase{
		"ns": {
			Document: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata":   map[string]interface{}{},
			},
		},
	}

	seq := &genSeqCounters{}
	rel, err := resolveMatrixInventoryItem(map[string]interface{}{"base": "ns"}, bases, samplesDir, outDir, "Case_One", seq, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	full := filepath.Join(outDir, filepath.FromSlash(rel))
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal generated yaml: %v", err)
	}
	meta := doc["metadata"].(map[string]interface{})
	if got := meta["name"]; got != "case-one" {
		t.Fatalf("unexpected metadata.name: got %v want %q", got, "case-one")
	}
}

func TestResolveMatrixObject_InvalidPodNameFails(t *testing.T) {
	outDir := t.TempDir()
	samplesDir := filepath.Join(outDir, "test_samples")
	if err := os.MkdirAll(samplesDir, 0o755); err != nil {
		t.Fatalf("mkdir samples dir: %v", err)
	}

	bases := map[string]matrixBase{
		"pod": {
			Document: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata":   map[string]interface{}{},
			},
		},
	}

	seq := &genSeqCounters{}
	_, err := resolveMatrixObject(
		map[string]interface{}{"base": "pod", "podName": "My_Pod..Name"},
		bases,
		samplesDir,
		outDir,
		"Case_Name",
		"",
		seq,
		nil,
	)
	if err == nil {
		t.Fatal("expected error for invalid podName")
	}
}

func TestResolveMatrixObject_FallbackFromCaseSlugIsNormalized(t *testing.T) {
	outDir := t.TempDir()
	samplesDir := filepath.Join(outDir, "test_samples")
	if err := os.MkdirAll(samplesDir, 0o755); err != nil {
		t.Fatalf("mkdir samples dir: %v", err)
	}

	bases := map[string]matrixBase{
		"pod": {
			Document: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata":   map[string]interface{}{},
			},
		},
	}

	seq := &genSeqCounters{}
	rel, err := resolveMatrixObject(
		map[string]interface{}{"base": "pod"},
		bases,
		samplesDir,
		outDir,
		"Case_Name",
		"",
		seq,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	full := filepath.Join(outDir, filepath.FromSlash(rel))
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal generated yaml: %v", err)
	}
	meta := doc["metadata"].(map[string]interface{})
	if got := meta["name"]; got != "case-name" {
		t.Fatalf("unexpected pod metadata.name: got %v want %q", got, "case-name")
	}
	if !strings.Contains(rel, "test_samples/pods/") {
		t.Fatalf("unexpected generated object path: %s", rel)
	}
}
