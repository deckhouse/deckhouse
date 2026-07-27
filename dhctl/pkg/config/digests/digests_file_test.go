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

package digests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetImageFromDigestsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "images_digests.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"cloudProviderDvp":{"terraformManager":"sha256:feed"}}`), 0o644))
	t.Setenv(ImagesDigestsFileEnv, path)

	digest, err := GetImage("cloudProviderDvp", "terraformManager")
	require.NoError(t, err)
	require.Equal(t, "sha256:feed", digest)
}

func TestGetImageDigestsFileMissingFails(t *testing.T) {
	// A set-but-unreadable file must not silently fall back to embedded
	// digests — that would resurrect the stale-digest bug.
	t.Setenv(ImagesDigestsFileEnv, filepath.Join(t.TempDir(), "nope.json"))

	_, err := GetImage("cloudProviderDvp", "terraformManager")
	require.Error(t, err)
	require.Contains(t, err.Error(), ImagesDigestsFileEnv)
}

func TestGetImageFallsBackToEmbedded(t *testing.T) {
	t.Setenv(ImagesDigestsFileEnv, "")

	// The embedded stub carries the "something/app" entry.
	digest, err := GetImage("something", "app")
	require.NoError(t, err)
	require.NotEmpty(t, digest)
}
