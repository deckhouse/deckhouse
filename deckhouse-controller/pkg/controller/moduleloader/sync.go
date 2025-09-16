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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
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

// restoreAbsentModulesFromOverrides checks ModulePullOverrides and restore modules on the FS
func (l *Loader) restoreAbsentModulesFromOverrides(ctx context.Context) error {
	currentNodeName := os.Getenv("DECKHOUSE_NODE_NAME")
	if len(currentNodeName) == 0 {
		return errors.New("determine the node name deckhouse pod is running on: missing or empty DECKHOUSE_NODE_NAME env")
	}

	mpos := new(v1alpha2.ModulePullOverrideList)
	if err := l.client.List(ctx, mpos); err != nil {
		return fmt.Errorf("list module pull overrides: %w", err)
	}

	for _, mpo := range mpos.Items {
		// ignore deleted mpo
		if !mpo.ObjectMeta.DeletionTimestamp.IsZero() {
			continue
		}

		// skip unready mpo
		if mpo.Status.Message != v1alpha1.ModulePullOverrideMessageReady {
			continue
		}

		module := new(v1alpha1.Module)
		if err := l.client.Get(ctx, client.ObjectKey{Name: mpo.Name}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				l.logger.Error("failed to get module", slog.String("name", mpo.Name), log.Err(err))
				return err
			}
			l.logger.Info("the module does not exist, skip restoring module pull override process", slog.String("name", mpo.Name))
			continue
		}

		// skip embedded module
		if module.IsEmbedded() {
			l.logger.Info("the module is embedded, skip restoring module pull override process", slog.String("name", mpo.Name))
			continue
		}

		// module must be enabled
		if !module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionTrue) {
			l.logger.Info("module disabled, skip restoring module pull override process", slog.String("name", mpo.Name))
			continue
		}

		// source must be
		if module.Properties.Source == "" {
			l.logger.Info("the module does have an active source, skip restoring module pull override process", slog.String("name", mpo.Name))
			continue
		}

		err := utils.Update[*v1alpha1.Module](ctx, l.client, module, func(module *v1alpha1.Module) bool {
			module.Properties.Version = mpo.Spec.ImageTag
			return true
		})
		if err != nil {
			return fmt.Errorf("set the '%s' module version: %w", module.Name, err)
		}

		// get relevant module source
		source := new(v1alpha1.ModuleSource)
		if err = l.client.Get(ctx, client.ObjectKey{Name: module.Properties.Source}, source); err != nil {
			return fmt.Errorf("get the '%s' module source for the '%s' module: %w", module.Properties.Source, mpo.Name, err)
		}

		// mpo's status.weight field isn't set - get it from the module's definition
		if mpo.Status.Weight == 0 {
			opts := utils.GenerateRegistryOptionsFromModuleSource(source, l.clusterUUID, l.logger)
			md := downloader.NewModuleDownloader(l.dependencyContainer, l.downloadedModulesDir, source, l.logger.Named("downloader"), opts)

			imageInfo, err := md.DownloadReleaseImageInfoByVersion(ctx, mpo.Name, mpo.Spec.ImageTag)
			if err != nil {
				return fmt.Errorf("get the '%s' module definition from repository: %w", mpo.Name, err)
			}

			mpo.Status.UpdatedAt = metav1.NewTime(l.dependencyContainer.GetClock().Now().UTC())
			mpo.Status.Weight = imageInfo.ModuleDefinition.Weight
			// we don`t need to be bothered - even if the update fails, the weight will be set one way or another
			_ = l.client.Status().Update(ctx, &mpo)
		}

		// if deployedOn annotation isn't set or its value doesn't equal to current node name - overwrite the module from the repository
		if deployedOn, set := mpo.GetAnnotations()[v1alpha1.ModulePullOverrideAnnotationDeployedOn]; !set || deployedOn != currentNodeName {
			l.logger.Info("reinitialize module pull override due to stale/absent deployedOn annotation", slog.String("name", mpo.Name))
			if err = os.RemoveAll(filepath.Join(l.downloadedModulesDir, mpo.Name, downloader.DefaultDevVersion)); err != nil {
				return fmt.Errorf("delete the stale directory of the '%s' module: %w", mpo.Name, err)
			}

			if len(mpo.ObjectMeta.Annotations) == 0 {
				mpo.ObjectMeta.Annotations = make(map[string]string)
			}
			mpo.ObjectMeta.Annotations[v1alpha1.ModulePullOverrideAnnotationDeployedOn] = currentNodeName

			if err = l.client.Update(ctx, &mpo); err != nil {
				l.logger.Warn("failed to annotate module pull override", slog.String("name", mpo.Name), log.Err(err))
			}
		}

		// if annotation is ok - we have to check that the file system is in sync
		moduleSymLink := filepath.Join(l.symlinksDir, fmt.Sprintf("%d-%s", mpo.Status.Weight, mpo.Name))
		if _, err = os.Stat(moduleSymLink); err != nil {
			// module symlink not found
			if !os.IsNotExist(err) {
				return fmt.Errorf("check the '%s' module symlink: %w", mpo.Name, err)
			}
			l.logger.Info("module symlink is absent on file system, restore it", slog.String("name", mpo.Name))
			if err := l.createModuleSymlink(ctx, mpo.Name, mpo.Spec.ImageTag, source, mpo.Status.Weight, true); err != nil {
				return fmt.Errorf("create the '%s' module symlink: %w", mpo.Name, err)
			}
		} else {
			downloadedModulePath, err := filepath.EvalSymlinks(moduleSymLink)
			if err != nil {
				return fmt.Errorf("evaluate the '%s' module symlink %s: %w", mpo.Name, moduleSymLink, err)
			}

			// check if module symlink leads to current version
			if filepath.Base(downloadedModulePath) != downloader.DefaultDevVersion {
				l.logger.Info("module symlink is incorrect, restore it", slog.String("name", mpo.Name))
				if err := l.createModuleSymlink(ctx, mpo.Name, mpo.Spec.ImageTag, source, mpo.Status.Weight, true); err != nil {
					return fmt.Errorf("create the '%s' module symlink: %w", mpo.Name, err)
				}
			}
		}

		l.registries[mpo.GetModuleName()] = utils.BuildRegistryValue(source)
	}

	return nil
}

