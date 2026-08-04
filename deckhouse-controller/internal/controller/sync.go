// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
	"maps"
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// deleteReleasesAfter is how long a module may stay disabled before it is uninstalled.
	deleteReleasesAfter = 72 * time.Hour

	// deleteStaleModuleReleasesInterval is how often stale modules are looked for.
	deleteStaleModuleReleasesInterval = 3 * time.Hour
)

// loadInitialConfiguration seeds the runtime with the ModuleConfig state already
// present in the cluster, before the module loader starts syncing packages. At
// this point no package is tracked yet, so UpdateModulesSettings records only the
// enabled/disabled intent (it lives in the global module and is read by the
// scheduler's config rule the moment a package registers); the per-package
// settings are dropped here and supplied later by the loader via UpdateModule.
func (c *Controller) loadInitialConfiguration(ctx context.Context) error {
	configs := new(v1alpha1.ModuleConfigList)
	if err := c.ctrl.GetClient().List(ctx, configs); err != nil {
		return fmt.Errorf("list module configs: %w", err)
	}

	for _, conf := range configs.Items {
		if conf.DeletionTimestamp != nil {
			continue
		}

		c.runtime.UpdateModulesSettings(conf.Name, conf.Spec.Version, conf.Spec.Settings.GetMap(), conf.Spec.Maintenance, conf.Spec.Enabled)
	}

	return nil
}

// restoreOverrides checks ModulePullOverrides and restore them on the FS
func (c *Controller) restoreOverrides(ctx context.Context) error {
	cli := c.ctrl.GetClient()

	mpos := new(v1alpha2.ModulePullOverrideList)
	if err := cli.List(ctx, mpos); err != nil {
		return fmt.Errorf("list module pull overrides: %w", err)
	}

	for _, mpo := range mpos.Items {
		// ignore deleted mpo or unready mpo
		if !mpo.ObjectMeta.DeletionTimestamp.IsZero() || mpo.Status.Message != v1alpha1.ModulePullOverrideMessageReady {
			continue
		}

		module := new(v1alpha1.Module)
		if err := cli.Get(ctx, client.ObjectKey{Name: mpo.Name}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("get the %q module: %w", mpo.Name, err)
			}

			c.logger.Info("module not exist, skip restoring module pull override", slog.String("name", mpo.Name))
			continue
		}

		// skip embedded module
		if module.IsEmbedded() {
			c.logger.Info("module is embedded, skip restoring module pull override", slog.String("name", mpo.Name))
			continue
		}

		// source must be
		if module.Properties.Source == "" {
			c.logger.Info("module does not have an active source, skip restoring module pull override process", slog.String("name", mpo.Name))
			continue
		}

		err := utils.Update[*v1alpha1.Module](ctx, cli, module, func(module *v1alpha1.Module) bool {
			module.Properties.Version = mpo.Spec.ImageTag
			return true
		})
		if err != nil {
			return fmt.Errorf("set the %q module version: %w", module.Name, err)
		}

		// get relevant module source
		source := new(v1alpha1.ModuleSource)
		if err = cli.Get(ctx, client.ObjectKey{Name: module.Properties.Source}, source); err != nil {
			return fmt.Errorf("get the %q module source for the %q module: %w", module.Properties.Source, mpo.Name, err)
		}

		// Not forced: this restores the image the override already reported Ready, so a
		// cached copy of that tag is the right one. If the tag moved while the controller
		// was down, the override's own reconcile compares digests and forces the update.
		c.runtime.UpdateModule(registry.BuildRemote(source), runtime.Module{
			Name: mpo.Name,
			Definition: modules.Definition{
				Version: mpo.Spec.ImageTag,
			},
		}, false)
	}

	return nil
}

// restoreReleases checks ModuleReleases with Deployed status and restores them on the FS
func (c *Controller) restoreReleases(ctx context.Context) error {
	releases, err := c.getDeployedReleases(ctx)
	if err != nil {
		return err
	}

	for _, release := range releases {
		moduleName := release.GetModuleName()

		// get relevant module source
		source := new(v1alpha1.ModuleSource)
		if err = c.ctrl.GetClient().Get(ctx, client.ObjectKey{Name: release.GetModuleSource()}, source); err != nil {
			return fmt.Errorf("get the %q module source for the %q module: %w", release.GetModuleSource(), moduleName, err)
		}

		// Releases carry immutable version tags, so a cached copy is never stale.
		c.runtime.UpdateModule(registry.BuildRemote(source), runtime.Module{
			Name: moduleName,
			Definition: modules.Definition{
				Version: release.GetModuleVersion(),
			},
		}, false)
	}

	return nil
}

