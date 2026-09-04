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
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCheckedInBundle(t *testing.T) {
	if err := checkBundle("../vendor"); err != nil {
		t.Fatal(err)
	}

	paths, err := filepath.Glob("../vendor/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	generatedDir := t.TempDir()
	for _, path := range paths {
		objects, err := load(path)
		if err != nil {
			t.Fatal(err)
		}
		generatedPath := filepath.Join(generatedDir, filepath.Base(path))
		if err := writeYAML(generatedPath, objects[0].node); err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(generatedPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s is not in canonical generated form", path)
		}
	}
}

func TestWriteYAMLPreservesLeadingNewlinesAndUsesCompactSequences(t *testing.T) {
	const input = `description: |2-


  Valid Options: A, B
versions:
- name: v1
`
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(input), &node); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "object.yaml")
	if err := writeYAML(path, &node); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "versions:\n- name: v1") {
		t.Fatalf("expected compact sequence indentation, got:\n%s", data)
	}

	var result struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.Description != "\n\nValid Options: A, B" {
		t.Fatalf("leading newlines changed to %q", result.Description)
	}
}

func TestValidateRejectsMissingServedVersion(t *testing.T) {
	objects := validObjects(t)
	for i := range objects {
		if objects[i].crd.Metadata.Name == "destinationrules.networking.istio.io" {
			for version := range objects[i].crd.Spec.Versions {
				if objects[i].crd.Spec.Versions[version].Name == "v1alpha3" {
					objects[i].crd.Spec.Versions[version].Served = false
				}
			}
		}
	}

	err := validate(objects)
	if err == nil || !strings.Contains(err.Error(), "required served versions were removed: [v1alpha3]") {
		t.Fatalf("expected missing served version error, got %v", err)
	}
}

func TestValidateRejectsDuplicateCRD(t *testing.T) {
	objects := validObjects(t)
	objects[len(objects)-1] = objects[0]

	err := validate(objects)
	if err == nil || !strings.Contains(err.Error(), "duplicate CRD") {
		t.Fatalf("expected duplicate CRD error, got %v", err)
	}
}

func TestLoadSkipsEmptyDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bundle.yaml")
	contents := `---
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: example.example.com
spec:
  versions:
  - name: v1
    served: true
    storage: true
---
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	objects, err := load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 1 || objects[0].crd.Metadata.Name != "example.example.com" {
		t.Fatalf("unexpected objects: %#v", objects)
	}
}

func TestDownloadSources(t *testing.T) {
	allObjects := validObjects(t)
	var configObjects, sailObjects []object
	for _, object := range allObjects {
		name := object.crd.Metadata.Name
		if _, ok := requiredServedVersions[name]; ok {
			configObjects = append(configObjects, object)
		} else if name != "istiooperators.install.istio.io" {
			sailObjects = append(sailObjects, object)
		}
	}
	configBundle := marshalObjects(t, configObjects)
	sailBundle := marshalObjects(t, sailObjects)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/config":
			_, _ = writer.Write(configBundle)
		case "/sail":
			_, _ = writer.Write(sailBundle)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	sources := []remoteSource{
		{name: "config", url: server.URL + "/config", sha256: checksum(configBundle)},
		{name: "sail", url: server.URL + "/sail", sha256: checksum(sailBundle)},
	}
	objects, err := downloadSources(context.Background(), server.Client(), sources)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(objects); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadSourcesRejectsChecksumMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("data"))
	}))
	defer server.Close()

	_, err := downloadSources(context.Background(), server.Client(), []remoteSource{{
		name: "changed source", url: server.URL, sha256: strings.Repeat("0", 64),
	}})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestReplaceBundleLeavesNonYAMLFiles(t *testing.T) {
	outputDir := t.TempDir()
	readme := filepath.Join(outputDir, "README.md")
	if err := os.WriteFile(readme, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "obsolete.yaml"), []byte("obsolete"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := replaceBundle(outputDir, validObjects(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "obsolete.yaml")); !os.IsNotExist(err) {
		t.Fatalf("obsolete YAML was not removed: %v", err)
	}
	contents, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "keep me" {
		t.Fatalf("README was modified: %q", contents)
	}
	if err := checkBundle(outputDir); err != nil {
		t.Fatal(err)
	}
}

func marshalObjects(t *testing.T, objects []object) []byte {
	t.Helper()
	var result strings.Builder
	for _, object := range objects {
		data, err := yaml.Marshal(object.node)
		if err != nil {
			t.Fatal(err)
		}
		result.Write(data)
		result.WriteString("---\n")
	}
	return []byte(result.String())
}

func checksum(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func validObjects(t *testing.T) []object {
	t.Helper()

	names := make([]string, 0, len(requiredServedVersions)+len(operatorCRDs))
	for name := range requiredServedVersions {
		names = append(names, name)
	}
	for name := range operatorCRDs {
		names = append(names, name)
	}

	objects := make([]object, 0, len(names))
	for _, name := range names {
		definition := crd{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition"}
		definition.Metadata.Name = name
		versions := requiredServedVersions[name]
		if len(versions) == 0 {
			versions = stringSet("v1")
		}
		first := true
		for version := range versions {
			definition.Spec.Versions = append(definition.Spec.Versions, struct {
				Name    string `yaml:"name"`
				Served  bool   `yaml:"served"`
				Storage bool   `yaml:"storage"`
			}{Name: version, Served: true, Storage: first})
			first = false
		}

		data, err := yaml.Marshal(definition)
		if err != nil {
			t.Fatal(err)
		}
		var node yaml.Node
		if err := yaml.Unmarshal(data, &node); err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object{node: &node, crd: definition})
	}
	return objects
}
