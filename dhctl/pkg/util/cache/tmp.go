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
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const cleanupErrorPrefix = "Error during cleanup tmp dir:"

type ClearTmpParams struct {
	IsDebug         bool
	RemoveTombStone bool

	TmpDir        string
	DefaultTmpDir string

	LoggerProvider log.LoggerProvider
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
	loggerProvider log.LoggerProvider
	msg            string
}

func NewDummyTmpCleaner(loggerProvider log.LoggerProvider, msg string) *DummyTmpCleaner {
	return &DummyTmpCleaner{
		loggerProvider: loggerProvider,
		msg:            msg,
	}
}

func (d *DummyTmpCleaner) Cleanup() {
	if d.msg != "" {
		log.SafeProvideLogger(d.loggerProvider).LogInfoLn(d.msg)
	}
}

func (d *DummyTmpCleaner) DisableCleanup(msg string) {
	d.msg = msg
}

type regularTmpCleaner struct {
	params          *ClearTmpParams
	suffixesForSkip []string

	disableCleanup    bool
	disableCleanupMsg string
}

func newRegularTmpCleaner(params *ClearTmpParams, suffixesForSkip []string) *regularTmpCleaner {
	return &regularTmpCleaner{
		params:          params,
		suffixesForSkip: suffixesForSkip,
		disableCleanup:  false,
	}
}

func (r *regularTmpCleaner) Cleanup() {
	logger := log.SafeProvideLogger(r.params.LoggerProvider)

	if r.disableCleanup {
		// lock file will deleted by callback returned from AcquireTmpDirLock
		logger.LogDebugF(
			"Cleanup tmp dir '%s' was skipped with reason: %s\n",
			r.params.TmpDir,
			r.disableCleanupMsg,
		)
		return
	}

	tmpDir := r.params.TmpDir

	logger.LogDebugF("Clean temp dir '%s' started\n", tmpDir)
	defer func() {
		logger.LogDebugF("Clean temp dir '%s' was finished\n", tmpDir)
	}()

	// do not clean tmp dir, because user may need temporary files to debug infra
	dirsForDeletion := make([]string, 0)
	keepFiles := make([]string, 0)
	removeFiles := make([]string, 0)
	lockFiles := make([]string, 0)

	skipDirs := []string{
		tmpDir,
		r.params.DefaultTmpDir,
	}

	err := filepath.Walk(tmpDir, func(fullPath string, info os.FileInfo, err error) error {
		if err != nil {
			logger.LogDebugF("%s %s because walk returns err: %v\n", cleanupErrorPrefix, fullPath, err)
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

			if slices.Contains(skipDirs, fullPath) {
				logger.LogDebugF("Skip cleaning dir '%s'\n", fullPath)
				return nil
			}

			dirsForDeletion = append(dirsForDeletion, fullPath)
			return nil
		}

		if strings.HasSuffix(fullPath, lockTmpDirFile) {
			lockFiles = append(lockFiles, fullPath)
			return nil
		}

		for _, suffix := range r.suffixesForSkip {
			if strings.HasSuffix(fullPath, suffix) {
				keepFiles = append(keepFiles, fullPath)
				return nil
			}
		}

		removeFiles = append(removeFiles, fullPath)

		return nil
	})

	if err != nil {
		logger.LogDebugF("%s walking '%s' got error: %v\n", cleanupErrorPrefix, tmpDir, err)
		return
	}

	// lock file will delete after all
	if len(lockFiles) > 1 {
		logger.LogWarnF("%s found multiple lock files: %v. Skip cleaning %s\n", cleanupErrorPrefix, lockFiles, tmpDir)
		return
	}

	sortByDepthDescending(removeFiles)
	for _, fullPath := range removeFiles {
		remove(fullPath, logger, "Delete tmp file")
	}

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

	logger.LogDebugF("Cleaning temp dir. Keep next files: %v\nDirs for deletion: %v\n", keepFiles, dirsForDeletion)

	sortByDepthDescending(dirsForDeletion)
	for _, dir := range dirsForDeletion {
		if skipDeleteDir(dir) {
			logger.LogDebugF("Skip cleaning temp sub dir '%s'\n", dir)
			continue
		}

		remove(dir, logger, "Delete tmp sub dir")
	}

	// we have one or nothing lock files here
	if len(lockFiles) != 0 {
		remove(lockFiles[0], logger, "Delete tmp dir lock file")
	}

	if len(keepFiles) == 0 && tmpDir != r.params.DefaultTmpDir {
		remove(tmpDir, logger, "Delete tmp dir")
	} else {
		logger.LogDebugF("Cleaning temp dir '%s' skipeed because it default or have keept files %d\n", tmpDir, len(keepFiles))
	}
}

func (r *regularTmpCleaner) DisableCleanup(msg string) {
	r.disableCleanup = true

	if msg == "" {
		return
	}

	r.disableCleanupMsg = msg
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

func remove(fullPath string, logger log.Logger, msg string) {
	logger.LogDebugF("%s: '%s'\n", msg, fullPath)
	err := os.Remove(fullPath)
	if err != nil && !os.IsNotExist(err) {
		logger.LogInfoF("%s % did not success'%s': %v\n", cleanupErrorPrefix, msg, fullPath, err)
	}
}
