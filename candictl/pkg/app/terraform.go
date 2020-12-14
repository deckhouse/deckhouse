package app

import (
	"os"
	"path/filepath"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	CacheDir  = filepath.Join(os.TempDir(), "candictl")
	DropCache = false
)

func DefineCacheFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("cache-dir", "Directory to store the cache.").
		Envar(configEnvName("CACHE_DIR")).
		StringVar(&CacheDir)
}

func DefineDropCacheFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("yes-i-want-to-drop-cache", "All cached information will be deleted from your local cache.").
		Default("false").
		BoolVar(&DropCache)
}
