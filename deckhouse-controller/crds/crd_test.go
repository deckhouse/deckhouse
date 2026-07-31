// Copyright 2026 Flant JSC
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

package crds

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The checkout directory is named after the CI project, and its parent is named
// after the CI group, so a lookup by directory name finds the group directory
// instead of the repository root.
func TestCrdsDirIgnoresCheckoutName(t *testing.T) {
	// t.TempDir() is under a symlinked /var on darwin, os.Getwd() reports the
	// resolved path.
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	root := filepath.Join(tmp, "builds", "deckhouse", "deckhouse-test-1")
	want := filepath.Join(root, "deckhouse-controller", "crds")
	require.NoError(t, os.MkdirAll(want, 0o755))

	pkgDir := filepath.Join(root, "deckhouse-controller", "pkg", "controller", "module-controllers", "release")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	t.Chdir(pkgDir)

	got, err := crdsDir()
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestCrdsDirNotFound(t *testing.T) {
	t.Chdir(t.TempDir())

	_, err := crdsDir()
	require.Error(t, err)
}

func TestList(t *testing.T) {
	list, err := List()
	require.NoError(t, err)
	require.NotEmpty(t, list)

	for _, crd := range list {
		require.NotEmpty(t, crd.Spec.Names.Kind, "crd without kind: %s", crd.Name)
	}
}
