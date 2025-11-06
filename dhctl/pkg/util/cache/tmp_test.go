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

	"github.com/google/uuid"
	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

const loggerErrorPrefix = "Error while do cleanup"

func TestSortByDepthDescending(t *testing.T) {
	emptySlice := make([]string, 0)

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "nil slice",
			input:    nil,
			expected: nil,
		},

		{
			name:     "empty slice",
			input:    emptySlice,
			expected: emptySlice,
		},

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
		{
			name: "long path vs deep",
			input: []string{
				"/a/jf48jf84hw4fu4hug3hguhugh3uhgu3hgu3hguh3u4hgu3hguhg",
				"/a/b",
				"/a/b/c",
			},
			expected: []string{
				"/a/b/c",
				"/a/jf48jf84hw4fu4hug3hguhugh3uhgu3hgu3hguh3u4hgu3hguhg",
				"/a/b",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := tt.input
			if len(paths) > 0 {
				paths = make([]string, len(tt.input))
				copy(paths, tt.input)
			}

			sortByDepthDescending(paths)

			require.Equal(t, tt.expected, paths, "Paths should be sorted by depth descending")
		})
	}
}

func TestClearAllInSubDirDisableCleanup(t *testing.T) {
	dirs := []fileDirToCreate{
		{path: "outside_dir", outsideTmpRoot: true},
		{path: "state"},
		{path: "state/empty"},
	}

	files := []fileDirToCreate{
		{path: "outside.file", outsideTmpRoot: true},
		{path: "outside_dir/outside_in_sub.file", outsideTmpRoot: true},
		{path: "in_root.txt"},
		{path: "in_root2.txt"},
		{path: "state/.tombstone"},
		{path: "state/instate2.txt"},
	}

	params := testClearFuncParams{
		testName:              "TestClearAllInSubDirDisableCleanup",
		isDebug:               false,
		tmpSubDir:             "disableCleaning",
		defaultTmpDirAsSubdir: false,
		removeTombstones:      true,
		makeDirs:              dirs,
		makeFiles:             files,
	}

	f := getTestClearFunc(t, params)

	defer func() {
		clearTest(t, f)
	}()

	require.IsType(t, &regularTmpCleaner{}, f.cleaner)

	const disableLogMsg = "Test disable cleanup"

	f.cleaner.DisableCleanup(disableLogMsg)
	f.cleaner.Cleanup()

	assertKeep(t, f, testJoinFilesDirs(files, dirs))
	assertKeepPath(t, f.tmpDir)

	assertHasEntityInLogWithSuffix(t, f, fmt.Sprintf("%s\n", disableLogMsg))
}

func TestClearAllInSubDirWithTombstone(t *testing.T) {
	keeptDirs := []fileDirToCreate{
		{path: "outside_dir", outsideTmpRoot: true},
	}

	dirsForRemove := []fileDirToCreate{
		{path: "state"},
		{path: "state/iefiwhguwurguhurgog4ogoggo4g5ohwrfjiorjgf842h3hfu34hgh4hg4hg4hgh54gh45h4"},
		{path: "state/empty"},
		{path: "state/subdir"},
		{path: "state/subdir/sub"},
	}

	keeptFiles := []fileDirToCreate{
		{path: "outside.file", outsideTmpRoot: true},
		{path: "outside_dir/outside_in_sub.file", outsideTmpRoot: true},
	}

	filesToRemove := []fileDirToCreate{
		{path: "in_root.txt"},
		{path: "in_root2.txt"},
		{path: "state/instate.txt"},
		{path: "state/instateb.txt"},
		{path: "state/.tombstone"},
		{path: "state/instate2.txt"},
		{path: "state/subdir/subdir.json"},
		{path: "state/subdir/sub/file"},
	}

	params := testClearFuncParams{
		testName:              "TestClearAllInSubDirWithTombstone",
		isDebug:               false,
		tmpSubDir:             "allInSubDir",
		defaultTmpDirAsSubdir: false,
		removeTombstones:      true,
		makeDirs:              testJoinFilesDirs(keeptDirs, dirsForRemove),
		makeFiles:             testJoinFilesDirs(keeptFiles, filesToRemove),
	}

	f := getTestClearFunc(t, params)

	defer func() {
		clearTest(t, f)
	}()

	f.cleaner.Cleanup()

	assertKeepAndRemoved(
		t,
		testJoinFilesDirs(filesToRemove, dirsForRemove),
		f,
		testJoinFilesDirs(keeptDirs, keeptFiles),
	)
	// assert removing tmp dir because it is not default
	assertRemovedPath(t, f.tmpDir)
}

