// Copyright 2025 Flant JSC
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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/digests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	fsutils "github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
)

var (
	terraformImageName = "baseTerraform"
	opentofuImageName  = "baseOpentofu"
)

const (
	terraformManagerDir = "terraform-manager"
	// baseTerraform and baseOpentofu live in this section too.
	installerToolsSection = "terraformManager"
)

type InfrastructureUtilProvider struct {
	m sync.Mutex

	binariesDir string
	// download replaces the registry pull in tests.
	download image.DownloadFunc
}

func newInfrastructureUtilProvider(binariesDir string) *InfrastructureUtilProvider {
	return &InfrastructureUtilProvider{
		binariesDir: binariesDir,
	}
}

func (p *InfrastructureUtilProvider) DownloadTerraform(ctx context.Context, _ cloud.InfrastructureUtilProviderParams, destination string, conf *config.MetaConfig) error {
	return p.setupBinary(ctx, conf, "terraform", terraformImageName, destination)
}

func (p *InfrastructureUtilProvider) DownloadOpenTofu(ctx context.Context, _ cloud.InfrastructureUtilProviderParams, destination string, conf *config.MetaConfig) error {
	return p.setupBinary(ctx, conf, "opentofu", opentofuImageName, destination)
}

func (p *InfrastructureUtilProvider) setupBinary(ctx context.Context, conf *config.MetaConfig, binaryName, imageName, destination string) error {
	p.m.Lock()
	defer p.m.Unlock()

	bundled := filepath.Join(p.binariesDir, binaryName)
	if _, err := os.Stat(bundled); err == nil {
		return fsutils.CreateLinkIfNotExists(ctx, bundled, checkIsExecFile, destination)
	}

	// Edition-wide binaries, not provider ones: no provider module publishes them.
	dir, err := ensureInstallerImage(ctx, conf, binaryName, installerToolsSection, imageName, p.download)
	if err != nil {
		return err
	}

	return fsutils.CreateLinkIfNotExists(ctx, filepath.Join(dir, binaryName), checkIsExecFile, destination)
}

// ensureInstallerImage unpacks the image this installer's digests name and returns its directory.
// The digest is read on every call: an in-cluster dhctl takes it from a file that changes under it.
func ensureInstallerImage(ctx context.Context, conf *config.MetaConfig, dirName, section, imageName string, download image.DownloadFunc) (string, error) {
	digest, err := digests.GetImage(section, imageName)
	if err != nil {
		return "", fmt.Errorf("get %s image digest from section %s: %w", imageName, section, err)
	}

	return image.EnsureUnpacked(ctx, image.UnpackRequest{
		Root:   conf.DownloadRootDir,
		Name:   dirName,
		Digest: digest,
		Registry: func(context.Context) (*image.RegistryConfig, error) {
			return imageSource(conf)
		},
	}, image.EnsureUnpackedOptions{
		Download:     download,
		ShowProgress: conf.ShowProgress,
	})
}

// imageSource is the registry to pull an installer-side image from.
//
// Two sources, and which one applies is decided by whether the legacy field says anything at all.
// `MetaConfig.DeckhouseConfig` is a struct value rather than a pointer, so the nil check this used to
// make was true unconditionally and the second branch was unreachable — invisible for every
// installation that names a registry in `InitConfiguration.deckhouse`, and fatal for one that names
// none: an empty dockercfg decodes to no bytes at all, and the download dies with "unmarshaling
// dockerconfig JSON: unexpected end of JSON input" before it ever asks where the images are.
//
// An installation from a bundle is exactly that case, and it is not an exotic one: the images are
// served by a local registry reached over a reverse tunnel, which has no credentials to state and no
// address anybody typed. `conf.Registry` is the resolved registry for every mode, so reaching for it
// is the general answer rather than a special case — the legacy field simply keeps precedence
// wherever it does state a registry.
func imageSource(conf *config.MetaConfig) (*image.RegistryConfig, error) {
	if dc := conf.DeckhouseConfig; dc.RegistryDockerCfg != "" && dc.ImagesRepo != "" {
		cfg, err := image.DecodeDockerConfig(dc.RegistryDockerCfg)
		if err != nil {
			return nil, err
		}
		scheme := "HTTPS"
		if upper := strings.ToUpper(dc.RegistryScheme); upper == "HTTP" || upper == "HTTPS" {
			scheme = upper
		}
		regConfig, err := image.RegistryConfigFromDockerConfig(cfg, scheme, dc.ImagesRepo)
		if err == nil {
			// RegistryConfigFromDockerConfig only decodes credentials, so a registry behind a
			// private CA fails here with x509 unless the CA is carried over by hand.
			regConfig.SetCA(dc.RegistryCA)
		}
		return regConfig, err
	}

	remote := conf.Registry.Settings.RemoteData
	regConfig, err := image.NewRegistryConfig(
		string(remote.Scheme), remote.ImagesRepo, remote.Username, remote.Password, remote.CA)
	return regConfig, err
}
