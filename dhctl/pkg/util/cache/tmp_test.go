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
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"

	dhlogger "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

// captureSlog installs a buffer-backed slog logger as the process default so
// that production code logging via dhlogger.FromContext(context.Background())
// (which routes to slog.Default() in tests, since cmd/dhctl's slog.SetDefault
// is never called here) is captured. The migrated cleanup code logs the
// disable/skip and "multiple lock files" messages via slog rather than the
// legacy in-memory logger, so these tests assert on the returned buffer.
//
// These tests do not call t.Parallel(), so mutating the global slog default is
// safe; the previous default is restored on cleanup.
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(dhlogger.NewBufferLogger(&buf))
	t.Cleanup(func() { slog.SetDefault(prev) })

	return &buf
}

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
		makeDirsFiles: makeDirsFiles{
			makeDirs:  dirs,
			makeFiles: files,
		},
	}

	f := getTestClearFunc(t, params)

	defer func() {
		clearTmpCleanTest(t, f)
	}()

	require.IsType(t, &regularTmpCleaner{}, f.cleaner)

	const disableLogMsg = "Test disable cleanup"

	f.cleaner.DisableCleanup(disableLogMsg)

	// The migrated cleaner logs the skip reason via slog (DebugContext), not the
	// legacy in-memory logger, so capture slog output to assert on the message.
	buf := captureSlog(t)
	f.cleaner.Cleanup()

	assertKeep(t, f, testJoinFilesDirs(files, dirs), true)
	assertKeepPath(t, f.tmpDir)

	require.Contains(t, buf.String(), disableLogMsg)
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
		// lock should removed
		{path: ".dhctl-tmp-dir.lock"},
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
		makeDirsFiles: makeDirsFiles{
			makeDirs:  testJoinFilesDirs(keeptDirs, dirsForRemove),
			makeFiles: testJoinFilesDirs(keeptFiles, filesToRemove),
		},
	}

	f := getTestClearFunc(t, params)

	defer func() {
		clearTmpCleanTest(t, f)
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
		{path: "state/sub/"},
		{path: "state/loooooooooooong/"},
	}

	files := []fileDirToCreate{
		// lock should removed
		{path: ".dhctl-tmp-dir.lock"},
		{path: "in_root.txt"},
		{path: "state/instate.txt"},
		{path: "state/.hidden"},
		{path: "state/sub/file.txt"},
		{path: "state/loooooooooooong/loooooooong_file.json"},
	}

	params := testClearFuncParams{
		testName:              "TestClearAllInSubDirWithoutTombstoneWithDefaultDir",
		isDebug:               false,
		tmpSubDir:             "allInSubDirWithDefault",
		defaultTmpDirAsSubdir: true,
		removeTombstones:      true,
		makeDirsFiles: makeDirsFiles{
			makeDirs:  dirs,
			makeFiles: files,
		},
	}

	f := getTestClearFunc(t, params)

	defer func() {
		clearTmpCleanTest(t, f)
	}()

	f.cleaner.Cleanup()

	assertRemoved(t, f, testJoinFilesDirs(files, dirs))
	assertKeepPath(t, f.tmpDir)
}

func TestMultipleLocksKeeptAll(t *testing.T) {
	dirs := []fileDirToCreate{
		{path: "state"},
		{path: "state/empty"},
		{path: "state/sub/"},
		{path: "state/loooooooooooong/"},
		{path: "another_instance/"},
	}

	files := []fileDirToCreate{
		{path: ".dhctl-tmp-dir.lock"},
		{path: "in_root.txt"},
		{path: "state/instate.txt"},
		{path: "state/.hidden"},
		{path: "state/sub/file.txt"},
		{path: "state/loooooooooooong/loooooooong_file.json"},
		{path: "another_instance/.dhctl-tmp-dir.lock"},
	}

	params := testClearFuncParams{
		testName:              "TestMultipleLocksKeeptAll",
		isDebug:               false,
		tmpSubDir:             "testMultipleLocksKeeptAll",
		defaultTmpDirAsSubdir: false,
		removeTombstones:      true,
		makeDirsFiles: makeDirsFiles{
			makeDirs:  dirs,
			makeFiles: files,
		},
	}

	f := getTestClearFunc(t, params)

	defer func() {
		clearTmpCleanTest(t, f)
	}()

	// The migrated cleaner logs the multiple-lock warning via slog (WarnContext),
	// not the legacy in-memory logger, so capture slog output to assert on it.
	buf := captureSlog(t)
	f.cleaner.Cleanup()

	assertKeep(t, f, testJoinFilesDirs(files, dirs), false)
	require.Contains(t, buf.String(), "found multiple lock files")
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
		// lock should removed
		{path: ".dhctl-tmp-dir.lock"},
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
		makeDirsFiles: makeDirsFiles{
			makeDirs:  testJoinFilesDirs(keeptDirs, dirsForRemove),
			makeFiles: testJoinFilesDirs(keeptFiles, filesForRemove),
		},
	}

	f := getTestClearFunc(t, params)

	defer func() {
		clearTmpCleanTest(t, f)
	}()

	f.cleaner.Cleanup()

	assertKeepAndRemoved(
		t,
		testJoinFilesDirs(dirsForRemove, filesForRemove),
		f,
		testJoinFilesDirs(keeptDirs, keeptFiles),
	)
	assertKeepPath(t, f.tmpDir)
}

