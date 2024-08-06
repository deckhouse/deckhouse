// Copyright 2024 Flant JSC
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

package release

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_copyDirectory(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "copy_case")
	err := copyDirectory("testdata/copy_case", dest)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dest, "README.md"))
	require.NoError(t, err)
	require.True(t, info != nil)
	require.True(t, info.Name() == "README.md")
	require.False(t, info.IsDir())

	info, err = os.Stat(filepath.Join(dest, "bar"))
	require.NoError(t, err)
	require.True(t, info != nil)
	require.True(t, info.Name() == "bar")
	require.True(t, info.IsDir())

	info, err = os.Stat(filepath.Join(dest, "foo", "ccc"))
	require.NoError(t, err)
	require.True(t, info != nil)
	require.True(t, info.Name() == "ccc")
	require.True(t, info.IsDir())

	info, err = os.Stat(filepath.Join(dest, "bar", "aaa.txt"))
	require.NoError(t, err)
	require.True(t, info != nil)
	require.True(t, info.Name() == "aaa.txt")
	require.False(t, info.IsDir())

	info, err = os.Stat(filepath.Join(dest, "foo", "ccc", "bbb.txt"))
	require.NoError(t, err)
	require.True(t, info != nil)
	require.True(t, info.Name() == "bbb.txt")
	require.False(t, info.IsDir())
}
