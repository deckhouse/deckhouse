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

package controller

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pkgmodules "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	pkgruntime "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// dummyModules are modules that should be skipped.
var dummyModules = []string{
	"000-common",
	"007-registrypackages",
}

// syncModulesSettings mirrors every module config onto the module of the same name, so a
// module carries its settings before loadModulesV2 hands it to the runtime. It does the
// same work the module config controller does per event, for the configs that predate it.
func (c *Controller) syncModulesSettings(ctx context.Context) error {
	cli := c.ctrl.GetClient()

	configs := new(v1alpha1.ModuleConfigList)
	if err := cli.List(ctx, configs); err != nil {
		return fmt.Errorf("list module configs: %w", err)
	}

	for _, conf := range configs.Items {
		// a config on its way out is the config controller's business, not the bootstrap's
		if !conf.DeletionTimestamp.IsZero() {
			continue
		}

		module := new(v1alpha2.Module)
		if err := cli.Get(ctx, client.ObjectKey{Name: conf.Name}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("get module '%s': %w", conf.Name, err)
			}

			c.logger.Debug("module not found, skip settings sync", slog.String("name", conf.Name))

			continue
		}

		patch := client.MergeFrom(module.DeepCopy())
		module.Spec.Settings = conf.Spec.Settings
		module.Spec.SettingsVersion = conf.Spec.Version
		module.Spec.Maintenance = conf.Spec.Maintenance
		module.Spec.Enabled = conf.Spec.Enabled

		if err := cli.Patch(ctx, module, patch); err != nil {
			c.logger.Error("failed to patch the module", slog.String("name", conf.Name), log.Err(err))
			return fmt.Errorf("patch module '%s': %w", conf.Name, err)
		}
	}

	return nil
}

// restoreModulesV2ByOverrides places a v1alpha2 module for every ready pull override, so an
// overridden module keeps the version the override pinned rather than the released one.
// It runs before restoreModulesV2ByReleases, which then leaves those modules alone.
func (c *Controller) restoreModulesV2ByOverrides(ctx context.Context) error {
	cli := c.ctrl.GetClient()

	overrides := new(v1alpha2.ModulePullOverrideList)
	if err := cli.List(ctx, overrides); err != nil {
		return fmt.Errorf("list module overrides: %w", err)
	}

	for _, mpo := range overrides.Items {
		// ignore deleted mpo or unready mpo
		if !mpo.DeletionTimestamp.IsZero() || mpo.Status.Message != v1alpha1.ModulePullOverrideMessageReady {
			continue
		}

		// the v1alpha1 module is read only for its source, which the override does not carry
		module := new(v1alpha1.Module)
		if err := cli.Get(ctx, client.ObjectKey{Name: mpo.Name}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				c.logger.Error("failed to get the module", slog.String("name", mpo.Name), log.Err(err))
				return fmt.Errorf("get module '%s': %w", mpo.Name, err)
			}

			c.logger.Info("module not exist, skip restoring module pull override", slog.String("name", mpo.Name))

			continue
		}

		if err := c.ensureModuleV2(ctx, mpo.Name, module.Properties.Source, mpo.Spec.ImageTag, true); err != nil {
			return fmt.Errorf("restore module '%s' by override: %w", mpo.Name, err)
		}
	}

	return nil
}

// restoreModulesV2ByReleases places a v1alpha2 module for every deployed release, so a module
// installed before the restart is tracked again.
func (c *Controller) restoreModulesV2ByReleases(ctx context.Context) error {
	deployed, err := c.resolveDeployedReleases(ctx)
	if err != nil {
		return fmt.Errorf("resolve deployed releases: %w", err)
	}

	for name, release := range deployed {
		// The override, not the release, decides an overridden module's version, and
		// restoreModulesV2ByOverrides has already placed that module on it.
		overridden, err := utils.ModulePullOverrideExists(ctx, c.ctrl.GetClient(), name)
		if err != nil {
			return fmt.Errorf("get module pull override for the '%s' module: %w", name, err)
		}

		if overridden {
			c.logger.Info("module is overridden, skip release restoring", slog.String("name", name))
			continue
		}

		if err := c.ensureModuleV2(ctx, name, release.GetModuleSource(), release.GetModuleVersion(), false); err != nil {
			return fmt.Errorf("restore module '%s' by release: %w", name, err)
		}
	}

	return nil
}

// resolveDeployedReleases returns the newest deployed release per module, superseding the
// older duplicates it passes on the way — two releases both marked deployed is the state a
// restart in the middle of a version bump leaves behind.
func (c *Controller) resolveDeployedReleases(ctx context.Context) (map[string]v1alpha1.ModuleRelease, error) {
	selector := client.MatchingLabels{
		v1alpha1.ModuleReleaseLabelStatus: v1alpha1.ModuleReleaseLabelDeployed,
	}

	releaseList := new(v1alpha1.ModuleReleaseList)
	if err := c.ctrl.GetClient().List(ctx, releaseList, selector); err != nil {
		return nil, fmt.Errorf("list module releases: %w", err)
	}

	// sort releases by version, so the one seen later always supersedes the one held
	releases := releaseList.Items
	slices.SortFunc(releases, func(a, b v1alpha1.ModuleRelease) int {
		return a.GetVersion().Compare(b.GetVersion())
	})

	deployed := make(map[string]v1alpha1.ModuleRelease)
	for _, release := range releases {
		// ignore deleted release and not deployed
		if release.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed || !release.DeletionTimestamp.IsZero() {
			continue
		}

		name := release.GetModuleName()

		// superseding is hygiene on its own: it runs even for modules left out below
		if previous, ok := deployed[name]; ok {
			if err := c.supersedeRelease(ctx, &previous); err != nil {
				c.logger.Error("failed to supersede the previous deployed module release",
					slog.String("name", previous.GetName()), log.Err(err))
			}
		}

		deployed[name] = release
	}

	return deployed, nil
}

