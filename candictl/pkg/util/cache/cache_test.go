package cache

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/log"
)

func TestNewTempStateCache(t *testing.T) {
	log.InitLogger("simple")

	dir, err := ioutil.TempDir(os.TempDir(), "candictl-test-cache-*")
	require.NoError(t, err)
	app.CacheDir = dir

	defer os.RemoveAll(dir)

	t.Run("Save load delete operations", func(t *testing.T) {
		stateCache, err := NewTempStateCache("test-identity")
		require.NoError(t, err)

		require.Equal(t,
			"f2883e125771cadcd9918a8d991d8002c8b5b0c52dbe9811f89f3f4eee53ccab",
			filepath.Base(stateCache.dir),
		)

		stateCache.Save("test", []byte(`test-1`))
		stateCache.Save("test.tfstate", []byte(`test-2`))
		stateCache.Save("test2.tfstate", []byte(`test-3`))

		require.Equal(t, true, stateCache.InCache("test"))
		require.Equal(t, true, stateCache.InCache("test.tfstate"))
		require.Equal(t, true, stateCache.InCache("test2.tfstate"))

		require.Equal(t, []byte("test-1"), stateCache.Load("test"))
		require.Equal(t, []byte("test-2"), stateCache.Load("test.tfstate"))
		require.Equal(t, []byte("test-3"), stateCache.Load("test2.tfstate"))

		structForTest := map[string]int{"abc": 10, "def": 1000, "xyz": 10}
		err = stateCache.SaveStruct("test-struct", structForTest)
		require.NoError(t, err)

		var test map[string]int
		err = stateCache.LoadStruct("test-struct", &test)
		require.NoError(t, err)

		require.Equal(t, structForTest, test)

		var objectsInCache []string
		err = stateCache.Iterate(func(s string, _ []byte) error {
			objectsInCache = append(objectsInCache, s)
			return nil
		})
		require.NoError(t, err)

		require.Equal(t, []string{"test", "test-struct", "test.tfstate", "test2.tfstate"}, objectsInCache)

		stateCache.Delete("test")
		var objectsInCacheAfterDelete []string
		err = stateCache.Iterate(func(s string, _ []byte) error {
			objectsInCacheAfterDelete = append(objectsInCacheAfterDelete, s)
			return nil
		})
		require.NoError(t, err)

		require.Equal(t, []string{"test-struct", "test.tfstate", "test2.tfstate"}, objectsInCacheAfterDelete)

		stateCache.Clean()

		var objectsInCacheAfterClean []string
		err = stateCache.Iterate(func(s string, _ []byte) error {
			objectsInCacheAfterClean = append(objectsInCacheAfterClean, s)
			return nil
		})
		require.NoError(t, err)

		require.Equal(t, true, stateCache.InCache(".tombstone"))
	})
}