func TestClearAllInSubDirWithoutTombstoneWithDefaultDir(t *testing.T) {
	dirs := []fileDirToCreate{
		{path: "state"},
		{path: "state/empty"},
	}

	files := []fileDirToCreate{
		{path: "in_root.txt"},
		{path: "state/instate.txt"},
		{path: "state/.hidden"},
	}

	params := testClearFuncParams{
		testName:              "TestClearAllInSubDirWithoutTombstoneWithDefaultDir",
		isDebug:               false,
		tmpSubDir:             "allInSubDirWithDefault",
		defaultTmpDirAsSubdir: true,
		removeTombstones:      true,
		makeDirs:              dirs,
		makeFiles:             files,
	}

	f := getTestClearFunc(t, params)

	defer func() {
		clearTest(t, f)
	}()

	f.cleaner.Cleanup()

	assertRemoved(t, f, testJoinFilesDirs(files, dirs))
	// assert removing tmp dir because it is not default
	assertKeepPath(t, f.tmpDir)
}

func TestKeepLogsAndTombstounes(t *testing.T) {
	keeptDirs := []fileDirToCreate{
		{path: "state"},
		{path: "state2"},
		{path: "state2/sub"},
	}

	dirsForRemove := []fileDirToCreate{
		{path: "state/iefiwhguwurguhurgog4ogoggo4g5ohwrfjiorjgf842h3hfu34hgh4hg4hg4hgh54gh45h4"},
		{path: "state/empty"},
		{path: "state/subdir"},
		{path: "state/subdir/sub"},
		{path: "state3"},
		{path: "state3/subdirb"},
	}

	keeptFiles := []fileDirToCreate{
		{path: ".tombstone"},
		{path: "state/.tombstone"},
		{path: "bootstrap-111.log"},
		{path: "state2/sub/bootstrap-222.log"},
	}

	filesForRemove := []fileDirToCreate{
		{path: "in_root.txt"},
		{path: "in_root2.txt"},
		{path: "state/instate.txt"},
		{path: "state/instateb.txt"},
		{path: "state/subdir/subdir.json"},
		{path: "state/subdir/sub/file"},
		{path: "state/subdir/sub/file2"},
		{path: "state2/log.txt"},
		{path: "state3/another.bin"},
	}

	params := testClearFuncParams{
		testName:              "TestKeepLogsAndTombstounes",
		isDebug:               false,
		tmpSubDir:             "keepLogsAndTombstounes",
		defaultTmpDirAsSubdir: false,
		removeTombstones:      false,
		makeDirs:              testJoinFilesDirs(keeptDirs, dirsForRemove),
		makeFiles:             testJoinFilesDirs(keeptFiles, filesForRemove),
	}

	f := getTestClearFunc(t, params)

	defer func() {
		clearTest(t, f)
	}()

	f.cleaner.Cleanup()

	assertKeepAndRemoved(
		t,
		testJoinFilesDirs(dirsForRemove, filesForRemove),
		f,
		testJoinFilesDirs(keeptDirs, keeptFiles),
	)
}

