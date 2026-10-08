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
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
)

func testKubernetesPluginSettings(cloudName string) *settings.Simple {
	return &settings.Simple{
		NamespaceVal:         ptr.To("hashicorp"),
		TypeVal:              ptr.To("kubernetes"),
		CloudNameVal:         ptr.To(cloudName),
		DestinationBinaryVal: ptr.To("terraform-provider-kubernetes"),
		UseOpenTofuVal:       ptr.To(true),
	}
}

func writeTestPlugin(t *testing.T, dir, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "terraform-provider-kubernetes"), []byte(body), 0o755))
}

// An in-tree provider's plugin comes from the terraform-manager image this installer's digests
// name. A flat terraform-manager dir left by another dhctl version is not that image.
func TestDownloadPluginIgnoresFlatTerraformManagerDir(t *testing.T) {
	tests.StubImagesDigests(t, `{"cloudProviderAws": {"terraformManager": "`+testInstallerDigest+`"}}`)

	root := t.TempDir()
	writeTestPlugin(t, filepath.Join(root, "terraform-manager"), "#!/bin/sh\n# old\n")

	var downloads atomic.Int32
	p := newPluginsProvider(filepath.Join(root, "no-plugins-dir"))
	p.download = stubImageWith(filepath.Join("terraform-manager", "terraform-provider-kubernetes"), &downloads)

	dest := filepath.Join(t.TempDir(), "terraform-provider-kubernetes")
	require.NoError(t, p.DownloadPlugin(context.Background(), cloud.InfrastructurePluginProviderParams{
		Version:  cloud.Version{Version: "2.38.0", Arch: "linux_amd64"},
		Settings: testKubernetesPluginSettings("AWS"),
	}, dest, testDownloadConf(root)))

	target, err := os.Readlink(dest)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "aws@"+testInstallerDigest, "terraform-manager", "terraform-provider-kubernetes"), target)
	require.Equal(t, int32(1), downloads.Load())
}

// Two operations with different bundle builds share one download dir. Each must get the plugin
// of its own bundle, and an alias left by an older dhctl must not be read at all.
func TestDownloadPluginReadsTheBundleOfItsOwnOperation(t *testing.T) {
	root := t.TempDir()
	bundleA := filepath.Join(root, "dvp@sha256:aaa")
	bundleB := filepath.Join(root, "dvp@sha256:bbb")
	writeTestPlugin(t, filepath.Join(bundleA, "terraform-manager"), "#!/bin/sh\n# a\n")
	writeTestPlugin(t, filepath.Join(bundleB, "terraform-manager"), "#!/bin/sh\n# b\n")
	writeTestPlugin(t, filepath.Join(root, "dvp", "terraform-manager"), "#!/bin/sh\n# alias of an older dhctl\n")
	writeTestPlugin(t, filepath.Join(root, "terraform-manager"), "#!/bin/sh\n# flat\n")

	for _, bundleDir := range []string{bundleA, bundleB} {
		conf := testDownloadConf(root)
		conf.ProviderBundleDir = bundleDir

		p := newPluginsProvider(filepath.Join(root, "no-plugins-dir"))
		p.download = func(context.Context, string, string, image.RegistryConfig, bool) error {
			t.Error("the plugin of an unpacked bundle must not be pulled from a registry")
			return errors.New("unexpected registry pull")
		}

		dest := filepath.Join(t.TempDir(), "terraform-provider-kubernetes")
		require.NoError(t, p.DownloadPlugin(context.Background(), cloud.InfrastructurePluginProviderParams{
			Version:  cloud.Version{Version: "2.38.0", Arch: "linux_amd64"},
			Settings: testKubernetesPluginSettings("DVP"),
		}, dest, conf))

		target, err := os.Readlink(dest)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(bundleDir, "terraform-manager", "terraform-provider-kubernetes"), target)
	}
}

// Without a bundle the plugin comes from this installer's own image. An alias left by an older
// dhctl is not read.
func TestDownloadPluginWithoutABundleIgnoresTheAliasOfAnOlderDhctl(t *testing.T) {
	tests.StubImagesDigests(t, `{"cloudProviderDvp": {"terraformManager": "`+testInstallerDigest+`"}}`)

	root := t.TempDir()
	writeTestPlugin(t, filepath.Join(root, "dvp", "terraform-manager"), "#!/bin/sh\n# alias of an older dhctl\n")

	var downloads atomic.Int32
	p := newPluginsProvider(filepath.Join(root, "no-plugins-dir"))
	p.download = stubImageWith(filepath.Join("terraform-manager", "terraform-provider-kubernetes"), &downloads)

	dest := filepath.Join(t.TempDir(), "terraform-provider-kubernetes")
	require.NoError(t, p.DownloadPlugin(context.Background(), cloud.InfrastructurePluginProviderParams{
		Version:  cloud.Version{Version: "2.38.0", Arch: "linux_amd64"},
		Settings: testKubernetesPluginSettings("DVP"),
	}, dest, testDownloadConf(root)))

	target, err := os.Readlink(dest)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "dvp@"+testInstallerDigest, "terraform-manager", "terraform-provider-kubernetes"), target)
	require.Equal(t, int32(1), downloads.Load())
}

// A set bundle dir is the build to run. A plugin missing from it is an error naming where it was
// looked for, not a reason to take the installer's in-tree image instead.
func TestDownloadPluginRequiresThePluginInASetBundleDir(t *testing.T) {
	// The installer ships this provider in-tree too: a pinned bundle must not be mixed with it.
	tests.StubImagesDigests(t, `{"cloudProviderDvp": {"terraformManager": "`+testInstallerDigest+`"}}`)

	root := t.TempDir()
	bundleDir := filepath.Join(root, "dvp@sha256:aaa")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	conf := testDownloadConf(root)
	conf.ProviderBundleDir = bundleDir

	p := newPluginsProvider(filepath.Join(root, "no-plugins-dir"))
	p.download = func(context.Context, string, string, image.RegistryConfig, bool) error {
		t.Error("a bundle dir is set, the installer's image must not be pulled")
		return errors.New("unexpected registry pull")
	}

	err := p.DownloadPlugin(context.Background(), cloud.InfrastructurePluginProviderParams{
		Version:  cloud.Version{Version: "2.38.0", Arch: "linux_amd64"},
		Settings: testKubernetesPluginSettings("DVP"),
	}, filepath.Join(t.TempDir(), "terraform-provider-kubernetes"), conf)
	require.ErrorContains(t, err, filepath.Join(bundleDir, "terraform-manager", "terraform-provider-kubernetes"))
}
