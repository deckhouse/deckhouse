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

package cloud

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/tofu"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/dvp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fsstatic"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/version"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

var versionContentProviders = map[string]versionContentProvider{
	vcd.ProviderName: vcd.VersionContentProvider,
}

var contentProviderMutex sync.Mutex

func getVersionContentProvider(s settings.ProviderSettings, provider string) versionContentProvider {
	contentProviderMutex.Lock()
	defer contentProviderMutex.Unlock()

	choicer, ok := versionContentProviders[provider]
	if ok {
		return choicer
	}

	return func(settings settings.ProviderSettings, metaConfig *config.MetaConfig) (string, error) {
		if len(settings.Versions()) != 1 {
			return "", fmt.Errorf("no one version found for provider %s", provider)
		}

		return version.GetVersionContent(s, settings.Versions()[0]), nil
	}
}

type ProviderDI struct {
	SettingsProvider    SettingsProvider
	InfraUtilProvider   InfrastructureUtilProvider
	InfraPluginProvider InfrastructurePluginProvider
	ModulesProvider     ProviderModulesProvider
}

type ProviderParams struct {
	AdditionalParams ProviderAdditionalParams
	Settings         settings.ProviderSettings
}

type Provider struct {
	prefix string
	layout string
	name   string
	uuid   string

	rootDir string

	params     ProviderParams
	metaConfig *config.MetaConfig

	di     *ProviderDI
	logger log.Logger
}

func NewProvider(metaConfig *config.MetaConfig, uuid string, di *ProviderDI, params ProviderParams, tmpDir string, logger log.Logger) *Provider {
	p := &Provider{
		prefix:     metaConfig.ClusterPrefix,
		layout:     metaConfig.Layout,
		name:       metaConfig.ProviderName,
		uuid:       uuid,
		di:         di,
		params:     params,
		metaConfig: metaConfig,
		logger:     logger,
	}

	p.generateRootDir(tmpDir)
	return p
}

func (p *Provider) generateRootDir(tmpDir string) {
	id := fmt.Sprintf("%s/%s/%s", p.name, p.prefix, p.uuid)
	hash := stringsutil.Sha256Encode(id)

	p.rootDir = filepath.Join(tmpDir, "infra", hash)
}

func (p *Provider) Name() string {
	return p.name
}

func (p *Provider) RootDir() string {
	return p.rootDir
}

func (p *Provider) NeedToUseTofu() bool {
	return p.params.Settings.UseOpenTofu()
}

func (p *Provider) IsVMChange(rc plan.ResourceChange) bool {
	if p.name == dvp.ProviderName {
		return dvp.IsVMManifest(rc, p.logger)
	}

	return rc.Type == p.params.Settings.VmResourceType()
}

func (p *Provider) OutputExecutor(ctx context.Context, logger log.Logger) (infrastructure.OutputExecutor, error) {
	infraUtilDestination, err := p.makeRootDirAndDownloadInfraUtil(ctx, "Failed init output executor")
	if err != nil {
		return nil, err
	}

	if !p.params.Settings.UseOpenTofu() {
		p.logger.LogDebugF("Create terraform output executor for provider %s\n", p.name)
		return terraform.NewOutputExecutor(terraform.OutputExecutorParams{
			RunExecutorParams: terraform.RunExecutorParams{
				RootDir:          p.rootDir,
				TerraformBinPath: infraUtilDestination,
			},
		}, logger), nil
	}

	p.logger.LogDebugF("Create opentofu output executor for provider %s with step %s\n", p.name)

	return tofu.NewOutputExecutor(tofu.OutputExecutorParams{
		RunExecutorParams: tofu.RunExecutorParams{
			RootDir:     p.rootDir,
			TofuBinPath: infraUtilDestination,
		},
	}, logger), nil
}

func (p *Provider) Executor(ctx context.Context, step infrastructure.Step, logger log.Logger) (infrastructure.Executor, error) {
	infraUtilDestination, err := p.makeRootDirAndDownloadInfraUtil(ctx, "Failed init executor")
	if err != nil {
		return nil, err
	}

	pluginsDir, err := p.downloadAllPluginsVersions(ctx)
	if err != nil {
		return nil, err
	}

	modulesDir, err := p.downloadModules(ctx)
	if err != nil {
		return nil, err
	}

	stepStr := string(step)
	stepDir := filepath.Join(modulesDir, "layouts", p.layout, stepStr)

	p.logger.LogDebugF("Got step dir %s provider %s. Getting version content\n", stepDir, p.name)

	vContentProvider := getVersionContentProvider(p.params.Settings, p.name)
	versionContent, err := vContentProvider(p.params.Settings, p.metaConfig)
	if err != nil {
		return nil, fmt.Errorf("Cannot get version content for provider %s: %w", p.name, err)
	}

	versionsFile := filepath.Join(stepDir, "versions.tf")

	p.logger.LogDebugF("Got version content for provider %s:\n%s\nWrite to destination %s\n", p.name, versionContent, versionsFile)

	err = os.WriteFile(versionsFile, []byte(versionContent), 0644)
	if err != nil {
		return nil, fmt.Errorf("Cannot write versions %s file for layout %s with step %s: %v", versionsFile, p.layout, stepStr, err)
	}

	if !p.params.Settings.UseOpenTofu() {
		p.logger.LogDebugF("Create terraform executor for provider %s for layout %s with step %s\n", p.name, p.layout, step)
		return terraform.NewExecutor(terraform.ExecutorParams{
			WorkingDir: stepDir,
			PluginsDir: pluginsDir,
			RunExecutorParams: terraform.RunExecutorParams{
				RootDir:          p.rootDir,
				TerraformBinPath: infraUtilDestination,
			},
			Step:           step,
			VmChangeTester: p.IsVMChange,
		}, logger), nil
	}

	p.logger.LogDebugF("Create opentofu executor for provider %s for layout %s with step %s\n", p.name, p.layout, step)

	return tofu.NewExecutor(tofu.ExecutorParams{
		WorkingDir: stepDir,
		PluginsDir: pluginsDir,
		RunExecutorParams: tofu.RunExecutorParams{
			RootDir:     p.rootDir,
			TofuBinPath: infraUtilDestination,
		},
		Step:           step,
		VMChangeTester: p.IsVMChange,
	}, logger), nil
}

