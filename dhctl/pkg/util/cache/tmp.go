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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const errorPrefix = "Error during cleanup tmp dir:"

type LoggerProvider func() log.Logger

type ClearTmpParams struct {
	IsDebug         bool
	RemoveTombStone bool

	TmpDir        string
	DefaultTmpDir string

	LoggerProvider LoggerProvider
}

type TmpCleaner interface {
	Cleanup()
	DisableCleanup(msg string)
}

var (
	globalTmpCleanerMutex sync.Mutex
	globalTmpCleaner      TmpCleaner
)

func GetGlobalTmpCleaner() TmpCleaner {
	globalTmpCleanerMutex.Lock()
	defer globalTmpCleanerMutex.Unlock()

	if govalue.IsNil(globalTmpCleaner) {
		return NewDummyTmpCleaner(nil, "")
	}

	return globalTmpCleaner
}

func SetGlobalTmpCleaner(c TmpCleaner) {
	globalTmpCleanerMutex.Lock()
	defer globalTmpCleanerMutex.Unlock()

	globalTmpCleaner = c
}

func NewTmpCleaner(params ClearTmpParams) TmpCleaner {
	if params.IsDebug {
		msg := fmt.Sprintf("Skip cleaning temp dir '%s' because dhctl work in debug mode\n", params.TmpDir)
		return NewDummyTmpCleaner(params.LoggerProvider, msg)
	}

	tmpDir := params.TmpDir

	if tmpDir != "" {
		tmpDir = path.Clean(tmpDir)
	}

	if tmpDir == "" || tmpDir == "/" || tmpDir == "." || tmpDir == ".." {
		msg := fmt.Sprintf("Skip clean tmp dir because pass empty tmp dir or incorrect: '%s'\n", params.TmpDir)
		return NewDummyTmpCleaner(params.LoggerProvider, msg)
	}

	suffixesForSkip := []string{
		".log",
	}

	if !params.RemoveTombStone {
		suffixesForSkip = append(suffixesForSkip, state.TombstoneKey)
	}

	paramsCopy := params
	paramsCopy.TmpDir = tmpDir

	return newRegularTmpCleaner(&paramsCopy, suffixesForSkip)
}

type DummyTmpCleaner struct {
	loggerProvider LoggerProvider
	msg            string
}

func NewDummyTmpCleaner(loggerProvider LoggerProvider, msg string) *DummyTmpCleaner {
	return &DummyTmpCleaner{
		loggerProvider: loggerProvider,
		msg:            msg,
	}
}

func (d *DummyTmpCleaner) Cleanup() {
	if d.msg != "" {
		safeLoggerProvider(d.loggerProvider).LogInfoLn(d.msg)
	}
}

func (d *DummyTmpCleaner) DisableCleanup(msg string) {
	d.msg = msg
}

type regularTmpCleaner struct {
	params            *ClearTmpParams
	suffixesForSkip   []string
	disableCleanupMsg string
}

func newRegularTmpCleaner(params *ClearTmpParams, suffixesForSkip []string) *regularTmpCleaner {
	return &regularTmpCleaner{
		params:          params,
		suffixesForSkip: suffixesForSkip,
	}
}

func (r *regularTmpCleaner) Cleanup() {
	logger := safeLoggerProvider(r.params.LoggerProvider)

	if r.disableCleanupMsg != "" {
		logger.LogDebugF("Disable regular cleanup: %s\n", r.disableCleanupMsg)
		return
	}

	tmpDir := r.params.TmpDir

	logger.LogDebugF("Clear temp dir: %s\n", tmpDir)
	// do not clean tmp dir, because user may need temporary files to debug infra
	dirsForDeletion := make([]string, 0)
	keepFiles := make([]string, 0)

	err := filepath.Walk(tmpDir, func(fullPath string, info os.FileInfo, err error) error {
		if err != nil {
			log.DebugF("%s %s because walk returns err: %v\n", errorPrefix, fullPath, err)
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

			if fullPath == r.params.DefaultTmpDir {
				logger.LogDebugF("Skip cleaning default temp dir '%s'\n", fullPath)
				return nil
			}

			dirsForDeletion = append(dirsForDeletion, fullPath)
			return nil
		}

		for _, suffix := range r.suffixesForSkip {
			if strings.HasSuffix(fullPath, suffix) {
				keepFiles = append(keepFiles, fullPath)
				return nil
			}
		}

		logger.LogDebugF("Delete tmp file '%s'\n", fullPath)
		err = os.Remove(fullPath)
		if err != nil {
			logger.LogDebugF("%s file did not deleted '%s': %v\n", errorPrefix, fullPath, err)
		}
		return nil
	})

	if err != nil {
		logger.LogDebugF("%s walking '%s' got error: %v\n", errorPrefix, tmpDir, err)
	}

	sortByDepthDescending(dirsForDeletion)

	skipDeleteDir := func(dir string) bool {
		for _, keep := range keepFiles {
			// Check if the directory is an ancestor of the kept file
			keepDir := filepath.Dir(keep)
			if keepDir == dir || strings.HasPrefix(keepDir, dir) {
				return true
			}
		}

		return false
	}

	log.DebugF("Cleaning temp dir. Keep next files: %v\nDirs for deletion: %v\n", keepFiles, dirsForDeletion)

	for _, dir := range dirsForDeletion {
		if skipDeleteDir(dir) {
			logger.LogDebugF("Skip cleaning temp sub dir '%s'\n", dir)
			continue
		}

		err := os.Remove(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				logger.LogDebugF("%s directory '%s' deleting returns error: %v\n", errorPrefix, dir, err)
			}
		}
	}
}

func (r *regularTmpCleaner) DisableCleanup(msg string) {
	if msg == "" {
		return
	}

	r.disableCleanupMsg = msg
}

func safeLoggerProvider(provider LoggerProvider) log.Logger {
	if provider != nil {
		logger := provider()
		if !govalue.IsNil(logger) {
			return logger
		}
	}

	return log.GetDefaultLogger()
}

// sortByDepthDescending sorts paths by depth (number of slashes) in descending order
// This ensures that deeper directories are deleted first, preventing "directory not empty" errors
func sortByDepthDescending(paths []string) {
	if len(paths) == 0 {
		return
	}

	sort.Slice(paths, func(i, j int) bool {
		depthI := strings.Count(paths[i], string(filepath.Separator))
		depthJ := strings.Count(paths[j], string(filepath.Separator))

		if depthI != depthJ {
			return depthI > depthJ
		}

		return paths[i] > paths[j]
	})
}
