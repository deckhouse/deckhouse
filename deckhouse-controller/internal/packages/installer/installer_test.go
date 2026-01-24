/*
Copyright 2025 Flant JSC

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

package installer_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/installer/symlink"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// mockDownloader simulates registry download by creating a directory with a marker file.
type mockDownloader struct {
	downloadErr error
}

func (m *mockDownloader) Download(_ context.Context, _ registry.Remote, out, packageName, tag string) error {
	if m.downloadErr != nil {
		return m.downloadErr
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	markerFile := filepath.Join(out, "package.yaml")
	return os.WriteFile(markerFile, []byte("name: "+packageName+"\nversion: "+tag), 0644)
}

// errAny is a sentinel indicating any error is expected (when specific error type doesn't matter).
var errAny = errors.New("any error expected")

// setupVersionDir creates a version directory with optional marker file content.
func setupVersionDir(t *testing.T, basePath, version, markerContent string) string {
	t.Helper()
	versionPath := filepath.Join(basePath, version)
	require.NoError(t, os.MkdirAll(versionPath, 0755))
	if markerContent != "" {
		require.NoError(t, os.WriteFile(filepath.Join(versionPath, "version"), []byte(markerContent), 0644))
	}
	return versionPath
}

// setupSymlink creates a symlink from deployed to target.
func setupSymlink(t *testing.T, deployed, target string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(deployed), 0755))
	require.NoError(t, os.Symlink(target, deployed))
}

func TestDownload(t *testing.T) {
	tests := []struct {
		name        string
		downloadErr error
		cancelCtx   bool
		wantErrIs   error // nil = success, errAny = any error, specific = errors.Is check
		checkResult func(t *testing.T, downloaded string)
	}{
		{
			name: "success",
			checkResult: func(t *testing.T, downloaded string) {
				versionPath := filepath.Join(downloaded, "1.0.0")
				assert.DirExists(t, versionPath)
				assert.FileExists(t, filepath.Join(versionPath, "package.yaml"))
			},
		},
		{
			name:      "context_canceled",
			cancelCtx: true,
			wantErrIs: context.Canceled,
		},
		{
			name:        "registry_error",
			downloadErr: errors.New("registry unavailable"),
			wantErrIs:   errAny,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			downloaded := filepath.Join(tmpDir, "downloaded", "my-package")

			inst := symlink.NewInstaller(&mockDownloader{downloadErr: tc.downloadErr}, log.NewNop())
			repo := registry.Remote{Name: "test-repo", Repository: "registry.example.com"}

			ctx := context.Background()
			if tc.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := inst.Download(ctx, repo, downloaded, "my-package", "1.0.0")

			if tc.wantErrIs != nil {
				require.Error(t, err)
				if !errors.Is(tc.wantErrIs, errAny) {
					assert.ErrorIs(t, err, tc.wantErrIs)
				}
				// Verify status.Error wrapping for registry errors
				if tc.downloadErr != nil {
					var statusErr *status.Error
					assert.True(t, errors.As(err, &statusErr), "error should be wrapped as status.Error")
				}
				return
			}

			require.NoError(t, err)
			if tc.checkResult != nil {
				tc.checkResult(t, downloaded)
			}
		})
	}
}

func TestInstall(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		setup       func(t *testing.T, downloaded, deployed string)
		cancelCtx   bool
		wantErrIs   error
		checkResult func(t *testing.T, downloaded, deployed string)
	}{
		{
			name:    "creates_symlink",
			version: "1.0.0",
			setup: func(t *testing.T, downloaded, deployed string) {
				setupVersionDir(t, downloaded, "1.0.0", "1.0.0")
				require.NoError(t, os.MkdirAll(filepath.Dir(deployed), 0755))
			},
			checkResult: func(t *testing.T, downloaded, deployed string) {
				versionPath := filepath.Join(downloaded, "1.0.0")
				linkTarget, err := os.Readlink(deployed)
				require.NoError(t, err)
				assert.Equal(t, versionPath, linkTarget)

				content, err := os.ReadFile(filepath.Join(deployed, "version"))
				require.NoError(t, err)
				assert.Equal(t, "1.0.0", string(content))
			},
		},
		{
			name:    "replaces_existing_symlink",
			version: "2.0.0",
			setup: func(t *testing.T, downloaded, deployed string) {
				v1 := setupVersionDir(t, downloaded, "1.0.0", "1.0.0")
				setupVersionDir(t, downloaded, "2.0.0", "2.0.0")
				setupSymlink(t, deployed, v1)
			},
			checkResult: func(t *testing.T, _, deployed string) {
				content, err := os.ReadFile(filepath.Join(deployed, "version"))
				require.NoError(t, err)
				assert.Equal(t, "2.0.0", string(content))
			},
		},
		{
			name:      "context_canceled",
			version:   "1.0.0",
			cancelCtx: true,
			wantErrIs: context.Canceled,
		},
		{
			name:    "version_dir_missing",
			version: "1.0.0",
			setup: func(t *testing.T, _, deployed string) {
				require.NoError(t, os.MkdirAll(filepath.Dir(deployed), 0755))
			},
			wantErrIs: errAny,
		},
		{
			name:    "downgrade_version",
			version: "1.0.0",
			setup: func(t *testing.T, downloaded, deployed string) {
				setupVersionDir(t, downloaded, "1.0.0", "1.0.0")
				v2 := setupVersionDir(t, downloaded, "2.0.0", "2.0.0")
				setupSymlink(t, deployed, v2)
			},
			checkResult: func(t *testing.T, _, deployed string) {
				content, err := os.ReadFile(filepath.Join(deployed, "version"))
				require.NoError(t, err)
				assert.Equal(t, "1.0.0", string(content))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			downloaded := filepath.Join(tmpDir, "downloaded", "my-package")
			deployed := filepath.Join(tmpDir, "deployed", "my-package")

			if tc.setup != nil {
				tc.setup(t, downloaded, deployed)
			}

			inst := symlink.NewInstaller(new(mockDownloader), log.NewNop())

			ctx := context.Background()
			if tc.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := inst.Install(ctx, downloaded, deployed, "my-package", tc.version)

			if tc.wantErrIs != nil {
				require.Error(t, err)
				if !errors.Is(tc.wantErrIs, errAny) {
					assert.ErrorIs(t, err, tc.wantErrIs)
				}
				return
			}

			require.NoError(t, err)
			if tc.checkResult != nil {
				tc.checkResult(t, downloaded, deployed)
			}
		})
	}
}

func TestUninstall(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, downloaded, deployed string)
		keep        bool
		checkResult func(t *testing.T, downloaded, deployed string)
	}{
		{
			name: "removes_symlink_keeps_files",
			setup: func(t *testing.T, downloaded, deployed string) {
				versionPath := setupVersionDir(t, downloaded, "1.0.0", "1.0.0")
				setupSymlink(t, deployed, versionPath)
			},
			keep: true,
			checkResult: func(t *testing.T, downloaded, deployed string) {
				_, err := os.Lstat(deployed)
				assert.True(t, os.IsNotExist(err), "symlink should be removed")
				assert.DirExists(t, filepath.Join(downloaded, "1.0.0"), "version dir should be kept")
			},
		},
		{
			name: "removes_symlink_deletes_files",
			setup: func(t *testing.T, downloaded, deployed string) {
				versionPath := setupVersionDir(t, downloaded, "1.0.0", "1.0.0")
				setupSymlink(t, deployed, versionPath)
			},
			keep: false,
			checkResult: func(t *testing.T, downloaded, deployed string) {
				_, err := os.Lstat(deployed)
				assert.True(t, os.IsNotExist(err), "symlink should be removed")
				_, err = os.Stat(downloaded)
				assert.True(t, os.IsNotExist(err), "downloaded dir should be removed")
			},
		},
		{
			name:  "idempotent_when_not_exists",
			setup: nil,
			keep:  true,
			checkResult: func(t *testing.T, _, deployed string) {
				_, err := os.Lstat(deployed)
				assert.True(t, os.IsNotExist(err))
			},
		},
		{
			name: "keeps_multiple_versions",
			setup: func(t *testing.T, downloaded, deployed string) {
				setupVersionDir(t, downloaded, "1.0.0", "1.0.0")
				versionPath := setupVersionDir(t, downloaded, "2.0.0", "2.0.0")
				setupSymlink(t, deployed, versionPath)
			},
			keep: true,
			checkResult: func(t *testing.T, downloaded, deployed string) {
				_, err := os.Lstat(deployed)
				assert.True(t, os.IsNotExist(err), "symlink should be removed")
				assert.DirExists(t, filepath.Join(downloaded, "1.0.0"), "v1 should be kept")
				assert.DirExists(t, filepath.Join(downloaded, "2.0.0"), "v2 should be kept")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			downloaded := filepath.Join(tmpDir, "downloaded", "my-package")
			deployed := filepath.Join(tmpDir, "deployed", "my-package")

			if tc.setup != nil {
				tc.setup(t, downloaded, deployed)
			}

			inst := symlink.NewInstaller(new(mockDownloader), log.NewNop())

			err := inst.Uninstall(context.Background(), downloaded, deployed, "my-package", tc.keep)
			require.NoError(t, err)

			if tc.checkResult != nil {
				tc.checkResult(t, downloaded, deployed)
			}
		})
	}
}

func TestLifecycle(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		cleanup  bool
	}{
		{
			name:     "single_version_with_cleanup",
			versions: []string{"1.0.0"},
			cleanup:  true,
		},
		{
			name:     "single_version_keep_files",
			versions: []string{"1.0.0"},
			cleanup:  false,
		},
		{
			name:     "version_upgrade_path",
			versions: []string{"1.0.0", "1.1.0", "2.0.0"},
			cleanup:  true,
		},
		{
			name:     "version_downgrade",
			versions: []string{"2.0.0", "1.0.0"},
			cleanup:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			downloaded := filepath.Join(tmpDir, "downloaded", "my-package")
			deployed := filepath.Join(tmpDir, "deployed", "my-package")

			// Create parent directory for deployed symlink
			require.NoError(t, os.MkdirAll(filepath.Dir(deployed), 0755))

			inst := symlink.NewInstaller(new(mockDownloader), log.NewNop())
			repo := registry.Remote{Name: "test-repo", Repository: "registry.example.com"}
			ctx := context.Background()

			// Download and install each version
			for _, version := range tc.versions {
				err := inst.Download(ctx, repo, downloaded, "my-package", version)
				require.NoError(t, err, "download %s", version)

				err = inst.Install(ctx, downloaded, deployed, "my-package", version)
				require.NoError(t, err, "install %s", version)

				// Verify correct version is deployed
				content, err := os.ReadFile(filepath.Join(deployed, "package.yaml"))
				require.NoError(t, err)
				assert.Contains(t, string(content), "version: "+version)
			}

			// All version directories should exist before uninstall
			for _, version := range tc.versions {
				assert.DirExists(t, filepath.Join(downloaded, version))
			}

			// Uninstall
			err := inst.Uninstall(ctx, downloaded, deployed, "my-package", !tc.cleanup)
			require.NoError(t, err)

			// Verify symlink removed
			_, err = os.Lstat(deployed)
			assert.True(t, os.IsNotExist(err), "symlink should be removed")

			// Verify cleanup behavior
			_, err = os.Stat(downloaded)
			if tc.cleanup {
				assert.True(t, os.IsNotExist(err), "downloaded dir should be removed")
			} else {
				assert.NoError(t, err, "downloaded dir should be kept")
			}
		})
	}
}
