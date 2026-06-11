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
	"os"
	"path/filepath"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fsprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	fsutils "github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

type CloudProviderGetterParams struct {
	// TmpDir is the working directory used by the cloud provider for transient
	// state (terraform/tofu working tree, infra plan files). When empty, falls
	// back to options.DefaultTmpDir().
	TmpDir string
	// DownloadDir locates the on-disk Deckhouse install tree (cloud-providers,
	// plugins, infrastructure_versions.json). When empty, falls back to
	// options.DefaultTmpDir() — same behavior as the previous global default.
	AdditionalParams cloud.ProviderAdditionalParams
	Logger           log.Logger
	FSDIParams       *fsprovider.DIParams
	IsDebug          bool

	VersionProviderGetter cloud.VersionsContentProviderGetter
	ProvidersCache        CloudProvidersCache

	GlobalOptions *options.GlobalOptions
}

func (p *CloudProviderGetterParams) getProvidersCache() (CloudProvidersCache, error) {
	logger, err := p.getLogger()
	if err != nil {
		return nil, err
	}

	providersCache := p.ProvidersCache
	providersCacheLogMessage := "Provider cache is not nil. Using custom\n"
	if govalue.IsNil(providersCache) {
		providersCacheLogMessage = "Provider cache is nil. Using default\n"
		providersCache = defaultProvidersCache
	}

	logger.LogDebugF(providersCacheLogMessage)

	return providersCache, nil
}

func (p *CloudProviderGetterParams) getFSDIParams() (*fsprovider.DIParams, error) {
	logger, err := p.getLogger()
	if err != nil {
		return nil, err
	}

	if p.FSDIParams != nil {
		logger.LogDebugF("Using custom FSDIParams: %+v\n", p.FSDIParams)
		return p.FSDIParams, nil
	}

	infraVersionsFile, err := fsutils.DoAbsolutePath(p.GlobalOptions.InfrastructureVersions, false)
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare infra versions file: %w", err)
	}

	dhctlPath, err := fsutils.DoAbsolutePath(p.GlobalOptions.DhctlPath, true)
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare dhctl path: %w", err)
	}

	diDefaultParams := &fsprovider.DIParams{
		InfraVersionsFile: infraVersionsFile,
		BinariesDir:       filepath.Join(dhctlPath, "bin"),
		CloudProviderDir:  filepath.Join(p.GlobalOptions.CandiDir, "cloud-providers"),
		PluginsDir:        filepath.Join(dhctlPath, "plugins"),
		DownloadDir:       p.GlobalOptions.DownloadDir,
	}

	if _, err := os.Stat(diDefaultParams.BinariesDir); err != nil {
		// fallback to /bin
		diDefaultParams.BinariesDir = "/bin"
	}

	if _, err = os.Stat(diDefaultParams.PluginsDir); err != nil {
		// fallback to /tmp
		diDefaultParams.PluginsDir = filepath.Join(p.GlobalOptions.DownloadDir, "plugins")
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
		logger.LogDebugF("fs.GetDI provided our own VersionProviderGetter\n")
		return nil
	}

	versionProviderGetter := cloud.DefaultVersionContentProvider
	logMessage := "Using default VersionProviderGetter\n"

	if p.VersionProviderGetter != nil {
		logMessage = "Using custom VersionProviderGetter\n"
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
	logMsg := "Using passed tmp dir."
	if tmpDir == "" {
		tmpDir = options.DefaultTmpDir()
		logMsg = "CloudProviderGetterParams tmp dir is empty. Using default."
	}

	preparedTmpDir, err := fsutils.DoAbsolutePath(tmpDir, true)
	if err != nil {
		return "", fmt.Errorf("Cannot prepare tmp dir %s: %w", tmpDir, err)
	}

	logger.LogDebugF("%s Before preparation: '%s', absolute path: '%s'\n", logMsg, tmpDir, preparedTmpDir)

	return preparedTmpDir, nil
}

func (p *CloudProviderGetterParams) getLogger() (log.Logger, error) {
	if govalue.IsNil(p.Logger) {
		return nil, fmt.Errorf("CloudProviderGetterParams must have a non-nil pointer logger")
	}

	return p.Logger, nil
}

func (p *CloudProviderGetterParams) getAdditionalParams() cloud.ProviderAdditionalParams {
	return p.AdditionalParams
}
