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
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTestProviderSchema(t *testing.T, dir, kind string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "openapi"), 0o755))
	schema := fmt.Sprintf(`kind: %s
apiVersions:
- apiVersion: deckhouse.io/v1
  openAPISpec:
    type: object
    additionalProperties: false
    required: [apiVersion, kind]
    properties:
      apiVersion:
        type: string
      kind:
        type: string
      layout:
        type: string
`, kind)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "openapi", "cluster_configuration.yaml"), []byte(schema), 0o644))
}

func TestLoadProviderDirAddsAndReplacesSchemas(t *testing.T) {
	store := newSchemaStore(nil, nil)

	dirV1 := t.TempDir()
	writeTestProviderSchema(t, dirV1, "TestProviderConfiguration")

	require.NoError(t, store.LoadProviderDir("testprov", "sha256:v1", dirV1))
	require.True(t, store.ProviderSchemasLoaded("testprov", "sha256:v1"))

	index := &SchemaIndex{Kind: "TestProviderConfiguration", Version: "deckhouse.io/v1"}
	require.NotNil(t, store.Get(index))

	doc := []byte("apiVersion: deckhouse.io/v1\nkind: TestProviderConfiguration\nlayout: Standard\n")
	_, err := store.Validate(&doc)
	require.NoError(t, err)

	// Same digest: no-op even though the dir is gone.
	require.NoError(t, os.RemoveAll(dirV1))
	require.NoError(t, store.LoadProviderDir("testprov", "sha256:v1", dirV1))
	require.NotNil(t, store.Get(index))

	// New digest with a different kind replaces the provider's schemas.
	dirV2 := t.TempDir()
	writeTestProviderSchema(t, dirV2, "TestProviderConfigurationV2")
	require.NoError(t, store.LoadProviderDir("testprov", "sha256:v2", dirV2))

	require.Nil(t, store.Get(index), "old provider schema must be dropped on digest change")
	require.NotNil(t, store.Get(&SchemaIndex{Kind: "TestProviderConfigurationV2", Version: "deckhouse.io/v1"}))
	require.False(t, store.ProviderSchemasLoaded("testprov", "sha256:v1"))
	require.True(t, store.ProviderSchemasLoaded("testprov", "sha256:v2"))
}

func TestLoadProviderDirConcurrentWithValidate(t *testing.T) {
	store := newSchemaStore(nil, nil)

	dir := t.TempDir()
	writeTestProviderSchema(t, dir, "TestProviderConfiguration")
	require.NoError(t, store.LoadProviderDir("testprov", "sha256:v1", dir))

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				doc := []byte("apiVersion: deckhouse.io/v1\nkind: TestProviderConfiguration\nlayout: Standard\n")
				_, _ = store.Validate(&doc)
			}
		}()
	}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				digest := fmt.Sprintf("sha256:rotating-%d-%d", n, j)
				require.NoError(t, store.LoadProviderDir("testprov", digest, dir))
			}
		}(i)
	}
	wg.Wait()
}