// getDeployedReleases returns the latest deployed release of every module that is not
// currently overridden by a ModulePullOverride, superseding the older deployed releases
// it finds along the way.
func (c *Controller) getDeployedReleases(ctx context.Context) ([]v1alpha1.ModuleRelease, error) {
	labelSelector := client.MatchingLabels{
		v1alpha1.ModuleReleaseLabelStatus: v1alpha1.ModuleReleaseLabelDeployed,
	}

	releaseList := new(v1alpha1.ModuleReleaseList)
	if err := c.ctrl.GetClient().List(ctx, releaseList, labelSelector); err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}

	// sort releases by version (to check previous deployed)
	releases := releaseList.Items
	slices.SortFunc(releases, func(a, b v1alpha1.ModuleRelease) int {
		return a.GetVersion().Compare(b.GetVersion())
	})

	deployed := make(map[string]v1alpha1.ModuleRelease)
	for _, release := range releases {
		// ignore deleted release and not deployed
		if release.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed || !release.ObjectMeta.DeletionTimestamp.IsZero() {
			continue
		}

		moduleName := release.GetModuleName()

		// if ModulePullOverride exists, don't check and restore overridden release
		exists, err := utils.ModulePullOverrideExists(ctx, c.ctrl.GetClient(), moduleName)
		if err != nil {
			return nil, fmt.Errorf("get module pull override for the %q module: %w", moduleName, err)
		}
		if exists {
			c.logger.Info("module is overridden, skip release restoring", slog.String("name", moduleName))
			continue
		}

		// if we already have deployed release - make it superseded
		if deployedRelease, ok := deployed[moduleName]; ok {
			updatedDeployedRelease := deployedRelease.DeepCopy()
			updatedDeployedRelease.Status.Phase = v1alpha1.ModuleReleasePhaseSuperseded
			updatedDeployedRelease.Status.Message = ""
			updatedDeployedRelease.Status.TransitionTime = metav1.NewTime(c.dc.GetClock().Now().UTC())

			if err := c.ctrl.GetClient().Status().Patch(ctx, updatedDeployedRelease, client.MergeFrom(&deployedRelease)); err != nil {
				c.logger.Error("patch previous deployed module release", slog.String("name", deployedRelease.GetName()), log.Err(err))
			}
		}

		deployed[moduleName] = release
	}

	return slices.Collect(maps.Values(deployed)), nil
}

// runDeleteStaleModuleReleasesLoop periodically deletes the releases of the modules
// that have been disabled for too long. It blocks until ctx is cancelled.
func (c *Controller) runDeleteStaleModuleReleasesLoop(ctx context.Context) {
	_ = wait.PollUntilContextCancel(ctx, deleteStaleModuleReleasesInterval, true, func(_ context.Context) (bool, error) {
		if err := c.deleteStaleModuleReleases(ctx); err != nil {
			c.logger.Warn("failed to delete stale modules", log.Err(err))
		}
		return false, nil
	})
}

// deleteStaleModuleReleases deletes module releases for modules that disabled too long
func (c *Controller) deleteStaleModuleReleases(ctx context.Context) error {
	cli := c.ctrl.GetClient()

	moduleList := new(v1alpha1.ModuleList)
	if err := cli.List(ctx, moduleList); err != nil {
		return fmt.Errorf("list all modules: %w", err)
	}

	for _, module := range moduleList.Items {
		// handle too long disabled modules only
		if !module.DisabledByModuleConfigMoreThan(deleteReleasesAfter) || module.IsEmbedded() {
			continue
		}

		// delete module releases of a stale module
		c.logger.Debug("the module disabled too long, delete module releases", slog.String("name", module.Name))
		moduleReleases := new(v1alpha1.ModuleReleaseList)
		if err := cli.List(ctx, moduleReleases, &client.MatchingLabels{"module": module.Name}); err != nil {
			return fmt.Errorf("list module releases for the %q module: %w", module.Name, err)
		}

		for _, release := range moduleReleases.Items {
			if err := cli.Delete(ctx, &release); err != nil {
				return fmt.Errorf("delete the %q module release for the %q module: %w", release.Name, module.Name, err)
			}
		}

		// clear module
		err := ctrlutils.UpdateWithRetry(ctx, cli, &module, func() error {
			availableSources := module.Properties.AvailableSources
			module.Properties = v1alpha1.ModuleProperties{
				AvailableSources: availableSources,
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("clear the %q module: %w", module.Name, err)
		}

		// set available and skip
		err = ctrlutils.UpdateStatusWithRetry(ctx, cli, &module, func() error {
			module.Status.Phase = v1alpha1.ModulePhaseAvailable
			module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonNotInstalled, v1alpha1.ModuleMessageNotInstalled)
			return nil
		})
		if err != nil {
			return fmt.Errorf("set the Available module phase for the %q module: %w", module.Name, err)
		}
	}

	return nil
}
