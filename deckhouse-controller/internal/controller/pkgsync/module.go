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
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// syncModules writes a Module for every module whose files the cluster carries.
// A module the sync finds nowhere keeps the object another writer gave it.
//
// The sources are read in the order the files win each other:
//   - the embedded copy, it is what the binary carries on disk right now
//   - a ready pull override, the loader put its dev copy on disk over the release
//   - the newest deployed release
func (s *syncer) syncModules(ctx context.Context) error {
	moduleConfigs, err := s.moduleConfigsByName(ctx)
	if err != nil {
		return err
	}

	embeddedModuleNames, err := s.embeddedModuleNames()
	if err != nil {
		return err
	}

	// every module of one build carries the same version, the one
	// app.EmbeddedPackageVersion reduces the Deckhouse version to
	embeddedPackageVersion := app.EmbeddedPackageVersion(s.deckhouseVersion)

	syncedModules := make(map[string]struct{}, len(embeddedModuleNames))

	for _, moduleName := range embeddedModuleNames {
		if err := s.ensureEmbeddedModule(ctx, moduleName, embeddedPackageVersion, moduleConfigs[moduleName]); err != nil {
			return err
		}

		syncedModules[moduleName] = struct{}{}
	}

	pullOverrides, err := s.readyModulePullOverridesByModule(ctx)
	if err != nil {
		return err
	}

	// the modules are independent: the order only keeps the log readable
	for _, moduleName := range slices.Sorted(maps.Keys(pullOverrides)) {
		if _, ok := syncedModules[moduleName]; ok {
			continue
		}

		repositoryName, ok := s.repositoryForOverriddenModule(ctx, moduleName, moduleConfigs[moduleName])
		if !ok {
			continue
		}

		if err := s.ensureOverriddenModule(ctx, moduleName, repositoryName, pullOverrides[moduleName].Spec.ImageTag, moduleConfigs[moduleName]); err != nil {
			return err
		}

		syncedModules[moduleName] = struct{}{}
	}

	deployedReleases, err := s.deployedModuleReleasesByModule(ctx)
	if err != nil {
		return err
	}

	for _, moduleName := range slices.Sorted(maps.Keys(deployedReleases)) {
		if _, ok := syncedModules[moduleName]; ok {
			continue
		}

		moduleRelease := deployedReleases[moduleName]
		repositoryName := PackageRepositoryNameForModuleSource(moduleRelease.GetModuleSource())

		// the version parses: deployedModuleReleasesByModule dropped the releases it does not
		if err := s.ensureReleasedModule(ctx, moduleName, repositoryName, moduleRelease.GetModuleVersion(), moduleConfigs[moduleName]); err != nil {
			return err
		}

		syncedModules[moduleName] = struct{}{}
	}

	return nil
}

// embeddedModuleNames reads the names of the modules from the embedded modules dir.
// A dir with no readable definition is skipped with a warning, the same way the
// ModulePackageVersion pass skips it.
func (s *syncer) embeddedModuleNames() ([]string, error) {
	dirEntries, err := os.ReadDir(s.embeddedModulesDir)
	if err != nil {
		return nil, fmt.Errorf("read embedded modules dir: %w", err)
	}

	moduleNames := make([]string, 0, len(dirEntries))

	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() || slices.Contains(app.DummyModules, dirEntry.Name()) {
			continue
		}

		moduleDir := filepath.Join(s.embeddedModulesDir, dirEntry.Name())

		moduleDefinition, err := loader.LoadEmbeddedDefinition(moduleDir)
		if err != nil {
			s.logger.Warn("module dir holds no readable definition, skip its module",
				slog.String("dir", moduleDir), log.Err(err))

			continue
		}

		moduleNames = append(moduleNames, moduleDefinition.Name)
	}

	return moduleNames, nil
}

