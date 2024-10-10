/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package requirements

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

func TestKubernetesVersionRequirement(t *testing.T) {
	t.Run("in-config requirement met", func(t *testing.T) {
		requirements.SaveValue(l2LoadBalancerModuleDeprecatedKey, false)
		ok, err := requirements.CheckRequirement(l2LoadBalancerModuleDeprecatedKeyRequirementsKey, "")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("in-config requirement failed", func(t *testing.T) {
		requirements.SaveValue(l2LoadBalancerModuleDeprecatedKey, true)
		ok, err := requirements.CheckRequirement(l2LoadBalancerModuleDeprecatedKeyRequirementsKey, "")
		assert.False(t, ok)
		require.Error(t, err)
	})
}
