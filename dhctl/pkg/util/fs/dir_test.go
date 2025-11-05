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
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const testLockFileToCheck = ".TestFileExistsInDirAndParentsDirs"

var testFileExistsInDirAndParentsDirsRoot = path.Join(os.TempDir(), "dhctl-test-file-exists-in-parents")

func cleanupTestFileExistsInDirAndParentsDirs(logger log.Logger) {
	incorrect := []string{"", ".", ".."}
	if slices.Contains(incorrect, testFileExistsInDirAndParentsDirsRoot) {
		return
	}

	if filepath.Clean(testFileExistsInDirAndParentsDirsRoot) == "/" {
		return
	}

	if !IsDirExists(testFileExistsInDirAndParentsDirsRoot) {
		return
	}

	err := os.RemoveAll(testFileExistsInDirAndParentsDirsRoot)

	if err != nil {
		logger.LogErrorF("Error cleaning up test dir '%s': %v\n", testFileExistsInDirAndParentsDirsRoot, err)
		return
	}

	logger.LogInfoF("Test dir '%s' was removed\n", testFileExistsInDirAndParentsDirsRoot)
}

func TestFileExistsInDirAndParentsDirs(t *testing.T) {
	logger := log.GetDefaultLogger()

	defer func() {
		cleanupTestFileExistsInDirAndParentsDirs(logger)
	}()

	testGlobalRoot := testFileExistsInDirAndParentsDirs{
		testFileExistsInDirAndParentsDirsBase: testFileExistsInDirAndParentsDirsBase{
			title:      "not exists in global root dir",
			dirToCheck: "/",
			existsIn:   "",
			isErr:      false,
		},
		logger: logger,
	}

	testNonExistError := testFileExistsInDirAndParentsDirs{
		testFileExistsInDirAndParentsDirsBase: testFileExistsInDirAndParentsDirsBase{
			title:      "error if dir is not exists",
			dirToCheck: "/wfuhgijogj8395h5gh4545",
			existsIn:   "",
			isErr:      true,
		},
		logger: logger,
	}

	t.Run(testGlobalRoot.title, func(t *testing.T) {
		testGlobalRoot.do(t)
	})

	t.Run(testNonExistError.title, func(t *testing.T) {
		testNonExistError.do(t)
	})

	tests := []testFileExistsInDirAndParentsDirsParams{
		{
			testFileExistsInDirAndParentsDirsBase: testFileExistsInDirAndParentsDirsBase{
				title:      "no in root",
				dirToCheck: "/",
				existsIn:   "",
				isErr:      false,
			},
			dirToCreate: "/",
			writeFileIn: "",
		},

		{
			testFileExistsInDirAndParentsDirsBase: testFileExistsInDirAndParentsDirsBase{
				title:      "in root",
				dirToCheck: "/",
				existsIn:   "/",
				isErr:      false,
			},
			dirToCreate: "/",
			writeFileIn: "/",
		},

		{
			testFileExistsInDirAndParentsDirsBase: testFileExistsInDirAndParentsDirsBase{
				title:      "exists in root from subdir",
				dirToCheck: "/sub",
				existsIn:   "/",
				isErr:      false,
			},
			dirToCreate: "/sub",
			writeFileIn: "/",
		},

		{
			testFileExistsInDirAndParentsDirsBase: testFileExistsInDirAndParentsDirsBase{
				title:      "exists in root from subdir deeper",
				dirToCheck: "/sub/1/2",
				existsIn:   "/",
				isErr:      false,
			},
			dirToCreate: "/sub/1/2",
			writeFileIn: "/",
		},

		{
			testFileExistsInDirAndParentsDirsBase: testFileExistsInDirAndParentsDirsBase{
				title:      "exists in subdir from subdir",
				dirToCheck: "/sub",
				existsIn:   "/sub",
				isErr:      false,
			},
			dirToCreate: "/sub",
			writeFileIn: "/sub",
		},

		{
			testFileExistsInDirAndParentsDirsBase: testFileExistsInDirAndParentsDirsBase{
				title:      "exists in middle from subdir deeper",
				dirToCheck: "/sub/1/2/3",
				existsIn:   "/sub/1",
				isErr:      false,
			},
			dirToCreate: "/sub/1/2/3",
			writeFileIn: "/sub/1",
		},

		{
			testFileExistsInDirAndParentsDirsBase: testFileExistsInDirAndParentsDirsBase{
				title:      "not exists from subdir deeper",
				dirToCheck: "/sub/1/2/3",
				existsIn:   "",
				isErr:      false,
			},
			dirToCreate: "/sub/1/2/3",
			writeFileIn: "",
		},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			tt := testCreateFileExistsInDirAndParentsDirsTest(t, test, logger)
			tt.do(t)
		})
	}
}

