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

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/deployer"
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

// setupDeployer creates a test deployer and returns package-specific paths.
func setupDeployer(tmpDir string, downloader *mockDownloader) (*symlink.Deployer, string, string) {
	root := filepath.Join(tmpDir, "root")
	packageDir := filepath.Join(root, "apps", "test-repo", "my-package")
	deployed := filepath.Join(root, "apps", "deployed", "my-package")

	return symlink.NewDeployer(downloader, filepath.Join(root, "apps"), log.NewNop()), packageDir, deployed
}

// TestDeployDownloadsPackage verifies that Deploy downloads package contents and exposes the version path.
func TestDeployDownloadsPackage(t *testing.T) {
	tests := []struct {
		name        string
		downloadErr error
		cancelCtx   bool
		wantErrIs   error // nil = success, errAny = any error, specific = errors.Is check
		checkResult func(t *testing.T, packageDir, deployed string)
	}{
		{
			name: "success",
			checkResult: func(t *testing.T, packageDir, deployed string) {
				versionPath := filepath.Join(packageDir, "1.0.0")
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
			deployer, packageDir, deployed := setupDeployer(tmpDir, &mockDownloader{downloadErr: tc.downloadErr})

			repo := registry.Remote{Name: "test-repo", Repository: "registry.example.com"}

			ctx := context.Background()
			if tc.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := deployer.Deploy(ctx, repo, "my-package", "my-package", "1.0.0")

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
				tc.checkResult(t, packageDir, deployed)
			}
		})
	}
}

