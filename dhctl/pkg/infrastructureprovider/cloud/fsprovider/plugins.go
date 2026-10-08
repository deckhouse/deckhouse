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
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fsproviderpath"
	fsutils "github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
)

var versionFile = "terraform_versions.yml"

type pluginsProvider struct {
	m sync.Mutex

	pluginsDir string
	// download replaces the registry pull in tests.
	download image.DownloadFunc
}

func newPluginsProvider(pluginsDir string) *pluginsProvider {
	return &pluginsProvider{
		pluginsDir: pluginsDir,
	}
}

func (p *pluginsProvider) DownloadPlugin(ctx context.Context, params cloud.InfrastructurePluginProviderParams, destination string, conf *config.MetaConfig) error {
	p.m.Lock()
	defer p.m.Unlock()

	source := fsproviderpath.GetPluginDir(p.pluginsDir, params.Settings, params.Version.Version, params.Version.Arch)
	if _, err := os.Stat(source); err == nil {
		return fsutils.CreateLinkIfNotExists(ctx, source, checkIsExecFile, destination)
	}

	cloudName := strings.ToLower(params.Settings.CloudName())
	binary := params.Settings.DestinationBinary()

	// An external or pinned provider's plugin ships inside its bundle, which the configuration parse
	// unpacked. That bundle is the build to run, so the installer's own image is no substitute.
	if conf.ProviderBundleDir != "" {
		source = filepath.Join(conf.ProviderBundleDir, terraformManagerDir, binary)
		if _, err := os.Stat(source); err != nil {
			return fmt.Errorf("find infrastructure plugin in provider bundle %s: %w", conf.ProviderBundleDir, err)
		}
		return fsutils.CreateLinkIfNotExists(ctx, source, checkIsExecFile, destination)
	}

	// No bundle means in-tree: its terraform-manager image is named by this installer's digests.
	dir, err := ensureInstallerImage(ctx, conf, cloudName, digests.ProviderImagesSection(cloudName), digests.TerraformManagerImage, p.download)
	if err != nil {
		return err
	}

	return fsutils.CreateLinkIfNotExists(ctx, filepath.Join(dir, terraformManagerDir, binary), checkIsExecFile, destination)
}
