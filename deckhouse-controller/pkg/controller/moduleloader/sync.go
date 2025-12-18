// Copyright 2024 Flant JSC
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

package moduleloader

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// if a module is disabled more than three days, it will be uninstalled
	deleteReleasesAfter = 72 * time.Hour

	deleteStaleModuleLoopInterval = 3 * time.Hour
)

func (l *Loader) runDeleteStaleModuleReleasesLoop(ctx context.Context) {
	_ = wait.PollUntilContextCancel(ctx, deleteStaleModuleLoopInterval, true, func(_ context.Context) (bool, error) {
		if err := l.deleteStaleModuleReleases(ctx); err != nil {
			l.logger.Warn("failed to delete stale modules", log.Err(err))
		}
		return false, nil
	})
}

// deleteStaleModuleReleases deletes module releases for modules that disabled too long
func (l *Loader) deleteStaleModuleReleases(ctx context.Context) error {
	modules := new(v1alpha1.ModuleList)
	if err := l.client.List(ctx, modules); err != nil {
		return fmt.Errorf("list all modules: %w", err)
	}

	for _, module := range modules.Items {
		// handle too long disabled modules
		if module.DisabledByModuleConfigMoreThan(deleteReleasesAfter) && !module.IsEmbedded() {
			// delete module releases of a stale module
			l.logger.Debug("the module disabled too long, delete module releases", slog.String("name", module.Name))
			moduleReleases := new(v1alpha1.ModuleReleaseList)
			if err := l.client.List(ctx, moduleReleases, &client.MatchingLabels{"module": module.Name}); err != nil {
				return fmt.Errorf("list module releases for the '%s' module: %w", module.Name, err)
			}

			for _, release := range moduleReleases.Items {
				if err := l.client.Delete(ctx, &release); err != nil {
					return fmt.Errorf("delete the '%s' module release for the '%s' module: %w", release.Name, module.Name, err)
				}
			}

			// clear module
			err := ctrlutils.UpdateWithRetry(ctx, l.client, &module, func() error {
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
			err = ctrlutils.UpdateStatusWithRetry(ctx, l.client, &module, func() error {
				module.Status.Phase = v1alpha1.ModulePhaseAvailable
				module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonNotInstalled, v1alpha1.ModuleMessageNotInstalled)
				return nil
			})
			if err != nil {
				return fmt.Errorf("set the Available module phase for the '%s' module: %w", module.Name, err)
			}
		}
	}

	return nil
}

// restoreModulesByOverrides checks ModulePullOverrides and restore them on the FS
func (l *Loader) restoreModulesByOverrides(ctx context.Context) error {
	mpos := new(v1alpha2.ModulePullOverrideList)
	if err := l.client.List(ctx, mpos); err != nil {
		return fmt.Errorf("list module pull overrides: %w", err)
	}

	for _, mpo := range mpos.Items {
		moduleName := mpo.GetModuleName()

		// ignore deleted mpo or unready mpo
		if !mpo.ObjectMeta.DeletionTimestamp.IsZero() || mpo.Status.Message != v1alpha1.ModulePullOverrideMessageReady {
			continue
		}

		module := new(v1alpha1.Module)
		if err := l.client.Get(ctx, client.ObjectKey{Name: mpo.Name}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				l.logger.Error("failed to get module", slog.String("name", mpo.Name), log.Err(err))
				return err
			}

			l.logger.Info("module not exist, skip restoring module pull override", slog.String("name", mpo.Name))
			continue
		}

		// skip embedded module
		if module.IsEmbedded() {
			l.logger.Info("module is embedded, skip restoring module pull override", slog.String("name", mpo.Name))
			continue
		}

		// module must be enabled
		if !module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionTrue) {
			l.logger.Info("module disabled, skip restoring module pull override process", slog.String("name", mpo.Name))
			continue
		}

		// source must be
		if module.Properties.Source == "" {
			l.logger.Info("module does not have an active source, skip restoring module pull override process", slog.String("name", mpo.Name))
			continue
		}

		err := utils.Update[*v1alpha1.Module](ctx, l.client, module, func(module *v1alpha1.Module) bool {
			module.Properties.Version = mpo.Spec.ImageTag
			return true
		})
		if err != nil {
			return fmt.Errorf("set the module version '%s': %w", module.Name, err)
		}

		// get relevant module source
		source := new(v1alpha1.ModuleSource)
		if err = l.client.Get(ctx, client.ObjectKey{Name: module.Properties.Source}, source); err != nil {
			return fmt.Errorf("get the module source '%s' for the module '%s': %w", module.Properties.Source, mpo.Name, err)
		}

		if err = l.installer.Restore(ctx, source, moduleName, mpo.Spec.ImageTag); err != nil {
			return fmt.Errorf("restore the module '%s': %w", moduleName, err)
		}

		l.registries[moduleName] = utils.BuildRegistryValue(source)
	}

	return nil
}

