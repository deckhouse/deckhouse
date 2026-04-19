// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testScriptPath struct {
	sudo      bool
	uploadDir string
}

func (s *testScriptPath) IsSudo() bool {
	return s.sudo
}
func (s *testScriptPath) UploadDir() string {
	return s.uploadDir
}

func TestExecuteRemoteScriptPath(t *testing.T) {
	type test struct {
		name         string
		sudo         bool
		uploadDir    string
		expectedPath string
		full         bool
	}

	const (
		testWithUploadDir         = "/tmp"
		testWithUploadDirExpected = "/tmp/script"
		testWithSudoExpected      = "/opt/deckhouse/tmp/script"
	)

	tests := []test{
		{
			name:         "with upload dir no sudo no full",
			sudo:         false,
			uploadDir:    testWithUploadDir,
			expectedPath: testWithUploadDirExpected,
			full:         false,
		},

		{
			name:         "with upload dir no sudo with full",
			sudo:         false,
			uploadDir:    testWithUploadDir,
			expectedPath: testWithUploadDirExpected,
			full:         true,
		},

		{
			name:         "with upload dir with sudo with full",
			sudo:         true,
			uploadDir:    testWithUploadDir,
			expectedPath: testWithUploadDirExpected,
			full:         true,
		},

		{
			name:         "without upload dir no sudo no full",
			sudo:         false,
			uploadDir:    "",
			expectedPath: ".",
			full:         false,
		},
		{
			name:         "without upload dir with sudo no full",
			sudo:         true,
			uploadDir:    "",
			expectedPath: testWithSudoExpected,
			full:         false,
		},
		{
			name:         "without upload dir with sudo with full",
			sudo:         true,
			uploadDir:    "",
			expectedPath: testWithSudoExpected,
			full:         true,
		},
		{
			name:         "without upload dir without sudo with full",
			sudo:         false,
			uploadDir:    "",
			expectedPath: "./script",
			full:         true,
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			script := &testScriptPath{
				sudo:      tst.sudo,
				uploadDir: tst.uploadDir,
			}
			res := ExecuteRemoteScriptPath(script, "script", tst.full)
			require.Equal(t, tst.expectedPath, res)
		})
	}
}
