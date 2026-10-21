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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metautils "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1beta1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// legacyModuleConditions are the conditions the addon-operator stack wrote. The package runtime
// owns the status now and never touches a condition type of its own, so they linger until dropped.
var legacyModuleConditions = []string{
	v1alpha1.ModuleConditionEnabledByModuleManager,
	v1alpha1.ModuleConditionEnabledByModuleConfig,
	v1alpha1.ModuleConditionIsReady,
	v1alpha1.ModuleConditionLastReleaseDeployed,
	v1alpha1.ModuleConditionIsOverridden,
}

// moduleMigrationConfigMapName marks that the modules were migrated once; from then on the user owns them.
const moduleMigrationConfigMapName = "module-migration"

// syncModules writes a Module for every module the cluster runs. The last pass to write a name
// wins, so the order below is the precedence: embedded over a deployed release. Once migrated,
// the deployed modules are skipped and the embedded ones are only created, so no user edit is reverted.
func (s *syncer) syncModules(ctx context.Context) error {
	migrated, err := s.modulesMigrated(ctx)
	if err != nil {
		return fmt.Errorf("check module migration: %w", err)
	}

	configs, err := s.getModuleConfigs(ctx)
	if err != nil {
		return fmt.Errorf("get module configs: %w", err)
	}

	updatePolicies, err := s.getUpdatePolicies(ctx)
	if err != nil {
		return fmt.Errorf("get update policies: %w", err)
	}

	deckhouseReleaseChannel := s.getDeckhouseReleaseChannel(configs)

	if !migrated {
		if err := s.syncDeployedModules(ctx, deckhouseReleaseChannel, updatePolicies, configs); err != nil {
			return fmt.Errorf("sync deployed modules: %w", err)
		}
	}

	if err := s.deleteModuleReleases(ctx); err != nil {
		return fmt.Errorf("delete module releases: %w", err)
	}

	if err := s.syncEmbeddedModules(ctx, deckhouseReleaseChannel, configs, migrated); err != nil {
		return fmt.Errorf("sync embedded modules: %w", err)
	}

	if err := s.syncGlobalModule(ctx, configs, migrated); err != nil {
		return fmt.Errorf("sync global module: %w", err)
	}

	if err := s.deleteUnresolvableModules(ctx); err != nil {
		return fmt.Errorf("delete unresolvable modules: %w", err)
	}

	if err := s.dropLegacyConditions(ctx); err != nil {
		return fmt.Errorf("drop legacy conditions: %w", err)
	}

	if !migrated {
		if err := s.markModulesMigrated(ctx); err != nil {
			return fmt.Errorf("mark module migration: %w", err)
		}
	}

	return nil
}

// modulesMigrated reports whether the migration marker exists.
func (s *syncer) modulesMigrated(ctx context.Context) (bool, error) {
	key := client.ObjectKey{Namespace: app.NamespaceDeckhouse, Name: moduleMigrationConfigMapName}
	if err := s.reader.Get(ctx, key, new(corev1.ConfigMap)); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("get the '%s' config map: %w", moduleMigrationConfigMapName, err)
	}

	return true, nil
}

// markModulesMigrated writes the migration marker; it goes last, so a failed migration reruns.
func (s *syncer) markModulesMigrated(ctx context.Context) error {
	marker := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      moduleMigrationConfigMapName,
			Namespace: app.NamespaceDeckhouse,
			Labels:    map[string]string{"heritage": "deckhouse"},
		},
	}

	if err := s.writer.Create(ctx, marker); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create the '%s' config map: %w", moduleMigrationConfigMapName, err)
	}

	s.logger.Info("modules migrated")

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
// name and a dir of its own, so nothing is read from disk to name it. With createOnly an existing one is kept.
func (s *syncer) syncGlobalModule(ctx context.Context, configs map[string]*v1alpha1.ModuleConfig, createOnly bool) error {
	// every module of one build carries the same version, the one
	// embeddedPackageVersion reduces the Deckhouse version to
	embeddedPackageVersion := app.EmbeddedPackageVersion()

	if err := s.ensureModule(ctx, packageNameGlobal, repositoryNameEmbedded, embeddedPackageVersion, "", nil, configs["global"], createOnly); err != nil {
		return fmt.Errorf("ensure global module: %w", err)
	}

	return nil
}

