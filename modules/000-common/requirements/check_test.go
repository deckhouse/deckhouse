/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package requirements

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

func TestKubernetesVersionRequirement(t *testing.T) {
	t.Run("complies with the requirements", func(t *testing.T) {
		requirements.SaveValue(cniConfigurationSettledKey, "")
		ok, err := requirements.CheckRequirement(cniConfigurationSettledRequirementsKey, "")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("fail: Misconfigured", func(t *testing.T) {
		requirements.SaveValue(cniConfigurationSettledKey, "false")
		ok, err := requirements.CheckRequirement(cniConfigurationSettledRequirementsKey, "")
		assert.False(t, ok)
		require.Error(t, err)
	})
}
