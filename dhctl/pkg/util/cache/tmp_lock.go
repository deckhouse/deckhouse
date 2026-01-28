// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

const (
	lockTmpDirFile = ".dhctl-tmp-dir.lock"
)

type ReleaseLockFunc func()

func getLockFullPath(dir string) string {
	return filepath.Join(dir, lockTmpDirFile)
}

func getTmpLockedByErr(existsIn string, tmpDir string) error {
	lockFullPath := getLockFullPath(existsIn)
	lockedBy, err := os.ReadFile(lockFullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// does not exist. try to lock
			return nil
		}

		return fmt.Errorf("Cannot read tmp dir lock file '%s': %w", lockFullPath, err)
	}

	msg := `DHCTL found lock tmp dir file '%s' for tmp dir '%s'.' Probably another dhctl instance '%s' running. Please pass another directory.`
	return fmt.Errorf(msg, lockFullPath, tmpDir, string(lockedBy))
}

// findLockInSubDirs returns nil if not found
func findLockInSubDirs(tmpDir string) error {
	return filepath.Walk(tmpDir, func(fullPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// If tmp folder doesn't exist or dir
		if info == nil || info.IsDir() {
			return nil
		}

		if strings.HasSuffix(fullPath, lockTmpDirFile) {
			return getTmpLockedByErr(filepath.Dir(fullPath), tmpDir)
		}

		return nil
	})
}

// TmpDirLockAlreadyAcquired returns nil if lock free
func TmpDirLockAlreadyAcquired(tmpDir string) error {
	existsIn, err := fs.FileExistsInDirAndParentsDirs(tmpDir, lockTmpDirFile)
	if err != nil {
		return err
	}

	if existsIn == "" {
		return findLockInSubDirs(tmpDir)
	}

	return getTmpLockedByErr(existsIn, tmpDir)
}

func AcquireTmpDirLock(tmpDir string, loggerProvider log.LoggerProvider, cmdName string) (ReleaseLockFunc, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	username := "unknown"
	osUser, err := user.Current()
	if err == nil {
		username = osUser.Username
	}

	acquireBy := fmt.Sprintf("%s@%s $ dhctl %s", username, hostname, cmdName)

	lockFullPath := getLockFullPath(tmpDir)
	err = os.WriteFile(lockFullPath, []byte(acquireBy), 0644)
	if err != nil {
		return nil, fmt.Errorf("Cannot acquire tmp dir lock '%s': %w", lockFullPath, err)
	}

	return func() {
		err := os.Remove(lockFullPath)
		if err != nil && !os.IsNotExist(err) {
			log.SafeProvideLogger(loggerProvider).LogWarnF("Cannot remove tmp dir lock '%s': %v\n", lockFullPath, err)
		}
	}, nil
}
