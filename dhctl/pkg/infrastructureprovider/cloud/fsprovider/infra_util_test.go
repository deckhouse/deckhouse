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
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registryconfig "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/tests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// TestAnInstallationThatNamesNoRegistryStillFindsItsImages is the failure this exists for.
//
// When the only statement a cluster makes about its registry is the registry module's own
// ModuleConfig, the legacy dockercfg is empty — and decoding it comes first, so the installation dies
// with
//
//	Cannot download infrastructure util … unmarshaling dockerconfig JSON: unexpected end of JSON input
//
// while the images are there the whole time, served by the bundle registry on the loopback address.
func TestAnInstallationThatNamesNoRegistryStillFindsItsImages(t *testing.T) {
	conf := &config.MetaConfig{}
	conf.Registry.Settings.RemoteData = registryconfig.Data{
		ImagesRepo: constant.BundleImagesRepo,
		Scheme:     constant.BundleScheme,
	}

	regConfig, err := imageSource(conf)
	require.NoError(t, err, "an installation from a bundle names no registry and must not be asked for one")
	require.Equal(t, constant.BundleImagesRepo, regConfig.GetRegistry())
	require.Equal(t, string(constant.BundleScheme), regConfig.GetScheme())

	// Anonymous, because the bundle registry has nobody to authenticate as.
	require.Empty(t, regConfig.GetUsername())
	require.Empty(t, regConfig.GetPassword())
}

// TestTheLegacyFieldKeepsPrecedenceWhenItStatesARegistry: the fallback above must not take over an
// ordinary installation. Everything that names a registry in `InitConfiguration.deckhouse` keeps
// being installed from exactly that registry, with exactly those credentials.
func TestTheLegacyFieldKeepsPrecedenceWhenItStatesARegistry(t *testing.T) {
	dockerCfg := base64.StdEncoding.EncodeToString([]byte(
		`{"auths":{"r.example.com":{"auth":"` +
			base64.StdEncoding.EncodeToString([]byte("user:secret")) + `"}}}`))

	conf := &config.MetaConfig{}
	conf.DeckhouseConfig.ImagesRepo = "r.example.com/deckhouse/ee"
	conf.DeckhouseConfig.RegistryDockerCfg = dockerCfg
	conf.DeckhouseConfig.RegistryScheme = "https"
	// Deliberately different, to show which of the two was read.
	conf.Registry.Settings.RemoteData = registryconfig.Data{
		ImagesRepo: constant.BundleImagesRepo,
		Scheme:     constant.BundleScheme,
	}

	regConfig, err := imageSource(conf)
	require.NoError(t, err)
	require.Equal(t, "r.example.com/deckhouse/ee", regConfig.GetRegistry())
	require.Equal(t, "HTTPS", regConfig.GetScheme())
	require.Equal(t, "user", regConfig.GetUsername())
	require.Equal(t, "secret", regConfig.GetPassword())
}

// TestAHalfStatedRegistryIsNotAStatedRegistry pins the boundary. A `deckhouse` section carrying a
// devBranch and nothing else — which is what a bundle installation's InitConfiguration looks like —
// states no registry, and reading it as one is what produced the failure above.
func TestAHalfStatedRegistryIsNotAStatedRegistry(t *testing.T) {
	for _, half := range []config.DeckhouseClusterConfig{
		{ImagesRepo: "r.example.com/deckhouse/ee"}, // no credentials
		{RegistryDockerCfg: "e30="},                // credentials, no repo
	} {
		conf := &config.MetaConfig{DeckhouseConfig: half}
		conf.Registry.Settings.RemoteData = registryconfig.Data{
			ImagesRepo: constant.BundleImagesRepo,
			Scheme:     constant.BundleScheme,
		}

		regConfig, err := imageSource(conf)
		require.NoError(t, err)
		require.Equal(t, constant.BundleImagesRepo, regConfig.GetRegistry(),
			"a half-stated registry must not win over the resolved one")
	}
}

const testInstallerDigest = "sha256:2222222222222222222222222222222222222222222222222222222222222222"

func testDownloadConf(root string) *config.MetaConfig {
	conf := &config.MetaConfig{DownloadRootDir: root}
	conf.Registry.Settings.RemoteData = registryconfig.Data{
		ImagesRepo: constant.BundleImagesRepo,
		Scheme:     constant.BundleScheme,
	}

	return conf
}

// stubImageWith returns a download that unpacks one executable at relPath.
func stubImageWith(relPath string, calls *atomic.Int32) image.DownloadFunc {
	return func(_ context.Context, _, dir string, _ image.RegistryConfig, _ bool) error {
		calls.Add(1)
		path := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create stub image dir: %w", err)
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\n# new\n"), 0o755); err != nil {
			return fmt.Errorf("write stub image binary: %w", err)
		}
		return nil
	}
}

// A binary left in the download dir by another dhctl version must not be taken for this one's.
func TestSetupBinaryIgnoresBinaryOfAnotherDigest(t *testing.T) {
	tests.StubImagesDigests(t, `{"terraformManager": {"baseOpentofu": "`+testInstallerDigest+`"}}`)

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "opentofu"), []byte("#!/bin/sh\n# old\n"), 0o755))

	var downloads atomic.Int32
	p := newInfrastructureUtilProvider(filepath.Join(root, "no-bin-dir"))
	p.download = stubImageWith("opentofu", &downloads)

	dest := filepath.Join(t.TempDir(), "opentofu")
	require.NoError(t, p.DownloadOpenTofu(t.Context(), cloud.InfrastructureUtilProviderParams{}, dest, testDownloadConf(root)))

	target, err := os.Readlink(dest)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "opentofu@"+testInstallerDigest, "opentofu"), target)
	require.Equal(t, int32(1), downloads.Load())

	// The digest is on disk now: a second call downloads nothing.
	require.NoError(t, p.DownloadOpenTofu(t.Context(), cloud.InfrastructureUtilProviderParams{}, dest, testDownloadConf(root)))
	require.Equal(t, int32(1), downloads.Load())
}
