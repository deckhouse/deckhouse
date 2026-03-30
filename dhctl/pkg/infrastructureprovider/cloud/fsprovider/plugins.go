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
	"sync"
	"unicode"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fsproviderpath"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	fsutils "github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

var (
	versionFile = "terraform_versions.yml"
)

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

	sectionName := params.Settings.CloudName()
	runes := []rune(sectionName)
	runes[0] = unicode.ToUpper(runes[0])
	sectionName = "cloudProvider" + string(runes)

	if err = downloadImage(ctx, conf, "terraformManager", sectionName); err != nil {
		return err
	}
	terraformManagerDir := filepath.Join(conf.DownloadRootDir, "terraform-manager")

	source = filepath.Join(terraformManagerDir, params.Settings.DestinationBinary())
	if err = copyTFVersionFile(conf.DownloadRootDir); err != nil {
		return fmt.Errorf("could not copy terraform_versions.yml: %w", err)
	}

	return fsutils.CreateLinkIfNotExists(source, checkIsExecFile, destination, p.logger)
}

func copyTFVersionFile(downloadRootDir string) error {
	terraformManagerDir := filepath.Join(downloadRootDir, "terraform-manager")
	downloadCandiPath := filepath.Join(downloadRootDir, "deckhouse", "candi")
	src := filepath.Join(terraformManagerDir, versionFile)

	f, err := os.OpenFile(src, os.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not read file %s: %w\n", src, err)
	}

	if err = os.RemoveAll(filepath.Join(downloadCandiPath, versionFile)); err != nil {
		return fmt.Errorf("could not delete %s: %w", versionFile, err)
	}

	dst, err := os.OpenFile(filepath.Join(downloadCandiPath, versionFile), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("could not open file %s: %w\n", filepath.Join(downloadCandiPath, versionFile), err)
	}
	_, err = io.Copy(dst, f)

	return err
}
