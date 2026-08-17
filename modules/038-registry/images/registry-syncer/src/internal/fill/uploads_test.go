/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fill

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func upload(t *testing.T, root, repository, id string, age time.Duration) {
	t.Helper()

	dir := filepath.Join(root, "docker", "registry", "v2", "repositories",
		filepath.FromSlash(repository), "_uploads", id)
	require.NoError(t, os.MkdirAll(dir, 0o755))

	path := filepath.Join(dir, "data")
	require.NoError(t, os.WriteFile(path, []byte("partial"), 0o644))

	touched := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(path, touched, touched))
}

// TestUploadInProgressSeesAPushAndNotAnOpenEndpoint is the one writer this process cannot take turns
// with: an operator pushing a bundle through the publication endpoint.
//
// The question is about activity, not configuration. The endpoint may stay open for the life of a
// cluster while pushes happen twice a year, so refusing to collect while it is open would mean never
// collecting — and reclaiming blobs underneath a push in flight deletes what it has uploaded and not
// yet referenced by a manifest.
func TestUploadInProgressSeesAPushAndNotAnOpenEndpoint(t *testing.T) {
	within := 2 * time.Minute

	t.Run("a store nobody is writing to", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "data")
		active, err := UploadInProgress(root, within, time.Now())
		require.NoError(t, err)
		assert.False(t, active, "an absent store is not a push in progress")
	})

	t.Run("a push in flight", func(t *testing.T) {
		root := t.TempDir()
		upload(t, root, "system/deckhouse", "9f2b-4c11", 5*time.Second)

		active, err := UploadInProgress(root, within, time.Now())
		require.NoError(t, err)
		assert.True(t, active)
	})

	t.Run("an upload nobody has touched for days", func(t *testing.T) {
		// Abandoned uploads keep their directory until distribution purges them, days later.
		// Counting one as activity would mean a store that is never reclaimed again.
		root := t.TempDir()
		upload(t, root, "system/deckhouse", "abandoned", 72*time.Hour)

		active, err := UploadInProgress(root, within, time.Now())
		require.NoError(t, err)
		assert.False(t, active)
	})

	t.Run("a repository merely named like the marker", func(t *testing.T) {
		root := t.TempDir()
		revisionLink(t, root, "system/deckhouse/_uploads-notreally", digestOne)

		active, err := UploadInProgress(root, within, time.Now())
		require.NoError(t, err)
		assert.False(t, active, "a manifest is not an upload, whatever its repository is called")
	})

	t.Run("held content is not activity", func(t *testing.T) {
		root := t.TempDir()
		revisionLink(t, root, "system/deckhouse", digestOne)

		active, err := UploadInProgress(root, within, time.Now())
		require.NoError(t, err)
		assert.False(t, active)
	})
}
