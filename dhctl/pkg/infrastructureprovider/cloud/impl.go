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
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/tofu"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/dvp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fsstatic"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	dhctlfs "github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

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
	id := fmt.Sprintf("%s/%s/%s/%s", p.prefix, p.uuid, p.name, p.layout)
	hash := stringsutil.Sha256Encode(id)

	first16Runes := fmt.Sprintf("%.16s", hash)

	p.rootDir = filepath.Join(tmpDir, "infra", first16Runes)
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

func (p *Provider) String() string {
	return fmt.Sprintf("provider %s for cluster %s/%s with layout %s", p.name, p.uuid, p.prefix, p.layout)
}

func (p *Provider) OutputExecutor(ctx context.Context, logger log.Logger) (infrastructure.OutputExecutor, error) {
	const errPrefix = "Failed init output executor"
	err := p.makeRootDir(errPrefix)
	if err != nil {
		return nil, err
	}

	infraUtilDestination, err := p.downloadInfraUtil(ctx, p.rootDir, errPrefix)
	if err != nil {
		return nil, err
	}

	if !p.params.Settings.UseOpenTofu() {
		p.logger.LogDebugF("Create terraform output executor %s\n", p.String())
		return terraform.NewOutputExecutor(terraform.OutputExecutorParams{
			RunExecutorParams: terraform.RunExecutorParams{
				RootDir:          p.rootDir,
				TerraformBinPath: infraUtilDestination,
			},
		}, logger), nil
	}

	p.logger.LogDebugF("Create opentofu output executor %s\n", p.String())

	return tofu.NewOutputExecutor(tofu.OutputExecutorParams{
		RunExecutorParams: tofu.RunExecutorParams{
			RootDir:     p.rootDir,
			TofuBinPath: infraUtilDestination,
		},
	}, logger), nil
}

func (p *Provider) Executor(ctx context.Context, step infrastructure.Step, logger log.Logger) (infrastructure.Executor, error) {
	const errPrefix = "Failed init executor"

	if err := p.makeRootDir(errPrefix); err != nil {
		return nil, err
	}

	p.logger.LogDebugF("Getting version content\n", p.String())

	vContentProvider := getVersionContentProvider(p.params.Settings, p.name, p.logger)
	versionContent, version, err := vContentProvider(ctx, p.params.Settings, p.metaConfig, p.logger)
	if err != nil {
		return nil, fmt.Errorf("Cannot get version content for %s: %w", p.String(), err)
	}

	infraRootDir := filepath.Join(p.rootDir, version)

	p.logger.LogDebugF(
		"Got version %s for %s with content:\n%s\n Infra root dir will be %s\n",
		version,
		p.String(),
		versionContent,
		infraRootDir,
	)

	err = p.makeDir(infraRootDir, errPrefix)
	if err != nil {
		return nil, err
	}

	infraUtilDestination, err := p.downloadInfraUtil(ctx, infraRootDir, errPrefix)
	if err != nil {
		return nil, err
	}

	pluginsDir, err := p.downloadPluginVersion(ctx, infraRootDir, version)
	if err != nil {
		return nil, err
	}

	modulesDir, err := p.downloadModules(ctx, infraRootDir)
	if err != nil {
		return nil, err
	}

	stepStr := string(step)
	stepDir := filepath.Join(modulesDir, fsstatic.LayoutsDir, p.layout, stepStr)

	err = p.fillVersionsToModulesAndLayoutStep(versionContent, infraRootDir, stepDir, modulesDir)
	if err != nil {
		return nil, err
	}

	if !p.params.Settings.UseOpenTofu() {
		p.logger.LogDebugF("Create terraform executor for %s with step %s\n", p.String(), step)
		return terraform.NewExecutor(terraform.ExecutorParams{
			WorkingDir: stepDir,
			PluginsDir: pluginsDir,
			RunExecutorParams: terraform.RunExecutorParams{
				RootDir:          infraRootDir,
				TerraformBinPath: infraUtilDestination,
			},
			Step:           step,
			VmChangeTester: p.IsVMChange,
		}, logger), nil
	}

	p.logger.LogDebugF("Create opentofu executor for %s with step %s\n", p.String(), step)

	return tofu.NewExecutor(tofu.ExecutorParams{
		WorkingDir: stepDir,
		PluginsDir: pluginsDir,
		RunExecutorParams: tofu.RunExecutorParams{
			RootDir:     infraRootDir,
			TofuBinPath: infraUtilDestination,
		},
		Step:           step,
		VMChangeTester: p.IsVMChange,
	}, logger), nil
}

