// Copyright 2026 Flant JSC
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

package pkgsync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// syncModules writes a Module for every module the cluster runs. The last pass to write a name
// wins, so the order below is the precedence: embedded over an override over a deployed release.
func (s *syncer) syncModules(ctx context.Context) error {
	configs, err := s.getModuleConfigs(ctx)
	if err != nil {
		return fmt.Errorf("get module configs: %w", err)
	}

	releaseChannels, err := s.getReleaseChannelsByUpdatePolicy(ctx)
	if err != nil {
		return fmt.Errorf("get release channels: %w", err)
	}

	deckhouseReleaseChannel := s.getEmbeddedReleaseChannel(configs)

	if err := s.syncDeployedModules(ctx, deckhouseReleaseChannel, releaseChannels, configs); err != nil {
		return fmt.Errorf("sync deployed modules: %w", err)
	}

	if err := s.syncOverrideModules(ctx, deckhouseReleaseChannel, releaseChannels, configs); err != nil {
		return fmt.Errorf("sync override modules: %w", err)
	}

	if err := s.syncEmbeddedModules(ctx, deckhouseReleaseChannel, configs); err != nil {
		return fmt.Errorf("sync embedded modules: %w", err)
	}

	if err := s.syncGlobalModule(ctx, configs); err != nil {
		return fmt.Errorf("sync global module: %w", err)
	}

	if err := s.cleanupModules(ctx); err != nil {
		return fmt.Errorf("cleanup modules: %w", err)
	}

	return nil
}

// getModuleConfigs reads the module configs the cluster carries.
// A config under deletion counts as gone: its settings are on their way out.
func (s *syncer) getModuleConfigs(ctx context.Context) (map[string]*v1alpha1.ModuleConfig, error) {
	moduleConfigList := new(v1alpha1.ModuleConfigList)
	if err := s.reader.List(ctx, moduleConfigList); err != nil {
		return nil, fmt.Errorf("list module configs: %w", err)
	}

	moduleConfigs := make(map[string]*v1alpha1.ModuleConfig, len(moduleConfigList.Items))

	for index := range moduleConfigList.Items {
		moduleConfig := &moduleConfigList.Items[index]
		if !moduleConfig.DeletionTimestamp.IsZero() {
			continue
		}

		moduleConfigs[moduleConfig.Name] = moduleConfig
	}

	return moduleConfigs, nil
}

// syncGlobalModule writes the Module of the global module, which the image ships at a reserved
// name and a dir of its own, so nothing is read from disk to name it.
func (s *syncer) syncGlobalModule(ctx context.Context, configs map[string]*v1alpha1.ModuleConfig) error {
	// every module of one build carries the same version, the one
	// embeddedPackageVersion reduces the Deckhouse version to
	embeddedPackageVersion := app.EmbeddedPackageVersion()

	if err := s.ensureModule(ctx, "global", repositoryNameEmbedded, embeddedPackageVersion, "", false, configs["global"]); err != nil {
		return fmt.Errorf("ensure global module: %w", err)
	}

	return nil
}

// syncEmbeddedModules writes a Module for every module the running image ships, on the reserved
// embedded repository. A dir holding no readable definition is skipped with a warning.
func (s *syncer) syncEmbeddedModules(ctx context.Context, deckhouseReleaseChannel string, configs map[string]*v1alpha1.ModuleConfig) error {
	dirEntries, err := os.ReadDir(s.embeddedModulesDir)
	if err != nil {
		return fmt.Errorf("read embedded modules dir: %w", err)
	}

	// every module of one build carries the same version, the one
	// embeddedPackageVersion reduces the Deckhouse version to
	embeddedPackageVersion := app.EmbeddedPackageVersion()

	releaseChannel := deckhouseReleaseChannel

	modules := make([]string, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() || slices.Contains(app.DummyModules, dirEntry.Name()) {
			continue
		}

		moduleDir := filepath.Join(s.embeddedModulesDir, dirEntry.Name())

		def, err := loader.LoadEmbeddedDefinition(moduleDir)
		if err != nil {
			s.logger.Warn("module dir holds no readable definition, skip its module", slog.String("dir", moduleDir), log.Err(err))

			continue
		}

		modules = append(modules, def.Name)
	}

	for _, module := range modules {
		if err := s.ensureModule(ctx, module, repositoryNameEmbedded, embeddedPackageVersion, releaseChannel, false, configs[module]); err != nil {
			return fmt.Errorf("ensure embedded module %s: %w", module, err)
		}
	}

	return nil
}

