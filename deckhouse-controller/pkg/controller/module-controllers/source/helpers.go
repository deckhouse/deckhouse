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
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/release"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
)

func (r *reconciler) cleanSourceInModule(ctx context.Context, sourceName, moduleName string) error {
	return retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			module := new(v1alpha1.Module)
			if err := r.client.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
				return err
			}

			// delete modules without sources, it seems impossible, but just in case
			if len(module.Properties.AvailableSources) == 0 {
				// don`t delete enabled module
				if !module.ConditionStatus(v1alpha1.ModuleConditionEnabledByModuleManager) {
					return r.client.Delete(ctx, module)
				}
				return nil
			}

			// delete modules with this source as the last source
			if len(module.Properties.AvailableSources) == 1 && module.Properties.AvailableSources[0] == sourceName {
				// don`t delete enabled module
				if !module.ConditionStatus(v1alpha1.ModuleConditionEnabledByModuleManager) {
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

	for _, moduleRelease := range moduleReleases.Items {
		if moduleRelease.Status.Phase == v1alpha1.PhaseDeployed {
			for _, ref := range moduleRelease.GetOwnerReferences() {
				if ref.UID == source.UID && ref.Name == source.Name && ref.Kind == v1alpha1.ModuleSourceGVK.Kind {
					// update the values.yaml file in downloaded-modules/<module_name>/v<module_version/openapi path
					modulePath := filepath.Join(r.downloadedModulesDir, moduleRelease.Spec.ModuleName, fmt.Sprintf("v%s", moduleRelease.Spec.Version))
					if err = downloader.InjectRegistryToModuleValues(modulePath, source); err != nil {
						return fmt.Errorf("update the '%s' module release registry settings: %w", moduleRelease.Name, err)
					}

					if len(moduleRelease.ObjectMeta.Annotations) == 0 {
						moduleRelease.ObjectMeta.Annotations = make(map[string]string)
					}

					moduleRelease.ObjectMeta.Annotations[release.RegistrySpecChangedAnnotation] = r.dependencyContainer.GetClock().Now().UTC().Format(time.RFC3339)
					if err = r.client.Update(ctx, &moduleRelease); err != nil {
						return fmt.Errorf("set RegistrySpecChangedAnnotation to the '%s' module release: %w", moduleRelease.Name, err)
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
		return false, fmt.Errorf("list module releases: %v", err)
	}
	if len(moduleReleases.Items) == 0 {
		r.log.Debugf("no module release with '%s' checksum for the '%s' module of the '%s' source", checksum, moduleName, sourceName)
		return false, nil
	}

	r.log.Debugf("the module release with '%s' checksum exist for the '%s' module of the '%s' source", checksum, moduleName, sourceName)
	return true, nil
}

func (r *reconciler) ensureModuleRelease(ctx context.Context, sourceUID types.UID, sourceName, moduleName, policy string, meta downloader.ModuleDownloadResult) error {
	moduleRelease := new(v1alpha1.ModuleRelease)
	if err := r.client.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-%s", moduleName, meta.ModuleVersion)}, moduleRelease); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get module release: %w", err)
		}
		moduleRelease = &v1alpha1.ModuleRelease{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha1.ModuleReleaseGVK.Kind,
				APIVersion: "deckhouse.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s", moduleName, meta.ModuleVersion),
				Labels: map[string]string{
					v1alpha1.ModuleReleaseLabelModule: moduleName,
					v1alpha1.ModuleReleaseLabelSource: sourceName,
					// image digest has 64 symbols, while label can have maximum 63 symbols, so make md5 sum here
					v1alpha1.ModuleReleaseLabelReleaseChecksum: fmt.Sprintf("%x", md5.Sum([]byte(meta.Checksum))),
					release.UpdatePolicyLabel:                  policy,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: v1alpha1.ModuleSourceGVK.GroupVersion().String(),
						Kind:       v1alpha1.ModuleSourceGVK.Kind,
						Name:       sourceName,
						UID:        sourceUID,
						Controller: ptr.To(true),
					},
				},
			},
			Spec: v1alpha1.ModuleReleaseSpec{
				ModuleName: moduleName,
				Version:    semver.MustParse(meta.ModuleVersion),
				Weight:     meta.ModuleWeight,
				Changelog:  meta.Changelog,
			},
		}
		if meta.ModuleDefinition != nil {
			moduleRelease.Spec.Requirements = meta.ModuleDefinition.Requirements
		}

		// if it's a first release for a Module, we have to install it immediately
		// without any update Windows and update.mode manual approval
		// the easiest way is to check the count or ModuleReleases for this module
		{
			mrList := new(v1alpha1.ModuleReleaseList)
			err = r.client.List(ctx, mrList, client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: moduleName}, client.Limit(1))
			if err != nil {
				return fmt.Errorf("failed to fetch ModuleRelease list: %w", err)
			}
			if len(mrList.Items) == 0 {
				// no any other releases
				if len(moduleRelease.Annotations) == 0 {
					moduleRelease.Annotations = make(map[string]string, 1)
				}
				moduleRelease.Annotations["release.deckhouse.io/apply-now"] = "true"
			}
		}

		if err = r.client.Create(ctx, moduleRelease); err != nil {
			return fmt.Errorf("create module release: %w", err)
		}
		return nil
	}

	// seems weird to update already deployed/suspended release
	if moduleRelease.Status.Phase != v1alpha1.PhasePending {
		return nil
	}

	moduleRelease.Spec = v1alpha1.ModuleReleaseSpec{
		ModuleName: moduleName,
		Version:    semver.MustParse(meta.ModuleVersion),
		Weight:     meta.ModuleWeight,
		Changelog:  meta.Changelog,
	}

	if err := r.client.Update(ctx, moduleRelease); err != nil {
		return fmt.Errorf("update module release: %w", err)
	}

	return nil
}