func (p *Provider) makeRootDirAndDownloadInfraUtil(ctx context.Context, errorPref string) (string, error) {
	if err := p.makeRootDir(); err != nil {
		return "", fmt.Errorf("%s. %w", errorPref, err)
	}

	infraUtilDestination, err := p.downloadInfraUtil(ctx)
	if err != nil {
		return "", fmt.Errorf("%s. %w", errorPref, err)
	}

	return infraUtilDestination, nil
}

func (p *Provider) makeRootDir() error {
	err := os.MkdirAll(p.rootDir, 0755)
	if err == nil {
		return nil
	}

	if os.IsExist(err) {
		p.logger.LogDebugF("Directory %s already exists for provider %s, skipping creation", p.rootDir, p.name)
		return nil
	}

	return fmt.Errorf("Failed to make root dir %s for provider %s: %w", p.rootDir, p.name, err)
}

func (p *Provider) downloadModules(ctx context.Context) (string, error) {
	destination := filepath.Join(p.rootDir, "modules")

	p.logger.LogDebugF("Create modules destination %s for cloud %s\n", destination, p.name)

	err := os.MkdirAll(destination, 0755)
	if err != nil {
		return "", fmt.Errorf("Cannot create destination modules dir %s: %w", destination, err)
	}

	p.logger.LogDebugF("Download modules config %s for cloud %s\n", destination, p.name)

	err = p.di.ModulesProvider.DownloadModules(ctx, DownloadModulesParams{}, destination)
	if err != nil {
		return "", fmt.Errorf("Cannot download modules for provider %s: %w", p.name, err)
	}

	return destination, nil
}

func (p *Provider) downloadAllPluginsVersions(ctx context.Context) (string, error) {
	pluginsDir := filepath.Join(p.rootDir, "plugins")

	arch := p.arch()

	for _, v := range p.params.Settings.Versions() {
		destination := fsstatic.GetPluginDir(pluginsDir, p.params.Settings, v, arch)
		destinationDir := path.Dir(destination)

		p.logger.LogDebugF("Create plugins dir destination %s\n", destinationDir)

		err := os.MkdirAll(destinationDir, 0755)
		if err != nil {
			return "", fmt.Errorf("Cannot create plugins destination dir %s: %w", destinationDir, err)
		}
		params := InfrastructurePluginProviderParams{
			Version: Version{
				Version: v,
				Arch:    arch,
			},
			Settings: p.params.Settings,
		}

		p.logger.LogDebugF(
			"Download cloud %s plugin %s to destination %s\n",
			p.name,
			params.Version.String(),
			destinationDir,
		)

		err = p.di.InfraPluginProvider.DownloadPlugin(ctx, params, destination)
		if err != nil {
			return "", fmt.Errorf("Cannot download plugin to %s: %w", destination, err)
		}
	}

	return pluginsDir, nil
}

func (p *Provider) downloadInfraUtil(ctx context.Context) (string, error) {
	destination := fsstatic.GetInfraUtilPath(p.rootDir, p.params.Settings)

	params := InfrastructureUtilProviderParams{
		Version{
			Version: p.params.Settings.InfrastructureVersion(),
			Arch:    p.arch(),
		},
	}

	var err error

	if p.params.Settings.UseOpenTofu() {
		p.logger.LogDebugF("Downloading opentofu %s for provider %s\n", params.Version.String(), p.name)
		err = p.di.InfraUtilProvider.DownloadOpenTofu(ctx, params, destination)
	} else {
		p.logger.LogDebugF("Downloading terraform %s for provider %s\n", params.Version.String(), p.name)
		err = p.di.InfraUtilProvider.DownloadTerraform(ctx, params, destination)
	}

	if err != nil {
		return "", fmt.Errorf("Cannot download infrastructure util to %s: %w", destination, err)
	}

	return destination, nil

}

func (p *Provider) arch() string {
	return "linux_amd64"
}

func (p *Provider) Cleanup() error {
	return os.RemoveAll(p.rootDir)
}