func TestFileExistsInDirAndParentsDirsInGlobalRootExists(t *testing.T) {
	if os.Getenv("TEST_PARENTS_EXISTS_IN_GLOBAL_ROOT") == "" {
		t.Skip("Use TEST_PARENTS_EXISTS_IN_GLOBAL_ROOT env for enable")
	}

	logger := log.GetDefaultLogger()

	testGlobalRootExists := testFileExistsInDirAndParentsDirs{
		testFileExistsInDirAndParentsDirsBase: testFileExistsInDirAndParentsDirsBase{
			title:      "not exists in global root dir",
			dirToCheck: "/",
			existsIn:   "/",
			isErr:      false,
		},
		logger: logger,
	}

	testGlobalRootExists.do(t)
}

type testFileExistsInDirAndParentsDirsBase struct {
	title      string
	dirToCheck string
	existsIn   string
	isErr      bool
}

type testFileExistsInDirAndParentsDirsParams struct {
	testFileExistsInDirAndParentsDirsBase

	dirToCreate string
	writeFileIn string
}

type testFileExistsInDirAndParentsDirs struct {
	testFileExistsInDirAndParentsDirsBase

	logger log.Logger
}

func (tt *testFileExistsInDirAndParentsDirs) do(t *testing.T) {
	require.NotEmpty(t, tt.dirToCheck)
	require.False(t, govalue.IsNil(tt.logger))

	existsIn, err := FileExistsInDirAndParentsDirs(tt.dirToCheck, testLockFileToCheck)
	if tt.isErr {
		require.Error(t, err, tt.dirToCheck)
		return
	}

	require.NoError(t, err, tt.dirToCheck)
	tt.logger.LogInfoF("FileExistsInDirAndParentsDirs returns: '%s'\n", existsIn)
	require.Equal(t, tt.existsIn, existsIn)
}

func testCreateFileExistsInDirAndParentsDirsTest(t *testing.T, params testFileExistsInDirAndParentsDirsParams, logger log.Logger) *testFileExistsInDirAndParentsDirs {
	t.Helper()

	assertFromRoot := func(t *testing.T, p string) {
		t.Helper()

		require.NotEmpty(t, p)
		require.True(t, strings.HasPrefix(p, "/"))
	}

	require.NotEmpty(t, params.title)
	assertFromRoot(t, params.dirToCreate)
	assertFromRoot(t, params.dirToCheck)

	writeFileIn := params.writeFileIn
	if writeFileIn != "" {
		assertFromRoot(t, writeFileIn)
	}

	rootDir, err := RandomTmpDirWith10Runes(testFileExistsInDirAndParentsDirsRoot, params.title, 8)
	require.NoError(t, err)

	logger.LogInfoF("Test root dir '%s' for test '%s' was created\n", rootDir, params.title)

	fullTestDirChain := filepath.Join(rootDir, params.dirToCreate)
	err = os.MkdirAll(fullTestDirChain, 0o777)
	require.NoError(t, err)

	logger.LogInfoF("Full dir '%s' for test '%s' was created\n", fullTestDirChain, params.title)

	if writeFileIn != "" {
		fullPath := filepath.Join(rootDir, writeFileIn, testLockFileToCheck)
		testMkFile(t, fullPath)
		logger.LogInfoF("File '%s' for test '%s' was created\n", fullPath, params.title)
	}

	existsIn := ""
	if params.existsIn != "" {
		existsIn = filepath.Join(rootDir, params.existsIn)
	}

	return &testFileExistsInDirAndParentsDirs{
		testFileExistsInDirAndParentsDirsBase: testFileExistsInDirAndParentsDirsBase{
			title:      params.title,
			dirToCheck: filepath.Join(rootDir, params.dirToCheck),
			existsIn:   existsIn,
			isErr:      params.isErr,
		},

		logger: logger,
	}

}

func testMkFile(t *testing.T, file string) {
	t.Helper()

	err := os.WriteFile(file, make([]byte, 0), 0644)
	require.NoError(t, err, file)
}
