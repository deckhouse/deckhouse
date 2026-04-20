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
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/digests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	fsutils "github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
)

var (
	terraformImageName = "baseTerraform"
	opentofuImageName  = "baseOpentofu"
)

type InfrastructureUtilProvider struct {
	m sync.Mutex

	logger      log.Logger
	binariesDir string
}

func newInfrastructureUtilProvider(logger log.Logger, binariesDir string) *InfrastructureUtilProvider {
	return &InfrastructureUtilProvider{
		logger:      logger,
		binariesDir: binariesDir,
	}
}

func (p *InfrastructureUtilProvider) DownloadTerraform(ctx context.Context, _ cloud.InfrastructureUtilProviderParams, destination string, conf *config.MetaConfig) error {
	p.m.Lock()
	defer p.m.Unlock()

	_, err := os.Stat(filepath.Join(p.binariesDir, "terraform"))
	if err == nil {
		return fsutils.CreateLinkIfNotExists(filepath.Join(p.binariesDir, "terraform"), checkIsExecFile, destination, p.logger)
	}
	if err = downloadImage(ctx, conf, terraformImageName, "terraformManager"); err != nil {
		return err
	}

	return fsutils.CreateLinkIfNotExists(filepath.Join(conf.DownloadRootDir, "terraform"), checkIsExecFile, destination, p.logger)
}

func (p *InfrastructureUtilProvider) DownloadOpenTofu(ctx context.Context, _ cloud.InfrastructureUtilProviderParams, destination string, conf *config.MetaConfig) error {
	p.m.Lock()
	defer p.m.Unlock()

	_, err := os.Stat(filepath.Join(p.binariesDir, "opentofu"))
	if err == nil {
		return fsutils.CreateLinkIfNotExists(filepath.Join(p.binariesDir, "opentofu"), checkIsExecFile, destination, p.logger)
	}
	if err = downloadImage(ctx, conf, opentofuImageName, "terraformManager"); err != nil {
		return err
	}

	return fsutils.CreateLinkIfNotExists(filepath.Join(conf.DownloadRootDir, "opentofu"), checkIsExecFile, destination, p.logger)
}

func downloadImage(ctx context.Context, conf *config.MetaConfig, name, section string) error {
	regConfig, err := image.NewRegistryConfig(string(conf.Registry.Settings.RemoteData.Scheme), conf.Registry.Settings.RemoteData.ImagesRepo, conf.Registry.Settings.RemoteData.Username, conf.Registry.Settings.RemoteData.Password, conf.Registry.Settings.RemoteData.CA)
	if err != nil {
		return err
	}
	tfImage, err := digests.GetImage(section, name)
	if err != nil {
		return err
	}

	return image.DownloadAndUnpackImage(ctx, conf.Registry.Settings.RemoteData.ImagesRepo+"@"+tfImage, conf.DownloadRootDir, conf.DownloadCacheDir, *regConfig)
}