// syncEmbeddedModules writes a Module for every module the running image ships, on the reserved
// embedded repository. A dir holding no readable definition is skipped with a warning.
// With createOnly the existing modules are kept and only the ones a new image ships are created.
func (s *syncer) syncEmbeddedModules(ctx context.Context, deckhouseReleaseChannel string, configs map[string]*v1alpha1.ModuleConfig, createOnly bool) error {
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
		if err := s.ensureModule(ctx, module, repositoryNameEmbedded, embeddedPackageVersion, releaseChannel, nil, configs[module], createOnly); err != nil {
			return fmt.Errorf("ensure embedded module %s: %w", module, err)
		}
	}

	return nil
}

// syncDeployedModules writes a Module for every module running a deployed release. The channel and
// the update settings come from the update policy the module names; with none, Deckhouse ones apply.
func (s *syncer) syncDeployedModules(ctx context.Context, deckhouseReleaseChannel string, updatePolicies map[string]*v1alpha2.ModuleUpdatePolicy, configs map[string]*v1alpha1.ModuleConfig) error {
	deployed, err := s.getDeployedModuleReleases(ctx)
	if err != nil {
		return fmt.Errorf("get deployed module releases: %w", err)
	}

	for _, release := range deployed {
		config := configs[release.GetModuleName()]
		moduleName := release.GetModuleName()
		moduleVersion := release.GetModuleVersion()
		repositoryName := packageRepositoryNameForModuleSource(release.GetModuleSource())

		releaseChannel := deckhouseReleaseChannel

		var update *v1beta1.ModuleUpdate
		if config != nil {
			if policy, ok := updatePolicies[config.Spec.UpdatePolicy]; ok {
				releaseChannel = policy.Spec.ReleaseChannel
				update = moduleUpdateFromPolicy(policy)
			}
		}

		if err := s.ensureModule(ctx, moduleName, repositoryName, moduleVersion, releaseChannel, update, config, false); err != nil {
			return fmt.Errorf("ensure module %s: %w", moduleName, err)
		}
	}

	return nil
}

// getDeployedModuleReleases reads the deployed release of every module.
// A release naming no module source is skipped with a warning: it resolves to no repository.
func (s *syncer) getDeployedModuleReleases(ctx context.Context) ([]*v1alpha1.ModuleRelease, error) {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := s.reader.List(ctx, releases); err != nil {
		return nil, fmt.Errorf("list module releases: %w", err)
	}

	deployed := make([]*v1alpha1.ModuleRelease, 0, len(releases.Items))
	for _, release := range releases.Items {
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

// deleteModuleReleases deletes every module release once its module has taken the version over. The
// module owns the version from here on, so a release left behind would roll it back on the next
// start. No release controller runs here to release the finalizers, so they are dropped first.
func (s *syncer) deleteModuleReleases(ctx context.Context) error {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := s.reader.List(ctx, releases); err != nil {
		return fmt.Errorf("list module releases: %w", err)
	}

	for i := range releases.Items {
		release := &releases.Items[i]

		if err := s.dropModuleReleaseFinalizers(ctx, release); err != nil {
			return fmt.Errorf("drop finalizers of the '%s' module release: %w", release.Name, err)
		}

		if err := s.writer.Delete(ctx, release); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("delete the '%s' module release: %w", release.Name, err)
		}

		s.logger.Debug("module release deleted", slog.String("name", release.Name))
	}

	return nil
}

// dropModuleReleaseFinalizers removes the finalizers of the old release controller. A release
// carrying none is not patched.
func (s *syncer) dropModuleReleaseFinalizers(ctx context.Context, release *v1alpha1.ModuleRelease) error {
	patch := client.MergeFrom(release.DeepCopy())

	controllerutil.RemoveFinalizer(release, v1alpha1.ModuleReleaseFinalizerExistOnFs)
	controllerutil.RemoveFinalizer(release, v1alpha1.ModuleReleaseFinalizerMetricsRegistered)

	patchData, err := patch.Data(release)
	if err != nil {
		return fmt.Errorf("build the patch: %w", err)
	}

	if string(patchData) == "{}" {
		return nil
	}

	if err = s.writer.Patch(ctx, release, client.RawPatch(patch.Type(), patchData)); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("patch: %w", err)
	}

	return nil
}

