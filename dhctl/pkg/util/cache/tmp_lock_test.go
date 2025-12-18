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
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"

	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

var testTmpDirLockCheckDir = path.Join(os.TempDir(), "dhctl-check-lock-tmp")

func cleanupTmpLockCheckTestDir(logger log.Logger) {
	incorrect := []string{"", ".", ".."}
	if slices.Contains(incorrect, testTmpDirLockCheckDir) {
		return
	}

	if filepath.Clean(testTmpDirLockCheckDir) == "/" {
		return
	}

	if !fs.IsDirExists(testTmpDirLockCheckDir) {
		return
	}

	err := os.RemoveAll(testTmpDirLockCheckDir)

	if err != nil {
		logger.LogErrorF("Error cleaning up test dir '%s': %v\n", testTmpDirLockCheckDir, err)
		return
	}

	logger.LogInfoF("Test dir '%s' was removed\n", testTmpDirLockCheckDir)
}

func TestTmpDirLockAlreadyAcquired(t *testing.T) {
	logger := log.GetDefaultLogger()

	defer cleanupTmpLockCheckTestDir(logger)

	errorPrefix := pointer.String("DHCTL found lock tmp dir file")

	withoutLock := makeDirsFiles{
		makeDirs: []fileDirToCreate{
			{path: "empty"},
			{path: "sub"},
			{path: "sub/sub"},
			{path: "sub/empty"},
		},
		makeFiles: []fileDirToCreate{
			{path: "in_root.txt"},
			{path: ".hidden"},
			{path: "sub/state.json"},
			{path: "sub/state2.json"},
			{path: "sub/sub/another.txt"},
		},
	}

	tests := []tmpDirLockAlreadyAcquiredParams{
		{
			tmpDirLockAlreadyAcquiredBase: tmpDirLockAlreadyAcquiredBase{
				title:  "no lock from root",
				logger: logger,
			},
			dirsFiles:      withoutLock,
			subDirForCheck: "/",
		},

		{
			tmpDirLockAlreadyAcquiredBase: tmpDirLockAlreadyAcquiredBase{
				title:  "no lock in empty subdir",
				logger: logger,
			},
			dirsFiles:      withoutLock,
			subDirForCheck: "empty",
		},

		{
			tmpDirLockAlreadyAcquiredBase: tmpDirLockAlreadyAcquiredBase{
				title:  "no lock in subdir",
				logger: logger,
			},
			dirsFiles:      withoutLock,
			subDirForCheck: "sub",
		},
		{
			tmpDirLockAlreadyAcquiredBase: tmpDirLockAlreadyAcquiredBase{
				title:  "no lock in subdir deeper",
				logger: logger,
			},
			dirsFiles:      withoutLock,
			subDirForCheck: "sub/sub",
		},

		{
			tmpDirLockAlreadyAcquiredBase: tmpDirLockAlreadyAcquiredBase{
				title:       "lock in root",
				logger:      logger,
				errorPrefix: errorPrefix,
			},
			dirsFiles: makeDirsFiles{
				makeDirs: []fileDirToCreate{
					{path: "empty"},
					{path: "sub"},
					{path: "sub/sub"},
					{path: "sub/empty"},
				},
				makeFiles: []fileDirToCreate{
					{path: ".dhctl-tmp-dir.lock"},
					{path: "in_root.txt"},
					{path: ".hidden"},
					{path: "sub/state2.json"},
					{path: "sub/sub/another.txt"},
				},
			},
			subDirForCheck: "/",
		},

		{
			tmpDirLockAlreadyAcquiredBase: tmpDirLockAlreadyAcquiredBase{
				title:       "lock in subdir",
				logger:      logger,
				errorPrefix: errorPrefix,
			},
			dirsFiles: makeDirsFiles{
				makeDirs: []fileDirToCreate{
					{path: "empty"},
					{path: "sub"},
					{path: "sub/sub"},
					{path: "sub/empty"},
				},
				makeFiles: []fileDirToCreate{
					{path: "in_root.txt"},
					{path: ".hidden"},
					{path: "sub/.dhctl-tmp-dir.lock"},
					{path: "sub/state2.json"},
					{path: "sub/sub/another.txt"},
				},
			},
			subDirForCheck: "/",
		},

		{
			tmpDirLockAlreadyAcquiredBase: tmpDirLockAlreadyAcquiredBase{
				title:       "lock in subdir deeper",
				logger:      logger,
				errorPrefix: errorPrefix,
			},
			dirsFiles: makeDirsFiles{
				makeDirs: []fileDirToCreate{
					{path: "empty"},
					{path: "sub"},
					{path: "sub/sub"},
					{path: "sub/empty"},
				},
				makeFiles: []fileDirToCreate{
					{path: "in_root.txt"},
					{path: ".hidden"},
					{path: "sub/state2.json"},
					{path: "sub/sub/.dhctl-tmp-dir.lock"},
					{path: "sub/sub/another.txt"},
				},
			},
			subDirForCheck: "/",
		},

		{
			tmpDirLockAlreadyAcquiredBase: tmpDirLockAlreadyAcquiredBase{
				title:       "lock in parent",
				logger:      logger,
				errorPrefix: errorPrefix,
			},
			dirsFiles: makeDirsFiles{
				makeDirs: []fileDirToCreate{
					{path: "empty"},
					{path: "sub"},
					{path: "sub/sub"},
					{path: "sub/empty"},
				},
				makeFiles: []fileDirToCreate{
					{path: ".dhctl-tmp-dir.lock"},
					{path: "in_root.txt"},
					{path: ".hidden"},
					{path: "sub/state2.json"},
					{path: "sub/sub/another.txt"},
				},
			},
			subDirForCheck: "sub/",
		},

		{
			tmpDirLockAlreadyAcquiredBase: tmpDirLockAlreadyAcquiredBase{
				title:       "lock in parent deeper",
				logger:      logger,
				errorPrefix: errorPrefix,
			},
			dirsFiles: makeDirsFiles{
				makeDirs: []fileDirToCreate{
					{path: "empty"},
					{path: "sub"},
					{path: "sub/sub"},
					{path: "sub/empty"},
				},
				makeFiles: []fileDirToCreate{
					{path: ".dhctl-tmp-dir.lock"},
					{path: "in_root.txt"},
					{path: ".hidden"},
					{path: "sub/state2.json"},
					{path: "sub/sub/another.txt"},
				},
			},
			subDirForCheck: "sub/sub/",
		},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			tt := createTmpDirLockAlreadyAcquiredTest(t, test)
			err := TmpDirLockAlreadyAcquired(tt.dirForCheck)
			if tt.errorPrefix == nil {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.Contains(t, err.Error(), *test.errorPrefix)
		})
	}
}

