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

package source

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/ctrlutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
)

var (
	ErrRequireResync = errors.New("require resync")
)

func (r *reconciler) cleanSourceInModule(ctx context.Context, sourceName, moduleName string) error {
	return retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			module := new(v1alpha1.Module)
			if err := r.client.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return fmt.Errorf("get the '%s' module: %w", moduleName, err)
			}

			// delete modules without sources, it seems impossible, but just in case
			if len(module.Properties.AvailableSources) == 0 {
				// don`t delete enabled module
				if !module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleManager, corev1.ConditionTrue) {
					return r.client.Delete(ctx, module)
				}
				return nil
			}

			// delete modules with this source as the last source
			if len(module.Properties.AvailableSources) == 1 && module.Properties.AvailableSources[0] == sourceName {
				// don`t delete enabled module
				if !module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleManager, corev1.ConditionTrue) {
					return r.client.Delete(ctx, module)
				}
				module.Properties.AvailableSources = []string{}
			}

			if len(module.Properties.AvailableSources) > 1 {
				for idx, source := range module.Properties.AvailableSources {
					if source == sourceName {
						module.Properties.AvailableSources = append(module.Properties.AvailableSources[:idx], module.Properties.AvailableSources[idx+1:]...)
						break
					}
				}
			}

			return r.client.Update(ctx, module)
		})
	})
}

// syncRegistrySettings checks if modules source registry settings were updated
// (comparing moduleSourceAnnotationRegistryChecksum annotation and the current registry spec)
// and update relevant module releases' openapi values files if it is the case
func (r *reconciler) syncRegistrySettings(ctx context.Context, source *v1alpha1.ModuleSource) error {
	marshaled, err := json.Marshal(source.Spec.Registry)
	if err != nil {
		return fmt.Errorf("marshal the '%s' module source registry spec: %w", source.Name, err)
	}

	currentChecksum := fmt.Sprintf("%x", md5.Sum(marshaled))

	// if no annotations - only set the current checksum value
	if len(source.ObjectMeta.Annotations) == 0 {
		source.ObjectMeta.Annotations = map[string]string{
			v1alpha1.ModuleSourceAnnotationRegistryChecksum: currentChecksum,
		}

		return nil
	}

	// if the annotation matches current checksum - there is nothing to do here
	if source.ObjectMeta.Annotations[v1alpha1.ModuleSourceAnnotationRegistryChecksum] == currentChecksum {
		return ErrSettingsNotChanged
	}

	// get related releases
	moduleReleases := new(v1alpha1.ModuleReleaseList)
	if err = r.client.List(ctx, moduleReleases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelSource: source.Name}); err != nil {
		return fmt.Errorf("list module releases to update registry settings: %w", err)
	}

	for _, release := range moduleReleases.Items {
		if release.Status.Phase == v1alpha1.ModuleReleasePhaseDeployed {
			for _, ref := range release.GetOwnerReferences() {
				if ref.UID == source.UID && ref.Name == source.Name && ref.Kind == v1alpha1.ModuleSourceGVK.Kind {
					if len(release.ObjectMeta.Annotations) == 0 {
						release.ObjectMeta.Annotations = make(map[string]string)
					}

					release.ObjectMeta.Annotations[v1alpha1.ModuleReleaseAnnotationRegistrySpecChanged] = r.dc.GetClock().Now().UTC().Format(time.RFC3339)
					if err = r.client.Update(ctx, &release); err != nil {
						return fmt.Errorf("set RegistrySpecChanged annotation to the '%s' module release: %w", release.Name, err)
					}

					break
				}
			}
		}
	}

	source.ObjectMeta.Annotations[v1alpha1.ModuleSourceAnnotationRegistryChecksum] = currentChecksum

	return nil
}

