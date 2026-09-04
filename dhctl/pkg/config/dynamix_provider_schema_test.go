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
	"testing"

	"github.com/stretchr/testify/require"
)

// dynamixConfigYAML builds a minimal, otherwise-valid DynamixClusterConfiguration
// document with the given extra fields spliced into the root and into
// masterNodeGroup.instanceClass, so tests can focus on the storagePolicy contract.
func dynamixConfigYAML(rootExtra, instanceClassExtra string) string {
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
%s`, rootExtra, instanceClassExtra)
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

	t.Run("accepts a config with storagePolicy", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML("storagePolicy: storage_policy01\n", ""))
		_, err := store.Validate(&doc)
		require.NoError(t, err)
	})

	t.Run("accepts storagePolicy overridden per instanceClass", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML(
			"storagePolicy: storage_policy01\n",
			"    storagePolicy: storage_policy02\n",
		))
		_, err := store.Validate(&doc)
		require.NoError(t, err)
	})

	t.Run("rejects a config without storagePolicy", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML("", ""))
		_, err := store.Validate(&doc)
		require.Error(t, err)
		require.ErrorContains(t, err, "storagePolicy")
	})

	t.Run("rejects storageEndpoint and pool", func(t *testing.T) {
		doc := []byte(dynamixConfigYAML(
			"storagePolicy: storage_policy01\n",
			"    storageEndpoint: SharedTatlin_G1_SEP\n    pool: pool_a\n",
		))
		_, err := store.Validate(&doc)
		require.Error(t, err)
	})
}
