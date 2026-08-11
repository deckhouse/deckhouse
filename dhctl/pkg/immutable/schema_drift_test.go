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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// TestGoldenPayloadMatchesNodeConfigCRD guards the hand-duplicated contract:
// every field path the golden payload writes must exist in the NodeConfig CRD
// schema. Types/enums are left to the API server (validating them needs cel-go).
func TestGoldenPayloadMatchesNodeConfigCRD(t *testing.T) {
	nodeConfigDoc := goldenNodeConfigDocument(t)

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
	versioned := crd.Spec.Versions[0].Schema.OpenAPIV3Schema

	var unknown []string
	collectUnknownFields(nodeConfigDoc, versioned, "", &unknown)
	require.Empty(t, unknown,
		"the payload writes fields the NodeConfig CRD does not know; the agent parses the same shape, so either the payload or the CRD is behind")

}

// goldenNodeConfigDocument digs the NodeConfig document out of the golden
// cloud-config: the file the node reads from /config/nodeconfig.yaml.
func goldenNodeConfigDocument(t *testing.T) map[string]interface{} {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", "master-cloud-init.yaml"))
	require.NoError(t, err)

	var cloudConfig struct {
		WriteFiles []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"write_files"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &cloudConfig))

	for _, f := range cloudConfig.WriteFiles {
		if f.Path != "/config/nodeconfig.yaml" {
			continue
		}
		doc := map[string]interface{}{}
		require.NoError(t, yaml.Unmarshal([]byte(f.Content), &doc))
		return doc
	}
	t.Fatal("the golden cloud-config carries no /config/nodeconfig.yaml")
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