type tmpDirLockAlreadyAcquiredBase struct {
	title       string
	errorPrefix *string
	logger      log.Logger
}

type tmpDirLockAlreadyAcquiredParams struct {
	tmpDirLockAlreadyAcquiredBase

	dirsFiles      makeDirsFiles
	subDirForCheck string
}

type tmpDirLockAlreadyAcquiredTest struct {
	tmpDirLockAlreadyAcquiredBase

	dirForCheck string
	logger      log.Logger
}

func createTmpDirLockAlreadyAcquiredTest(t *testing.T, params tmpDirLockAlreadyAcquiredParams) tmpDirLockAlreadyAcquiredTest {
	require.NotEmpty(t, params.title)
	require.NotEmpty(t, params.subDirForCheck)
	require.False(t, govalue.IsNil(params.logger))

	rootDir, err := fs.RandomTmpDirWith10Runes(testTmpDirLockCheckDir, params.title, 8)
	require.NoError(t, err)

	params.dirsFiles.makeAll(t, rootDir, params.logger, rootDir)

	return tmpDirLockAlreadyAcquiredTest{
		tmpDirLockAlreadyAcquiredBase: tmpDirLockAlreadyAcquiredBase{
			title:       params.title,
			errorPrefix: params.errorPrefix,
			logger:      params.logger,
		},

		dirForCheck: filepath.Join(rootDir, params.subDirForCheck),
	}
}