func TestSkipIncorrectAndDebug(t *testing.T) {
	files := []fileDirToCreate{
		// lock should keept
		{path: ".dhctl-tmp-dir.lock"},
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
			clearTmpCleanTest(t, f)
		}()

		f.cleaner.Cleanup()

		assertKeep(t, f, testJoinFilesDirs(files, dirs), true)
	}

	params := testClearFuncParams{
		testName:              "TestSkipIncorrectAndDebug",
		isDebug:               false,
		tmpSubDir:             "skipLogsAndTombstones",
		defaultTmpDirAsSubdir: false,
		removeTombstones:      true,
		makeDirsFiles: makeDirsFiles{
			makeDirs:  dirs,
			makeFiles: files,
		},
		rewriteTmpDirTo: pointer.String(""),
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

func TestGlobalCleanerProvider(t *testing.T) {
	cleaner := GetGlobalTmpCleaner()
	require.False(t, govalue.IsNil(cleaner))

	dummyCleaner := NewDummyTmpCleaner("")
	SetGlobalTmpCleaner(dummyCleaner)

	cleaner = GetGlobalTmpCleaner()
	require.False(t, govalue.IsNil(cleaner))
	require.Equal(t, cleaner, dummyCleaner)
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

type makeDirsFiles struct {
	makeDirs  []fileDirToCreate
	makeFiles []fileDirToCreate
}

func (m *makeDirsFiles) makeAll(t *testing.T, root string, logger *slog.Logger, tmpDir string) {
	fullPathToCreate := func(f fileDirToCreate) string {
		fullPath := filepath.Join(tmpDir, f.path)
		if f.outsideTmpRoot {
			fullPath = filepath.Join(root, f.path)
		}

		return fullPath
	}

	makeDirs := sortFileDirToCreate(m.makeDirs)
	for _, dir := range makeDirs {
		fullPath := fullPathToCreate(dir)
		logger.Info(fmt.Sprintf("Create dir %s\n", fullPath))
		testMkDir(t, fullPath)
	}

	makeFiles := sortFileDirToCreate(m.makeFiles)
	for _, file := range makeFiles {
		fullPath := fullPathToCreate(file)
		logger.Info(fmt.Sprintf("Create file %s\n", fullPath))
		testMkFile(t, fullPath)
	}
}

type testClearFuncParams struct {
	testName              string
	isDebug               bool
	tmpSubDir             string
	defaultTmpDirAsSubdir bool
	removeTombstones      bool
	rewriteTmpDirTo       *string

	makeDirsFiles
}

type testFunc struct {
	cleaner     TmpCleaner
	tmpRoot     string
	tmpDir      string
	clearParams ClearTmpParams
	logger      *slog.Logger
	logBuf      *bytes.Buffer
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

	var logBuf bytes.Buffer
	logger := dhlogger.NewBufferLogger(&logBuf)
	logger.Info(fmt.Sprintf("Tmp dir for test %s is %s\n", params.testName, testTmpDir))

	tmpDir := testTmpDir
	if params.tmpSubDir != "" {
		tmpDir = filepath.Join(testTmpDir, params.tmpSubDir)
		testMkDir(t, tmpDir)
	}

	params.makeAll(t, testTmpDir, logger, tmpDir)

	defaultTmpDir := testTmpDir
	if params.defaultTmpDirAsSubdir {
		defaultTmpDir = tmpDir
	}

	clearParams := ClearTmpParams{
		IsDebug:         params.isDebug,
		TmpDir:          tmpDir,
		RemoveTombStone: params.removeTombstones,
		DefaultTmpDir:   defaultTmpDir,
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
		logBuf:      &logBuf,
		testName:    params.testName,
	}
}

func clearTmpCleanTest(t *testing.T, params testFunc) {
	t.Helper()

	require.False(t, govalue.IsNil(params.logger))

	logger := params.logger

	require.False(t, govalue.IsNil(logger))

	require.NotEqual(t, path.Clean(params.tmpRoot), "/", params.testName)
	require.NotEqual(t, path.Clean(params.tmpRoot), ".", params.testName)
	require.NotEqual(t, path.Clean(params.tmpRoot), "..", params.testName)

	err := os.RemoveAll(params.tmpRoot)
	if err != nil {
		logger.Error(fmt.Sprintf(
			"Couldn't remove tmp dir '%s' for test %s: %v",
			params.tmpRoot,
			params.testName,
			err,
		))
		return
	}

	logger.Info(fmt.Sprintf(
		"Tmp dir %s for test %s was removed\n",
		params.tmpRoot,
		params.testName,
	))
}

func assertNoErrorsInLog(t *testing.T, f testFunc) {
	t.Helper()

	require.False(t, govalue.IsNil(f.logger))
	require.False(t, govalue.IsNil(f.logBuf))

	require.NotContains(t, f.logBuf.String(), "level=ERROR",
		fmt.Sprintf("Expected no errors in log: %s", f.logBuf.String()))
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

func assertKeep(t *testing.T, f testFunc, l []fileDirToCreate, noErrorsInLog bool) {
	t.Helper()

	tmpDir := f.tmpDir

	require.NotEmpty(t, l)
	require.NotEmpty(t, tmpDir)

	for _, e := range l {
		assertKeepOne(t, f, e)
	}

	if noErrorsInLog {
		assertNoErrorsInLog(t, f)
	}
}

func assertKeepAndRemoved(t *testing.T, removed []fileDirToCreate, f testFunc, keept []fileDirToCreate) {
	t.Helper()

	require.NotEmpty(t, removed)
	require.NotEmpty(t, keept)

	assertRemoved(t, f, removed)
	assertKeep(t, f, keept, true)

	assertNoErrorsInLog(t, f)
}
