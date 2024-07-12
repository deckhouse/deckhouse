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
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/utils/logger"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
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
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	defaultScanInterval        = 3 * time.Minute
	registryChecksumAnnotation = "modules.deckhouse.io/registry-spec-checksum"
)

var (
	ErrNoPolicyFound = errors.New("no matching update policy found")
)

type moduleSourceReconciler struct {
	client             client.Client
	externalModulesDir string

	deckhouseEmbeddedPolicy *v1alpha1.ModuleUpdatePolicySpec

	dc dependency.Container

	logger logger.Logger

	rwlock                sync.RWMutex
	moduleSourcesChecksum sourceChecksum
}

func NewModuleSourceController(mgr manager.Manager, dc dependency.Container, embeddedPolicy *v1alpha1.ModuleUpdatePolicySpec) error {
	lg := log.WithField("component", "ModuleSourceController")

	c := &moduleSourceReconciler{
		client:             mgr.GetClient(),
		externalModulesDir: os.Getenv("EXTERNAL_MODULES_DIR"),
		dc:                 dc,
		logger:             lg,

		deckhouseEmbeddedPolicy: embeddedPolicy,
		moduleSourcesChecksum:   make(sourceChecksum),
	}

	ctr, err := controller.New("module-source", mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		CacheSyncTimeout:        15 * time.Minute,
		NeedLeaderElection:      pointer.Bool(false),
		Reconciler:              c,
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ModuleSource{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(ctr)
}

func (c *moduleSourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	sourceName := req.Name

	var ms v1alpha1.ModuleSource
	err := c.client.Get(ctx, req.NamespacedName, &ms)
	if err != nil {
		// The ModuleSource resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			// if source is not exists anymore - drop the checksum cache
			c.saveSourceChecksums(sourceName, make(moduleChecksum))
			return ctrl.Result{}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	if !ms.DeletionTimestamp.IsZero() {
		return c.deleteReconcile(ctx, &ms)
	}

	return c.createOrUpdateReconcile(ctx, &ms)
}

func (c *moduleSourceReconciler) createOrUpdateReconcile(ctx context.Context, ms *v1alpha1.ModuleSource) (ctrl.Result, error) {
	ms.Status.Msg = ""
	ms.Status.ModuleErrors = make([]v1alpha1.ModuleError, 0)

	opts := controllerUtils.GenerateRegistryOptions(ms)

	regCli, err := c.dc.GetRegistryClient(ms.Spec.Registry.Repo, opts...)
	if err != nil {
		ms.Status.Msg = err.Error()
		if e := c.updateModuleSourceStatus(ctx, ms); e != nil {
			return ctrl.Result{Requeue: true}, e
		}

		// error can occur on wrong auth only, we don't want to requeue the source until auth is fixed
		return ctrl.Result{Requeue: false}, nil
	}

	moduleNames, err := regCli.ListTags()
	if err != nil {
		ms.Status.Msg = err.Error()
		if e := c.updateModuleSourceStatus(ctx, ms); e != nil {
			return ctrl.Result{Requeue: true}, e
		}
		return ctrl.Result{Requeue: true}, err
	}

	// check, by means of comparing registry settings to the checkSum annotation, if new registry settings should be propagated to deployed module release
	updateNeeded, err := c.checkAndPropagateRegistrySettings(ctx, ms)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	// new registry settings checksum should be applied to module source
	if updateNeeded {
		if err := c.client.Update(ctx, ms); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		// requeue ms after modifying annotation
		return ctrl.Result{Requeue: true}, nil
	}

	sort.Strings(moduleNames)

	// form available modules structure
	availableModules := make([]v1alpha1.AvailableModule, 0, len(moduleNames))

	ms.Status.ModulesCount = len(moduleNames)

	modulesChecksums := c.getModuleSourceChecksum(ms.Name)

	md := downloader.NewModuleDownloader(c.dc, c.externalModulesDir, ms, opts)

	// get all policies regardless of their labels
	var policies v1alpha1.ModuleUpdatePolicyList
	err = c.client.List(ctx, &policies)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	for _, moduleName := range moduleNames {
		if moduleName == "modules" {
			c.logger.Warn("'modules' name for module is forbidden. Skip module.")
			continue
		}

		newChecksum, av, err := c.processSourceModule(ctx, md, ms, moduleName, modulesChecksums[moduleName], policies.Items)
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

	err = c.updateModuleSourceStatus(ctx, ms)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// save checksums
	c.saveSourceChecksums(ms.Name, modulesChecksums)

	// everything is ok, check source on the other iteration
	return ctrl.Result{RequeueAfter: defaultScanInterval}, nil
}

func (c *moduleSourceReconciler) deleteReconcile(ctx context.Context, ms *v1alpha1.ModuleSource) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(ms, "modules.deckhouse.io/release-exists") {
		v := ms.GetAnnotations()["modules.deckhouse.io/force-delete"]
		if v != "true" {
			// check releases
			var releases v1alpha1.ModuleReleaseList

			err := c.client.List(ctx, &releases, client.MatchingLabels{"source": ms.Name, "status": "deployed"})
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}

			if len(releases.Items) > 0 {
				ms.Status.Msg = "ModuleSource contains at least 1 Deployed release and cannot be deleted. Please delete target ModuleReleases manually to continue"
				if err := c.updateModuleSourceStatus(ctx, ms); err != nil {
					return ctrl.Result{Requeue: true}, nil
				}

				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
		}

		controllerutil.RemoveFinalizer(ms, "modules.deckhouse.io/release-exists")

		err := c.client.Update(ctx, ms)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	c.saveSourceChecksums(ms.Name, make(moduleChecksum))
	return ctrl.Result{}, nil
}

func (c *moduleSourceReconciler) processSourceModule(ctx context.Context, md *downloader.ModuleDownloader, ms *v1alpha1.ModuleSource, moduleName, moduleChecksum string, policies []v1alpha1.ModuleUpdatePolicy) ( /*checksum*/ string, v1alpha1.AvailableModule, error) {
	av := v1alpha1.AvailableModule{
		Name:       moduleName,
		Policy:     "",
		Overridden: false,
	}

	// check if we have a ModulePullOverride for source/module
	exists, err := c.isModulePullOverrideExists(ctx, ms.Name, moduleName)
	if err != nil {
		c.logger.Warnf("Unexpected error on getting ModulePullOverride for %s/%s", ms.Name, moduleName)
		return "", av, err
	}

	if exists {
		av.Overridden = true
		return "", av, nil
	}
	// check if we have an update policy for the moduleName
	policy, err := c.getReleasePolicy(ms.Name, moduleName, policies)
	if err != nil {
		// if policy not found - drop all previous module's errors
		if errors.Is(err, ErrNoPolicyFound) {
			return "", av, nil
			// if another error - update module's error status field
		}
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
		c.logger.Infof("Module %q checksum in the %q release channel has not been changed. Skip update.", moduleName, policy.Spec.ReleaseChannel)
		return "", av, nil
	}

	err = c.createModuleRelease(ctx, ms, moduleName, policy.Name, downloadResult)
	if err != nil {
		return "", av, err
	}

	return downloadResult.Checksum, av, nil
}

func (c *moduleSourceReconciler) createModuleRelease(ctx context.Context, ms *v1alpha1.ModuleSource, moduleName, policyName string, result downloader.ModuleDownloadResult) error {
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
					Controller: pointer.Bool(true),
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

	err := c.client.Create(ctx, rl)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			var prevMR v1alpha1.ModuleRelease

			err = c.client.Get(ctx, client.ObjectKey{Name: rl.Name}, &prevMR)
			if err != nil {
				return err
			}

			// seems weird to update already deployed/suspended release
			if prevMR.Status.Phase != v1alpha1.PhasePending {
				return nil
			}

			prevMR.Spec = rl.Spec
			return c.client.Update(ctx, &prevMR)
		}

		return err
	}
	return nil
}

// getReleasePolicy checks if any update policy matches the module release and if it's so - returns the policy and its release channel.
// if several policies match the module release labels, conflict=true is returned
func (c *moduleSourceReconciler) getReleasePolicy(sourceName, moduleName string, policies []v1alpha1.ModuleUpdatePolicy) (*v1alpha1.ModuleUpdatePolicy, error) {
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
		c.logger.Infof("ModuleUpdatePolicy for ModuleSource: %q, Module: %q not found, using Embedded policy: %+v", sourceName, moduleName, *c.deckhouseEmbeddedPolicy)
		return &v1alpha1.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha1.ModuleUpdatePolicyGVK.Kind,
				APIVersion: v1alpha1.ModuleUpdatePolicyGVK.GroupVersion().String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "", // special empty default policy, inherits Deckhouse settings for update mode
			},
			Spec: *c.deckhouseEmbeddedPolicy,
		}, nil
	}

	return matchedPolicy, nil
}

