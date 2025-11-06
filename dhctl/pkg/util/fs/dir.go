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

package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
	"github.com/google/uuid"
)

func IsDirExists(dir string) bool {
	if dir == "" {
		return false
	}

	stat, err := os.Stat(dir)
	if err != nil {
		return false
	}

	return stat.IsDir()
}

func IsRoot(dir string) bool {
	if runtime.GOOS != "windows" {
		return dir == "/"
	}

	withoutDiskLetter := stringsutil.TrimLeftChars(dir, 1)
	return withoutDiskLetter == ":\\\\"
}

func RandomTmpDirWith10Runes(rootDir, idSalt string, firstIdRunes int) (string, error) {
	if rootDir == "" {
		rootDir = os.TempDir()
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	hash := stringsutil.Sha256Encode(id.String() + idSalt)

	runesCountStr := strconv.Itoa(firstIdRunes)

	// "%.8s"
	f := `%.` + runesCountStr + "s"
	first8Runes := fmt.Sprintf(f, hash)

	return filepath.Join(rootDir, first8Runes), nil
}

// FileExistsInDirAndParentsDirs
// returns empty string if not found otherwise full path
func FileExistsInDirAndParentsDirs(dir, fileName string) (string, error) {
	if fileName == "" || dir == "" {
		return "", fmt.Errorf("file or dir can't be empty")
	}

	if !filepath.IsAbs(filepath.Join(dir)) {
		return "", fmt.Errorf("'%s' is not an absolute path", dir)
	}

	if !IsDirExists(dir) {
		return "", fmt.Errorf("'%s' is not a directory or does not exists", dir)
	}

	parentDir := dir

	for {
		exists, err := IsExists(filepath.Join(parentDir, fileName))
		if err != nil {
			return "", err
		}

		if exists {
			return parentDir, nil
		}

		parentDir = filepath.Dir(parentDir)
		if IsRoot(parentDir) {
			break
		}
	}

	// if pass / return early because we check file in cycle
	if parentDir == dir {
		return "", nil
	}

	exists, err := IsExists(filepath.Join(parentDir, fileName))
	if err != nil {
		return "", err
	}

	if exists {
		return parentDir, nil
	}

	return "", nil
}

var systemDirectories = []string{
	"/bin", "/lost+found", "/run",
	"/etc", "/sbin", "/usr",
	"/boot", "/mnt", "/var",
	"/cdrom", "/lib", "/opt", "/srv",
	"/lib64", "/proc", "/sys", "/dev",
}

func GetSystemDirectories() []string {
	dst := make([]string, len(systemDirectories))
	copy(dst, systemDirectories)

	return dst
}

// IsSystemDirOrUserHome dir must be absolute and cleaned
func IsSystemDirOrUserHome(dir string) (bool, []string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false, nil, err
	}

	additionalSystems := []string{homeDir, "/home", "/media"}

	all := append(additionalSystems, systemDirectories...)
	return slices.Contains(all, dir), all, nil
}

func IsInSystemDirs(dir string) (bool, []string) {
	for _, d := range systemDirectories {
		if strings.HasPrefix(dir, d) {
			return true, GetSystemDirectories()
		}
	}

	return false, GetSystemDirectories()
}