// getDeckhouseReleaseChannel reads the channel Deckhouse itself follows, the one its embedded
// modules come on. It lives in the deckhouse module config; with none set the build default applies.
func (s *syncer) getDeckhouseReleaseChannel(configs map[string]*v1alpha1.ModuleConfig) string {
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

// getUpdatePolicies reads the module update policies by name.
// A module naming no policy, or one the cluster has lost, falls back to the settings of Deckhouse.
func (s *syncer) getUpdatePolicies(ctx context.Context) (map[string]*v1alpha2.ModuleUpdatePolicy, error) {
	updatePolicyList := new(v1alpha2.ModuleUpdatePolicyList)
	if err := s.reader.List(ctx, updatePolicyList); err != nil {
		return nil, fmt.Errorf("list module update policies: %w", err)
	}

	updatePolicies := make(map[string]*v1alpha2.ModuleUpdatePolicy, len(updatePolicyList.Items))

	for index := range updatePolicyList.Items {
		updatePolicy := &updatePolicyList.Items[index]
		updatePolicies[updatePolicy.Name] = updatePolicy
	}

	return updatePolicies, nil
}

// moduleUpdateFromPolicy converts the update settings of the policy into the module ones.
// The Ignore mode has no module counterpart, so it yields none and the Deckhouse settings apply.
func moduleUpdateFromPolicy(policy *v1alpha2.ModuleUpdatePolicy) *v1beta1.ModuleUpdate {
	if policy.Spec.Update.Mode == v1alpha2.ModuleUpdatePolicyModeIgnore {
		return nil
	}

	update := &v1beta1.ModuleUpdate{Mode: policy.Spec.Update.Mode}

	for _, window := range policy.Spec.Update.Windows {
		update.Windows = append(update.Windows, v1beta1.ModuleUpdateWindow{
			From: window.From,
			To:   window.To,
			Days: window.Days,
		})
	}

	return update
}

// ensureModule brings the module in line with the files it runs and its config, creating it when
// the cluster carries none; with createOnly an existing module is kept as is. The module has other
// writers, so a patch with no drift is not sent.
func (s *syncer) ensureModule(ctx context.Context, moduleName, repositoryName, packageVersion, releaseChannel string, update *v1beta1.ModuleUpdate, moduleConfig *v1alpha1.ModuleConfig, createOnly bool) error {
	module := new(v1beta1.Module)

	if err := s.reader.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get the '%s' module: %w", moduleName, err)
		}

		return s.createModule(ctx, moduleName, repositoryName, packageVersion, releaseChannel, update, moduleConfig)
	}

	if createOnly {
		return nil
	}

	patch := client.MergeFrom(module.DeepCopy())

	applyModuleVersion(module, repositoryName, packageVersion, releaseChannel, update)
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
func (s *syncer) createModule(ctx context.Context, moduleName, repositoryName, packageVersion, releaseChannel string, update *v1beta1.ModuleUpdate, moduleConfig *v1alpha1.ModuleConfig) error {
	module := &v1beta1.Module{ObjectMeta: metav1.ObjectMeta{Name: moduleName}}

	applyModuleVersion(module, repositoryName, packageVersion, releaseChannel, update)
	applyModuleConfig(module, moduleConfig)

	if err := s.writer.Create(ctx, module); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create the '%s' module: %w", moduleName, err)
	}

	s.logger.Debug("module created", slog.String("name", moduleName), slog.String("version", packageVersion))

	return nil
}

