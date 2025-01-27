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
	"sort"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/gofrs/uuid/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/release"
	controllerUtils "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	d8env "github.com/deckhouse/deckhouse/go_lib/deckhouse-config/env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	defaultScanInterval        = 3 * time.Minute
	registryChecksumAnnotation = "modules.deckhouse.io/registry-spec-checksum"
)

type moduleSourceReconciler struct {
	client               client.Client
	downloadedModulesDir string

	deckhouseEmbeddedPolicy *helpers.ModuleUpdatePolicySpecContainer

	dc dependency.Container

	logger *log.Logger

	rwlock                sync.RWMutex
	moduleSourcesChecksum sourceChecksum
	preflightCountDown    *sync.WaitGroup
	clusterUUID           string
}

func NewModuleSourceController(mgr manager.Manager, dc dependency.Container, embeddedPolicyContainer *helpers.ModuleUpdatePolicySpecContainer,
	preflightCountDown *sync.WaitGroup, logger *log.Logger,
) error {
	lg := logger.With("component", "ModuleSourceController")

	r := &moduleSourceReconciler{
		client:               mgr.GetClient(),
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		dc:                   dc,
		logger:               lg,

		deckhouseEmbeddedPolicy: embeddedPolicyContainer,
		moduleSourcesChecksum:   make(sourceChecksum),

		preflightCountDown: preflightCountDown,
	}

	// Add Preflight Check
	err := mgr.Add(manager.RunnableFunc(r.PreflightCheck))
	if err != nil {
		return err
	}
	r.preflightCountDown.Add(1)

	ctr, err := controller.New("module-source", mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		CacheSyncTimeout:        3 * time.Minute,
		NeedLeaderElection:      ptr.To(false),
		Reconciler:              r,
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ModuleSource{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(ctr)
}

func (r *moduleSourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var result ctrl.Result
	sourceName := req.Name

	var ms v1alpha1.ModuleSource
	err := r.client.Get(ctx, req.NamespacedName, &ms)
	if err != nil {
		// The ModuleSource resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			// if source is not exists anymore - drop the checksum cache
			r.saveSourceChecksums(sourceName, make(moduleChecksum))
			return result, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	if !ms.DeletionTimestamp.IsZero() {
		return r.deleteReconcile(ctx, &ms)
	}

	return r.createOrUpdateReconcile(ctx, &ms)
}

func (r *moduleSourceReconciler) PreflightCheck(ctx context.Context) (err error) {
	defer func() {
		if err == nil {
			r.preflightCountDown.Done()
		}
	}()

	r.clusterUUID = r.getClusterUUID(ctx)
	return nil
}

func (r *moduleSourceReconciler) getClusterUUID(ctx context.Context) string {
	var secret corev1.Secret
	key := types.NamespacedName{Namespace: "d8-system", Name: "deckhouse-discovery"}
	err := r.client.Get(ctx, key, &secret)
	if err != nil {
		r.logger.Warnf("Read clusterUUID from secret %s failed: %v. Generating random uuid", key, err)
		return uuid.Must(uuid.NewV4()).String()
	}

	if clusterUUID, ok := secret.Data["clusterUUID"]; ok {
		return string(clusterUUID)
	}

	return uuid.Must(uuid.NewV4()).String()
}

func (r *moduleSourceReconciler) createOrUpdateReconcile(ctx context.Context, ms *v1alpha1.ModuleSource) (ctrl.Result, error) {
	ms.Status.Msg = ""
	ms.Status.ModuleErrors = make([]v1alpha1.ModuleError, 0)

	opts := controllerUtils.GenerateRegistryOptionsFromModuleSource(ms, r.clusterUUID, r.logger)

	regCli, err := r.dc.GetRegistryClient(ms.Spec.Registry.Repo, opts...)
	if err != nil {
		ms.Status.Msg = err.Error()
		if e := r.updateModuleSourceStatus(ctx, ms); e != nil {
			return ctrl.Result{Requeue: true}, e
		}

		// error can occur on wrong auth only, we don't want to requeue the source until auth is fixed
		return ctrl.Result{Requeue: false}, nil
	}

	moduleNames, err := regCli.ListTags(ctx)
	if err != nil {
		ms.Status.Msg = err.Error()
		if e := r.updateModuleSourceStatus(ctx, ms); e != nil {
			return ctrl.Result{Requeue: true}, e
		}
		return ctrl.Result{Requeue: true}, err
	}

	// check, by means of comparing registry settings to the checkSum annotation, if new registry settings should be propagated to deployed module release
	updateNeeded, err := r.checkAndPropagateRegistrySettings(ctx, ms)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	// new registry settings checksum should be applied to module source
	if updateNeeded {
		if err := r.client.Update(ctx, ms); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		// requeue ms after modifying annotation
		return ctrl.Result{Requeue: true}, nil
	}

	sort.Strings(moduleNames)

	// form available modules structure
	availableModules := make([]v1alpha1.AvailableModule, 0, len(moduleNames))

	ms.Status.ModulesCount = len(moduleNames)

	modulesChecksums := r.getModuleSourceChecksum(ms.Name)

	md := downloader.NewModuleDownloader(r.dc, r.downloadedModulesDir, ms, opts)

	// get all policies regardless of their labels
	var policies v1alpha1.ModuleUpdatePolicyList
	err = r.client.List(ctx, &policies)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	for _, moduleName := range moduleNames {
		if moduleName == "modules" {
			r.logger.Warn("'modules' name for module is forbidden. Skip module.")
			continue
		}

		newChecksum, av, err := r.processSourceModule(ctx, md, ms, moduleName, modulesChecksums[moduleName], policies.Items)
		availableModules = append(availableModules, av)
		if err != nil {
			ms.Status.ModuleErrors = append(ms.Status.ModuleErrors, v1alpha1.ModuleError{
				Name:  moduleName,
				Error: err.Error(),
			})
			continue
		}

		if newChecksum != "" {
			modulesChecksums[moduleName] = newChecksum
		}
	}

	ms.Status.AvailableModules = availableModules

	if len(ms.Status.ModuleErrors) > 0 {
		ms.Status.Msg = "Some errors occurred. Inspect status for details"
	}

	err = r.updateModuleSourceStatus(ctx, ms)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// save checksums
	r.saveSourceChecksums(ms.Name, modulesChecksums)

	// everything is ok, check source on the other iteration
	return ctrl.Result{RequeueAfter: defaultScanInterval}, nil
}

func (r *moduleSourceReconciler) deleteReconcile(ctx context.Context, ms *v1alpha1.ModuleSource) (ctrl.Result, error) {
	var result ctrl.Result

	if controllerutil.ContainsFinalizer(ms, "modules.deckhouse.io/release-exists") {
		v := ms.GetAnnotations()["modules.deckhouse.io/force-delete"]
		if v != "true" {
			// check releases
			var releases v1alpha1.ModuleReleaseList

			err := r.client.List(ctx, &releases, client.MatchingLabels{"source": ms.Name, "status": "deployed"})
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}

			if len(releases.Items) > 0 {
				ms.Status.Msg = "ModuleSource contains at least 1 Deployed release and cannot be deleted. Please delete target ModuleReleases manually to continue"
				if err := r.updateModuleSourceStatus(ctx, ms); err != nil {
					return ctrl.Result{Requeue: true}, nil
				}

				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
		}

		controllerutil.RemoveFinalizer(ms, "modules.deckhouse.io/release-exists")

		err := r.client.Update(ctx, ms)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	r.saveSourceChecksums(ms.Name, make(moduleChecksum))
	return result, nil
}

func (r *moduleSourceReconciler) processSourceModule(ctx context.Context, md *downloader.ModuleDownloader, ms *v1alpha1.ModuleSource, moduleName, moduleChecksum string, policies []v1alpha1.ModuleUpdatePolicy) ( /*checksum*/ string, v1alpha1.AvailableModule, error) {
	av := v1alpha1.AvailableModule{
		Name:       moduleName,
		Policy:     "",
		Overridden: false,
	}

	// check if we have a ModulePullOverride for source/module
	exists, err := r.isModulePullOverrideExists(ctx, ms.Name, moduleName)
	if err != nil {
		r.logger.Warnf("Unexpected error on getting ModulePullOverride for %s/%s", ms.Name, moduleName)
		return "", av, err
	}

	if exists {
		av.Overridden = true
		return "", av, nil
	}
	// get an update policy for the moduleName or, if there is no matching policy, use the embedded on
	policy, err := r.getReleasePolicy(ms.Name, moduleName, policies)
	if err != nil {
		return "", av, err
	}
	av.Policy = policy.Name

	if policy.Spec.Update.Mode == "Ignore" {
		return "", av, nil
	}

	downloadResult, err := md.DownloadMetadataFromReleaseChannel(moduleName, policy.Spec.ReleaseChannel, moduleChecksum)
	if err != nil {
		return "", av, err
	}

	if downloadResult.Checksum == moduleChecksum {
		r.logger.Infof("Module %q checksum in the %q release channel has not been changed. Skip update.", moduleName, policy.Spec.ReleaseChannel)
		return "", av, nil
	}

	err = r.createModuleRelease(ctx, ms, moduleName, policy.Name, downloadResult)
	if err != nil {
		return "", av, err
	}

	return downloadResult.Checksum, av, nil
}

func (r *moduleSourceReconciler) createModuleRelease(ctx context.Context, ms *v1alpha1.ModuleSource, moduleName, policyName string, result downloader.ModuleDownloadResult) error {
	// image digest has 64 symbols, while label can have maximum 63 symbols
	// so make md5 sum here
	checksum := fmt.Sprintf("%x", md5.Sum([]byte(result.Checksum)))

	rl := &v1alpha1.ModuleRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ModuleRelease",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", moduleName, result.ModuleVersion),
			Labels: map[string]string{
				"module":                  moduleName,
				"source":                  ms.Name,
				"release-checksum":        checksum,
				release.UpdatePolicyLabel: policyName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1alpha1.ModuleSourceGVK.GroupVersion().String(),
					Kind:       v1alpha1.ModuleSourceGVK.Kind,
					Name:       ms.Name,
					UID:        ms.GetUID(),
					Controller: ptr.To(true),
				},
			},
		},
		Spec: v1alpha1.ModuleReleaseSpec{
			ModuleName: moduleName,
			Version:    semver.MustParse(result.ModuleVersion),
			Weight:     result.ModuleWeight,
			Changelog:  v1alpha1.Changelog(result.Changelog),
		},
	}
	if result.ModuleDefinition != nil {
		rl.Spec.Requirements = result.ModuleDefinition.GetRequirements()
	}

	err := r.client.Create(ctx, rl)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			var prevMR v1alpha1.ModuleRelease

			err = r.client.Get(ctx, client.ObjectKey{Name: rl.Name}, &prevMR)
			if err != nil {
				return err
			}

			// seems weird to update already deployed/suspended release
			if prevMR.Status.Phase != v1alpha1.PhasePending {
				return nil
			}

			prevMR.Spec = rl.Spec
			return r.client.Update(ctx, &prevMR)
		}

		return err
	}
	return nil
}

