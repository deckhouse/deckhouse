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

package fs

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func assertFileExistsWithContent(t *testing.T, fileName, expectContent string) {
	_, err := os.Stat(fileName)
	require.NoError(t, err)

	cont, err := os.ReadFile(fileName)
	require.NoError(t, err)

	require.Equal(t, string(cont), expectContent)
}

func TestTouchFile(t *testing.T) {
	t.Run("Creates file if it not exists with empty content", func(t *testing.T) {
		fileName := RandomTmpFileName()
		_, err := os.Stat(fileName)
		if err != nil && !os.IsNotExist(err) {
			t.Fail()
		}

		if err == nil {
			err := os.Remove(fileName)
			require.NoError(t, err)
		}

		err = TouchFile(fileName)
		require.NoError(t, err)

		defer os.Remove(fileName)

		assertFileExistsWithContent(t, fileName, "")
	})

	t.Run("Does not rewrite file content if exists", func(t *testing.T) {
		fileName := RandomTmpFileName()

		const content = "test content"
		err := os.WriteFile(fileName, []byte(content), 0o600)
		require.NoError(t, err)

		defer os.Remove(fileName)

		err = TouchFile(fileName)
		require.NoError(t, err)

		assertFileExistsWithContent(t, fileName, content)
	})
}