func TestSkipIncorrectAndDebug(t *testing.T) {
	files := []fileDirToCreate{
		{path: "in_root.txt"},
		{path: "in_root2.txt"},
		{path: "state/instate.txt"},
	}

	dirs := []fileDirToCreate{
		{path: "state"},
		{path: "state/iefiwhguwurguhurgog4ogoggo4g5ohwrfjiorjgf842h3hfu34hgh4hg4hg4hgh54gh45h4"},
	}

	doTest := func(p testClearFuncParams) {
		f := getTestClearFunc(t, p)
		defer func() {
			clearTest(t, f)
		}()

		f.cleaner.Cleanup()

		assertKeep(t, f, testJoinFilesDirs(files, dirs))
		assertNoErrorsInLog(t, f)
	}

	params := testClearFuncParams{
		testName:              "TestSkipIncorrectAndDebug",
		isDebug:               false,
		tmpSubDir:             "skipLogsAndTombstones",
		defaultTmpDirAsSubdir: false,
		removeTombstones:      true,
		makeDirs:              dirs,
		makeFiles:             files,
		rewriteTmpDirTo:       pointer.String(""),
	}

	// empty dir
	doTest(params)

	// root dir
	params.rewriteTmpDirTo = pointer.String("/")
	doTest(params)

	// current dir
	params.rewriteTmpDirTo = pointer.String(".")
	doTest(params)

	// parent dir
	params.rewriteTmpDirTo = pointer.String("..")
	doTest(params)

	// is debug
	params.rewriteTmpDirTo = nil
	params.isDebug = true
	doTest(params)
}

type fileDirToCreate struct {
	path           string
	outsideTmpRoot bool
}

func testJoinFilesDirs(l ...[]fileDirToCreate) []fileDirToCreate {
	result := make([]fileDirToCreate, 0)
	for _, filesDirs := range l {
		result = append(result, filesDirs...)
	}

	return result
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
	rewriteTmpDirTo       *string

	makeDirs  []fileDirToCreate
	makeFiles []fileDirToCreate
}

type testFunc struct {
	cleaner     TmpCleaner
	tmpRoot     string
	tmpDir      string
	clearParams ClearTmpParams
	logger      *log.InMemoryLogger
	testName    string
}

func (tf *testFunc) fullPath(f fileDirToCreate) string {
	base := tf.tmpDir
	if f.outsideTmpRoot {
		base = tf.tmpRoot
	}

	return filepath.Join(base, f.path)
}

func (tf *testFunc) statFor(t *testing.T, f fileDirToCreate) (os.FileInfo, string, error) {
	require.NotEmpty(t, f.path)
	fullPath := tf.fullPath(f)
	require.NotEmpty(t, fullPath)

	stat, err := os.Stat(fullPath)

	return stat, fullPath, err
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

	logger := log.NewInMemoryLoggerWithParent(log.GetDefaultLogger()).WithErrorPrefix(loggerErrorPrefix)
	logger.Parent().LogInfoF("Tmp dir for test %s is %s\n", params.testName, testTmpDir)

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
		logger.Parent().LogInfoF("Create dir %s\n", fullPath)
		testMkDir(t, fullPath)
	}

	makeFiles := sortFileDirToCreate(params.makeFiles)
	for _, file := range makeFiles {
		fullPath := fullPathToCreate(file)
		logger.Parent().LogInfoF("Create file %s\n", fullPath)
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

	if params.rewriteTmpDirTo != nil {
		clearParams.TmpDir = *params.rewriteTmpDirTo
	}

	return testFunc{
		cleaner:     NewTmpCleaner(clearParams),
		tmpRoot:     testTmpDir,
		tmpDir:      tmpDir,
		clearParams: clearParams,
		logger:      logger,
		testName:    params.testName,
	}
}

func clearTest(t *testing.T, params testFunc) {
	t.Helper()

	require.False(t, govalue.IsNil(params.logger))

	logger := params.logger.Parent()

	require.False(t, govalue.IsNil(logger))

	require.NotEqual(t, path.Clean(params.tmpRoot), "/", params.testName)
	require.NotEqual(t, path.Clean(params.tmpRoot), ".", params.testName)
	require.NotEqual(t, path.Clean(params.tmpRoot), "..", params.testName)

	err := os.RemoveAll(params.tmpRoot)
	if err != nil {
		logger.LogErrorF(
			"Couldn't remove tmp dir '%s' for test %s: %v",
			params.tmpRoot,
			params.testName,
			err,
		)
		return
	}

	logger.LogInfoF(
		"Tmp dir %s for test %s was removed\n",
		params.tmpRoot,
		params.testName,
	)
}

