package cache

import (
	"os"
	"path/filepath"

	"flant/candictl/pkg/app"
)

func ClearTerraformDir() {
	// do not clean tmp dir, because user may need temporary files to debug terraform
	if app.IsDebug {
		return
	}

	_ = os.RemoveAll(filepath.Join(app.TmpDirName, "tf_candictl"))
}

func ClearTemporaryDirs() {
	// do not clean tmp dir, because user may need temporary files to debug terraform
	if app.IsDebug {
		return
	}

	_ = filepath.Walk(app.TmpDirName, func(path string, info os.FileInfo, err error) error {
		// If tmp folder doesn't exist
		if info == nil {
			return nil
		}
		if info.IsDir() {
			if path != app.TmpDirName {
				return filepath.SkipDir
			}
			return nil
		}

		_ = os.Remove(path)
		return nil
	})
}
