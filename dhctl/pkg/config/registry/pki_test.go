// Copyright 2025 Flant JSC
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

package registry

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func TestLazyPKIGenerator_Get(t *testing.T) {
	t.Run("should generate PKI and cache result", func(t *testing.T) {
		generator := NewLazyPKIGenerator()

		// First call generates PKI
		pki1, err := generator.Get()
		require.NoError(t, err)
		assert.NotNil(t, pki1.CA)

		// Second call returns cached instance
		pki2, err := generator.Get()
		require.NoError(t, err)
		assert.Equal(t, pki1, pki2)
	})

	t.Run("should handle concurrent access safely", func(t *testing.T) {
		generator := NewLazyPKIGenerator()

		const size = 10
		var wg sync.WaitGroup
		results := make([]PKI, size)

		for i := 0; i < size; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				results[index], _ = generator.Get()
			}(i)
		}
		wg.Wait()

		first := results[0]
		for i := 1; i < size; i++ {
			assert.Equal(t, first, results[i])
		}
	})
}

func TestClusterPKIManager_Get(t *testing.T) {
	t.Run("should cache PKI on repeated calls", func(t *testing.T) {
		kubeClient := client.NewFakeKubernetesClient()
		require.NoError(t, createInitSecret(t.Context(), kubeClient, false))

		manager := NewClusterPKIManager(kubeClient)

		// First call save PKI in cache
		pki1, err := manager.Get(t.Context())
		require.NoError(t, err)
		assert.NotNil(t, pki1.CA)

		// Second call returns cached PKI data
		pki2, err := manager.Get(t.Context())
		require.NoError(t, err)
		assert.Equal(t, pki1, pki2)
	})

	t.Run("should handle concurrent access with proper synchronization", func(t *testing.T) {
		kubeClient := client.NewFakeKubernetesClient()
		manager := NewClusterPKIManager(kubeClient)
		require.NoError(t, createInitSecret(t.Context(), kubeClient, false))

		const size = 10
		var wg sync.WaitGroup
		results := make([]PKI, size)
		errs := make([]error, size)

		for i := 0; i < size; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				results[index], errs[index] = manager.Get(t.Context())
			}(i)
		}
		wg.Wait()

		first := results[0]
		for i := 0; i < size; i++ {
			require.NoError(t, errs[i])
			assert.Equal(t, first, results[i])
		}
	})
}