// syncDeployedModules writes a Module for every module running a deployed release. The channel
// comes from the update policy the module names, falling back to the one Deckhouse follows.
func (s *syncer) syncDeployedModules(ctx context.Context, deckhouseReleaseChannel string, releaseChannels map[string]string, configs map[string]*v1alpha1.ModuleConfig) error {
	deployed, err := s.getDeployedModuleReleases(ctx)
	if err != nil {
		return fmt.Errorf("get deployed module releases: %w", err)
	}

	for _, release := range deployed {
		config := configs[release.GetModuleName()]
		moduleName := release.GetModuleName()
		moduleVersion := release.GetModuleVersion()
		repositoryName := PackageRepositoryNameForModuleSource(release.GetModuleSource())

		releaseChannel := deckhouseReleaseChannel
		if config != nil {
			if channel, ok := releaseChannels[config.Spec.UpdatePolicy]; ok {
				releaseChannel = channel
			}
		}

		if err := s.ensureModule(ctx, moduleName, repositoryName, moduleVersion, releaseChannel, false, config); err != nil {
			return fmt.Errorf("ensure module %s: %w", moduleName, err)
		}
	}

	return nil
}

// getDeployedModuleReleases reads the deployed release of every module no pull override covers.
// A release naming no module source is skipped with a warning: it resolves to no repository.
func (s *syncer) getDeployedModuleReleases(ctx context.Context) ([]*v1alpha1.ModuleRelease, error) {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := s.reader.List(ctx, releases); err != nil {
		return nil, fmt.Errorf("list module releases: %w", err)
	}

	deployed := make([]*v1alpha1.ModuleRelease, 0, len(releases.Items))
	for _, release := range releases.Items {
		override := new(v1alpha2.ModulePullOverride)
		if err := s.reader.Get(ctx, client.ObjectKey{Name: release.GetModuleName()}, override); err == nil {
			continue
		}

		if release.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed || !release.DeletionTimestamp.IsZero() {
			continue
		}

		if release.GetModuleSource() == "" {
			s.logger.Warn("release has no module source, skip its module", slog.String("release", release.Name))

			continue
		}

		deployed = append(deployed, &release)
	}

	return deployed, nil
}

// syncOverrideModules writes a Module for every module running the dev copy a pull override put on
// disk. Its version is the image tag the override names, not a package version.
func (s *syncer) syncOverrideModules(ctx context.Context, deckhouseReleaseChannel string, releaseChannels map[string]string, configs map[string]*v1alpha1.ModuleConfig) error {
	overrides, err := s.getOverrides(ctx)
	if err != nil {
		return fmt.Errorf("get overrides: %w", err)
	}

	for _, override := range overrides {
		config := configs[override.Name]
		moduleName := override.Name
		imageTag := override.Spec.ImageTag

		repositories, err := s.availableRepositories(ctx, moduleName)
		if err != nil {
			return fmt.Errorf("get the available repositories of the '%s' module: %w", moduleName, err)
		}

		repositoryName, ok := overrideRepository(config, repositories)
		if !ok {
			s.logger.Warn("no config source and no single available repository for the dev module, skip it",
				slog.String("name", moduleName), slog.Any("repositories", repositories))

			continue
		}

		releaseChannel := deckhouseReleaseChannel
		if config != nil {
			if channel, ok := releaseChannels[config.Spec.UpdatePolicy]; ok {
				releaseChannel = channel
			}
		}

		if err := s.ensureModule(ctx, moduleName, repositoryName, imageTag, releaseChannel, true, config); err != nil {
			return fmt.Errorf("ensure module %s: %w", moduleName, err)
		}
	}

	return nil
}