// getReleasePolicy checks if any update policy matches the module release and if it's so - returns the policy and its release channel.
// if several policies match the module release labels, conflict=true is returned
// if no policy matches the module release, deckhouseEmbeddedPolicy is returned
func (r *moduleSourceReconciler) getReleasePolicy(sourceName, moduleName string, policies []v1alpha1.ModuleUpdatePolicy) (*v1alpha1.ModuleUpdatePolicy, error) {
	var releaseLabelsSet labels.Set = map[string]string{"module": moduleName, "source": sourceName}
	var matchedPolicy *v1alpha1.ModuleUpdatePolicy
	var found bool

	for _, policy := range policies {
		if policy.Spec.ModuleReleaseSelector.LabelSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(policy.Spec.ModuleReleaseSelector.LabelSelector)
			if err != nil {
				return nil, err
			}
			selectorSourceName, sourceLabelExists := selector.RequiresExactMatch("source")
			if sourceLabelExists && selectorSourceName != sourceName {
				// 'source' label is set, but does not match the given ModuleSource
				continue
			}

			if selector.Matches(releaseLabelsSet) {
				// ModuleUpdatePolicy matches ModuleSource and specified Module
				if found {
					return nil, fmt.Errorf("more than one update policy matches the module: %s and %s", matchedPolicy.Name, policy.Name)
				}
				found = true
				matchedPolicy = &policy
			}
		}
	}

	if !found {
		r.logger.Infof("ModuleUpdatePolicy for ModuleSource: %q, Module: %q not found, using Embedded policy: %+v", sourceName, moduleName, *r.deckhouseEmbeddedPolicy.Get())
		return &v1alpha1.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha1.ModuleUpdatePolicyGVK.Kind,
				APIVersion: v1alpha1.ModuleUpdatePolicyGVK.GroupVersion().String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "", // special empty default policy, inherits Deckhouse settings for update mode
			},
			Spec: *r.deckhouseEmbeddedPolicy.Get(),
		}, nil
	}

	return matchedPolicy, nil
}

