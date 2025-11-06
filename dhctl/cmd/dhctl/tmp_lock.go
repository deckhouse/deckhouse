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

package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

const dhctlLockTmpFile = ".dhctl-tmp-dir.lock"

type tmpLockCleanupFunc func()

func emptyTmpLockCleanupFunc() {}

func getTmpLockedByErr(existsIn string, tmpDir string) error {
	lockFullPath := filepath.Join(existsIn, dhctlLockTmpFile)
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

// wasTmpLockAcquired returns nil if lock free
func wasTmpDirLockAcquired(tmpDir string) error {
	existsIn, err := fs.FileExistsInDirAndParentsDirs(tmpDir, dhctlLockTmpFile)
	if err != nil {
		return err
	}

	if existsIn == "" {
		return nil
	}

	return getTmpLockedByErr(existsIn, tmpDir)
}

func acquireTmpDirLock(tmpDir string, cmdName string) (tmpLockCleanupFunc, error) {
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

	lockFullPath := filepath.Join(tmpDir, dhctlLockTmpFile)
	err = os.WriteFile(lockFullPath, []byte(acquireBy), 0644)
	if err != nil {
		return nil, fmt.Errorf("Cannot acquire tmp dir lock '%s': %w", lockFullPath, err)
	}

	return func() {
		err := os.Remove(lockFullPath)
		if err != nil && !os.IsNotExist(err) {
			log.GetDefaultLogger().LogWarnF("Cannot remove tmp dir lock '%s': %v\n", lockFullPath, err)
		}
	}, nil
}