// restoreAbsentModulesFromReleases checks ModuleReleases with Deployed status and restore them on the FS
func (l *Loader) restoreAbsentModulesFromReleases(ctx context.Context) error {
	releaseList := new(v1alpha1.ModuleReleaseList)
	if err := l.client.List(
		ctx,
		releaseList,
		client.MatchingLabels{
			v1alpha1.ModuleReleaseLabelStatus: v1alpha1.ModuleReleaseLabelDeployed,
		},
	); err != nil {
		return fmt.Errorf("list releases: %w", err)
	}

	// sorting releases by version (to check previous deployed)
	releases := releaseList.Items
	slices.SortFunc(releases, func(a, b v1alpha1.ModuleRelease) int {
		return a.GetVersion().Compare(b.GetVersion())
	})

	deployedReleases := make(map[string]v1alpha1.ModuleRelease)

	// TODO: add labels to list only Deployed releases
	for _, release := range releases {
		// ignore deleted release and not deployed
		if release.Status.Phase != v1alpha1.ModuleReleasePhaseDeployed || !release.ObjectMeta.DeletionTimestamp.IsZero() {
			continue
		}

		// if we already have deployed release - make it superseded
		deployedRelease, ok := deployedReleases[release.Spec.ModuleName]
		if ok {
			updatedDeployedRelease := deployedRelease.DeepCopy()
			updatedDeployedRelease.Status.Phase = v1alpha1.ModuleReleasePhaseSuperseded
			updatedDeployedRelease.Status.Message = ""
			updatedDeployedRelease.Status.TransitionTime = metav1.NewTime(l.dependencyContainer.GetClock().Now().UTC())

			if err := l.client.Status().Patch(ctx, updatedDeployedRelease, client.MergeFrom(&deployedRelease)); err != nil {
				l.logger.Error("patch previous deployed module release", slog.String("name", release.GetName()), log.Err(err))
			}
		}

		deployedReleases[release.Spec.ModuleName] = release

		// if ModulePullOverride exists, don't check and restore overridden release
		exists, err := utils.ModulePullOverrideExists(ctx, l.client, release.Spec.ModuleName)
		if err != nil {
			return fmt.Errorf("get module pull override for the '%s' module: %w", release.Spec.ModuleName, err)
		}
		if exists {
			l.logger.Info("module is overridden, skip release restoring", slog.String("name", release.Spec.ModuleName))
			continue
		}

		// update module version
		module := new(v1alpha1.Module)
		if err = l.client.Get(ctx, client.ObjectKey{Name: release.GetModuleName()}, module); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("get '%s' module: %w", release.Spec.ModuleName, err)
			}
			l.logger.Warn("module is missing, skip setting version", slog.String("name", release.Spec.ModuleName))
		} else {
			l.logger.Debug("set module version", slog.String("name", release.GetModuleName()), slog.String("version", release.GetModuleVersion()))

			err = ctrlutils.UpdateWithRetry(ctx, l.client, module, func() error {
				module.Properties.Version = release.GetModuleVersion()
				return nil
			})
			if err != nil {
				return fmt.Errorf("update the '%s' module: %w", release.GetModuleName(), err)
			}
		}

		// get relevant module source
		source := new(v1alpha1.ModuleSource)
		if err = l.client.Get(ctx, client.ObjectKey{Name: release.GetModuleSource()}, source); err != nil {
			return fmt.Errorf("get the '%s' module source for the '%s' module: %w", source.Name, release.Spec.ModuleName, err)
		}

		moduleSymlink := filepath.Join(l.symlinksDir, fmt.Sprintf("%d-%s", release.Spec.Weight, release.Spec.ModuleName))
		if _, err = os.Stat(moduleSymlink); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("check the '%s' module symlink: %w", release.Spec.ModuleName, err)
			}
			l.logger.Info("module symlink is absent on file system, restore it", slog.String("name", release.Spec.ModuleName))
			if err := l.createModuleSymlink(ctx, release.Spec.ModuleName, release.GetModuleVersion(), source, release.Spec.Weight, false); err != nil {
				return fmt.Errorf("create module symlink: %w", err)
			}
		} else {
			moduleVersion, err := utils.GetModuleVersion(moduleSymlink)
			if err != nil {
				return fmt.Errorf("get module version: %w", err)
			}

			// skip overridden modules
			if moduleVersion == downloader.DefaultDevVersion {
				l.logger.Warn("module symlink is overridden, skip it", slog.String("name", release.Spec.ModuleName))
				continue
			}

			// check if module symlink leads to the current version
			if moduleVersion != release.GetModuleVersion() {
				l.logger.Info("module symlink is incorrect, restore it", slog.String("name", release.Spec.ModuleName), slog.String("current_version", moduleVersion), slog.String("desired_version", release.GetModuleVersion()))
				if err := l.createModuleSymlink(ctx, release.Spec.ModuleName, release.GetModuleVersion(), source, release.Spec.Weight, false); err != nil {
					return fmt.Errorf("create the '%s' module symlink: %w", release.Spec.ModuleName, err)
				}
			}
		}

		l.registries[release.GetModuleName()] = utils.BuildRegistryValue(source)
	}

	return nil
}