func assertNoErrorsInLog(t *testing.T, f testFunc) {
	t.Helper()

	require.False(t, govalue.IsNil(f.logger))

	matcher := &log.Match{
		Prefix: []string{loggerErrorPrefix, errorPrefix},
	}

	errorMsgs, err := f.logger.AllMatches(matcher)
	require.NoError(t, err)
	require.Empty(t, errorMsgs, fmt.Sprintf("Expected no errors in log: %v", errorMsgs))
}

func assertHasEntityInLogWithSuffix(t *testing.T, f testFunc, msg string) {
	t.Helper()

	require.False(t, govalue.IsNil(f.logger))

	matcher := &log.Match{
		Suffix: []string{msg},
	}

	errorMsgs, err := f.logger.AllMatches(matcher)
	require.NoError(t, err, msg)
	require.Len(t, errorMsgs, 1, msg)
	require.Contains(t, errorMsgs[0], msg, msg)
}

func assertIsRemovedError(t *testing.T, err error, fullPath string) {
	t.Helper()

	require.Error(t, err, fullPath)
	require.True(t, os.IsNotExist(err), fullPath)
}

func assertRemovedPath(t *testing.T, fullPath string) {
	t.Helper()

	_, err := os.Stat(fullPath)
	assertIsRemovedError(t, err, fullPath)
}

func assertRemovedOne(t *testing.T, f testFunc, fd fileDirToCreate) {
	t.Helper()

	_, fullPath, err := f.statFor(t, fd)
	assertIsRemovedError(t, err, fullPath)
}

func assertKeepPath(t *testing.T, fullPath string) {
	t.Helper()

	_, err := os.Stat(fullPath)
	require.NoError(t, err, fullPath)
}

func assertKeepOne(t *testing.T, f testFunc, fd fileDirToCreate) {
	t.Helper()

	_, fullPath, err := f.statFor(t, fd)
	require.NoError(t, err, fullPath)
}

func assertRemoved(t *testing.T, f testFunc, l []fileDirToCreate) {
	t.Helper()

	tmpDir := f.tmpDir

	require.NotEmpty(t, l)
	require.NotEmpty(t, tmpDir)

	for _, e := range l {
		assertRemovedOne(t, f, e)
	}

	assertNoErrorsInLog(t, f)
}

func assertKeep(t *testing.T, f testFunc, l []fileDirToCreate) {
	t.Helper()

	tmpDir := f.tmpDir

	require.NotEmpty(t, l)
	require.NotEmpty(t, tmpDir)

	for _, e := range l {
		assertKeepOne(t, f, e)
	}

	assertNoErrorsInLog(t, f)
}

func assertKeepAndRemoved(t *testing.T, removed []fileDirToCreate, f testFunc, keept []fileDirToCreate) {
	t.Helper()

	require.NotEmpty(t, removed)
	require.NotEmpty(t, keept)

	assertRemoved(t, f, removed)
	assertKeep(t, f, keept)

	assertNoErrorsInLog(t, f)
}

func TestDefaultLoggerProvider(t *testing.T) {
	logger := safeLoggerProvider(nil)
	require.False(t, govalue.IsNil(logger))

	logger = safeLoggerProvider(func() log.Logger {
		return nil
	})
	require.False(t, govalue.IsNil(logger))
}

func TestGlobalCleanerProvider(t *testing.T) {
	cleaner := GetGlobalTmpCleaner()
	require.False(t, govalue.IsNil(cleaner))

	dummyCleaner := NewDummyTmpCleaner(nil, "")
	SetGlobalTmpCleaner(dummyCleaner)

	cleaner = GetGlobalTmpCleaner()
	require.False(t, govalue.IsNil(cleaner))
	require.Equal(t, cleaner, dummyCleaner)
}
