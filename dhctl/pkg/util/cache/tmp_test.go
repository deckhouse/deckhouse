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
	"path"
	"path/filepath"
	"sort"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

func TestClearAllInSubDir(t *testing.T) {
	app.IsDebug = true

	files := []fileDirToCreate{
		{path: "in_root.txt"},
		{path: "in_root2.txt"},
		{path: "state/instate.txt"},
		{path: "state/instate.txt"},
		{path: "state/.tombstone"},
		{path: "state/instate2.txt"},
		{path: "state/subdir/subdir.json"},
		{path: "state/subdir/sub/file"},
	}

	dirs := []fileDirToCreate{
		{path: "state"},
		{path: "state/iefiwhguwurguhurgog4ogoggo4g5ohwrfjiorjgf842h3hfu34hgh4hg4hg4hgh54gh45h4"},
		{path: "state/empty"},
		{path: "state/subdir"},
		{path: "state/subdir/sub"},
	}

	params := testClearFuncParams{
		testName:              "TestClearAllInSubDir",
		isDebug:               false,
		tmpSubDir:             "allInSubDir",
		defaultTmpDirAsSubdir: true,
		removeTombstones:      true,
		makeDirs:              dirs,
		makeFiles:             files,
	}

	f := getTestClearFunc(t, params)

	defer func() {
		clearTest(t, f)
	}()

	f.clear()

	all := make([]fileDirToCreate, 0, len(files)+len(dirs))
	all = append(all, files...)
	all = append(all, dirs...)

	assertRemovedAll(t, f, all)
}

type fileDirToCreate struct {
	path           string
	outsideTmpRoot bool
}

func sortFileDirToCreate(l []fileDirToCreate) []fileDirToCreate {
	if len(l) == 0 {
		return make([]fileDirToCreate, 0)
	}

	dst := make([]fileDirToCreate, len(l))
	copy(dst, l)

	sort.SliceStable(dst, func(i, j int) bool {
		return dst[i].path < dst[j].path
	})

	return dst
}

type testClearFuncParams struct {
	testName              string
	isDebug               bool
	tmpSubDir             string
	defaultTmpDirAsSubdir bool
	removeTombstones      bool

	makeDirs  []fileDirToCreate
	makeFiles []fileDirToCreate
}

type testFunc struct {
	clear       func()
	tmpRoot     string
	clearParams ClearTmpParams
	logger      log.Logger
	testName    string
}

func testMkDir(t *testing.T, dir string) {
	t.Helper()

	err := os.MkdirAll(dir, 0o777)
	require.NoError(t, err)
}

func testMkFile(t *testing.T, file string) {
	t.Helper()

	id, err := uuid.NewRandom()
	require.NoError(t, err)

	err = os.WriteFile(file, []byte(id.String()), 0644)
	require.NoError(t, err)
}

func getTestClearFunc(t *testing.T, params testClearFuncParams) testFunc {
	t.Helper()

	require.NotEmpty(t, params.testName)

	id, err := uuid.NewRandom()
	require.NoError(t, err)

	hash := stringsutil.Sha256Encode(id.String() + params.testName)
	first8Runes := fmt.Sprintf("%.8s", hash)

	testTmpDir := filepath.Join(os.TempDir(), "dhctl-clear-tmp-tests", first8Runes)
	testMkDir(t, testTmpDir)

	logger := log.GetDefaultLogger()
	logger.LogInfoF("Tmp dir for test %s is %s\n", params.testName, testTmpDir)

	tmpDir := testTmpDir
	if params.tmpSubDir != "" {
		tmpDir = filepath.Join(testTmpDir, params.tmpSubDir)
		testMkDir(t, tmpDir)
	}

	fullPathToCreate := func(f fileDirToCreate) string {
		fullPath := filepath.Join(tmpDir, f.path)
		if f.outsideTmpRoot {
			fullPath = filepath.Join(testTmpDir, f.path)
		}

		return fullPath
	}

	makeDirs := sortFileDirToCreate(params.makeDirs)
	for _, dir := range makeDirs {
		fullPath := fullPathToCreate(dir)
		logger.LogInfoF("Create dir %s\n", fullPath)
		testMkDir(t, fullPath)
	}

	makeFiles := sortFileDirToCreate(params.makeFiles)
	for _, file := range makeFiles {
		fullPath := fullPathToCreate(file)
		logger.LogInfoF("Create file %s\n", fullPath)
		testMkFile(t, fullPath)
	}

	defaultTmpDir := testTmpDir
	if params.defaultTmpDirAsSubdir {
		defaultTmpDir = tmpDir
	}

	clearParams := ClearTmpParams{
		IsDebug:         params.isDebug,
		TmpDir:          tmpDir,
		RemoveTombStone: params.removeTombstones,
		DefaultTmpDir:   defaultTmpDir,
		LoggerProvider: func() log.Logger {
			return logger
		},
	}

	return testFunc{
		clear:       GetClearTemporaryDirsFunc(clearParams),
		tmpRoot:     testTmpDir,
		clearParams: clearParams,
		logger:      logger,
		testName:    params.testName,
	}
}

