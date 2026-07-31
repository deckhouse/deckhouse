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

package schema

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrantRefs(t *testing.T) {
	t.Run("collects top-level and nested string fields", func(t *testing.T) {
		settings := []byte(`
type: object
properties:
  storageClass:
    type: string
    x-deckhouse-grantable-resource: storageclasses
  postgres:
    type: object
    properties:
      storageClass:
        type: string
        x-deckhouse-grantable-resource: postgresclasses
  name:
    type: string
`)

		storage, err := NewStorage(settings, nil)
		require.NoError(t, err)

		refs, err := storage.GrantRefs()
		require.NoError(t, err)

		sort.Slice(refs, func(i, j int) bool {
			return len(refs[i].Path) < len(refs[j].Path)
		})

		require.Len(t, refs, 2)
		assert.Equal(t, []string{"storageClass"}, refs[0].Path)
		assert.Equal(t, "storageclasses", refs[0].Resource)
		assert.Equal(t, []string{"postgres", "storageClass"}, refs[1].Path)
		assert.Equal(t, "postgresclasses", refs[1].Resource)
	})

	t.Run("returns nil when no grant fields", func(t *testing.T) {
		settings := []byte(`
type: object
properties:
  name:
    type: string
`)
		storage, err := NewStorage(settings, nil)
		require.NoError(t, err)

		refs, err := storage.GrantRefs()
		require.NoError(t, err)
		assert.Empty(t, refs)
	})

	t.Run("returns nil when no settings schema", func(t *testing.T) {
		storage, err := NewStorage(nil, nil)
		require.NoError(t, err)

		refs, err := storage.GrantRefs()
		require.NoError(t, err)
		assert.Empty(t, refs)
	})

	t.Run("errors on non-string field", func(t *testing.T) {
		settings := []byte(`
type: object
properties:
  storageClass:
    type: integer
    x-deckhouse-grantable-resource: storageclasses
`)
		storage, err := NewStorage(settings, nil)
		require.NoError(t, err)

		_, err = storage.GrantRefs()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type: string")
	})

	t.Run("errors on empty resource", func(t *testing.T) {
		settings := []byte(`
type: object
properties:
  storageClass:
    type: string
    x-deckhouse-grantable-resource: ""
`)
		storage, err := NewStorage(settings, nil)
		require.NoError(t, err)

		_, 	err = storage.GrantRefs()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-empty")
	})
}