// deployedModuleReleasesByModule reads the release each module runs:
//   - only a deployed release counts, a pending one is not on disk yet
//   - the newest wins, a restart mid upgrade leaves two of them deployed
//   - a release with no module source or an unparsable version is skipped with a
//     warning, its version getters panic on such a value
func (s *syncer) deployedModuleReleasesByModule(ctx context.Context) (map[string]*v1alpha1.ModuleRelease, error) {
	moduleReleaseList := new(v1alpha1.ModuleReleaseList)
	if err := s.reader.List(ctx, moduleReleaseList); err != nil {
		return nil, fmt.Errorf("list module releases: %w", err)
	}

	deployedReleases := make(map[string]*v1alpha1.ModuleRelease, len(moduleReleaseList.Items))
	deployedVersions := make(map[string]*semver.Version, len(moduleReleaseList.Items))

	for index := range moduleReleaseList.Items {
		moduleRelease := &moduleReleaseList.Items[index]

		if moduleRelease.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed || !moduleRelease.DeletionTimestamp.IsZero() {
			continue
		}

		if moduleRelease.GetModuleSource() == "" {
			s.logger.Warn("release has no module source, skip its module",
				slog.String("release", moduleRelease.Name))

			continue
		}

		releaseVersion, err := semver.NewVersion(moduleRelease.Spec.Version)
		if err != nil {
			s.logger.Warn("release version is not a semver, skip its module",
				slog.String("release", moduleRelease.Name),
				slog.String("version", moduleRelease.Spec.Version), log.Err(err))

			continue
		}

		moduleName := moduleRelease.GetModuleName()

		if deployedVersion, ok := deployedVersions[moduleName]; ok && !releaseVersion.GreaterThan(deployedVersion) {
			continue
		}

		deployedVersions[moduleName] = releaseVersion
		deployedReleases[moduleName] = moduleRelease
	}

	return deployedReleases, nil
}

// readyModulePullOverridesByModule reads the pull override each module runs a dev copy by.
// Only a ready override counts: the loader has put its copy on disk.
func (s *syncer) readyModulePullOverridesByModule(ctx context.Context) (map[string]*v1alpha2.ModulePullOverride, error) {
	pullOverrideList := new(v1alpha2.ModulePullOverrideList)
	if err := s.reader.List(ctx, pullOverrideList); err != nil {
		return nil, fmt.Errorf("list module pull overrides: %w", err)
	}

	pullOverrides := make(map[string]*v1alpha2.ModulePullOverride, len(pullOverrideList.Items))

	for index := range pullOverrideList.Items {
		pullOverride := &pullOverrideList.Items[index]

		if pullOverride.Status.Message != v1alpha2.ModulePullOverrideMessageReady || !pullOverride.DeletionTimestamp.IsZero() {
			continue
		}

		pullOverrides[pullOverride.GetModuleName()] = pullOverride
	}

	return pullOverrides, nil
}

// repositoryForOverriddenModule answers which repository a dev module belongs to.
// The pull override names none, so the answer comes from what the cluster already
// knows, in this order:
//   - the repository the module object carries, written by a release or an earlier start
//   - the source the module config names
//   - the only module source offering the module, or "deckhouse" when several do
//
// Nothing left to ask means the module is not placed: the reason goes to the log,
// and the override controller reports it on the pull override itself.
func (s *syncer) repositoryForOverriddenModule(ctx context.Context, moduleName string, moduleConfig *v1alpha1.ModuleConfig) (string, bool) {
	module := new(v1alpha2.Module)
	if err := s.reader.Get(ctx, client.ObjectKey{Name: moduleName}, module); err == nil {
		if name := module.Spec.PackageRepositoryName; name != "" && name != repositoryNameEmbedded {
			return name, true
		}
	}

	if moduleConfig != nil && moduleConfig.Spec.Source != "" {
		return PackageRepositoryNameForModuleSource(moduleConfig.Spec.Source), true
	}

	moduleSourceNames, err := s.moduleSourceNamesOffering(ctx, moduleName)
	if err != nil {
		s.logger.Warn("cannot read the module sources, skip the overridden module",
			slog.String("name", moduleName), log.Err(err))

		return "", false
	}

	switch {
	case len(moduleSourceNames) == 1:
		return PackageRepositoryNameForModuleSource(moduleSourceNames[0]), true

	case slices.Contains(moduleSourceNames, moduleSourceNameDeckhouse):
		return repositoryNameDeckhouseModules, true
	}

	s.logger.Warn("no single module source offers the overridden module, skip it",
		slog.String("name", moduleName), slog.Any("sources", moduleSourceNames))

	return "", false
}