func clearTest(t *testing.T, params testFunc) {
	t.Helper()

	require.NotEqual(t, path.Clean(params.tmpRoot), "/", params.testName)

	err := os.RemoveAll(params.tmpRoot)
	if err != nil {
		params.logger.LogErrorF("Couldn't remove tmp dir '%s' for test %s: %v", params.tmpRoot, params.testName, err)
		return
	}

	params.logger.LogInfoF("Tmp dir %s for test %s was removed\n", params.tmpRoot, params.testName)
}

func assertRemovedAll(t *testing.T, f testFunc, l []fileDirToCreate) {
	t.Helper()

	require.NotEmpty(t, l)
	require.NotEmpty(t, f.clearParams.TmpDir)

	for _, e := range l {
		fullPath := filepath.Join(f.clearParams.TmpDir, e.path)
		_, err := os.Stat(fullPath)
		require.Error(t, err, fullPath)
		require.True(t, os.IsNotExist(err), fullPath)
	}
}

func TestSortByDepthDescending(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "basic depth sorting",
			input: []string{
				"/tmp/a",
				"/tmp/a/b",
				"/tmp/a/b/c",
				"/tmp/x",
			},
			expected: []string{
				"/tmp/a/b/c",
				"/tmp/a/b",
				"/tmp/x",
				"/tmp/a",
			},
		},
		{
			name: "paths with slashes in directory names",
			input: []string{
				"/tmp/dir",
				"/tmp/dir/sub",
				"/tmp/dir-with/slash",
				"/tmp/dir-with/slash/deep",
				"/tmp/dir-with",
			},
			expected: []string{
				"/tmp/dir-with/slash/deep",
				"/tmp/dir/sub",
				"/tmp/dir-with/slash",
				"/tmp/dir-with",
				"/tmp/dir",
			},
		},
		{
			name: "same depth lexicographic order",
			input: []string{
				"/tmp/z",
				"/tmp/a",
				"/tmp/m",
			},
			expected: []string{
				"/tmp/z",
				"/tmp/m",
				"/tmp/a",
			},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "single element",
			input:    []string{"/tmp/single"},
			expected: []string{"/tmp/single"},
		},
		{
			name: "paths with different separators",
			input: []string{
				"tmp/a",
				"tmp/a/b",
				"tmp/a/b/c",
				"tmp/x",
			},
			expected: []string{
				"tmp/a/b/c",
				"tmp/a/b",
				"tmp/x",
				"tmp/a",
			},
		},
		{
			name: "mixed depth with duplicates",
			input: []string{
				"/a/b/c",
				"/a/b",
				"/a/b/c",
				"/a",
			},
			expected: []string{
				"/a/b/c",
				"/a/b/c",
				"/a/b",
				"/a",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := make([]string, len(tt.input))
			copy(paths, tt.input)

			sortByDepthDescending(paths)

			require.Equal(t, tt.expected, paths, "Paths should be sorted by depth descending")
		})
	}
}

