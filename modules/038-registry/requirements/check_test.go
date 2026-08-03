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

package requirements

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

// getter reads what the module recorded, which is what the release check reads in a
// running cluster.
type getter struct{}

func (getter) Get(key string) (any, bool) { return requirements.GetValue(key) }

func TestCheck(t *testing.T) {
	t.Run("a cluster running what the release requires", func(t *testing.T) {
		requirements.RemoveValue(ImplementationKey)
		requirements.SaveValue(ImplementationKey, "V2")

		ok, err := check("V2", getter{})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	// The case the whole check exists for: the release that removes the legacy
	// implementation must not land on a cluster that still needs it to configure its
	// nodes.
	t.Run("a cluster still running the legacy implementation", func(t *testing.T) {
		requirements.RemoveValue(ImplementationKey)
		requirements.SaveValue(ImplementationKey, "Legacy")

		ok, err := check("V2", getter{})
		require.Error(t, err)
		assert.False(t, ok)
		assert.Contains(t, err.Error(), "implementation: V2")
	})

	// Passing rather than blocking. A cluster with no recorded value is one whose
	// registry module has not reported yet, and stranding it on missing information
	// would be worse than letting the update through.
	t.Run("a cluster that has recorded nothing", func(t *testing.T) {
		requirements.RemoveValue(ImplementationKey)

		ok, err := check("V2", getter{})
		require.NoError(t, err)
		assert.True(t, ok)
	})
}
