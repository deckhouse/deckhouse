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

// dynamixConfigYAML builds a minimal, otherwise-valid DynamixClusterConfiguration
// document. rootFields and instanceClassFields are spliced in as extra "key:
// value" lines at, respectively, the document root and
// masterNodeGroup.instanceClass (indentation handled here, callers pass plain
// unindented lines). Lets tests focus on the storagePolicy contract without
// hand-aligning YAML.
func dynamixConfigYAML(rootFields []string, instanceClassFields ...string) string {
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
	dir, err := filepath.Abs("../../../ee/modules/030-cloud-provider-dynamix/candi")
	require.NoError(t, err)
	// Absence is not a failure: the CE test image ships no ee/ directory at all.
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		t.Skipf("%s not present (CE checkout?); skip", dir)
	}

	store := newSchemaStore(nil, nil)
	require.NoError(t, store.LoadProviderDir("dynamix", "sha256:test", dir))

	storagePolicyField := []string{"storagePolicy: storage_policy01"}

	t.Run("accepts a config with storagePolicy", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML(storagePolicyField))
		_, err := store.Validate(&doc)
		require.NoError(t, err)
	})

	t.Run("accepts storagePolicy overridden per instanceClass", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML(storagePolicyField, "storagePolicy: storage_policy02"))
		_, err := store.Validate(&doc)
		require.NoError(t, err)
	})

	t.Run("rejects a config without storagePolicy", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML(nil))
		_, err := store.Validate(&doc)
		require.Error(t, err)
		require.ErrorContains(t, err, "storagePolicy")
	})

	t.Run("rejects storageEndpoint and pool in instanceClass", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML(storagePolicyField, "storageEndpoint: SharedTatlin_G1_SEP", "pool: pool_a"))
		_, err := store.Validate(&doc)
		require.Error(t, err)
		require.ErrorContains(t, err, "storageEndpoint is a forbidden property")
		require.ErrorContains(t, err, "pool is a forbidden property")
	})

	t.Run("rejects storageEndpoint and pool at the root", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML([]string{
			storagePolicyField[0],
			"storageEndpoint: SharedTatlin_G1_SEP",
			"pool: pool_a",
		}))
		_, err := store.Validate(&doc)
		require.Error(t, err)
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
// contract-relevant shape both must agree on: required fields and
// instanceClass property names.
func TestDynamixInternalValuesSchemaMatchesCandi(t *testing.T) {
	candiPath, err := filepath.Abs("../../../ee/modules/030-cloud-provider-dynamix/candi/openapi/cluster_configuration.yaml")
	require.NoError(t, err)
	valuesPath, err := filepath.Abs("../../../ee/modules/030-cloud-provider-dynamix/openapi/values.yaml")
	require.NoError(t, err)
	// Absence is not a failure: the CE test image ships no ee/ directory at all.
	if _, statErr := os.Stat(candiPath); statErr != nil {
		t.Skipf("%s not present (CE checkout?); skip", candiPath)
	}

	candi := loadYAMLMap(t, candiPath)
	apiVersions, ok := candi["apiVersions"].([]any)
	require.True(t, ok, "%s: apiVersions is not a sequence", candiPath)
	require.NotEmpty(t, apiVersions, "%s: apiVersions is empty", candiPath)
	candiSpec := digMap(t, asMap(t, apiVersions[0]), "openAPISpec")

	values := loadYAMLMap(t, valuesPath)
	valuesSpec := digMap(t, values, "properties", "internal", "properties", "providerClusterConfiguration")

	require.ElementsMatch(t, stringSlice(t, candiSpec["required"]), stringSlice(t, valuesSpec["required"]),
		"root `required` differs between candi/openapi/cluster_configuration.yaml and openapi/values.yaml")

	candiInstanceClass := digMap(t, candiSpec, "properties", "masterNodeGroup", "properties", "instanceClass")
	valuesInstanceClass := digMap(t, valuesSpec, "properties", "masterNodeGroup", "properties", "instanceClass")

	require.ElementsMatch(t, stringSlice(t, candiInstanceClass["required"]), stringSlice(t, valuesInstanceClass["required"]),
		"masterNodeGroup.instanceClass `required` differs between the two schema copies")

	candiInstanceClassProps := digMap(t, candiInstanceClass, "properties")
	valuesInstanceClassProps := digMap(t, valuesInstanceClass, "properties")

	require.ElementsMatch(t, mapKeys(candiInstanceClassProps), mapKeys(valuesInstanceClassProps),
		"masterNodeGroup.instanceClass property names differ between the two schema copies")
}

func loadYAMLMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, yaml.Unmarshal(data, &m), "unmarshal %s", path)
	return m
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	require.True(t, ok, "expected a mapping, got %T", v)
	return m
}

// digMap walks a chain of map keys, requiring each step to be a mapping.
func digMap(t *testing.T, m map[string]any, keys ...string) map[string]any {
	t.Helper()
	cur := m
	for _, k := range keys {
		v, ok := cur[k]
		require.True(t, ok, "missing key %q", k)
		cur = asMap(t, v)
	}
	return cur
}

func stringSlice(t *testing.T, v any) []string {
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

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