func (r *reconciler) ensureModule(ctx context.Context, sourceName, moduleName, releaseChannel string) (*v1alpha1.Module, error) {
	module := new(v1alpha1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: moduleName}, module); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		r.log.Debugf("the '%s' module not installed", moduleName)
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
		r.log.Debugf("the '%s' module not found, create it", moduleName)
		if err = r.client.Create(ctx, module); err != nil {
			return nil, err
		}
	}

	err := utils.UpdateStatus[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		// init just created downloaded modules
		if len(module.Status.Conditions) == 0 {
			module.Status.Phase = v1alpha1.ModulePhaseNotInstalled
			module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleConfig, v1alpha1.ModuleReasonDisabled, v1alpha1.ModuleMessageDisabled)
			module.SetConditionFalse(v1alpha1.ModuleConditionEnabledByModuleManager, "", "")
			module.SetConditionFalse(v1alpha1.ModuleConditionIsReady, v1alpha1.ModuleReasonNotInstalled, v1alpha1.ModuleMessageNotInstalled)
			return true
		}
		return false
	})
	if err != nil {
		return nil, err
	}

	err = utils.Update[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		if !slices.Contains(module.Properties.AvailableSources, sourceName) {
			module.Properties.AvailableSources = append(module.Properties.AvailableSources, sourceName)
			return true
		}
		return false
	})
	if err != nil {
		return nil, err
	}

	if module.Properties.Source != sourceName {
		r.log.Debugf("the '%s' source not active source for the '%s' module, skip it", sourceName, moduleName)
		return nil, nil
	}

	if !module.ConditionStatus(v1alpha1.ModuleConditionEnabledByModuleConfig) {
		r.log.Debugf("skip the '%s' disabled module", moduleName)
		return nil, nil
	}

	// update release channel
	err = utils.Update[*v1alpha1.Module](ctx, r.client, module, func(module *v1alpha1.Module) bool {
		if module.Properties.ReleaseChannel != releaseChannel {
			module.Properties.ReleaseChannel = releaseChannel
			return true
		}
		return false
	})
	if err != nil {
		return nil, fmt.Errorf("update release channel for the '%s' module: %w", moduleName, err)
	}

	return module, nil
}

func (r *reconciler) updateModuleSourceStatusMessage(ctx context.Context, source *v1alpha1.ModuleSource, message string) error {
	err := utils.UpdateStatus[*v1alpha1.ModuleSource](ctx, r.client, source, func(source *v1alpha1.ModuleSource) bool {
		source.Status.SyncTime = metav1.NewTime(r.dependencyContainer.GetClock().Now().UTC())
		source.Status.Message = message
		return true
	})
	if err != nil {
		return fmt.Errorf("update the '%s' module source status: %w", source.Name, err)
	}

	return nil
}
