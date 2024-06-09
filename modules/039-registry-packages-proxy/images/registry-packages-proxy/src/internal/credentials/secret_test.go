package credentials

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToClientConfig(t *testing.T) {
	t.Run("Path with leading slash", func(t *testing.T) {
		sd := registrySecretData{
			Address: "registry.deckhouse.io",
			Path:    "/deckhouse/ee",
		}
		c, err := sd.toClientConfig()
		require.NoError(t, err)
		require.Equal(t, c.Repository, "registry.deckhouse.io/deckhouse/ee")
	})
	t.Run("Path without leading slash", func(t *testing.T) {
		sd := registrySecretData{
			Address: "registry.deckhouse.io",
			Path:    "deckhouse/ee",
		}
		c, err := sd.toClientConfig()
		require.NoError(t, err)
		require.Equal(t, c.Repository, "registry.deckhouse.io/deckhouse/ee")
	})
}
