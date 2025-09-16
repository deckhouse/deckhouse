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

package fs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fsstatic"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type modulesProvider struct {
	m sync.Mutex

	logger log.Logger
}

func newModulesProvider(logger log.Logger) *modulesProvider {
	return &modulesProvider{
		logger: logger,
	}
}

// DownloadModules
// destination is dir which filled with next structures (should contain)
//
//	layouts/
//
// optional (if layouts do not use common modules)
//
//	terraform-modules/
func (p *modulesProvider) DownloadModules(_ context.Context, params cloud.DownloadModulesParams, destination string) error {
	p.m.Lock()
	defer p.m.Unlock()

	if err := p.copyDir(fsstatic.LayoutsDir, params, destination); err != nil {
		return err
	}

	return p.copyDir(fsstatic.InfraModulesDir, params, destination)
}

// DownloadSpecs
// destination is dir which filled with next structures (should contain)
//
//	cluster_configuration.yaml
//	cloud_discovery_data.yaml
func (p *modulesProvider) DownloadSpecs(ctx context.Context, _ cloud.DownloadSpecsParams, _ string) error {
	return fmt.Errorf("DownloadSpecs not implemented")
}

func (p *modulesProvider) copyDir(dir string, params cloud.DownloadModulesParams, destination string) error {
	sourceDir := path.Join(
		getFullPath("deckhouse/candi/cloud-providers"),
		strings.ToLower(params.Settings.CloudName()),
		dir,
	)

	destinationDir := path.Join(destination, dir)

	stat, err := os.Stat(sourceDir)
	if err != nil {
		if os.IsNotExist(err) && dir == fsstatic.LayoutsDir {
			p.logger.LogDebugF("Coping loud-providers modules (dir %s) from %s to %s skipped. Not found\n", dir, sourceDir, destinationDir)
			return nil
		}

		return err
	}

	if !stat.IsDir() {
		return fmt.Errorf("Coping cloud-providers modules (dir %s) from %s to %s failed is not dir", dir, sourceDir, destinationDir)
	}

	p.logger.LogDebugF("Copy cloud-providers modules (dir %s) from %s to %s\n", dir, sourceDir, destinationDir)

	err = os.CopyFS(destinationDir, os.DirFS(sourceDir))
	if errors.Is(err, fs.ErrExist) {
		p.logger.LogDebugF("Coping loud-providers modules (dir %s) from %s to %s skipped. Exists\n", dir, sourceDir, destinationDir)
		return nil
	}

	if err != nil {
		return fmt.Errorf("Coping cloud-providers modules (dir %s) from %s to %s failed: %w", dir, sourceDir, destinationDir, err)
	}

	return nil
}