// moduleSourceNamesOffering reads the module sources that offer the module.
func (s *syncer) moduleSourceNamesOffering(ctx context.Context, moduleName string) ([]string, error) {
	moduleSourceList := new(v1alpha1.ModuleSourceList)
	if err := s.reader.List(ctx, moduleSourceList); err != nil {
		return nil, fmt.Errorf("list module sources: %w", err)
	}

	moduleSourceNames := make([]string, 0, len(moduleSourceList.Items))

	for _, moduleSource := range moduleSourceList.Items {
		for _, availableModule := range moduleSource.Status.AvailableModules {
			if availableModule.Name == moduleName {
				moduleSourceNames = append(moduleSourceNames, moduleSource.Name)

				break
			}
		}
	}

	return moduleSourceNames, nil
}

// moduleConfigsByName reads the module configs the cluster carries.
// A config under deletion counts as gone: its settings are on their way out.
func (s *syncer) moduleConfigsByName(ctx context.Context) (map[string]*v1alpha1.ModuleConfig, error) {
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

// ensureEmbeddedModule writes a module the Deckhouse image ships.
func (s *syncer) ensureEmbeddedModule(ctx context.Context, moduleName, packageVersion string, moduleConfig *v1alpha1.ModuleConfig) error {
	return s.ensureModule(ctx, moduleName, repositoryNameEmbedded, packageVersion, false, moduleConfig)
}

// ensureOverriddenModule writes a module running the dev copy a pull override put on disk.
// The version is the image tag the override names, not a package version.
func (s *syncer) ensureOverriddenModule(ctx context.Context, moduleName, repositoryName, imageTag string, moduleConfig *v1alpha1.ModuleConfig) error {
	return s.ensureModule(ctx, moduleName, repositoryName, imageTag, true, moduleConfig)
}

// ensureReleasedModule writes a module running the package of its deployed release.
func (s *syncer) ensureReleasedModule(ctx context.Context, moduleName, repositoryName, packageVersion string, moduleConfig *v1alpha1.ModuleConfig) error {
	return s.ensureModule(ctx, moduleName, repositoryName, packageVersion, false, moduleConfig)
}

// ensureModule brings the module in line with the files it runs and its config:
// - the object is created when the cluster carries none
// - only the fields below are written, the module has other writers
// - a patch with no drift is not sent
func (s *syncer) ensureModule(ctx context.Context, moduleName, repositoryName, packageVersion string, dev bool, moduleConfig *v1alpha1.ModuleConfig) error {
	module := new(v1alpha2.Module)

	if err := s.reader.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get the '%s' module: %w", moduleName, err)
		}

		return s.createModule(ctx, moduleName, repositoryName, packageVersion, dev, moduleConfig)
	}

	patch := client.MergeFrom(module.DeepCopy())

	applyModuleVersion(module, repositoryName, packageVersion, dev)
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
// Rare: the old module stack creates an object for every module it knows.
func (s *syncer) createModule(ctx context.Context, moduleName, repositoryName, packageVersion string, dev bool, moduleConfig *v1alpha1.ModuleConfig) error {
	module := &v1alpha2.Module{ObjectMeta: metav1.ObjectMeta{Name: moduleName}}

	applyModuleVersion(module, repositoryName, packageVersion, dev)
	applyModuleConfig(module, moduleConfig)

	if err := s.writer.Create(ctx, module); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create the '%s' module: %w", moduleName, err)
	}

	s.logger.Debug("module created", slog.String("name", moduleName), slog.String("version", packageVersion))

	return nil
}

// applyModuleVersion writes spec.packageRepositoryName and spec.packageVersion,
// and marks where the files came from.
//
// The embedded mark is written and cleared on every pass: an upgrade drops the
// embedded copy and the module moves to a repository. The dev mark is only ever
// written: the override controller takes it off when the pull override goes away.
func applyModuleVersion(module *v1alpha2.Module, repositoryName, packageVersion string, dev bool) {
	module.Spec.PackageRepositoryName = repositoryName
	module.Spec.PackageVersion = packageVersion

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

// applyModuleConfig mirrors the module config into the module spec.
// The four fields belong to the config alone: with no config they are cleared,
// so a module keeps no settings the user has deleted.
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