// TestDeployRemovesPartialDownload verifies that failed downloads do not publish reusable version directories.
func TestDeployRemovesPartialDownload(t *testing.T) {
	tmpDir := t.TempDir()
	repo := registry.Remote{Name: "test-repo", Repository: "registry.example.com"}

	failingDownloader := &mockDownloader{partialBeforeErr: true}
	deployer, packageDir, deployed := setupDeployer(tmpDir, failingDownloader)

	err := deployer.Deploy(context.Background(), repo, "my-package", "my-package", "1.0.0")
	require.Error(t, err)

	_, statErr := os.Stat(filepath.Join(packageDir, "1.0.0"))
	require.True(t, os.IsNotExist(statErr), "partial version dir should not be published")
	_, statErr = os.Lstat(deployed)
	require.True(t, os.IsNotExist(statErr), "failed deploy should not create deployed symlink")

	successfulDownloader := &mockDownloader{}
	deployer, _, _ = setupDeployer(tmpDir, successfulDownloader)

	err = deployer.Deploy(context.Background(), repo, "my-package", "my-package", "1.0.0")
	require.NoError(t, err)
	require.Equal(t, 1, successfulDownloader.calls)

	content, err := os.ReadFile(filepath.Join(deployed, "package.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "version: 1.0.0")
}

// TestDeployReusesCompletedVersion verifies that an existing completed version is reused without registry access.
func TestDeployReusesCompletedVersion(t *testing.T) {
	tmpDir := t.TempDir()
	downloader := &mockDownloader{}
	deployer, packageDir, deployed := setupDeployer(tmpDir, downloader)
	versionPath := filepath.Join(packageDir, "1.0.0")
	require.NoError(t, os.MkdirAll(versionPath, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(versionPath, "package.yaml"), []byte("name: my-package\nversion: cached"), 0644))

	repo := registry.Remote{Name: "test-repo", Repository: "registry.example.com"}

	err := deployer.Deploy(context.Background(), repo, "my-package", "my-package", "1.0.0")
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
		setup       func(t *testing.T, packageDir, deployed string)
		cancelCtx   bool
		wantErrIs   error
		checkResult func(t *testing.T, packageDir, deployed string)
	}{
		{
			name:    "creates_symlink",
			version: "1.0.0",
			setup: func(t *testing.T, packageDir, deployed string) {
				setupVersionDir(t, packageDir, "1.0.0", "1.0.0")
				require.NoError(t, os.MkdirAll(filepath.Dir(deployed), 0755))
			},
			checkResult: func(t *testing.T, packageDir, deployed string) {
				versionPath := filepath.Join(packageDir, "1.0.0")
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
			setup: func(t *testing.T, packageDir, deployed string) {
				v1 := setupVersionDir(t, packageDir, "1.0.0", "1.0.0")
				setupVersionDir(t, packageDir, "2.0.0", "2.0.0")
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
			setup: func(t *testing.T, packageDir, deployed string) {
				setupVersionDir(t, packageDir, "1.0.0", "1.0.0")
				v2 := setupVersionDir(t, packageDir, "2.0.0", "2.0.0")
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
			deployer, packageDir, deployed := setupDeployer(tmpDir, new(mockDownloader))

			if tc.setup != nil {
				tc.setup(t, packageDir, deployed)
			}

			ctx := context.Background()
			if tc.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			repo := registry.Remote{Name: "test-repo", Repository: "registry.example.com"}
			err := deployer.Deploy(ctx, repo, "my-package", "my-package", tc.version)

			if tc.wantErrIs != nil {
				require.Error(t, err)
				if !errors.Is(tc.wantErrIs, errAny) {
					assert.ErrorIs(t, err, tc.wantErrIs)
				}
				return
			}

			require.NoError(t, err)
			if tc.checkResult != nil {
				tc.checkResult(t, packageDir, deployed)
			}
		})
	}
}

// TestUndeploy verifies symlink removal and package directory cleanup behavior.
func TestUndeploy(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, packageDir, deployed string)
		keep        bool
		checkResult func(t *testing.T, packageDir, deployed string)
	}{
		{
			name: "removes_symlink_keeps_files",
			setup: func(t *testing.T, packageDir, deployed string) {
				versionPath := setupVersionDir(t, packageDir, "1.0.0", "1.0.0")
				setupSymlink(t, deployed, versionPath)
			},
			keep: true,
			checkResult: func(t *testing.T, packageDir, deployed string) {
				_, err := os.Lstat(deployed)
				assert.True(t, os.IsNotExist(err), "symlink should be removed")
				assert.DirExists(t, filepath.Join(packageDir, "1.0.0"), "version dir should be kept")
			},
		},
		{
			name: "removes_symlink_deletes_files",
			setup: func(t *testing.T, packageDir, deployed string) {
				versionPath := setupVersionDir(t, packageDir, "1.0.0", "1.0.0")
				setupSymlink(t, deployed, versionPath)
			},
			keep: false,
			checkResult: func(t *testing.T, packageDir, deployed string) {
				_, err := os.Lstat(deployed)
				assert.True(t, os.IsNotExist(err), "symlink should be removed")
				_, err = os.Stat(packageDir)
				assert.True(t, os.IsNotExist(err), "package dir should be removed")
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
			setup: func(t *testing.T, packageDir, deployed string) {
				setupVersionDir(t, packageDir, "1.0.0", "1.0.0")
				versionPath := setupVersionDir(t, packageDir, "2.0.0", "2.0.0")
				setupSymlink(t, deployed, versionPath)
			},
			keep: true,
			checkResult: func(t *testing.T, packageDir, deployed string) {
				_, err := os.Lstat(deployed)
				assert.True(t, os.IsNotExist(err), "symlink should be removed")
				assert.DirExists(t, filepath.Join(packageDir, "1.0.0"), "v1 should be kept")
				assert.DirExists(t, filepath.Join(packageDir, "2.0.0"), "v2 should be kept")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			deployer, packageDir, deployed := setupDeployer(tmpDir, new(mockDownloader))

			if tc.setup != nil {
				tc.setup(t, packageDir, deployed)
			}

			err := deployer.Undeploy(context.Background(), "my-package", tc.keep)
			require.NoError(t, err)

			if tc.checkResult != nil {
				tc.checkResult(t, packageDir, deployed)
			}
		})
	}
}

// newCleanupDeployer creates a fresh deployer rooted at a tempdir-relative apps/ directory.
// Returns the deployer and the apps root that was passed to NewDeployer.
func newCleanupDeployer(t *testing.T) (*symlink.Deployer, string) {
	t.Helper()
	appsRoot := filepath.Join(t.TempDir(), "root", "apps")
	require.NoError(t, os.MkdirAll(appsRoot, 0755))
	return symlink.NewDeployer(new(mockDownloader), appsRoot, log.NewNop()), appsRoot
}

// makePackageVersion lays out <appsRoot>/<repo>/<pkg>/<version>/version and returns the version dir.
func makePackageVersion(t *testing.T, appsRoot, repo, pkg, version string) string {
	t.Helper()
	return setupVersionDir(t, filepath.Join(appsRoot, repo, pkg), version, version)
}

// makeDeployedSymlink creates <appsRoot>/deployed/<name> -> target and returns the symlink path.
func makeDeployedSymlink(t *testing.T, appsRoot, name, target string) string {
	t.Helper()
	deployed := filepath.Join(appsRoot, "deployed", name)
	setupSymlink(t, deployed, target)
	return deployed
}

// preserved is a shorthand for building a PreservePackage triple in test tables.
func preserved(repo, pkg, version string) deployer.PreservePackage {
	return deployer.PreservePackage{Repository: repo, Name: pkg, Version: version}
}

// TestCleanup verifies that Cleanup removes only the package versions and deployed
// symlinks that are not listed in the preserve set, and that empty parent directories
// collapse afterwards.
func TestCleanup(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, appsRoot string) []deployer.PreservePackage
		cancelCtx bool
		wantErrIs error
		check     func(t *testing.T, appsRoot string)
	}{
		{
			name: "empty_preserve_removes_all_packages",
			setup: func(t *testing.T, appsRoot string) []deployer.PreservePackage {
				makePackageVersion(t, appsRoot, "repo-a", "pkg-1", "1.0.0")
				makePackageVersion(t, appsRoot, "repo-a", "pkg-2", "2.0.0")
				makePackageVersion(t, appsRoot, "repo-b", "pkg-3", "3.0.0")
				return nil
			},
			check: func(t *testing.T, appsRoot string) {
				for _, repo := range []string{"repo-a", "repo-b"} {
					_, err := os.Stat(filepath.Join(appsRoot, repo))
					assert.True(t, os.IsNotExist(err), "%s should be removed", repo)
				}
			},
		},
		{
			name: "preserve_one_version_removes_other_versions",
			setup: func(t *testing.T, appsRoot string) []deployer.PreservePackage {
				makePackageVersion(t, appsRoot, "repo-a", "pkg-1", "1.0.0")
				makePackageVersion(t, appsRoot, "repo-a", "pkg-1", "2.0.0")
				return []deployer.PreservePackage{preserved("repo-a", "pkg-1", "1.0.0")}
			},
			check: func(t *testing.T, appsRoot string) {
				assert.DirExists(t, filepath.Join(appsRoot, "repo-a", "pkg-1", "1.0.0"))
				_, err := os.Stat(filepath.Join(appsRoot, "repo-a", "pkg-1", "2.0.0"))
				assert.True(t, os.IsNotExist(err), "non-preserved version should be removed")
			},
		},
		{
			name: "preserve_one_package_removes_siblings_in_same_repo",
			setup: func(t *testing.T, appsRoot string) []deployer.PreservePackage {
				makePackageVersion(t, appsRoot, "repo-a", "kept", "1.0.0")
				makePackageVersion(t, appsRoot, "repo-a", "dropped", "1.0.0")
				return []deployer.PreservePackage{preserved("repo-a", "kept", "1.0.0")}
			},
			check: func(t *testing.T, appsRoot string) {
				assert.DirExists(t, filepath.Join(appsRoot, "repo-a", "kept", "1.0.0"))
				_, err := os.Stat(filepath.Join(appsRoot, "repo-a", "dropped"))
				assert.True(t, os.IsNotExist(err), "sibling package should be removed")
			},
		},
		{
			name: "preserve_one_repo_removes_other_repos",
			setup: func(t *testing.T, appsRoot string) []deployer.PreservePackage {
				makePackageVersion(t, appsRoot, "repo-a", "pkg-1", "1.0.0")
				makePackageVersion(t, appsRoot, "repo-b", "pkg-1", "1.0.0")
				return []deployer.PreservePackage{preserved("repo-a", "pkg-1", "1.0.0")}
			},
			check: func(t *testing.T, appsRoot string) {
				assert.DirExists(t, filepath.Join(appsRoot, "repo-a", "pkg-1", "1.0.0"))
				_, err := os.Stat(filepath.Join(appsRoot, "repo-b"))
				assert.True(t, os.IsNotExist(err), "non-preserved repo should be removed")
			},
		},
		{
			name: "deployed_symlink_to_preserved_version_kept",
			setup: func(t *testing.T, appsRoot string) []deployer.PreservePackage {
				v := makePackageVersion(t, appsRoot, "repo-a", "pkg-1", "1.0.0")
				makeDeployedSymlink(t, appsRoot, "alias", v)
				return []deployer.PreservePackage{preserved("repo-a", "pkg-1", "1.0.0")}
			},
			check: func(t *testing.T, appsRoot string) {
				link := filepath.Join(appsRoot, "deployed", "alias")
				info, err := os.Lstat(link)
				require.NoError(t, err, "symlink should still exist")
				assert.NotZero(t, info.Mode()&os.ModeSymlink, "alias should still be a symlink")
				assert.DirExists(t, filepath.Join(appsRoot, "repo-a", "pkg-1", "1.0.0"))
			},
		},
		{
			name: "deployed_symlink_to_unpreserved_version_removed",
			setup: func(t *testing.T, appsRoot string) []deployer.PreservePackage {
				v1 := makePackageVersion(t, appsRoot, "repo-a", "pkg-1", "1.0.0")
				makePackageVersion(t, appsRoot, "repo-a", "pkg-1", "2.0.0")
				makeDeployedSymlink(t, appsRoot, "alias", v1)
				return []deployer.PreservePackage{preserved("repo-a", "pkg-1", "2.0.0")}
			},
			check: func(t *testing.T, appsRoot string) {
				_, err := os.Lstat(filepath.Join(appsRoot, "deployed", "alias"))
				assert.True(t, os.IsNotExist(err), "stale symlink should be removed")
				_, err = os.Stat(filepath.Join(appsRoot, "repo-a", "pkg-1", "1.0.0"))
				assert.True(t, os.IsNotExist(err), "stale version should be removed")
				assert.DirExists(t, filepath.Join(appsRoot, "repo-a", "pkg-1", "2.0.0"))
			},
		},
		{
			name: "relative_symlink_target_matches_preserve",
			setup: func(t *testing.T, appsRoot string) []deployer.PreservePackage {
				v := makePackageVersion(t, appsRoot, "repo-a", "pkg-1", "1.0.0")
				deployedRoot := filepath.Join(appsRoot, "deployed")
				require.NoError(t, os.MkdirAll(deployedRoot, 0755))
				rel, err := filepath.Rel(deployedRoot, v)
				require.NoError(t, err)
				require.NoError(t, os.Symlink(rel, filepath.Join(deployedRoot, "alias")))
				return []deployer.PreservePackage{preserved("repo-a", "pkg-1", "1.0.0")}
			},
			check: func(t *testing.T, appsRoot string) {
				_, err := os.Lstat(filepath.Join(appsRoot, "deployed", "alias"))
				require.NoError(t, err, "relative symlink should be kept when target is preserved")
			},
		},
		{
			name: "non_symlink_entry_in_deployed_left_alone",
			setup: func(t *testing.T, appsRoot string) []deployer.PreservePackage {
				deployedRoot := filepath.Join(appsRoot, "deployed")
				require.NoError(t, os.MkdirAll(deployedRoot, 0755))
				require.NoError(t, os.WriteFile(filepath.Join(deployedRoot, "stray.txt"), []byte("x"), 0644))
				return nil
			},
			check: func(t *testing.T, appsRoot string) {
				_, err := os.Stat(filepath.Join(appsRoot, "deployed", "stray.txt"))
				require.NoError(t, err, "non-symlink entry should be left alone")
			},
		},
		{
			name: "deployed_dir_skipped_during_package_walk",
			setup: func(t *testing.T, appsRoot string) []deployer.PreservePackage {
				v := makePackageVersion(t, appsRoot, "repo-a", "pkg-1", "1.0.0")
				makeDeployedSymlink(t, appsRoot, "alias", v)
				return nil
			},
			check: func(t *testing.T, appsRoot string) {
				assert.DirExists(t, filepath.Join(appsRoot, "deployed"), "deployed root should not be removed")
				_, err := os.Lstat(filepath.Join(appsRoot, "deployed", "alias"))
				assert.True(t, os.IsNotExist(err), "stale symlink should be removed")
				_, err = os.Stat(filepath.Join(appsRoot, "repo-a"))
				assert.True(t, os.IsNotExist(err), "non-preserved repo should be removed")
			},
		},
		{
			name: "preserve_with_missing_version_collapses_empty_dirs",
			setup: func(t *testing.T, appsRoot string) []deployer.PreservePackage {
				require.NoError(t, os.MkdirAll(filepath.Join(appsRoot, "repo-a", "pkg-1"), 0755))
				return []deployer.PreservePackage{preserved("repo-a", "pkg-1", "1.0.0")}
			},
			check: func(t *testing.T, appsRoot string) {
				_, err := os.Stat(filepath.Join(appsRoot, "repo-a"))
				assert.True(t, os.IsNotExist(err), "empty preserved repo should collapse")
			},
		},
		{
			name:  "idempotent_on_missing_tree",
			setup: func(_ *testing.T, _ string) []deployer.PreservePackage { return nil },
			check: func(*testing.T, string) {},
		},
		{
			name: "context_canceled_aborts_cleanup",
			setup: func(t *testing.T, appsRoot string) []deployer.PreservePackage {
				makePackageVersion(t, appsRoot, "repo-a", "pkg-1", "1.0.0")
				return nil
			},
			cancelCtx: true,
			wantErrIs: context.Canceled,
			check: func(t *testing.T, appsRoot string) {
				assert.DirExists(t, filepath.Join(appsRoot, "repo-a", "pkg-1", "1.0.0"), "tree should remain when ctx canceled")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, appsRoot := newCleanupDeployer(t)

			preserve := tc.setup(t, appsRoot)

			ctx := context.Background()
			if tc.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := d.Cleanup(ctx, preserve)
			if tc.wantErrIs != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErrIs)
			} else {
				require.NoError(t, err)
			}

			tc.check(t, appsRoot)
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
			deployer, packageDir, deployed := setupDeployer(tmpDir, new(mockDownloader))

			// Create parent directory for deployed symlink
			require.NoError(t, os.MkdirAll(filepath.Dir(deployed), 0755))

			repo := registry.Remote{Name: "test-repo", Repository: "registry.example.com"}
			ctx := context.Background()

			// Download and deploy each version.
			for _, version := range tc.versions {
				err := deployer.Deploy(ctx, repo, "my-package", "my-package", version)
				require.NoError(t, err, "deploy %s", version)

				// Verify correct version is deployed
				content, err := os.ReadFile(filepath.Join(deployed, "package.yaml"))
				require.NoError(t, err)
				assert.Contains(t, string(content), "version: "+version)
			}

			// All version directories should exist before undeploy.
			for _, version := range tc.versions {
				assert.DirExists(t, filepath.Join(packageDir, version))
			}

			// Undeploy
			err := deployer.Undeploy(ctx, "my-package", !tc.cleanup)
			require.NoError(t, err)

			// Verify symlink removed
			_, err = os.Lstat(deployed)
			assert.True(t, os.IsNotExist(err), "symlink should be removed")

			// Verify cleanup behavior
			_, err = os.Stat(packageDir)
			if tc.cleanup {
				assert.True(t, os.IsNotExist(err), "package dir should be removed")
			} else {
				assert.NoError(t, err, "package dir should be kept")
			}
		})
	}
}