// supersedeRelease marks a deployed release that a newer deployed one replaced.
func (c *Controller) supersedeRelease(ctx context.Context, release *v1alpha1.ModuleRelease) error {
	superseded := release.DeepCopy()
	superseded.Status.Phase = v1alpha1.ModuleReleasePhaseSuperseded
	superseded.Status.Message = ""
	superseded.Status.TransitionTime = metav1.NewTime(c.dc.GetClock().Now().UTC())

	if err := c.ctrl.GetClient().Status().Patch(ctx, superseded, client.MergeFrom(release)); err != nil {
		return fmt.Errorf("patch module release status: %w", err)
	}

	return nil
}

// ensureModuleV2 places the module on repository and version, whether or not it already
// exists. The restore runs on every start, so it must not trip over what the last one left.
func (c *Controller) ensureModuleV2(ctx context.Context, name, repository, version string, dev bool) error {
	cli := c.ctrl.GetClient()

	module := new(v1alpha2.Module)
	if err := cli.Get(ctx, client.ObjectKey{Name: name}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get module: %w", err)
		}

		module = &v1alpha2.Module{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: v1alpha2.ModuleSpec{
				PackageRepositoryName: repository,
				PackageVersion:        version,
			},
		}
		if dev {
			module.Annotations = map[string]string{
				v1alpha2.ModuleAnnotationDev: "true",
			}
		}

		// AlreadyExists means only that the informer cache had not caught up; the module
		// is placed either way, and the next start moves it if it drifted.
		if err := cli.Create(ctx, module); err != nil && !apierrors.IsAlreadyExists(err) {
			c.logger.Error("failed to create the module", slog.String("name", name), log.Err(err))
			return fmt.Errorf("create module: %w", err)
		}

		return nil
	}

	devAnnotationSet := module.Annotations[v1alpha2.ModuleAnnotationDev] == "true"
	if module.Spec.PackageRepositoryName == repository && module.Spec.PackageVersion == version && (!dev || devAnnotationSet) {
		return nil
	}

	patch := client.MergeFrom(module.DeepCopy())
	module.Spec.PackageRepositoryName = repository
	module.Spec.PackageVersion = version
	if dev {
		if module.Annotations == nil {
			module.Annotations = make(map[string]string)
		}

		module.Annotations[v1alpha2.ModuleAnnotationDev] = "true"
	}

	if err := cli.Patch(ctx, module, patch); err != nil {
		c.logger.Error("failed to patch the module", slog.String("name", name), log.Err(err))
		return fmt.Errorf("patch module: %w", err)
	}

	c.logger.Debug("module moved onto the restored version",
		slog.String("name", name), slog.String("version", version))

	return nil
}

// deleteUnplacedModules drops the modules the restore left behind. Module is one resource with
// two versions and a None conversion strategy, so the previous generation cannot be selected by
// listing v1alpha1 — that returns the same objects the restore just placed. What separates them
// is the package spec: a module carries a version only if the package system installed it, and
// nothing manages one that does not now that addon-operator is gone.
// Runs after both restore steps, so their modules already carry a version by this point.
func (c *Controller) deleteUnplacedModules(ctx context.Context) error {
	cli := c.ctrl.GetClient()

	modules := new(v1alpha2.ModuleList)
	if err := cli.List(ctx, modules); err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	for i := range modules.Items {
		module := &modules.Items[i]

		if module.Spec.PackageVersion != "" {
			continue
		}

		c.logger.Info("module is not backed by a package, delete it", slog.String("name", module.Name))

		// a module already gone is the outcome asked for
		if err := cli.Delete(ctx, module); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete module '%s': %w", module.Name, err)
		}
	}

	return nil
}

// loadModulesV2 hands every placed module to the package runtime, which starts the
// deploy-and-load pipeline for the modules the restore just recreated.
func (c *Controller) loadModulesV2(ctx context.Context) error {
	cli := c.ctrl.GetClient()

	modules := new(v1alpha2.ModuleList)
	if err := cli.List(ctx, modules); err != nil {
		return fmt.Errorf("list modules: %w", err)
	}

	for _, module := range modules.Items {
		if !module.DeletionTimestamp.IsZero() {
			c.logger.Debug("module is deleted, skip loading", slog.String("module", module.Name))
			continue
		}

		if module.Spec.PackageRepositoryName == "" {
			c.logger.Debug("module has no repository, skip loading", slog.String("module", module.Name))
			continue
		}

		repo := new(v1alpha1.PackageRepository)
		if err := cli.Get(ctx, client.ObjectKey{Name: module.Spec.PackageRepositoryName}, repo); err != nil {
			return fmt.Errorf("get package repository '%s' of the module '%s': %w",
				module.Spec.PackageRepositoryName, module.Name, err)
		}

		c.manager.UpdateModule(registry.BuildRemote(repo), pkgruntime.Module{
			Name: module.Name,
			Definition: pkgmodules.Definition{
				Name:    module.Name,
				Version: module.Spec.PackageVersion,
			},
			Settings:        module.Spec.Settings.GetMap(),
			SettingsVersion: module.Spec.SettingsVersion,
			Maintenance:     module.Spec.Maintenance,
			Enabled:         module.Spec.Enabled,
		}, false)
	}

	return nil
}
