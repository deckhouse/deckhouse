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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fsproviderpath"
	fsutils "github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

var versionFile = "terraform_versions.yml"

type pluginsProvider struct {
	m sync.Mutex

	pluginsDir string
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
	_, err := os.Stat(source)
	if err == nil {
		return fsutils.CreateLinkIfNotExists(ctx, source, checkIsExecFile, destination)
	}

	cloudName := strings.ToLower(params.Settings.CloudName())
	sectionName := "cloudProvider" + strings.ToUpper(cloudName[:1]) + cloudName[1:]

	// Fast-path: if the fallback source binary is already present under DownloadRootDir
	// (e.g. preserved across `wipe-state` or pre-injected for dev iteration), skip the
	// terraform-manager image download entirely. Saves ~10-15s per bootstrap and lets
	// us iterate with a custom-patched provider binary without dhctl clobbering it.
	//
	// downloadImage unpacks the terraform-manager image into DownloadRootDir, so its
	// binary and terraform_versions.yml land in <DownloadRootDir>/terraform-manager.
	// The fast-path and the download-fallback must read from that same directory.
	terraformManagerDir := filepath.Join(conf.DownloadRootDir, "terraform-manager")
	source = filepath.Join(terraformManagerDir, params.Settings.DestinationBinary())
	if _, statErr := os.Stat(source); statErr == nil {
		return fsutils.CreateLinkIfNotExists(ctx, source, checkIsExecFile, destination)
	}

	// External provider bundle: the plugin ships inside the OCI bundle that
	// EnsureProviderBundle unpacked under <DownloadRootDir>/<provider>/. Using
	// it avoids the lazy terraform-manager pull entirely, so converge does not
	// need registry credentials on the MetaConfig at all.
	bundleTerraformManagerDir := filepath.Join(conf.DownloadRootDir, cloudName, "terraform-manager")
	source = filepath.Join(bundleTerraformManagerDir, params.Settings.DestinationBinary())
	if _, statErr := os.Stat(source); statErr == nil {
		return fsutils.CreateLinkIfNotExists(ctx, source, checkIsExecFile, destination)
	}

	if err = downloadImage(ctx, conf, "terraformManager", sectionName, conf.ShowProgress); err != nil {
		return err
	}

	source = filepath.Join(terraformManagerDir, params.Settings.DestinationBinary())

	return fsutils.CreateLinkIfNotExists(ctx, source, checkIsExecFile, destination)
}
