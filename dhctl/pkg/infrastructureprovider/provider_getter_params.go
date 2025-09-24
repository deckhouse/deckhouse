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

package infrastructureprovider

import (
	"fmt"
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	global "github.com/deckhouse/deckhouse/dhctl/pkg/global/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fsstatic"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/value"
)

type CloudProviderGetterParams struct {
	TmpDir           string
	AdditionalParams cloud.ProviderAdditionalParams
	Logger           log.Logger
	FSDIParams       *fs.DIParams
	IsDebug          bool

	VersionProviderGetter cloud.VersionsContentProviderGetter
	ProvidersCache        CloudProvidersCache
}

func (p *CloudProviderGetterParams) getProvidersCache() (CloudProvidersCache, error) {
	logger, err := p.getLogger()
	if err != nil {
		return nil, err
	}

	providersCache := p.ProvidersCache
	providersCacheLogMessage := "Provider cache is not nil. Using custom\n"
	if value.IsNil(providersCache) {
		providersCacheLogMessage = "Provider cache is nil. Using default\n"
		providersCache = defaultProvidersCache
	}

	logger.LogDebugF(providersCacheLogMessage)

	return providersCache, nil
}

func (p *CloudProviderGetterParams) gtFSDIParams() (*fs.DIParams, error) {
	logger, err := p.getLogger()
	if err != nil {
		return nil, err
	}

	if p.FSDIParams != nil {
		logger.LogDebugF("Using custom FSDIParams: %+v\n", p.FSDIParams)
		return p.FSDIParams, nil
	}

	infraVersionsFile, err := fsstatic.PreparePath(global.GetInfrastructureVersions(), false)
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare infra versions file: %w", err)
	}

	dhctlPath, err := fsstatic.PreparePath(global.GetDhctlPath(), true)
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare dhctl path: %w", err)
	}

	diDefaultParams := &fs.DIParams{
		InfraVersionsFile: infraVersionsFile,
		BinariesDir:       filepath.Join(dhctlPath, "bin"),
		CloudProviderDir:  filepath.Join(dhctlPath, "deckhouse", "candi", "cloud-providers"),
		PluginsDir:        filepath.Join(dhctlPath, "plugins"),
	}

	logger.LogDebugF("Using default FSDIParams: %+v\n", diDefaultParams)
	return diDefaultParams, nil
}

func (p *CloudProviderGetterParams) setVersionsContentProviderGetter(di *cloud.ProviderDI) error {
	logger, err := p.getLogger()
	if err != nil {
		return err
	}

	if di.VersionsContentProviderGetter != nil {
		logger.LogDebugF("fs.GetDI provider our own VersionProviderGetter\n")
		return nil
	}

	versionProviderGetter := cloud.DefaultVersionContentProvider
	logMessage := "Use default VersionProviderGetter\n"

	if p.VersionProviderGetter != nil {
		logMessage = "Use custom VersionProviderGetter\n"
		versionProviderGetter = p.VersionProviderGetter
	}

	logger.LogDebugF(logMessage)
	di.VersionsContentProviderGetter = versionProviderGetter

	return nil
}

func (p *CloudProviderGetterParams) getTmpDir() (string, error) {
	logger, err := p.getLogger()
	if err != nil {
		return "", err
	}

	tmpDir := p.TmpDir
	logMsg := "Use passed tmp dir."

	if tmpDir == "" {
		tmpDir = app.TmpDirName
		logMsg = "CloudProviderGetterParams tmp dir is empty. Using global default."
	}

	preparedTmpDir, err := fsstatic.PreparePath(tmpDir, true)
	if err != nil {
		return "", fmt.Errorf("Cannot prepare tmp dir %s: %w", tmpDir, err)
	}

	logger.LogDebugF("%s Bofore prepare '%s' Absolute path '%s'\n", logMsg, tmpDir, preparedTmpDir)

	return preparedTmpDir, nil
}

func (p *CloudProviderGetterParams) getLogger() (log.Logger, error) {
	if value.IsNil(p.Logger) {
		return nil, fmt.Errorf("CloudProviderGetterParams must have a non-nil pointer logger")
	}

	return p.Logger, nil
}

func (p *CloudProviderGetterParams) isDebug() bool {
	return p.IsDebug
}

func (p *CloudProviderGetterParams) getAdditionalParams() (cloud.ProviderAdditionalParams, error) {
	return p.AdditionalParams, nil
}
