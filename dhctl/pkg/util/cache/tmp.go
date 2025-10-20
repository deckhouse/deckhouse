// Copyright 2021 Flant JSC
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
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

// sortByDepthDescending sorts paths by depth (number of slashes) in descending order
// This ensures that deeper directories are deleted first, preventing "directory not empty" errors
func sortByDepthDescending(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		depthI := strings.Count(paths[i], string(filepath.Separator))
		depthJ := strings.Count(paths[j], string(filepath.Separator))

		if depthI != depthJ {
			return depthI > depthJ
		}

		return paths[i] > paths[j]
	})
}

type ClearTmpParams struct {
	IsDebug          bool
	RemoveTombStone  bool
	PreserveStateDir bool

	TmpDir        string
	DefaultTmpDir string

	LoggerProvider func() log.Logger
}

func defaultLoggerProvider() log.Logger {
	return log.GetDefaultLogger()
}

func GetClearTemporaryDirsFunc(params ClearTmpParams) func() {
	loggerProvider := params.LoggerProvider
	if loggerProvider == nil {
		loggerProvider = defaultLoggerProvider
	}

	logger := loggerProvider()
	if govalue.IsNil(logger) {
		logger = defaultLoggerProvider()
	}

	tmpDir := path.Clean(params.TmpDir)

	if tmpDir == "" || tmpDir == "/" || tmpDir == "." {
		return func() {
			logger.LogDebugF("Skip clean tmp dir because pass empty tmp dir or incorrect: '%s'", tmpDir)
		}
	}

	suffixesForSkip := []string{
		".log",
	}

	if !params.RemoveTombStone {
		suffixesForSkip = append(suffixesForSkip, state.TombstoneKey)
	}

	return func() {
		logger.LogDebugF("Clear temp dir: %s\n", tmpDir)
		// do not clean tmp dir, because user may need temporary files to debug infra
		if params.IsDebug {
			logger.LogDebugF("Skip cleaning temp dir '%s' because dhctl work in debug mode\n", tmpDir)
			return
		}

		dirsForDeletion := make([]string, 0)
		keepFiles := make([]string, 0)

		err := filepath.Walk(params.TmpDir, func(fullPath string, info os.FileInfo, err error) error {
			if err != nil {
				log.DebugF("Skip cleaning temp %s because walk returns err: %v\n", fullPath, err)
				return nil
			}

			// If tmp folder doesn't exist
			if info == nil {
				return nil
			}

			if info.IsDir() {
				if fullPath == "/" {
					logger.LogWarnF("Found root dir '/' Skip all\n")
					return filepath.SkipDir
				}

				if fullPath == params.DefaultTmpDir {
					logger.LogDebugF("Skip cleaning default temp dir '%s'\n", fullPath)
					return nil
				}

				// Skip state directories if PreserveStateDir is enabled
				if params.PreserveStateDir {
					// Check if this directory contains tombstone or other state files
					tombstonePath := filepath.Join(fullPath, state.TombstoneKey)
					if _, err := os.Stat(tombstonePath); err == nil {
						logger.LogDebugF("Skip cleaning state dir '%s' (contains tombstone)\n", fullPath)
						return filepath.SkipDir
					}

					// Check if this directory has any state files (non-log files)
					entries, err := os.ReadDir(fullPath)
					if err == nil {
						hasStateFiles := false
						for _, entry := range entries {
							if !entry.IsDir() && !strings.HasSuffix(entry.Name(), ".log") {
								hasStateFiles = true
								break
							}
						}
						if hasStateFiles {
							logger.LogDebugF("Skip cleaning state dir '%s' (contains state files)\n", fullPath)
							return filepath.SkipDir
						}
					}
				}

				dirsForDeletion = append(dirsForDeletion, fullPath)
				return nil
			}

			for _, suffix := range suffixesForSkip {
				if strings.HasSuffix(fullPath, suffix) {
					keepFiles = append(keepFiles, fullPath)
					return nil
				}
			}

			err = os.Remove(fullPath)
			if err != nil {
				logger.LogDebugF("Error deleting temp file '%s': %v\n", fullPath, err)
			}
			return nil
		})

		if err != nil {
			logger.LogDebugF("Error cleaning temp dir while walking '%s': %v\n", tmpDir, err)
		}

		sortByDepthDescending(dirsForDeletion)

		skipDeleteDir := func(dir string) bool {
			for _, keep := range keepFiles {
				// Check if the directory is an ancestor of the kept file
				keepDir := filepath.Dir(keep)
				if keepDir == dir {
					return true
				}
				// Also check if the directory is a parent directory of the kept file
				rel, err := filepath.Rel(dir, keep)
				if err == nil && !strings.HasPrefix(rel, "..") {
					return true
				}
			}

			return false
		}

		log.DebugF("Cleaning temp dir. Keep next files: %v\nDirs for deletion: %v\n", keepFiles, dirsForDeletion)

		for _, dir := range dirsForDeletion {
			if skipDeleteDir(dir) {
				logger.LogDebugF("Skip cleaning temp dir '%s'\n", dir)
				continue
			}

			err := os.Remove(dir)
			if err != nil {
				if !os.IsNotExist(err) {
					logger.LogDebugF("Error cleaning temp dir '%s': %v\n", dir, err)
				}
			}
		}
	}
}