func doNotCheckSourceLink(string) error {
	return nil
}

func (p *Provider) createLinkToRootVersionsFileInModule(dir, rootVersionFile string) error {
	p.logger.LogDebugF("Create link to root versions file %s for module %s for %s\n", rootVersionFile, dir, p.String())

	versionsFile := fsstatic.GetVersionsFile(dir)

	return fsstatic.CreateLinkIfNotExists(rootVersionFile, doNotCheckSourceLink, versionsFile, p.logger)
}

func (p *Provider) needNewRootVersionsContentWrite(versionsRootFile, versionsSum string) (bool, error) {
	rootVersionsContent, err := os.ReadFile(versionsRootFile)
	if err == nil {
		rootVersionsContentSum := stringsutil.Sha256EncodeBytes(rootVersionsContent)
		p.logger.LogDebugF(`Got root version content for %s:
%s
SHA sum is %s
Root versions file %s
Versions content SHA sum is %s
`,
			p.String(),
			rootVersionsContent,
			rootVersionsContentSum,
			versionsRootFile,
			versionsSum,
		)

		return rootVersionsContentSum != versionsSum, nil
	}

	if os.IsNotExist(err) {
		p.logger.LogDebugF("Root versions file %s for %s not found. Should write\n", versionsRootFile, p.String())
		return true, nil
	}

	return false, fmt.Errorf("Cannot get root versions file %s for %s: %w", versionsRootFile, p.String(), err)
}

func (p *Provider) fillVersionsToModulesAndLayoutStep(versionContent []byte, infraRoot, stepDir, modulesDir string) error {
	versionsSum := stringsutil.Sha256EncodeBytes(versionContent)

	versionsRootFile := fsstatic.GetVersionsFile(infraRoot)

	p.logger.LogDebugF(`Got version content for %s:
%s
SHA sum is %s
Root versions file %s
`,
		p.String(),
		versionContent,
		versionsSum,
		versionsRootFile,
	)

	rewriteRootVersionsFile, err := p.needNewRootVersionsContentWrite(versionsRootFile, versionsSum)
	if err != nil {
		return err
	}

	if rewriteRootVersionsFile {
		p.logger.LogDebugF("Root versions file %s for %s needs to rewrite\n", versionsRootFile, p.String())

		err = os.WriteFile(versionsRootFile, versionContent, 0644)
		if err != nil {
			return fmt.Errorf("Cannot write root versions %s file for %s: %w", versionsRootFile, p.String(), err)
		}
	} else {
		p.logger.LogDebugF("Root versions file %s for %s does not need to rewrite\n", versionsRootFile, p.String())
	}

	if err := p.createLinkToRootVersionsFileInModule(stepDir, versionsRootFile); err != nil {
		return err
	}

	if !dhctlfs.IsDirExists(modulesDir) {
		p.logger.LogDebugF("Modules dir %s for %s does not exist. Skip create links to root version file\n", modulesDir, p.String())
		return nil
	}

	return filepath.WalkDir(modulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == modulesDir {
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, fmt.Sprintf("/%s", fsstatic.LayoutsDir)) {
			return nil
		}

		return p.createLinkToRootVersionsFileInModule(path, versionsRootFile)
	})
}

