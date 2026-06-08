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
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fsproviderpath"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	fsutils "github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

var versionFile = "terraform_versions.yml"

type pluginsProvider struct {
	m sync.Mutex

	logger     log.Logger
	pluginsDir string
}

func newPluginsProvider(logger log.Logger, pluginsDir string) *pluginsProvider {
	return &pluginsProvider{
		logger:     logger,
		pluginsDir: pluginsDir,
	}
}

func (p *pluginsProvider) DownloadPlugin(ctx context.Context, params cloud.InfrastructurePluginProviderParams, destination string, conf *config.MetaConfig) error {
	p.m.Lock()
	defer p.m.Unlock()

	source := fsproviderpath.GetPluginDir(p.pluginsDir, params.Settings, params.Version.Version, params.Version.Arch)
	_, err := os.Stat(source)
	if err == nil {
		return fsutils.CreateLinkIfNotExists(source, checkIsExecFile, destination, p.logger)
	}

	cloudName := strings.ToLower(params.Settings.CloudName())
	sectionName := "cloudProvider" + strings.ToUpper(cloudName[:1]) + cloudName[1:]

	// Fast-path: if the fallback source binary is already present under DownloadRootDir
	// (e.g. preserved across `wipe-state` or pre-injected for dev iteration), skip the
	// terraform-manager image download entirely. Saves ~10-15s per bootstrap and lets
	// us iterate with a custom-patched provider binary without dhctl clobbering it.
	terraformManagerDir := filepath.Join(conf.DownloadRootDir, cloudName, "terraform-manager")
	source = filepath.Join(terraformManagerDir, params.Settings.DestinationBinary())
	if _, statErr := os.Stat(source); statErr == nil {
		if err := copyTFVersionFile(conf.DownloadRootDir, terraformManagerDir); err != nil {
			return fmt.Errorf("could not copy terraform_versions.yml: %w", err)
		}
		return fsutils.CreateLinkIfNotExists(source, checkIsExecFile, destination, p.logger)
	}

	if err = downloadImage(ctx, conf, "terraformManager", sectionName, conf.ShowProgress); err != nil {
		return err
	}
	if err = copyTFVersionFile(conf.DownloadRootDir, terraformManagerDir); err != nil {
		return fmt.Errorf("could not copy terraform_versions.yml: %w", err)
	}

	providerPath := filepath.Join(conf.DownloadRootDir, cloudName, "terraform-manager", params.Settings.DestinationBinary())
	if _, err := os.Stat(providerPath); err == nil {
		source = providerPath
	} else {
		source = filepath.Join(terraformManagerDir, params.Settings.DestinationBinary())
	}

	return fsutils.CreateLinkIfNotExists(source, checkIsExecFile, destination, p.logger)
}

func copyTFVersionFile(downloadRootDir, terraformManagerDir string) error {
	downloadCandiPath := filepath.Join(downloadRootDir, "deckhouse", "candi")
	src := filepath.Join(terraformManagerDir, versionFile)
	dstPath := filepath.Join(downloadCandiPath, versionFile)

	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer f.Close()

	if err := os.MkdirAll(downloadCandiPath, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", downloadCandiPath, err)
	}
	if err := os.RemoveAll(dstPath); err != nil {
		return fmt.Errorf("remove %s: %w", dstPath, err)
	}

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", dstPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, f); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dstPath, err)
	}
	if err := dst.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", dstPath, err)
	}
	return nil
}