func (r *moduleSourceReconciler) updateModuleSourceStatus(ctx context.Context, msCopy *v1alpha1.ModuleSource) error {
	msCopy.Status.SyncTime = metav1.NewTime(r.dc.GetClock().Now().UTC())

	return r.client.Status().Update(ctx, msCopy)
}

// checkAndPropagateRegistrySettings checks if modules source registry settings were updated (comparing registryChecksumAnnotation annotation and current registry spec)
// and update relevant module releases' openapi values files if it the case
func (r *moduleSourceReconciler) checkAndPropagateRegistrySettings(ctx context.Context, ms *v1alpha1.ModuleSource) ( /* update required */ bool, error) {
	// get registry settings checksum
	marshaledSpec, err := json.Marshal(ms.Spec.Registry)
	if err != nil {
		return false, fmt.Errorf("couldn't marshal %s module source registry spec: %w", ms.Name, err)
	}

	currentChecksum := fmt.Sprintf("%x", md5.Sum(marshaledSpec))
	// if there is no annotations - only set the current checksum value
	if ms.ObjectMeta.Annotations == nil {
		ms.ObjectMeta.Annotations = make(map[string]string)
		ms.ObjectMeta.Annotations[registryChecksumAnnotation] = currentChecksum
		return true, nil
	}

	// if the annotation matches current checksum - there is nothing to do here
	if ms.ObjectMeta.Annotations[registryChecksumAnnotation] == currentChecksum {
		return false, nil
	}

	// get related releases
	var moduleReleasesFromSource v1alpha1.ModuleReleaseList
	err = r.client.List(ctx, &moduleReleasesFromSource, client.MatchingLabels{"source": ms.Name})
	if err != nil {
		return false, fmt.Errorf("couldn't list module releases to update registry settings: %w", err)
	}

	for _, rl := range moduleReleasesFromSource.Items {
		if rl.Status.Phase == v1alpha1.PhaseDeployed {
			ownerReferences := rl.GetOwnerReferences()
			for _, ref := range ownerReferences {
				if ref.UID == ms.UID && ref.Name == ms.Name && ref.Kind == "ModuleSource" {
					// update the values.yaml file in externam-modules/<module_name>/v<module_version/openapi path
					err = downloader.InjectRegistryToModuleValues(filepath.Join(r.downloadedModulesDir, rl.Spec.ModuleName, fmt.Sprintf("v%s", rl.Spec.Version)), ms)
					if err != nil {
						return false, fmt.Errorf("couldn't update module release %s registry settings: %w", rl.Name, err)
					}
					// annotate module release with the release.RegistrySpecChangedAnnotation annotation to notify module release controller about registry spec
					// change, if the module release isn't overridden by a module pull override
					mpoExists, err := r.isModulePullOverrideExists(ctx, ms.Name, rl.Spec.ModuleName)
					if err != nil {
						return false, fmt.Errorf("unexpected error on getting ModulePullOverride for %s/%s: %w", ms.Name, rl.Spec.ModuleName, err)
					}
					if mpoExists {
						break
					}

					if rl.ObjectMeta.Annotations == nil {
						rl.ObjectMeta.Annotations = make(map[string]string)
					}

					rl.ObjectMeta.Annotations[release.RegistrySpecChangedAnnotation] = r.dc.GetClock().Now().UTC().Format(time.RFC3339)
					if err := r.client.Update(ctx, &rl); err != nil {
						return false, fmt.Errorf("couldn't set RegistrySpecChangedAnnotation to %s the module release: %w", rl.Name, err)
					}

					break
				}
			}
		}
	}

	// get related module pull overrides
	var mposFromSource v1alpha1.ModulePullOverrideList
	err = r.client.List(ctx, &mposFromSource, client.MatchingLabels{"source": ms.Name})
	if err != nil {
		return false, fmt.Errorf("could list module pull overrides to update registry settings: %w", err)
	}

	for _, mpo := range mposFromSource.Items {
		// update the values.yaml file in externam-modules/<module_name>/dev/openapi path
		err = downloader.InjectRegistryToModuleValues(filepath.Join(r.downloadedModulesDir, mpo.Name, "dev"), ms)
		if err != nil {
			return false, fmt.Errorf("couldn't update module pull override %s registry settings: %w", mpo.Name, err)
		}
		// annotate module pull override with the release.RegistrySpecChangedAnnotation annotation to notify module pull override controller about registry spec change
		if mpo.ObjectMeta.Annotations == nil {
			mpo.ObjectMeta.Annotations = make(map[string]string)
		}

		mpo.ObjectMeta.Annotations[release.RegistrySpecChangedAnnotation] = r.dc.GetClock().Now().UTC().Format(time.RFC3339)
		if err := r.client.Update(ctx, &mpo); err != nil {
			return false, fmt.Errorf("couldn't set RegistrySpecChangedAnnotation to the %s module pull override: %w", mpo.Name, err)
		}
	}

	ms.ObjectMeta.Annotations[registryChecksumAnnotation] = currentChecksum

	return true, nil
}

func (r *moduleSourceReconciler) isModulePullOverrideExists(ctx context.Context, sourceName, moduleName string) (bool, error) {
	var mpo v1alpha1.ModulePullOverrideList
	err := r.client.List(ctx, &mpo, client.MatchingLabels{"source": sourceName, "module": moduleName}, client.Limit(1))
	if err != nil {
		return false, err
	}

	return len(mpo.Items) > 0, nil
}

func (r *moduleSourceReconciler) saveSourceChecksums(msName string, checksums moduleChecksum) {
	r.rwlock.Lock()
	r.moduleSourcesChecksum[msName] = checksums
	r.rwlock.Unlock()
}

func (r *moduleSourceReconciler) getModuleSourceChecksum(msName string) moduleChecksum {
	r.rwlock.RLock()
	defer r.rwlock.RUnlock()

	res, ok := r.moduleSourcesChecksum[msName]
	if ok {
		return res
	}

	return make(moduleChecksum)
}

type moduleChecksum map[string]string

type sourceChecksum map[string]moduleChecksum
