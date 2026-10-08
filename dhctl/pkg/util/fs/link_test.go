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

package fs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Link sources carry the image digest, so a new digest relinks a destination that already exists.
func TestCreateLinkIfNotExistsRelinksToANewSource(t *testing.T) {
	dir := t.TempDir()
	oldSource := filepath.Join(dir, "old")
	newSource := filepath.Join(dir, "new")
	destination := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(oldSource, destination))

	err := CreateLinkIfNotExists(t.Context(), newSource, func(string) error { return nil }, destination)
	require.NoError(t, err)

	link, err := os.Readlink(destination)
	require.NoError(t, err)
	require.Equal(t, newSource, link)
}
