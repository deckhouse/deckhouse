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

package symlink

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// TestModuleComplete pins the recovery gate: an empty version directory (what an
// interrupted download leaves behind) must be reported as incomplete so Restore
// re-downloads it instead of trusting bare existence.
func TestModuleComplete(t *testing.T) {
	t.Run("empty directory is incomplete", func(t *testing.T) {
		assert.False(t, moduleComplete(t.TempDir()))
	})

	t.Run("populated directory is complete", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "module.yaml"), []byte("x"), 0o600))
		assert.True(t, moduleComplete(dir))
	})

	t.Run("missing directory is incomplete", func(t *testing.T) {
		assert.False(t, moduleComplete(filepath.Join(t.TempDir(), "absent")))
	})
}

func TestAtomicCopyDir(t *testing.T) {
	t.Run("copies contents and leaves no scratch", func(t *testing.T) {
		src := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o600))
		require.NoError(t, os.MkdirAll(filepath.Join(src, "sub"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("b"), 0o600))

		parent := t.TempDir()
		dst := filepath.Join(parent, "v1.0.0")
		require.NoError(t, atomicCopyDir(src, dst))

		assertFileContent(t, filepath.Join(dst, "a.txt"), "a")
		assertFileContent(t, filepath.Join(dst, "sub", "b.txt"), "b")
		assertNoScratch(t, parent)
	})

	t.Run("replaces an existing directory atomically", func(t *testing.T) {
		src := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(src, "new.txt"), []byte("new"), 0o600))

		parent := t.TempDir()
		dst := filepath.Join(parent, "v1.0.0")
		require.NoError(t, os.MkdirAll(dst, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dst, "stale.txt"), []byte("stale"), 0o600))

		require.NoError(t, atomicCopyDir(src, dst))

		assertFileContent(t, filepath.Join(dst, "new.txt"), "new")
		assert.NoFileExists(t, filepath.Join(dst, "stale.txt"))
		assertNoScratch(t, parent)
	})
}

func TestInstall(t *testing.T) {
	downloaded := t.TempDir()
	inst := &Installer{
		downloaded: downloaded,
		symlinkDir: filepath.Join(downloaded, "modules"),
		logger:     log.NewNop(),
	}
	require.NoError(t, os.MkdirAll(inst.symlinkDir, 0o755))

	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "module.yaml"), []byte("name: mod\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(src, "images_digests.json"), []byte("{}"), 0o600))

	require.NoError(t, inst.Install(context.Background(), "mod", "v1.0.0", src))

	versionPath := filepath.Join(downloaded, "mod", "v1.0.0")
	assertFileContent(t, filepath.Join(versionPath, "module.yaml"), "name: mod\n")
	assertFileContent(t, filepath.Join(versionPath, "images_digests.json"), "{}")

	// the module symlink resolves to the freshly materialized version
	resolved, err := filepath.EvalSymlinks(filepath.Join(inst.symlinkDir, "mod"))
	require.NoError(t, err)
	want, err := filepath.EvalSymlinks(versionPath)
	require.NoError(t, err)
	assert.Equal(t, want, resolved)

	// re-installing the same version replaces its contents wholesale, not merges
	src2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src2, "module.yaml"), []byte("name: mod2\n"), 0o600))
	require.NoError(t, inst.Install(context.Background(), "mod", "v1.0.0", src2))

	assertFileContent(t, filepath.Join(versionPath, "module.yaml"), "name: mod2\n")
	assert.NoFileExists(t, filepath.Join(versionPath, "images_digests.json"))
	assertNoScratch(t, filepath.Join(downloaded, "mod"))
}

// TestRestoreFromRegistry checks that the module is downloaded even when a module is on the
// filesystem, and that the module on the filesystem is kept and linked when the download fails.
func TestRestoreFromRegistry(t *testing.T) {
	img, err := random.Image(1024, 1)
	require.NoError(t, err)

	errNoImage := errors.New("manifest unknown")

	tests := []struct {
		name           string
		onFilesystem   bool // whether a module is on the filesystem before the restore
		image          crv1.Image
		imageErr       error
		wantDownloaded bool
		wantErr        error
		wantOld        bool // whether the module on the filesystem is kept
		wantLinked     bool
	}{
		{name: "complete download replaces the module", onFilesystem: true, image: img, wantDownloaded: true, wantLinked: true},
		{name: "failed download links the module on the filesystem", onFilesystem: true, imageErr: errNoImage, wantOld: true, wantLinked: true},
		{name: "failed download without a module fails", imageErr: errNoImage, wantErr: errNoImage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := dependency.NewMockedContainer()
			dc.CRClient.ImageMock.Return(tt.image, tt.imageErr)

			downloaded := t.TempDir()
			inst := &Installer{
				downloaded: downloaded,
				symlinkDir: filepath.Join(downloaded, "modules"),
				registry:   registry.NewService(dc, log.NewNop()),
				logger:     log.NewNop(),
			}
			require.NoError(t, os.MkdirAll(inst.symlinkDir, 0o755))

			versionPath := filepath.Join(downloaded, "mod", "main")
			oldModule := filepath.Join(versionPath, "module.yaml")
			if tt.onFilesystem {
				require.NoError(t, os.MkdirAll(versionPath, 0o755))
				require.NoError(t, os.WriteFile(oldModule, []byte("name: mod\n"), 0o600))
			}

			source := &v1alpha1.ModuleSource{
				Spec: v1alpha1.ModuleSourceSpec{
					Registry: v1alpha1.ModuleSourceSpecRegistry{Repo: "registry.example.com/modules"},
				},
			}

			got, err := inst.RestoreFromRegistry(context.Background(), source, "mod", "main")
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantDownloaded, got)

			_, err = os.Stat(oldModule)
			assert.Equal(t, tt.wantOld, err == nil, "the module on the filesystem kept")

			wantTarget := ""
			if tt.wantLinked {
				wantTarget = versionPath
				assert.True(t, moduleComplete(versionPath), "the linked module must be complete")
			}

			target, _ := os.Readlink(filepath.Join(inst.symlinkDir, "mod"))
			assert.Equal(t, wantTarget, target)
			assertNoScratch(t, filepath.Join(downloaded, "mod"))
		})
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}

// assertNoScratch fails if a temporary scratch directory from atomicCopyDir was
// left behind in dir (the rename must consume it on success).
func assertNoScratch(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".tmp-", "leftover scratch dir")
	}
}
