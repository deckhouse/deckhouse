package registryclient

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadAuthConfig(t *testing.T) {
	authraw := `
{
  "auths": {
    "registry-1.deckhouse.io": {
      "auth": "YTpiCg=="
    },
    "registry-2.deckhouse.io": {
      "auth": "YTpiCg=="
    }
  }
}
`

	_, err := readAuthConfig("registry-1.deckhouse.io/module/foo/bar", authraw)
	require.NoError(t, err)

	_, err = readAuthConfig("registry-invalid.deckhouse.io/module/foo/bar", authraw)
	require.Error(t, err)
}