func TestGetClearTemporaryDirsFunc(t *testing.T) {
	tests := []struct {
		name   string
		params ClearTmpParams
		setup  func(t *testing.T, tmpDir string)
		verify func(t *testing.T, tmpDir string)
	}{
		{
			name: "debug mode skips cleaning",
			params: ClearTmpParams{
				IsDebug:       true,
				TmpDir:        "",
				DefaultTmpDir: "",
			},
			setup: func(t *testing.T, tmpDir string) {
				testMkFile(t, filepath.Join(tmpDir, "test.txt"))
			},
			verify: func(t *testing.T, tmpDir string) {
				_, err := os.Stat(filepath.Join(tmpDir, "test.txt"))
				require.NoError(t, err, "File should exist in debug mode")
			},
		},
		{
			name: "empty tmp dir path skips cleaning",
			params: ClearTmpParams{
				IsDebug:       false,
				TmpDir:        "",
				DefaultTmpDir: "",
			},
			setup:  func(t *testing.T, tmpDir string) {},
			verify: func(t *testing.T, tmpDir string) {},
		},
		{
			name: "root path skips cleaning",
			params: ClearTmpParams{
				IsDebug:       false,
				TmpDir:        "/",
				DefaultTmpDir: "/",
			},
			setup:  func(t *testing.T, tmpDir string) {},
			verify: func(t *testing.T, tmpDir string) {},
		},
		{
			name: "preserves log files",
			params: ClearTmpParams{
				IsDebug:         false,
				RemoveTombStone: false,
				TmpDir:          "",
				DefaultTmpDir:   "",
			},
			setup: func(t *testing.T, tmpDir string) {
				testMkFile(t, filepath.Join(tmpDir, "test.log"))
				testMkFile(t, filepath.Join(tmpDir, "other.txt"))
			},
			verify: func(t *testing.T, tmpDir string) {
				_, err := os.Stat(filepath.Join(tmpDir, "test.log"))
				require.NoError(t, err, "Log file should be preserved")

				_, err = os.Stat(filepath.Join(tmpDir, "other.txt"))
				require.True(t, os.IsNotExist(err), "Non-log file should be removed")
			},
		},
		{
			name: "preserves tombstone when RemoveTombStone is false",
			params: ClearTmpParams{
				IsDebug:         false,
				RemoveTombStone: false,
				TmpDir:          "",
				DefaultTmpDir:   "",
			},
			setup: func(t *testing.T, tmpDir string) {
				testMkFile(t, filepath.Join(tmpDir, ".tombstone"))
				testMkFile(t, filepath.Join(tmpDir, "other.txt"))
			},
			verify: func(t *testing.T, tmpDir string) {
				_, err := os.Stat(filepath.Join(tmpDir, ".tombstone"))
				require.NoError(t, err, "Tombstone file should be preserved")

				_, err = os.Stat(filepath.Join(tmpDir, "other.txt"))
				require.True(t, os.IsNotExist(err), "Other file should be removed")
			},
		},
		{
			name: "removes tombstone when RemoveTombStone is true",
			params: ClearTmpParams{
				IsDebug:         false,
				RemoveTombStone: true,
				TmpDir:          "",
				DefaultTmpDir:   "",
			},
			setup: func(t *testing.T, tmpDir string) {
				testMkFile(t, filepath.Join(tmpDir, ".tombstone"))
				testMkFile(t, filepath.Join(tmpDir, "other.txt"))
			},
			verify: func(t *testing.T, tmpDir string) {
				_, err := os.Stat(filepath.Join(tmpDir, ".tombstone"))
				require.True(t, os.IsNotExist(err), "Tombstone file should be removed")

				_, err = os.Stat(filepath.Join(tmpDir, "other.txt"))
				require.True(t, os.IsNotExist(err), "Other file should be removed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.params.TmpDir == "" || tt.params.TmpDir == "/" {
				clearFunc := GetClearTemporaryDirsFunc(tt.params)
				clearFunc()
				return
			}

			id, err := uuid.NewRandom()
			require.NoError(t, err)

			tmpDir := filepath.Join(os.TempDir(), "dhctl-test-clear-"+id.String())
			testMkDir(t, tmpDir)
			defer os.RemoveAll(tmpDir)

			tt.params.TmpDir = tmpDir
			if tt.params.DefaultTmpDir == "" {
				tt.params.DefaultTmpDir = tmpDir
			}

			tt.setup(t, tmpDir)

			clearFunc := GetClearTemporaryDirsFunc(tt.params)
			clearFunc()

			tt.verify(t, tmpDir)
		})
	}
}

func TestDefaultLoggerProvider(t *testing.T) {
	logger := defaultLoggerProvider()
	require.NotNil(t, logger)
}
