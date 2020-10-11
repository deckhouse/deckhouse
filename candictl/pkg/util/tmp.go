package util

import (
	"os"
	"path/filepath"

	"flant/candictl/pkg/app"
)

func ClearTMPDir() {
	// do not clean tmp dir, because user may need temporary files to debug terraform
	if app.IsDebug {
		return
	}

	_ = filepath.Walk(app.TmpDirName, func(path string, info os.FileInfo, err error) error {
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
