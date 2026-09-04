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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// dynamixModuleRelPath is the module directory shared by every test in this
// file, relative to this package.
const dynamixModuleRelPath = "../../../ee/modules/030-cloud-provider-dynamix"

// requireDynamixModuleDir returns the absolute path to the module directory.
// Absence is not a failure: the CE test image ships no ee/ directory at all.
func requireDynamixModuleDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(dynamixModuleRelPath)
	require.NoError(t, err)
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		t.Skipf("%s not present (CE checkout?); skip", dir)
	}
	return dir
}

// dynamixConfigYAML builds a minimal, otherwise-valid DynamixClusterConfiguration
// document. rootFields and instanceClassFields are spliced in as extra "key:
// value" lines at, respectively, the document root and
// masterNodeGroup.instanceClass (indentation handled here, callers pass plain
// unindented lines). Lets tests focus on the storagePolicy contract without
// hand-aligning YAML.
func dynamixConfigYAML(rootFields, instanceClassFields []string) string {
	var root strings.Builder
	for _, field := range rootFields {
		fmt.Fprintf(&root, "%s\n", field)
	}
	var instanceClass strings.Builder
	for _, field := range instanceClassFields {
		fmt.Fprintf(&instanceClass, "    %s\n", field)
	}
	return fmt.Sprintf(`apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa AAAA"
location: dynamix
account: acc_user
%sprovider:
  controllerUrl: "https://controller"
  oAuth2Url: "https://oauth"
  appId: "app"
  appSecret: "secret"
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 6
    memory: 16384
    imageName: "image"
    externalNetwork: "extnet"
%s`, root.String(), instanceClass.String())
}

// TestDynamixClusterConfigurationStoragePolicy locks the storagePolicy contract
// introduced for ee/modules/030-cloud-provider-dynamix (Dynamix 4.6, ticket
// 04-openapi-storage-policy-contract): storagePolicy is required at the cluster
// level, optional as a per-instanceClass override, and storageEndpoint/pool are
// gone from the schema entirely. It exercises the same SchemaStore.Validate path
// that `dhctl config parse` uses in production.
func TestDynamixClusterConfigurationStoragePolicy(t *testing.T) {
	candiDir := filepath.Join(requireDynamixModuleDir(t), "candi")

	store := newSchemaStore(nil, nil)
	require.NoError(t, store.LoadProviderDir("dynamix", "sha256:test", candiDir))

	storagePolicyField := []string{"storagePolicy: storage_policy01"}

	t.Run("accepts a config with storagePolicy", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML(storagePolicyField, nil))
		_, err := store.Validate(&doc)
		require.NoError(t, err)
	})

	t.Run("accepts storagePolicy overridden per instanceClass", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML(storagePolicyField, []string{"storagePolicy: storage_policy02"}))
		_, err := store.Validate(&doc)
		require.NoError(t, err)
	})

	t.Run("rejects a config without storagePolicy", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML(nil, nil))
		_, err := store.Validate(&doc)
		require.ErrorContains(t, err, "storagePolicy")
	})

	t.Run("rejects storageEndpoint and pool in instanceClass", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML(storagePolicyField, []string{"storageEndpoint: SharedTatlin_G1_SEP", "pool: pool_a"}))
		_, err := store.Validate(&doc)
		require.ErrorContains(t, err, "storageEndpoint is a forbidden property")
		require.ErrorContains(t, err, "pool is a forbidden property")
	})

	t.Run("rejects storageEndpoint and pool at the root", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML(
			[]string{"storagePolicy: storage_policy01", "storageEndpoint: SharedTatlin_G1_SEP", "pool: pool_a"},
			nil,
		))
		_, err := store.Validate(&doc)
		require.ErrorContains(t, err, "storageEndpoint is a forbidden property")
		require.ErrorContains(t, err, "pool is a forbidden property")
	})
}

