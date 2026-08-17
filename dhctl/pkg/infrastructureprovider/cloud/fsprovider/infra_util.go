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
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/name212/govalue"

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

type InfrastructureUtilProvider struct {
	m sync.Mutex

	binariesDir string
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

	downloaded := filepath.Join(conf.DownloadRootDir, binaryName)
	if _, err := os.Stat(downloaded); err != nil {
		if err := downloadImage(ctx, conf, imageName, "terraformManager", conf.ShowProgress); err != nil {
			return err
		}
	}

	return fsutils.CreateLinkIfNotExists(ctx, downloaded, checkIsExecFile, destination)
}

// imageSource is where to pull an installer-side image from: the registry to talk to and the prefix
// its images are named by.
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
func imageSource(conf *config.MetaConfig) (*image.RegistryConfig, string, error) {
	if dc := conf.DeckhouseConfig; dc.RegistryDockerCfg != "" && dc.ImagesRepo != "" {
		cfg, err := image.DecodeDockerConfig(dc.RegistryDockerCfg)
		if err != nil {
			return nil, "", err
		}
		scheme := "HTTPS"
		if upper := strings.ToUpper(dc.RegistryScheme); upper == "HTTP" || upper == "HTTPS" {
			scheme = upper
		}
		regConfig, err := image.RegistryConfigFromDockerConfig(cfg, scheme, dc.ImagesRepo)
		return regConfig, dc.ImagesRepo + "@", err
	}

	remote := conf.Registry.Settings.RemoteData
	regConfig, err := image.NewRegistryConfig(
		string(remote.Scheme), remote.ImagesRepo, remote.Username, remote.Password, remote.CA)
	return regConfig, remote.ImagesRepo + "@", err
}

func downloadImage(ctx context.Context, conf *config.MetaConfig, name, section string, showProgress bool) error {
	regConfig, imageName, err := imageSource(conf)

	if govalue.IsNil(conf.ShowProgress) {
		conf.ShowProgress = false
	}

	if err != nil {
		return err
	}
	tfImage, err := digests.GetImage(section, name)
	if err != nil {
		return err
	}
	imageName += tfImage

	return image.DownloadAndUnpackImage(ctx, imageName, conf.DownloadRootDir, conf.DownloadCacheDir, *regConfig, showProgress)
}
