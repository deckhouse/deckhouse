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

package symlink_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/deployer/symlink"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// mockDownloader simulates registry download by creating a directory with a marker file.
type mockDownloader struct {
	downloadErr      error
	partialBeforeErr bool
	calls            int
}

// Download writes a package marker file and optionally returns a configured failure.
func (m *mockDownloader) Download(_ context.Context, _ registry.Remote, out, packageName, tag string) error {
	m.calls++
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	markerFile := filepath.Join(out, "package.yaml")
	if err := os.WriteFile(markerFile, []byte("name: "+packageName+"\nversion: "+tag), 0644); err != nil {
		return err
	}
	if m.downloadErr != nil {
		return m.downloadErr
	}
	if m.partialBeforeErr {
		return errors.New("partial download failed")
	}
	return nil
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

// TestDeployDownloadsPackage verifies that Deploy downloads package contents and exposes the version path.
func TestDeployDownloadsPackage(t *testing.T) {
	tests := []struct {
		name        string
		downloadErr error
		cancelCtx   bool
		wantErrIs   error // nil = success, errAny = any error, specific = errors.Is check
		checkResult func(t *testing.T, downloaded, deployed string)
	}{
		{
			name: "success",
			checkResult: func(t *testing.T, downloaded, deployed string) {
				versionPath := filepath.Join(downloaded, "1.0.0")
				assert.DirExists(t, versionPath)
				assert.FileExists(t, filepath.Join(versionPath, "package.yaml"))

				linkTarget, err := os.Readlink(deployed)
				require.NoError(t, err)
				assert.Equal(t, versionPath, linkTarget)
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
			deployed := filepath.Join(tmpDir, "deployed", "my-package")

			deployer := symlink.NewDeployer(&mockDownloader{downloadErr: tc.downloadErr}, log.NewNop())
			repo := registry.Remote{Name: "test-repo", Repository: "registry.example.com"}

			ctx := context.Background()
			if tc.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := deployer.Deploy(ctx, repo, downloaded, deployed, "my-package", "my-package", "1.0.0")

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
				tc.checkResult(t, downloaded, deployed)
			}
		})
	}
}

// TestDeployRemovesPartialDownload verifies that failed downloads do not publish reusable version directories.
func TestDeployRemovesPartialDownload(t *testing.T) {
	tmpDir := t.TempDir()
	downloaded := filepath.Join(tmpDir, "downloaded", "my-package")
	deployed := filepath.Join(tmpDir, "deployed", "my-package")
	repo := registry.Remote{Name: "test-repo", Repository: "registry.example.com"}

	failingDownloader := &mockDownloader{partialBeforeErr: true}
	deployer := symlink.NewDeployer(failingDownloader, log.NewNop())

	err := deployer.Deploy(context.Background(), repo, downloaded, deployed, "my-package", "my-package", "1.0.0")
	require.Error(t, err)

	_, statErr := os.Stat(filepath.Join(downloaded, "1.0.0"))
	require.True(t, os.IsNotExist(statErr), "partial version dir should not be published")
	_, statErr = os.Lstat(deployed)
	require.True(t, os.IsNotExist(statErr), "failed deploy should not create deployed symlink")

	successfulDownloader := &mockDownloader{}
	deployer = symlink.NewDeployer(successfulDownloader, log.NewNop())

	err = deployer.Deploy(context.Background(), repo, downloaded, deployed, "my-package", "my-package", "1.0.0")
	require.NoError(t, err)
	require.Equal(t, 1, successfulDownloader.calls)

	content, err := os.ReadFile(filepath.Join(deployed, "package.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "version: 1.0.0")
}

// TestDeployReusesCompletedVersion verifies that an existing completed version is reused without registry access.
func TestDeployReusesCompletedVersion(t *testing.T) {
	tmpDir := t.TempDir()
	downloaded := filepath.Join(tmpDir, "downloaded", "my-package")
	deployed := filepath.Join(tmpDir, "deployed", "my-package")
	versionPath := filepath.Join(downloaded, "1.0.0")
	require.NoError(t, os.MkdirAll(versionPath, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(versionPath, "package.yaml"), []byte("name: my-package\nversion: cached"), 0644))

	downloader := &mockDownloader{}
	deployer := symlink.NewDeployer(downloader, log.NewNop())
	repo := registry.Remote{Name: "test-repo", Repository: "registry.example.com"}

	err := deployer.Deploy(context.Background(), repo, downloaded, deployed, "my-package", "my-package", "1.0.0")
	require.NoError(t, err)
	require.Zero(t, downloader.calls)

	content, err := os.ReadFile(filepath.Join(deployed, "package.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "version: cached")
}

// TestDeploy verifies symlink deployment across create, replace, cancel, and downgrade paths.
func TestDeploy(t *testing.T) {
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

			deployer := symlink.NewDeployer(new(mockDownloader), log.NewNop())

			ctx := context.Background()
			if tc.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			repo := registry.Remote{Name: "test-repo", Repository: "registry.example.com"}
			err := deployer.Deploy(ctx, repo, downloaded, deployed, "my-package", "my-package", tc.version)

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

// TestUndeploy verifies symlink removal and downloaded file cleanup behavior.
func TestUndeploy(t *testing.T) {
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

			deployer := symlink.NewDeployer(new(mockDownloader), log.NewNop())

			err := deployer.Undeploy(context.Background(), downloaded, deployed, "my-package", tc.keep)
			require.NoError(t, err)

			if tc.checkResult != nil {
				tc.checkResult(t, downloaded, deployed)
			}
		})
	}
}

// TestLifecycle verifies repeated deploy and undeploy operations across version transitions.
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

			deployer := symlink.NewDeployer(new(mockDownloader), log.NewNop())
			repo := registry.Remote{Name: "test-repo", Repository: "registry.example.com"}
			ctx := context.Background()

			// Download and deploy each version.
			for _, version := range tc.versions {
				err := deployer.Deploy(ctx, repo, downloaded, deployed, "my-package", "my-package", version)
				require.NoError(t, err, "deploy %s", version)

				// Verify correct version is deployed
				content, err := os.ReadFile(filepath.Join(deployed, "package.yaml"))
				require.NoError(t, err)
				assert.Contains(t, string(content), "version: "+version)
			}

			// All version directories should exist before undeploy.
			for _, version := range tc.versions {
				assert.DirExists(t, filepath.Join(downloaded, version))
			}

			// Undeploy
			err := deployer.Undeploy(ctx, downloaded, deployed, "my-package", !tc.cleanup)
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