func (p *Provider) makeDir(dir, errPrefix string) error {
	err := os.MkdirAll(dir, 0777)
	if err == nil {
		return nil
	}

	if os.IsExist(err) {
		p.logger.LogDebugF("Directory %s already exists for %s, skipping creation", dir, p.String())
		return nil
	}

	return fmt.Errorf("%s. Failed to make dir %s for %s: %w", errPrefix, dir, p.String(), err)
}

func (p *Provider) makeRootDir(errPrefix string) error {
	return p.makeDir(p.rootDir, errPrefix)
}

func (p *Provider) downloadModules(ctx context.Context, rootDir string) (string, error) {
	destination := filepath.Join(rootDir, "modules")

	p.logger.LogDebugF("Create modules destination %s for %s\n", destination, p.String())

	err := os.MkdirAll(destination, 0777)
	if err != nil {
		return "", fmt.Errorf("Cannot create destination modules dir %s for %s: %w", destination, p.String(), err)
	}

	p.logger.LogDebugF("Download modules config %s for %s\n", destination, p.String())

	err = p.di.ModulesProvider.DownloadModules(ctx, DownloadModulesParams{
		ModulesParams{
			Settings: p.params.Settings,
		},
	}, destination)
	if err != nil {
		return "", fmt.Errorf("Cannot download modules for %s: %w", p.String(), err)
	}

	return destination, nil
}

func (p *Provider) downloadPluginVersion(ctx context.Context, rootDir, version string) (string, error) {
	pluginsDir := filepath.Join(rootDir, "plugins")

	arch := p.arch()

	destination := fsstatic.GetPluginDir(pluginsDir, p.params.Settings, version, arch)
	destinationDir := path.Dir(destination)
	destinationDir = strings.TrimRight(destinationDir, "/")
	// for windows
	destinationDir = strings.TrimRight(destinationDir, "\\")

	p.logger.LogDebugF("Create plugins dir destination %s for %s version %s\n", destinationDir, p.String(), version)

	err := os.MkdirAll(destinationDir, 0755)
	if err != nil {
		return "", fmt.Errorf("Cannot create plugins destination dir %s for %s: %w", destinationDir, p.String(), err)
	}
	params := InfrastructurePluginProviderParams{
		Version: Version{
			Version: version,
			Arch:    arch,
		},
		Settings: p.params.Settings,
	}

	p.logger.LogDebugF(
		"Download cloud %s plugin %s version %s to destination %s for %s\n",
		p.name,
		params.Version.String(),
		version,
		destinationDir,
		p.String(),
	)

	err = p.di.InfraPluginProvider.DownloadPlugin(ctx, params, destination)
	if err != nil {
		return "", fmt.Errorf("Cannot download plugin version %s to %s for %s: %w", version, destination, p.String(), err)
	}

	return pluginsDir, nil
}

func (p *Provider) downloadInfraUtil(ctx context.Context, rootDir, errPrefix string) (string, error) {
	destination := fsstatic.GetInfraUtilPath(rootDir, p.params.Settings)

	params := InfrastructureUtilProviderParams{
		Version{
			Version: p.params.Settings.InfrastructureVersion(),
			Arch:    p.arch(),
		},
	}

	var err error

	if p.params.Settings.UseOpenTofu() {
		p.logger.LogDebugF("Downloading opentofu %s for %s\n", params.Version.String(), p.String())
		err = p.di.InfraUtilProvider.DownloadOpenTofu(ctx, params, destination)
	} else {
		p.logger.LogDebugF("Downloading terraform %s for %s\n", params.Version.String(), p.String())
		err = p.di.InfraUtilProvider.DownloadTerraform(ctx, params, destination)
	}

	if err != nil {
		return "", fmt.Errorf("%s. Cannot download infrastructure util to %s for %s: %w", errPrefix, destination, p.String(), err)
	}

	return destination, nil

}

func (p *Provider) arch() string {
	return "linux_amd64"
}

func (p *Provider) Cleanup() error {
	return os.RemoveAll(p.rootDir)
}