func (r *reconciler) releaseExists(ctx context.Context, sourceName, moduleName, checksum string) (bool, error) {
	// image digest has 64 symbols, while label can have maximum 63 symbols, so make md5 sum here
	checksum = fmt.Sprintf("%x", md5.Sum([]byte(checksum)))

	moduleReleases := new(v1alpha1.ModuleReleaseList)
	if err := r.client.List(ctx, moduleReleases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: moduleName, v1alpha1.ModuleReleaseLabelReleaseChecksum: checksum}); err != nil {
		return false, fmt.Errorf("list module releases: %w", err)
	}
	if len(moduleReleases.Items) == 0 {
		r.logger.Debug(
			"no module release with checksum for the module of source",
			slog.String("checksum", checksum),
			slog.String("name", moduleName),
			slog.String("source_name", sourceName),
		)
		return false, nil
	}

	r.logger.Debug(
		"module release with checksum exists for the module of source",
		slog.String("checksum", checksum),
		slog.String("name", moduleName),
		slog.String("source_name", sourceName),
	)
	return true, nil
}

// needToEnsureRelease checks that the module enabled, the source is the active source,
// release exists, and checksum not changed.
func (r *reconciler) needToEnsureRelease(
	source *v1alpha1.ModuleSource,
	module *v1alpha1.Module,
	sourceModule v1alpha1.AvailableModule,
	meta *downloader.ModuleDownloadResult,
	releaseExists bool) bool {
	// check the active source
	if module.Properties.Source != "" && module.Properties.Source != source.Name {
		r.logger.Debug("source not active, skip module",
			slog.String("source_name", source.Name),
			slog.String("name", module.Name))

		return false
	}

	//  not found or unknown
	if !module.HasCondition(v1alpha1.ModuleConditionEnabledByModuleConfig) || module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionUnknown) {
		enabledByBundle := false
		if meta.ModuleDefinition != nil {
			enabledByBundle = meta.ModuleDefinition.Accessibility.IsEnabled(r.edition.Name, r.edition.Bundle)
		}

		if !enabledByBundle {
			return false
		}

		if len(module.Properties.AvailableSources) > 1 && source.Name != "deckhouse" {
			return false
		}
	} else if module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleConfig, corev1.ConditionFalse) {
		// disabled by module config
		return false
	}

	return sourceModule.Checksum != meta.Checksum || !releaseExists
}

func (r *reconciler) ensureModule(ctx context.Context, sourceName, moduleName, releaseChannel string) (*v1alpha1.Module, error) {
	var requireResync bool

	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("get the '%s' module: %w", moduleName, err)
		}

		requireResync = true

		module = &v1alpha1.Module{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha1.ModuleGVK.Kind,
				APIVersion: v1alpha1.ModuleGVK.GroupVersion().String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: moduleName,
			},
			Properties: v1alpha1.ModuleProperties{
				AvailableSources: []string{sourceName},
			},
		}
		r.logger.Debug("module not found, create it", slog.String("name", moduleName))

		if err = r.client.Create(ctx, module); err != nil {
			return nil, fmt.Errorf("create the '%s' module: %w", moduleName, err)
		}
	}

	err := ctrlutils.UpdateStatusWithRetry(ctx, r.client, module, func() error {
		// init just created downloaded modules
		if module.Status.Phase == "" {
			module.Status.Phase = v1alpha1.ModulePhaseAvailable
			module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleManager, "", "")
			module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonNotInstalled, v1alpha1.ModuleMessageNotInstalled)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update the '%s' module status: %w", moduleName, err)
	}

	err = ctrlutils.UpdateWithRetry(ctx, r.client, module, func() error {
		if !slices.Contains(module.Properties.AvailableSources, sourceName) {
			module.Properties.AvailableSources = append(module.Properties.AvailableSources, sourceName)
			requireResync = true
		}

		module.Properties.ReleaseChannel = releaseChannel

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update the '%s' module: %w", moduleName, err)
	}

	if requireResync {
		return nil, ErrRequireResync
	}

	return module, nil
}

func (r *reconciler) updateModuleSourceStatusMessage(ctx context.Context, source *v1alpha1.ModuleSource, message string) error {
	err := utils.UpdateStatus(ctx, r.client, source, func(source *v1alpha1.ModuleSource) bool {
		source.Status.Phase = v1alpha1.ModuleSourcePhaseActive
		source.Status.SyncTime = metav1.NewTime(r.dc.GetClock().Now().UTC())
		source.Status.Message = message
		return true
	})
	if err != nil {
		return fmt.Errorf("update the '%s' module source status: %w", source.Name, err)
	}

	return nil
}
