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

// getter reads what the module recorded, the same way the release check reads it in a cluster.
type getter struct{}

func (getter) Get(key string) (any, bool) { return requirements.GetValue(key) }

func TestCheck(t *testing.T) {
	// The case the check exists for: the release that makes Baseline the default must not switch
	// a cluster from the implicit Privileged policy to Baseline without the administrator's decision.
	t.Run("a cluster on the implicit Privileged policy", func(t *testing.T) {
		requirements.RemoveValue(ImplicitPrivilegedKey)
		requirements.SaveValue(ImplicitPrivilegedKey, true)

		ok, err := check("false", getter{})
		require.Error(t, err)
		assert.False(t, ok)
		assert.Contains(t, err.Error(), "settings.podSecurityStandards.defaultPolicy")
	})

	t.Run("a cluster on the implicit Privileged policy, recorded as a string", func(t *testing.T) {
		requirements.RemoveValue(ImplicitPrivilegedKey)
		requirements.SaveValue(ImplicitPrivilegedKey, "true")

		ok, err := check("false", getter{})
		require.Error(t, err)
		assert.False(t, ok)
	})

	// An explicitly set policy, or an implicit Baseline one, stays the same after the update.
	t.Run("a cluster whose default policy does not change on update", func(t *testing.T) {
		requirements.RemoveValue(ImplicitPrivilegedKey)
		requirements.SaveValue(ImplicitPrivilegedKey, false)

		ok, err := check("false", getter{})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("a release that allows the implicit Privileged policy", func(t *testing.T) {
		requirements.RemoveValue(ImplicitPrivilegedKey)
		requirements.SaveValue(ImplicitPrivilegedKey, true)

		ok, err := check("true", getter{})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	// A cluster with no recorded value either has not reported yet or runs a version that no longer
	// derives the default policy; stranding it on missing information is worse than letting it through.
	t.Run("a cluster that has recorded nothing", func(t *testing.T) {
		requirements.RemoveValue(ImplicitPrivilegedKey)

		ok, err := check("false", getter{})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("an unparsable requirement value", func(t *testing.T) {
		requirements.RemoveValue(ImplicitPrivilegedKey)
		requirements.SaveValue(ImplicitPrivilegedKey, true)

		ok, err := check("maybe", getter{})
		require.Error(t, err)
		assert.False(t, ok)
	})
}
