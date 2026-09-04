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
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/pkgsync"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
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

// deleteStaleModuleReleases deletes the module releases of the modules disabled too long. The
// release controller uninstalls the deployed one and restarts Deckhouse, and the package sync
// then drops the module object nothing backs any more.
func (l *Loader) deleteStaleModuleReleases(ctx context.Context) error {
	modules := new(v1alpha2.ModuleList)
	if err := l.client.List(ctx, modules); err != nil {
		return fmt.Errorf("list all modules: %w", err)
	}

	for _, module := range modules.Items {
		if !module.DisabledByModuleConfigMoreThan(deleteReleasesAfter) || module.IsEmbedded() {
			continue
		}

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

		module := new(v1alpha2.Module)
		if err := l.client.Get(ctx, client.ObjectKey{Name: mpo.Name}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				l.logger.Error("failed to get module", slog.String("name", mpo.Name), log.Err(err))
				return fmt.Errorf("get: %w", err)
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
		if !module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, metav1.ConditionTrue) {
			l.logger.Info("module disabled, skip restoring module pull override process", slog.String("name", mpo.Name))
			continue
		}

		// the package sync resolved the repository the override pulls from, which names the source
		moduleSourceName := pkgsync.SourceNameForRepository(module.Spec.PackageRepositoryName)
		if moduleSourceName == "" {
			l.logger.Info("module does not have an active source, skip restoring module pull override process", slog.String("name", mpo.Name))
			continue
		}

		currentNode := app.NodeName()
		if len(currentNode) == 0 {
			return errors.New("determine the node name deckhouse pod is running on: missing or empty DECKHOUSE_NODE_NAME env")
		}

		// if deployedOn annotation value doesn't equal to current node name - overwrite the module from the repository
		if deployedOn := mpo.GetAnnotations()[v1alpha1.ModulePullOverrideAnnotationDeployedOn]; deployedOn != currentNode {
			l.logger.Info("reinitialize module pull override due to stale deployedOn annotation", slog.String("name", mpo.Name))
			if err := l.installer.Uninstall(ctx, moduleName); err != nil {
				return fmt.Errorf("uninstall module pull override: %w", err)
			}

			if len(mpo.ObjectMeta.Annotations) == 0 {
				mpo.ObjectMeta.Annotations = make(map[string]string)
			}
			mpo.ObjectMeta.Annotations[v1alpha1.ModulePullOverrideAnnotationDeployedOn] = currentNode

			if err := l.client.Update(ctx, &mpo); err != nil {
				l.logger.Warn("failed to annotate module pull override", slog.String("name", mpo.Name), log.Err(err))
			}
		}

		// get relevant module source
		source := new(v1alpha1.ModuleSource)
		if err := l.client.Get(ctx, client.ObjectKey{Name: moduleSourceName}, source); err != nil {
			return fmt.Errorf("get the module source '%s' for the module '%s': %w", moduleSourceName, mpo.Name, err)
		}

		if err := l.installer.Restore(ctx, source, moduleName, mpo.Spec.ImageTag); err != nil {
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

		// get relevant module source
		source := new(v1alpha1.ModuleSource)
		if err = l.client.Get(ctx, client.ObjectKey{Name: release.GetModuleSource()}, source); err != nil {
			return fmt.Errorf("get the module source '%s' for the module '%s': %w", source.Name, moduleName, err)
		}

		// While the embedded copy of the module is still shipped on the filesystem
		// it wins the module search path, so a downloaded module of the same name
		// must only be staged (no symlink/mount), not activated. Once the embedded
		// copy is dropped on Deckhouse upgrade, this restore activates the staged
		// module instead.
		if l.installer.IsEmbeddedPresent(moduleName) {
			l.logger.Info("module is still embedded, stage the release without activating it", slog.String("name", moduleName))
			if err = l.installer.StageFromRegistry(ctx, source, moduleName, release.GetModuleVersion()); err != nil {
				return fmt.Errorf("stage the module '%s': %w", moduleName, err)
			}

			// The embedded copy is still serving the module, so it must keep rendering
			// images from the embedded registry (digests baked into the Deckhouse image).
			// Do NOT inject the source registry here: that would make the embedded module
			// pull <sourceRepo>/modules/<name>@<embeddedDigest>, a path that does not exist
			// (the digest belongs to the embedded image, not the source's module image),
			// breaking the module with ImagePullBackOff. The source registry is injected
			// only once the embedded copy is dropped and the module is activated (below).
			continue
		}

		// The embedded copy is gone (otherwise it would have been staged above), so the module is
		// served from the downloaded source; the package sync placed it by this release.
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

	installed, err := l.installer.GetInstalled()
	if err != nil {
		return fmt.Errorf("get installed modules: %w", err)
	}

	l.logger.Debug("found installed modules", slog.Any("installed", installed))

	// exclude modules with release
	for _, release := range releases.Items {
		delete(installed, release.GetModuleName())
	}

	for module := range installed {
		mpo := new(v1alpha2.ModulePullOverride)
		err = l.client.Get(ctx, client.ObjectKey{Name: module}, mpo)
		if err == nil {
			// MPO exists - module is managed, don't delete
			continue
		}

		if !apierrors.IsNotFound(err) {
			l.logger.Warn("get module pull override", slog.String("name", module), log.Err(err))
			continue
		}

		l.logger.Debug("uninstall orphan module", slog.String("module", module))
		if err = l.installer.Uninstall(ctx, module); err != nil {
			return fmt.Errorf("uninstall the module '%s': %w", module, err)
		}
	}

	return nil
}
