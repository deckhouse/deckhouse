/*
Copyright 2026 Flant JSC

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

package immutable

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// TestGoldenPayloadMatchesNodeConfigCRD guards the hand-duplicated contract:
// every field path the golden payload writes must exist in the NodeConfig CRD
// schema. Types/enums are left to the API server (validating them needs cel-go).
func TestGoldenPayloadMatchesNodeConfigCRD(t *testing.T) {
	var unknown []string
	collectUnknownFields(goldenNodeConfigDocument(t), nodeConfigCRDSchema(t), "", &unknown)
	require.Empty(t, unknown,
		"the payload writes fields the NodeConfig CRD does not know; the agent parses the same shape, so either the payload or the CRD is behind")
}

// TestCustomizableFieldsExistInNodeConfigCRD covers what the golden payload
// never carries: the fields only an operator's document fills in. Left to the
// golden alone they drift from the CRD unnoticed.
func TestCustomizableFieldsExistInNodeConfigCRD(t *testing.T) {
	spec := nodeSpec{
		Storage: storage{
			Device: "/dev/sda",
			DiskSelector: &diskSelector{
				Size: "1Gi", Type: "SSD", Rotational: new(true), Model: "m",
				Serial: "s", WWID: "w", Name: "n", BusPath: "b",
			},
			Mounts: []mount{{Name: "data", Device: "/dev/sdb1", Filesystem: "ext4"}},
		},
		Network: network{
			DNS:    &dns{Servers: []string{"10.0.0.1"}, Search: []string{"example.com"}},
			Routes: []route{{Name: "r", Networks: []string{"10.1.0.0/16"}, Gateway: "10.0.0.1"}},
		},
		Kubelet: kubelet{NodeIP: "10.0.0.1"},
	}

	raw, err := json.Marshal(spec)
	require.NoError(t, err)
	document := map[string]any{}
	require.NoError(t, json.Unmarshal(raw, &document))

	specSchema := nodeConfigCRDSchema(t).Properties["spec"]

	var unknown []string
	collectUnknownFields(document, &specSchema, "spec", &unknown)
	require.Empty(t, unknown,
		"a customization writes fields the NodeConfig CRD does not know; the node parses the same shape strictly and would refuse to boot")
}

// nodeConfigCRDSchema is the schema both guards check a document against.
func nodeConfigCRDSchema(t *testing.T) *apiextensionsv1.JSONSchemaProps {
	t.Helper()

	crdPath := filepath.Join("..", "..", "..", "modules", "040-node-manager", "crds", "nodeconfig.yaml")
	raw, err := os.ReadFile(crdPath)
	if os.IsNotExist(err) {
		// The dhctl CI image ships dhctl alone, without the modules tree. The
		// guard still runs on every full checkout — a developer machine or a
		// whole-repository job — which is where the CRD gets edited anyway.
		t.Skipf("the NodeConfig CRD is not in this checkout (%s); run from a full repository checkout to exercise the contract guard", crdPath)
	}
	require.NoError(t, err, "the CRD lives in this repository; if it moved, move this path with it")

	crd := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, yaml.Unmarshal(raw, crd))
	require.Len(t, crd.Spec.Versions, 1, "a second CRD version needs this test to pick the one the payload speaks")

	return crd.Spec.Versions[0].Schema.OpenAPIV3Schema
}

// goldenNodeConfigDocument takes the NodeConfig out of the golden stream, the
// way the machine does: by kind, not by position.
func goldenNodeConfigDocument(t *testing.T) map[string]interface{} {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", "master-documents.yaml"))
	require.NoError(t, err)

	decoder := utilyaml.NewYAMLToJSONDecoder(bytes.NewReader(raw))
	for {
		doc := map[string]interface{}{}
		err := decoder.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		if doc["kind"] == "NodeConfig" {
			return doc
		}
	}
	t.Fatal("the golden stream carries no NodeConfig")
	return nil
}

// collectUnknownFields walks the document against the schema's properties tree
// and records every field path the schema has no entry for.
func collectUnknownFields(doc interface{}, schema *apiextensionsv1.JSONSchemaProps, path string, unknown *[]string) {
	if schema == nil {
		return
	}
	switch value := doc.(type) {
	case map[string]interface{}:
		// A subtree the schema deliberately leaves open is not drift.
		if schema.XPreserveUnknownFields != nil && *schema.XPreserveUnknownFields {
			return
		}
		if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
			for key, child := range value {
				collectUnknownFields(child, schema.AdditionalProperties.Schema, fmt.Sprintf("%s.%s", path, key), unknown)
			}
			return
		}
		for key, child := range value {
			// The apiserver owns metadata; a CRD schema describes it as an
			// opaque object, and its contents are not this contract's.
			if path == "" && key == "metadata" {
				continue
			}
			childPath := strings.TrimPrefix(fmt.Sprintf("%s.%s", path, key), ".")
			prop, ok := schema.Properties[key]
			if !ok {
				*unknown = append(*unknown, childPath)
				continue
			}
			collectUnknownFields(child, &prop, childPath, unknown)
		}
	case []interface{}:
		if schema.Items == nil || schema.Items.Schema == nil {
			return
		}
		for i, item := range value {
			collectUnknownFields(item, schema.Items.Schema, fmt.Sprintf("%s[%d]", path, i), unknown)
		}
	}
}

// TestDefaultsMatchTheNodeConfigCRD guards the three defaults nodeconfig.go
// copies out of the CRD: the payload is written to a file instead of created
// through the API server, so nothing defaults it on the way in.
func TestDefaultsMatchTheNodeConfigCRD(t *testing.T) {
	typesPath := filepath.Join("..", "..", "..", "modules", "040-node-manager", "images", "node-controller",
		"src", "api", "internal.deckhouse.io", "v1alpha1", "nodeconfig_types.go")
	if _, err := os.Stat(typesPath); os.IsNotExist(err) {
		// The dhctl CI image ships dhctl alone, without the modules tree; the
		// guard runs on every full checkout, which is where the CRD is edited.
		t.Skipf("the NodeConfig types are not in this checkout (%s); run from a full repository checkout", typesPath)
	}

	defaults := kubebuilderDefaults(t, typesPath)

	require.Equal(t, defaultContainerLogMaxSize, defaults["ContainerLogMaxSize"])
	require.Equal(t, strconv.Itoa(defaultContainerLogMaxFiles), defaults["ContainerLogMaxFiles"])
	require.Equal(t, strconv.Itoa(defaultMaxConcurrentDownloads), defaults["MaxConcurrentDownloads"])
}

// kubebuilderDefaults reads the +kubebuilder:default marker of every field in
// the file, unquoted, keyed by the Go field name.
func kubebuilderDefaults(t *testing.T, path string) map[string]string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	require.NoError(t, err)

	defaults := map[string]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		field, ok := node.(*ast.Field)
		if !ok || field.Doc == nil || len(field.Names) == 0 {
			return true
		}
		for _, line := range field.Doc.List {
			value, found := strings.CutPrefix(line.Text, "// +kubebuilder:default=")
			if !found {
				continue
			}
			defaults[field.Names[0].Name] = strings.Trim(value, `"`)
		}
		return true
	})

	return defaults
}