// overrideRepository answers which repository a dev module belongs to. The pull override names no
// source, so it comes from the module config, or from the only repository the catalog lists for the
// package - "deckhouse-modules" when it lists several. Nothing left to ask leaves the module unplaced.
func overrideRepository(config *v1alpha1.ModuleConfig, repositories []string) (string, bool) {
	if config != nil && config.Spec.Source != "" {
		return PackageRepositoryNameForModuleSource(config.Spec.Source), true
	}

	switch {
	case len(repositories) == 1:
		return repositories[0], true

	case slices.Contains(repositories, repositoryNameDeckhouseModules):
		return repositoryNameDeckhouseModules, true
	}

	return "", false
}

// availableRepositories reads the repositories the catalog entry of the module lists.
// A module the catalog does not name yet lists none.
func (s *syncer) availableRepositories(ctx context.Context, moduleName string) ([]string, error) {
	modulePackage := new(v1alpha1.ModulePackage)
	if err := s.reader.Get(ctx, client.ObjectKey{Name: moduleName}, modulePackage); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("get the '%s' module package: %w", moduleName, err)
	}

	return modulePackage.Status.AvailableRepositories, nil
}

// getOverrides reads the pull override each module runs a dev copy by.
// Only a ready override counts: the loader has put its copy on disk.
func (s *syncer) getOverrides(ctx context.Context) ([]*v1alpha2.ModulePullOverride, error) {
	overrides := new(v1alpha2.ModulePullOverrideList)
	if err := s.reader.List(ctx, overrides); err != nil {
		return nil, fmt.Errorf("list module pull overrides: %w", err)
	}

	result := make([]*v1alpha2.ModulePullOverride, 0, len(overrides.Items))
	for _, override := range overrides.Items {
		if override.Status.Message != v1alpha2.ModulePullOverrideMessageReady || !override.DeletionTimestamp.IsZero() {
			continue
		}

		result = append(result, &override)
	}

	return result, nil
}

// cleanupModules deletes the modules no pass above placed: one carrying no package version at all,
// and an embedded one left on the version of a build the image no longer ships.
func (s *syncer) cleanupModules(ctx context.Context) error {
	modules := new(v1alpha2.ModuleList)
	if err := s.reader.List(ctx, modules); err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	for _, module := range modules.Items {
		disposable := module.Spec.PackageVersion == "" ||
			(module.IsEmbedded() && module.Spec.PackageVersion != app.EmbeddedPackageVersion())

		if !disposable {
			continue
		}

		s.logger.Debug("delete orphan module", slog.String("name", module.Name))

		if err := s.writer.Delete(ctx, &module); err != nil {
			return fmt.Errorf("delete module %s: %w", module.Name, err)
		}
	}

	return nil
}

// getEmbeddedReleaseChannel reads the channel Deckhouse itself follows, the one its embedded
// modules come on. It lives in the deckhouse module config; with none set the build default applies.
func (s *syncer) getEmbeddedReleaseChannel(configs map[string]*v1alpha1.ModuleConfig) string {
	deckhouseConfig, ok := configs[packageNameDeckhouse]
	if !ok || deckhouseConfig.Spec.Settings == nil {
		return app.DefaultReleaseChannel
	}

	settings := struct {
		ReleaseChannel string `json:"releaseChannel"`
	}{}

	if err := json.Unmarshal(deckhouseConfig.Spec.Settings.Raw, &settings); err != nil {
		s.logger.Warn("the deckhouse module config settings do not parse, fall back to the build release channel",
			log.Err(err))

		return app.DefaultReleaseChannel
	}

	if settings.ReleaseChannel == "" {
		return app.DefaultReleaseChannel
	}

	return settings.ReleaseChannel
}

// getReleaseChannelsByUpdatePolicy reads the channel every module update policy follows.
// A module naming no policy, or one the cluster has lost, falls back to the channel of Deckhouse.
func (s *syncer) getReleaseChannelsByUpdatePolicy(ctx context.Context) (map[string]string, error) {
	updatePolicyList := new(v1alpha2.ModuleUpdatePolicyList)
	if err := s.reader.List(ctx, updatePolicyList); err != nil {
		return nil, fmt.Errorf("list module update policies: %w", err)
	}

	releaseChannels := make(map[string]string, len(updatePolicyList.Items))

	for _, updatePolicy := range updatePolicyList.Items {
		releaseChannels[updatePolicy.Name] = updatePolicy.Spec.ReleaseChannel
	}

	return releaseChannels, nil
}

