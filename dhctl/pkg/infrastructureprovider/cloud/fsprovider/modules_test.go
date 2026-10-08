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

package fsprovider

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
)

// Layouts candi does not carry come from the bundle of the operation, not from an alias left by
// an older dhctl beside it.
func TestDownloadModulesReadsTheBundleOfItsOwnOperation(t *testing.T) {
	root := t.TempDir()
	bundleDir := filepath.Join(root, "dvp@sha256:aaa")
	require.NoError(t, os.MkdirAll(filepath.Join(bundleDir, "layouts", "standard"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "layouts", "standard", "main.tf"), []byte("# bundle\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dvp", "layouts", "alias"), 0o755))

	dest := t.TempDir()
	p := newModulesProvider(filepath.Join(root, "no-cloud-providers"), bundleDir)
	require.NoError(t, p.DownloadModules(t.Context(), cloud.DownloadModulesParams{
		ModulesParams: cloud.ModulesParams{Settings: testKubernetesPluginSettings("DVP")},
	}, dest))

	require.FileExists(t, filepath.Join(dest, "layouts", "standard", "main.tf"))
	require.NoDirExists(t, filepath.Join(dest, "layouts", "alias"))
}

// A bundle that cannot be read is an error about the bundle, not a missing candi directory.
func TestDownloadModulesReportsAnUnreadableBundle(t *testing.T) {
	root := t.TempDir()
	// A file where the bundle dir should be: stat of anything under it fails with ENOTDIR.
	bundleDir := filepath.Join(root, "dvp@sha256:aaa")
	require.NoError(t, os.WriteFile(bundleDir, []byte("not a dir"), 0o644))

	p := newModulesProvider(filepath.Join(root, "no-cloud-providers"), bundleDir)
	err := p.DownloadModules(t.Context(), cloud.DownloadModulesParams{
		ModulesParams: cloud.ModulesParams{Settings: testKubernetesPluginSettings("DVP")},
	}, t.TempDir())
	require.ErrorContains(t, err, filepath.Join(bundleDir, "layouts"))
	require.ErrorIs(t, err, syscall.ENOTDIR)
}

// An external provider's layouts must come from its bundle, so when neither candi nor the bundle
// has them the error names both places.
func TestDownloadModulesNamesCandiAndBundleWhenLayoutsAreMissing(t *testing.T) {
	root := t.TempDir()
	cloudProviderDir := filepath.Join(root, "no-cloud-providers")
	bundleDir := filepath.Join(root, "dvp@sha256:aaa")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	p := newModulesProvider(cloudProviderDir, bundleDir)
	err := p.DownloadModules(t.Context(), cloud.DownloadModulesParams{
		ModulesParams: cloud.ModulesParams{Settings: testKubernetesPluginSettings("DVP")},
	}, t.TempDir())
	require.ErrorContains(t, err, filepath.Join(cloudProviderDir, "dvp", "layouts"))
	require.ErrorContains(t, err, filepath.Join(bundleDir, "layouts"))
	require.ErrorIs(t, err, fs.ErrNotExist)
}