// deleteModulesWithAbsentRelease deletes modules with absent releases
func (l *Loader) deleteModulesWithAbsentRelease(ctx context.Context) error {
	// TODO: delete in downloaded dir too
	symlinks, err := os.ReadDir(l.symlinksDir)
	if err != nil {
		return fmt.Errorf("read the '%s' symlinks directory: %w", l.symlinksDir, err)
	}

	modulesLinks := make(map[string]string, len(symlinks))
	for _, symlink := range symlinks {
		index := strings.Index(symlink.Name(), "-")
		if index == -1 {
			continue
		}

		moduleName := symlink.Name()[index+1:]
		modulesLinks[moduleName] = filepath.Join(l.symlinksDir, symlink.Name())
	}

	releases := new(v1alpha1.ModuleReleaseList)
	if err = l.client.List(ctx, releases); err != nil {
		return fmt.Errorf("list releases: %w", err)
	}

	l.logger.Debug("found releases", slog.Int("count", len(releases.Items)))

	// remove modules with release
	for _, release := range releases.Items {
		delete(modulesLinks, release.Spec.ModuleName)
	}

	for module, moduleLinkPath := range modulesLinks {
		mpo := new(v1alpha2.ModulePullOverride)
		if err = l.client.Get(ctx, client.ObjectKey{Name: module}, mpo); err != nil && apierrors.IsNotFound(err) {
			l.logger.Warn("module has neither release nor override, purge it from fs", slog.String("name", module))
			_ = os.RemoveAll(moduleLinkPath)
		}
	}

	return nil
}