// ensureModule brings the module in line with the files it runs and its config, creating it when
// the cluster carries none. The module has other writers, so a patch with no drift is not sent.
func (s *syncer) ensureModule(ctx context.Context, moduleName, repositoryName, packageVersion, releaseChannel string, dev bool, moduleConfig *v1alpha1.ModuleConfig) error {
	module := new(v1alpha2.Module)

	if err := s.reader.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get the '%s' module: %w", moduleName, err)
		}

		return s.createModule(ctx, moduleName, repositoryName, packageVersion, releaseChannel, dev, moduleConfig)
	}

	patch := client.MergeFrom(module.DeepCopy())

	applyModuleVersion(module, repositoryName, packageVersion, releaseChannel, dev)
	applyModuleConfig(module, moduleConfig)

	patchData, err := patch.Data(module)
	if err != nil {
		return fmt.Errorf("build the patch for the '%s' module: %w", moduleName, err)
	}

	if string(patchData) == "{}" {
		return nil
	}

	if err = s.writer.Patch(ctx, module, client.RawPatch(patch.Type(), patchData)); err != nil {
		return fmt.Errorf("patch the '%s' module: %w", moduleName, err)
	}

	s.logger.Debug("module synced", slog.String("name", moduleName), slog.String("version", packageVersion))

	return nil
}

// createModule writes a module the cluster does not carry yet.
func (s *syncer) createModule(ctx context.Context, moduleName, repositoryName, packageVersion, releaseChannel string, dev bool, moduleConfig *v1alpha1.ModuleConfig) error {
	module := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: moduleName}}

	applyModuleVersion(module, repositoryName, packageVersion, releaseChannel, dev)
	applyModuleConfig(module, moduleConfig)

	if err := s.writer.Create(ctx, module); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create the '%s' module: %w", moduleName, err)
	}

	s.logger.Debug("module created", slog.String("name", moduleName), slog.String("version", packageVersion))

	return nil
}

// applyModuleVersion writes the package coordinates and marks where the files came from. The
// embedded mark is reconciled both ways; the dev mark is only set, the override controller clears it.
func applyModuleVersion(module *v1alpha2.Module, repositoryName, packageVersion, releaseChannel string, dev bool) {
	module.Spec.PackageRepositoryName = repositoryName
	module.Spec.PackageVersion = packageVersion
	module.Spec.ReleaseChannel = releaseChannel

	if dev {
		setModuleAnnotation(module, v1alpha2.ModuleAnnotationDev)
	}

	if repositoryName != repositoryNameEmbedded {
		delete(module.Annotations, v1alpha2.ModuleAnnotationEmbedded)

		return
	}

	setModuleAnnotation(module, v1alpha2.ModuleAnnotationEmbedded)
}

// setModuleAnnotation marks the key true, allocating the map when the module carries none.
func setModuleAnnotation(module *v1alpha2.Module, key string) {
	if module.Annotations == nil {
		module.Annotations = make(map[string]string, 1)
	}

	module.Annotations[key] = "true"
}

// applyModuleConfig mirrors the module config into the module spec. The four fields belong to the
// config alone, so with no config they are cleared and no deleted setting survives.
func applyModuleConfig(module *v1alpha2.Module, moduleConfig *v1alpha1.ModuleConfig) {
	if moduleConfig == nil {
		module.Spec.Enabled = nil
		module.Spec.Settings = nil
		module.Spec.SettingsVersion = 0
		module.Spec.Maintenance = ""

		return
	}

	module.Spec.Enabled = moduleConfig.Spec.Enabled
	module.Spec.Settings = moduleConfig.Spec.Settings
	module.Spec.SettingsVersion = moduleConfig.Spec.Version
	module.Spec.Maintenance = moduleConfig.Spec.Maintenance
}
