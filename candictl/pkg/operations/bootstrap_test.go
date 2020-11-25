package operations

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"flant/candictl/pkg/config"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/util/cache"
)

func TestBootstrapGetNodesFromCache(t *testing.T) {
	log.InitLogger("simple")
	dir, err := ioutil.TempDir(os.TempDir(), "candictl-test-bootstrap-*")
	defer os.Remove(dir)

	require.NoError(t, err)

	for _, name := range []string{
		"base-infrastructure.tfstate",
		"some_trash",
		"test-master-0.tfstate",
		"test-master-1.tfstate",
		"test-master-without-index.tfstate",
		"test-master-1.tfstate.backup",
		"uuid.tfstate",
		"test-static-ingress-0.tfstate",
	} {
		_, err := os.Create(filepath.Join(dir, name))
		require.NoError(t, err)
	}

	t.Run("Should get only nodes state from cache", func(t *testing.T) {
		stateCache, err := cache.NewStateCache(dir)
		require.NoError(t, err)

		result, err := BootstrapGetNodesFromCache(&config.MetaConfig{ClusterPrefix: "test"}, stateCache)
		require.NoError(t, err)

		require.Len(t, result["master"], 2)
		require.Len(t, result["static-ingress"], 1)

		require.Equal(t, "test-master-0", result["master"][0])
		require.Equal(t, "test-master-1", result["master"][1])

		require.Equal(t, "test-static-ingress-0", result["static-ingress"][0])
	})
}