// createModuleSymlink checks if there are any other symlinks for a module in the symlink dir and deletes them before
// attempting to download version/tag of the module and creating correct symlink
func (l *Loader) createModuleSymlink(ctx context.Context, moduleName, moduleVersion string, moduleSource *v1alpha1.ModuleSource, moduleWeight uint32, mpo bool) error {
	l.logger.Info("module is absent on filesystem, restore it from source",
		slog.String("name", moduleName),
		slog.String("version", moduleVersion),
		slog.String("source_name", moduleSource.Name),
	)

	// remove possible symlink doubles
	if err := deleteModuleSymlinks(l.symlinksDir, moduleName); err != nil {
		return fmt.Errorf("delete the '%s' module symlinks: %w", moduleName, err)
	}

	var moduleTag string
	if mpo {
		moduleTag = moduleVersion
		moduleVersion = downloader.DefaultDevVersion
	}

	// check if module's directory exists on fs
	info, err := os.Stat(filepath.Join(l.downloadedModulesDir, moduleName, moduleVersion))
	if err != nil || !info.IsDir() {
		l.logger.Info("downloading the module from the registry", slog.String("name", moduleName), slog.String("version", moduleVersion))
		options := utils.GenerateRegistryOptionsFromModuleSource(moduleSource, l.clusterUUID, l.logger)
		md := downloader.NewModuleDownloader(l.dependencyContainer, l.downloadedModulesDir, moduleSource, l.logger.Named("downloader"), options)

		if mpo {
			_, _, err = md.DownloadDevImageTag(moduleName, moduleTag, "")
		} else {
			_, err = md.DownloadByModuleVersion(ctx, moduleName, moduleVersion)
		}
		if err != nil {
			return fmt.Errorf("download the '%s' module of the '%s' version/tag: %w", moduleName, moduleVersion, err)
		}
	}

	moduleRelativePath := filepath.Join("../", moduleName, moduleVersion)
	symlinkPath := filepath.Join(l.symlinksDir, fmt.Sprintf("%d-%s", moduleWeight, moduleName))
	if err = restoreModuleSymlink(l.downloadedModulesDir, symlinkPath, moduleRelativePath); err != nil {
		return fmt.Errorf("restore the '%s' module symlink: %w", moduleName, err)
	}
	l.logger.Info("module restored", slog.String("name", moduleName), slog.String("version", moduleVersion), slog.String("path", moduleRelativePath))

	return nil
}

func restoreModuleSymlink(downloadedModulesDir, symlinkPath, moduleRelativePath string) error {
	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(downloadedModulesDir, strings.TrimPrefix(moduleRelativePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return fmt.Errorf("get stat of the '%s': %v", moduleRelativePath, err)
	}

	return os.Symlink(moduleRelativePath, symlinkPath)
}

// deleteModuleSymlinks checks if there are symlinks for the module with different weight in the symlink folder
func deleteModuleSymlinks(symlinksDir, moduleName string) error {
	// delete all module's symlinks in a loop
	for {
		anotherModuleSymlink, err := utils.GetModuleSymlink(symlinksDir, moduleName)
		if err != nil {
			return fmt.Errorf("check if there are any other symlinks for the '%s' module: %w", moduleName, err)
		}

		if len(anotherModuleSymlink) > 0 {
			if err = os.Remove(anotherModuleSymlink); err != nil {
				return fmt.Errorf("delete the '%s' stale symlink for the '%s' module: %w", anotherModuleSymlink, moduleName, err)
			}
			// go for another spin
			continue
		}

		// no more symlinks found
		break
	}
	return nil
}