// applyModuleVersion writes the package coordinates, the update settings and marks where the files
// came from; the update settings and the embedded mark are reconciled both ways.
func applyModuleVersion(module *v1beta1.Module, repositoryName, packageVersion, releaseChannel string, update *v1beta1.ModuleUpdate) {
	module.Spec.PackageRepositoryName = repositoryName
	module.Spec.PackageVersion = packageVersion
	module.Spec.ReleaseChannel = releaseChannel
	module.Spec.Update = update

	if repositoryName != repositoryNameEmbedded {
		delete(module.Annotations, v1beta1.ModuleAnnotationEmbedded)

		return
	}

	setModuleAnnotation(module, v1beta1.ModuleAnnotationEmbedded)
}

// setModuleAnnotation marks the key true, allocating the map when the module carries none.
func setModuleAnnotation(module *v1beta1.Module, key string) {
	if module.Annotations == nil {
		module.Annotations = make(map[string]string, 1)
	}

	module.Annotations[key] = "true"
}

// applyModuleConfig mirrors the module config into the module spec. The four fields belong to the
// config alone, so with no config they are cleared and no deleted setting survives.
func applyModuleConfig(module *v1beta1.Module, moduleConfig *v1alpha1.ModuleConfig) {
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

// deleteUnresolvableModules deletes the modules no pass above placed and that name no package: no
// repository and no version, left by the old module stack for a module never downloaded. Nothing
// can install one, and the schema rejects every write to it (packageVersion is required), so the
// Module controller would fail on it forever. The global and the embedded modules are never touched.
func (s *syncer) deleteUnresolvableModules(ctx context.Context) error {
	modules := new(v1beta1.ModuleList)
	if err := s.reader.List(ctx, modules); err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	for i := range modules.Items {
		module := &modules.Items[i]
		if module.Name == packageNameGlobal || module.IsEmbedded() ||
			module.Spec.PackageRepositoryName != "" || module.Spec.PackageVersion != "" {
			continue
		}

		if err := s.writer.Delete(ctx, module); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("delete the '%s' module: %w", module.Name, err)
		}

		s.logger.Info("module without a repository and a version deleted", slog.String("name", module.Name))
	}

	return nil
}

// dropLegacyConditions removes the conditions of the old module stack from all modules.
func (s *syncer) dropLegacyConditions(ctx context.Context) error {
	modules := new(v1beta1.ModuleList)
	if err := s.reader.List(ctx, modules); err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	for _, module := range modules.Items {
		if err := s.dropModuleLegacyConditions(ctx, &module); err != nil {
			return fmt.Errorf("drop legacy conditions for the '%s' module: %w", module.Name, err)
		}
	}

	return nil
}

// dropModuleLegacyConditions removes the conditions of the old module stack from a module the package
// runtime now owns. A module carrying none is not patched.
func (s *syncer) dropModuleLegacyConditions(ctx context.Context, module *v1beta1.Module) error {
	patch := client.MergeFrom(module.DeepCopy())

	for _, conditionType := range legacyModuleConditions {
		metautils.RemoveStatusCondition(&module.Status.Conditions, conditionType)
	}

	patchData, err := patch.Data(module)
	if err != nil {
		return fmt.Errorf("build the status patch for the '%s' module: %w", module.Name, err)
	}

	if string(patchData) == "{}" {
		return nil
	}

	if err = s.writer.Status().Patch(ctx, module, client.RawPatch(patch.Type(), patchData)); err != nil {
		return fmt.Errorf("patch the status of the '%s' module: %w", module.Name, err)
	}

	s.logger.Debug("legacy module conditions dropped", slog.String("name", module.Name))

	return nil
}
