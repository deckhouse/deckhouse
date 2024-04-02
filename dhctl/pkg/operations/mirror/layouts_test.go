// Copyright 2023 Flant JSC
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

package mirror

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateEmptyImageLayoutAtPath(t *testing.T) {
	p, err := os.MkdirTemp(os.TempDir(), "create_layout_test")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(p)
	})

	_, err = CreateEmptyImageLayoutAtPath(p)
	require.NoError(t, err)
	require.DirExists(t, p)
	require.FileExists(t, filepath.Join(p, "oci-layout"))
	require.FileExists(t, filepath.Join(p, "index.json"))
}