func (c *moduleSourceReconciler) updateModuleSourceStatus(ctx context.Context, msCopy *v1alpha1.ModuleSource) error {
	msCopy.Status.SyncTime = metav1.NewTime(c.dc.GetClock().Now().UTC())

	return c.client.Status().Update(ctx, msCopy)
}

// checkAndPropagateRegistrySettings checks if modules source registry settings were updated (comparing registryChecksumAnnotation annotation and current registry spec)
// and update relevant module releases' openapi values files if it the case
func (c *moduleSourceReconciler) checkAndPropagateRegistrySettings(ctx context.Context, msCopy *v1alpha1.ModuleSource) ( /* update required */ bool, error) {
	// get registry settings checksum
	marshaledSpec, err := json.Marshal(msCopy.Spec.Registry)
	if err != nil {
		return false, fmt.Errorf("couldn't marshal %s module source registry spec: %w", msCopy.Name, err)
	}

	currentChecksum := fmt.Sprintf("%x", md5.Sum(marshaledSpec))
	// if there is no annotations - only set the current checksum value
	if msCopy.ObjectMeta.Annotations == nil {
		msCopy.ObjectMeta.Annotations = make(map[string]string)
		msCopy.ObjectMeta.Annotations[registryChecksumAnnotation] = currentChecksum
		return true, nil
	}

	// if the annotation matches current checksum - there is nothing to do here
	if msCopy.ObjectMeta.Annotations[registryChecksumAnnotation] == currentChecksum {
		return false, nil
	}

	// get related releases
	var moduleReleasesFromSource v1alpha1.ModuleReleaseList
	err = c.client.List(ctx, &moduleReleasesFromSource, client.MatchingLabels{"source": msCopy.Name})
	if err != nil {
		return false, fmt.Errorf("couldn't list module releases to update registry settings: %w", err)
	}

	for _, rl := range moduleReleasesFromSource.Items {
		if rl.Status.Phase == v1alpha1.PhaseDeployed {
			ownerReferences := rl.GetOwnerReferences()
			for _, ref := range ownerReferences {
				if ref.UID == msCopy.UID && ref.Name == msCopy.Name && ref.Kind == "ModuleSource" {
					// update the values.yaml file in externam-modules/<module_name>/v<module_version/openapi path
					err = downloader.InjectRegistryToModuleValues(filepath.Join(c.externalModulesDir, rl.Spec.ModuleName, fmt.Sprintf("v%s", rl.Spec.Version)), msCopy)
					if err != nil {
						return false, fmt.Errorf("couldn't update module release %s registry settings: %w", rl.Name, err)
					}
					// annotate module release with the release.RegistrySpecChangedAnnotation annotation to notify module release controller about registry spec
					// change, if the module release isn't overridden by a module pull override
					mpoExists, err := c.isModulePullOverrideExists(ctx, msCopy.Name, rl.Spec.ModuleName)
					if err != nil {
						return false, fmt.Errorf("unexpected error on getting ModulePullOverride for %s/%s: %w", msCopy.Name, rl.Spec.ModuleName, err)
					}
					if mpoExists {
						break
					}

					if rl.ObjectMeta.Annotations == nil {
						rl.ObjectMeta.Annotations = make(map[string]string)
					}

					rl.ObjectMeta.Annotations[release.RegistrySpecChangedAnnotation] = c.dc.GetClock().Now().UTC().Format(time.RFC3339)
					if err := c.client.Update(ctx, &rl); err != nil {
						return false, fmt.Errorf("couldn't set RegistrySpecChangedAnnotation to %s the module release: %w", rl.Name, err)
					}

					break
				}
			}
		}
	}

	// get related module pull overrides
	var mposFromSource v1alpha1.ModulePullOverrideList
	err = c.client.List(ctx, &mposFromSource, client.MatchingLabels{"source": msCopy.Name})
	if err != nil {
		return false, fmt.Errorf("could list module pull overrides to update registry settings: %w", err)
	}

	for _, mpo := range mposFromSource.Items {
		// update the values.yaml file in externam-modules/<module_name>/dev/openapi path
		err = downloader.InjectRegistryToModuleValues(filepath.Join(c.externalModulesDir, mpo.Name, "dev"), msCopy)
		if err != nil {
			return false, fmt.Errorf("couldn't update module pull override %s registry settings: %w", mpo.Name, err)
		}
		// annotate module pull override with the release.RegistrySpecChangedAnnotation annotation to notify module pull override controller about registry spec change
		if mpo.ObjectMeta.Annotations == nil {
			mpo.ObjectMeta.Annotations = make(map[string]string)
		}

		mpo.ObjectMeta.Annotations[release.RegistrySpecChangedAnnotation] = c.dc.GetClock().Now().UTC().Format(time.RFC3339)
		if err := c.client.Update(ctx, &mpo); err != nil {
			return false, fmt.Errorf("couldn't set RegistrySpecChangedAnnotation to the %s module pull override: %w", mpo.Name, err)
		}
	}

	msCopy.ObjectMeta.Annotations[registryChecksumAnnotation] = currentChecksum

	return true, nil
}

func (c *moduleSourceReconciler) isModulePullOverrideExists(ctx context.Context, sourceName, moduleName string) (bool, error) {
	var mpo v1alpha1.ModulePullOverrideList
	err := c.client.List(ctx, &mpo, client.MatchingLabels{"source": sourceName, "module": moduleName}, client.Limit(1))
	if err != nil {
		return false, err
	}

	return len(mpo.Items) > 0, nil
}

func (c *moduleSourceReconciler) saveSourceChecksums(msName string, checksums moduleChecksum) {
	c.rwlock.Lock()
	c.moduleSourcesChecksum[msName] = checksums
	c.rwlock.Unlock()
}

func (c *moduleSourceReconciler) getModuleSourceChecksum(msName string) moduleChecksum {
	c.rwlock.RLock()
	defer c.rwlock.RUnlock()

	res, ok := c.moduleSourcesChecksum[msName]
	if ok {
		return res
	}

	return make(moduleChecksum)
}

type moduleChecksum map[string]string

type sourceChecksum map[string]moduleChecksum