// restoreModulesByReleases checks ModuleReleases with Deployed status and restores them on the FS
func (l *Loader) restoreModulesByReleases(ctx context.Context) error {
	labelSelector := client.MatchingLabels{
		v1alpha1.ModuleReleaseLabelStatus: v1alpha1.ModuleReleaseLabelDeployed,
	}

	releaseList := new(v1alpha1.ModuleReleaseList)
	if err := l.client.List(ctx, releaseList, labelSelector); err != nil {
		return fmt.Errorf("list releases: %w", err)
	}

	// sort releases by version (to check previous deployed)
	releases := releaseList.Items
	slices.SortFunc(releases, func(a, b v1alpha1.ModuleRelease) int {
		return a.GetVersion().Compare(b.GetVersion())
	})

	deployedReleases := make(map[string]v1alpha1.ModuleRelease)
	for _, release := range releases {
		moduleName := release.GetModuleName()

		// ignore deleted release and not deployed
		if release.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed || !release.ObjectMeta.DeletionTimestamp.IsZero() {
			continue
		}

		// if we already have deployed release - make it superseded
		deployedRelease, ok := deployedReleases[moduleName]
		if ok {
			updatedDeployedRelease := deployedRelease.DeepCopy()
			updatedDeployedRelease.Status.Phase = v1alpha1.ModuleReleasePhaseSuperseded
			updatedDeployedRelease.Status.Message = ""
			updatedDeployedRelease.Status.TransitionTime = metav1.NewTime(l.dependencyContainer.GetClock().Now().UTC())

			if err := l.client.Status().Patch(ctx, updatedDeployedRelease, client.MergeFrom(&deployedRelease)); err != nil {
				l.logger.Error("patch previous deployed module release", slog.String("name", release.GetName()), log.Err(err))
			}
		}

		deployedReleases[moduleName] = release

		// if ModulePullOverride exists, don't check and restore overridden release
		exists, err := utils.ModulePullOverrideExists(ctx, l.client, moduleName)
		if err != nil {
			return fmt.Errorf("get module pull override for the '%s' module: %w", moduleName, err)
		}
		if exists {
			l.logger.Info("module is overridden, skip release restoring", slog.String("name", moduleName))
			continue
		}

		// update module version
		module := new(v1alpha1.Module)
		if err = l.client.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("get the module '%s': %w", moduleName, err)
			}
			l.logger.Warn("module is missing, skip setting version", slog.String("name", release.Spec.ModuleName))
		} else {
			l.logger.Debug("set module version", slog.String("name", moduleName), slog.String("version", release.GetModuleVersion()))
			err = ctrlutils.UpdateWithRetry(ctx, l.client, module, func() error {
				module.Properties.Version = release.GetModuleVersion()
				return nil
			})
			if err != nil {
				return fmt.Errorf("update the module '%s': %w", moduleName, err)
			}
		}

		// get relevant module source
		source := new(v1alpha1.ModuleSource)
		if err = l.client.Get(ctx, client.ObjectKey{Name: release.GetModuleSource()}, source); err != nil {
			return fmt.Errorf("get the module source '%s' for the module '%s': %w", source.Name, moduleName, err)
		}

		if err = l.installer.Restore(ctx, source, moduleName, release.GetModuleVersion()); err != nil {
			return fmt.Errorf("restore the module '%s': %w", moduleName, err)
		}

		l.registries[moduleName] = utils.BuildRegistryValue(source)
	}

	return nil
}

// deleteOrphanModules deletes modules without release and mpo
func (l *Loader) deleteOrphanModules(ctx context.Context) error {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := l.client.List(ctx, releases); err != nil {
		return fmt.Errorf("list releases: %w", err)
	}

	downloaded, err := l.installer.GetDownloaded()
	if err != nil {
		return fmt.Errorf("get downloaded modules: %w", err)
	}

	l.logger.Debug("found downloaded modules", slog.Any("downloaded", downloaded))

	// remove modules with release
	for _, release := range releases.Items {
		delete(downloaded, release.GetModuleName())
	}

	for module := range downloaded {
		mpo := new(v1alpha2.ModulePullOverride)
		if err = l.client.Get(ctx, client.ObjectKey{Name: module}, mpo); err == nil || !apierrors.IsNotFound(err) {
			continue
		}

		l.logger.Debug("uninstall orphan module", slog.String("module", module))
		if err = l.installer.Uninstall(ctx, module); err != nil {
			return fmt.Errorf("uninstall the module '%s': %w", module, err)
		}
	}

	return nil
}
