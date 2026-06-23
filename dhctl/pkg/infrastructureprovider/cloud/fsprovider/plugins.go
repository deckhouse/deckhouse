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
	"errors"
	"fmt"
	"io"
	"io/fs"
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
		log.InfoF("[DVP-DEBUG] DownloadPlugin: fast-path-1 pluginsDir hit source=%q\n", source)
		return fsutils.CreateLinkIfNotExists(source, checkIsExecFile, destination, p.logger)
	}
	log.InfoF("[DVP-DEBUG] DownloadPlugin: pluginsDir miss source=%q downloadRootDir=%q\n", source, conf.DownloadRootDir)

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
		log.InfoF("[DVP-DEBUG] DownloadPlugin: fast-path-2 terraform-manager binary hit source=%q\n", source)
		if err := copyTFVersionFile(conf.DownloadRootDir, terraformManagerDir); err != nil {
			return fmt.Errorf("could not copy terraform_versions.yml: %w", err)
		}
		return fsutils.CreateLinkIfNotExists(source, checkIsExecFile, destination, p.logger)
	}
	log.InfoF("[DVP-DEBUG] DownloadPlugin: slow-path downloadImage terraformManager section=%q (binary miss source=%q)\n", sectionName, source)

	if err = downloadImage(ctx, conf, "terraformManager", sectionName, conf.ShowProgress); err != nil {
		log.InfoF("[DVP-DEBUG] DownloadPlugin: downloadImage err: %v\n", err)
		return err
	}
	if err = copyTFVersionFile(conf.DownloadRootDir, terraformManagerDir); err != nil {
		return fmt.Errorf("could not copy terraform_versions.yml: %w", err)
	}

	source = filepath.Join(terraformManagerDir, params.Settings.DestinationBinary())

	return fsutils.CreateLinkIfNotExists(source, checkIsExecFile, destination, p.logger)
}

func copyTFVersionFile(downloadRootDir, terraformManagerDir string) error {
	downloadCandiPath := filepath.Join(downloadRootDir, "deckhouse", "candi")
	if err := os.MkdirAll(downloadCandiPath, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", downloadCandiPath, err)
	}

	// terraform_versions.yml is required. plan_rules.yml is optional (only
	// providers with a vmResource rule ship it) and must land next to the
	// versions file so loadPlanRules picks the rule up adjacent to it.
	if err := copyCandiFile(terraformManagerDir, downloadCandiPath, versionFile, true); err != nil {
		return err
	}
	return copyCandiFile(terraformManagerDir, downloadCandiPath, planRulesFilename, false)
}

func copyCandiFile(srcDir, dstDir, name string, required bool) error {
	src := filepath.Join(srcDir, name)
	f, err := os.Open(src)
	if err != nil {
		if !required && errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer f.Close()

	dstPath := filepath.Join(dstDir, name)
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