// TestDynamixInternalValuesSchemaMatchesCandi guards against a repeat of the
// gap found in review: openapi/values.yaml duplicates
// DynamixClusterConfiguration under internal.providerClusterConfiguration for
// the module's own values validation, and it drifted out of sync with
// candi/openapi/cluster_configuration.yaml when storagePolicy replaced
// storageEndpoint/pool there — dhctl accepted configs the module itself would
// then reject. It doesn't diff the two files byte-for-byte: they carry
// legitimate cosmetic differences (extra prose, x-rules). It only locks the
// contract-relevant shape both must agree on: root required fields and
// property names, and — for both masterNodeGroup.instanceClass and
// nodeGroups[].instanceClass — required fields and property names.
func TestDynamixInternalValuesSchemaMatchesCandi(t *testing.T) {
	moduleDir := requireDynamixModuleDir(t)
	candiPath := filepath.Join(moduleDir, "candi", "openapi", "cluster_configuration.yaml")
	valuesPath := filepath.Join(moduleDir, "openapi", "values.yaml")

	candi := dynamixLoadYAMLMap(t, candiPath)
	apiVersions, ok := candi["apiVersions"].([]any)
	require.True(t, ok, "%s: apiVersions is not a sequence", candiPath)
	require.NotEmpty(t, apiVersions, "%s: apiVersions is empty", candiPath)
	candiSpec := dynamixDigMap(t, dynamixAsMap(t, apiVersions[0]), "openAPISpec")

	values := dynamixLoadYAMLMap(t, valuesPath)
	valuesSpec := dynamixDigMap(t, values, "properties", "internal", "properties", "providerClusterConfiguration")

	require.ElementsMatch(t, dynamixStringSlice(t, candiSpec["required"]), dynamixStringSlice(t, valuesSpec["required"]),
		"root `required` differs between candi/openapi/cluster_configuration.yaml and openapi/values.yaml")
	require.ElementsMatch(t, dynamixMapKeys(dynamixDigMap(t, candiSpec, "properties")), dynamixMapKeys(dynamixDigMap(t, valuesSpec, "properties")),
		"root property names differ between the two schema copies")

	dynamixRequireInstanceClassParity(t, "masterNodeGroup.instanceClass",
		dynamixDigMap(t, candiSpec, "properties", "masterNodeGroup", "properties", "instanceClass"),
		dynamixDigMap(t, valuesSpec, "properties", "masterNodeGroup", "properties", "instanceClass"),
	)
	dynamixRequireInstanceClassParity(t, "nodeGroups[].instanceClass",
		dynamixDigMap(t, candiSpec, "properties", "nodeGroups", "items", "properties", "instanceClass"),
		dynamixDigMap(t, valuesSpec, "properties", "nodeGroups", "items", "properties", "instanceClass"),
	)
}

// dynamixRequireInstanceClassParity compares one instanceClass definition
// (found at name in both schema copies) by `required` fields and property
// names.
func dynamixRequireInstanceClassParity(t *testing.T, name string, candi, values map[string]any) {
	t.Helper()
	require.ElementsMatch(t, dynamixStringSlice(t, candi["required"]), dynamixStringSlice(t, values["required"]),
		"%s `required` differs between the two schema copies", name)
	require.ElementsMatch(t, dynamixMapKeys(dynamixDigMap(t, candi, "properties")), dynamixMapKeys(dynamixDigMap(t, values, "properties")),
		"%s property names differ between the two schema copies", name)
}

func dynamixLoadYAMLMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, yaml.Unmarshal(data, &m), "unmarshal %s", path)
	return m
}

func dynamixAsMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	require.True(t, ok, "expected a mapping, got %T", v)
	return m
}

// dynamixDigMap walks a chain of map keys, requiring each step to be a mapping.
func dynamixDigMap(t *testing.T, m map[string]any, keys ...string) map[string]any {
	t.Helper()
	cur := m
	for _, k := range keys {
		v, ok := cur[k]
		require.True(t, ok, "missing key %q", k)
		cur = dynamixAsMap(t, v)
	}
	return cur
}

func dynamixStringSlice(t *testing.T, v any) []string {
	t.Helper()
	if v == nil {
		return nil
	}
	items, ok := v.([]any)
	require.True(t, ok, "expected a sequence, got %T", v)
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		require.True(t, ok, "expected a string element, got %T", item)
		out = append(out, s)
	}
	return out
}

func dynamixMapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
