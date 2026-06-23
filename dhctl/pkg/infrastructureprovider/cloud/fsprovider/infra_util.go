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
		return fsutils.CreateLinkIfNotExists(bundled, checkIsExecFile, destination, p.logger)
	}

	downloaded := filepath.Join(conf.DownloadRootDir, binaryName)
	if _, err := os.Stat(downloaded); err != nil {
		if err := downloadImage(ctx, conf, imageName, "terraformManager", conf.ShowProgress); err != nil {
			return err
		}
	}

	return fsutils.CreateLinkIfNotExists(downloaded, checkIsExecFile, destination, p.logger)
}

func downloadImage(ctx context.Context, conf *config.MetaConfig, name, section string, showProgress bool) error {
	log.InfoF("[DVP-DEBUG] downloadImage: name=%s section=%s downloadRootDir=%q downloadCacheDir=%q deckhouseConfigNotNil=%t\n",
		name, section, conf.DownloadRootDir, conf.DownloadCacheDir, govalue.NotNil(conf.DeckhouseConfig))
	var regConfig *image.RegistryConfig
	var err error
	var imageName string
	if govalue.NotNil(conf.DeckhouseConfig) {
		log.InfoF("[DVP-DEBUG] downloadImage: DeckhouseConfig branch registryDockerCfgLen=%d imagesRepo=%q scheme=%q\n",
			len(conf.DeckhouseConfig.RegistryDockerCfg), conf.DeckhouseConfig.ImagesRepo, conf.DeckhouseConfig.RegistryScheme)
		dc, decodeErr := image.DecodeDockerConfig(conf.DeckhouseConfig.RegistryDockerCfg)
		if decodeErr != nil {
			log.InfoF("[DVP-DEBUG] downloadImage: DecodeDockerConfig err: %v\n", decodeErr)
			return decodeErr
		}
		scheme := "HTTPS"
		if strings.ToUpper(conf.DeckhouseConfig.RegistryScheme) == "HTTP" || strings.ToUpper(conf.DeckhouseConfig.RegistryScheme) == "HTTPS" {
			scheme = strings.ToUpper(conf.DeckhouseConfig.RegistryScheme)
		}
		regConfig, err = image.RegistryConfigFromDockerConfig(dc, scheme, conf.DeckhouseConfig.ImagesRepo)
		if err != nil {
			log.InfoF("[DVP-DEBUG] downloadImage: RegistryConfigFromDockerConfig err: %v\n", err)
		}
		imageName = conf.DeckhouseConfig.ImagesRepo + "@"
	} else {
		regConfig, err = image.NewRegistryConfig(string(conf.Registry.Settings.RemoteData.Scheme), conf.Registry.Settings.RemoteData.ImagesRepo, conf.Registry.Settings.RemoteData.Username, conf.Registry.Settings.RemoteData.Password, conf.Registry.Settings.RemoteData.CA)
		imageName = conf.Registry.Settings.RemoteData.ImagesRepo + "@"
	}

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

	log.InfoF("[DVP-DEBUG] downloadImage: pulling imageName=%q into rootDir=%q cacheDir=%q\n", imageName, conf.DownloadRootDir, conf.DownloadCacheDir)
	return image.DownloadAndUnpackImage(ctx, imageName, conf.DownloadRootDir, conf.DownloadCacheDir, *regConfig, showProgress)
}
