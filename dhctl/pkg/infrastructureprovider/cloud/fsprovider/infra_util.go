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
	"path/filepath"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	fsutils "github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
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

func (p *InfrastructureUtilProvider) DownloadTerraform(_ context.Context, _ cloud.InfrastructureUtilProviderParams, destination string) error {
	p.m.Lock()
	defer p.m.Unlock()

	return fsutils.CreateLinkIfNotExists(filepath.Join(p.binariesDir, "terraform"), checkIsExecFile, destination, p.logger)
}

func (p *InfrastructureUtilProvider) DownloadOpenTofu(_ context.Context, _ cloud.InfrastructureUtilProviderParams, destination string) error {
	p.m.Lock()
	defer p.m.Unlock()

	return fsutils.CreateLinkIfNotExists(filepath.Join(p.binariesDir, "opentofu"), checkIsExecFile, destination, p.logger)
}
