/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pkg

import (
	"fmt"
	"os"
	"path/filepath"
)

func IsPathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	} else {
		fmt.Println("Error checking for file or directory existence:", err)
		return false
	}
}

func MkdirAllForFile(filePath string, dirPerm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(filePath), dirPerm); err != nil {
		return err
	}
	return nil
}
