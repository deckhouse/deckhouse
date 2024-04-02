// Copyright 2023 Flant JSC
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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/utils/logger"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned"
	d8informers "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/informers/externalversions/deckhouse.io/v1alpha1"
	d8listers "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/listers/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/release"
	controllerUtils "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

// Controller is the controller implementation for ModuleSource resources
type Controller struct {
	// kubeClient is a clientset for our own API group
	kubeClient versioned.Interface

	moduleSourcesLister        d8listers.ModuleSourceLister
	moduleSourcesSynced        cache.InformerSynced
	moduleReleasesLister       d8listers.ModuleReleaseLister
	moduleReleasesSynced       cache.InformerSynced
	moduleUpdatePoliciesLister d8listers.ModuleUpdatePolicyLister
	moduleUpdatePoliciesSynced cache.InformerSynced
	modulePullOverridesLister  d8listers.ModulePullOverrideLister
	modulePullOverridesSynced  cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	logger logger.Logger

	externalModulesDir string

	deckhouseEmbeddedPolicy *v1alpha1.ModuleUpdatePolicySpec

	rwlock                sync.RWMutex
	moduleSourcesChecksum sourceChecksum
}

// NewController returns a new ModuleSource controller
func NewController(
	kubeClient versioned.Interface,
	moduleSourceInformer d8informers.ModuleSourceInformer,
	moduleReleaseInformer d8informers.ModuleReleaseInformer,
	moduleUpdatePolicyInformer d8informers.ModuleUpdatePolicyInformer,
	modulePullOverridesInformer d8informers.ModulePullOverrideInformer,
	embeddedPolicy *v1alpha1.ModuleUpdatePolicySpec,
) *Controller {
	ratelimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(500*time.Millisecond, 1000*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	lg := log.WithField("component", "ModuleSourceController")

	controller := &Controller{
		kubeClient:                 kubeClient,
		moduleSourcesLister:        moduleSourceInformer.Lister(),
		moduleSourcesSynced:        moduleSourceInformer.Informer().HasSynced,
		moduleReleasesLister:       moduleReleaseInformer.Lister(),
		moduleReleasesSynced:       moduleReleaseInformer.Informer().HasSynced,
		moduleUpdatePoliciesLister: moduleUpdatePolicyInformer.Lister(),
		moduleUpdatePoliciesSynced: moduleUpdatePolicyInformer.Informer().HasSynced,
		modulePullOverridesLister:  modulePullOverridesInformer.Lister(),
		modulePullOverridesSynced:  modulePullOverridesInformer.Informer().HasSynced,
		workqueue:                  workqueue.NewRateLimitingQueue(ratelimiter),

		logger: lg,

		externalModulesDir:      os.Getenv("EXTERNAL_MODULES_DIR"),
		moduleSourcesChecksum:   make(sourceChecksum),
		deckhouseEmbeddedPolicy: embeddedPolicy,
	}

	// Set up an event handler for when ModuleSource resources change
	_, _ = moduleSourceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueModuleSource,
		UpdateFunc: func(old, new interface{}) {
			newMS := new.(*v1alpha1.ModuleSource)
			oldMS := old.(*v1alpha1.ModuleSource)

			if newMS.Generation == oldMS.Generation {
				// Periodic resync will send update events for all known ModuleSources.
				return
			}

			controller.enqueueModuleSource(new)
		},
		DeleteFunc: controller.enqueueModuleSource,
	})

	return controller
}

// ModuleSource takes a ModuleSource resource and converts it into a name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than ModuleSource.
func (c *Controller) enqueueModuleSource(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.logger.Debugf("enqueue ModuleSource: %s", key)
	c.workqueue.Add(key)
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(ctx context.Context, workers int) {
	if c.externalModulesDir == "" {
		c.logger.Info("env: 'EXTERNAL_MODULES_DIR' is empty, we are not going to work with source modules")
		return
	}

	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	c.logger.Info("Starting ModuleSource controller")

	// Wait for the caches to be synced before starting workers
	c.logger.Debug("Waiting for ModuleSourceInformer caches to sync")

	if ok := cache.WaitForCacheSync(ctx.Done(), c.moduleSourcesSynced, c.moduleReleasesSynced, c.moduleUpdatePoliciesSynced, c.modulePullOverridesSynced); !ok {
		c.logger.Fatal("failed to wait for caches to sync")
	}

	c.logger.Infof("Starting workers count: %d", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	<-ctx.Done()
	c.logger.Info("Shutting down workers")
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			c.logger.Errorf("expected string in workqueue but got %#v", obj)
			return nil
		}

		// run reconcile loop
		result, err := c.Reconcile(ctx, key)
		switch {
		case result.RequeueAfter != 0:
			c.workqueue.AddAfter(key, result.RequeueAfter)

		case result.Requeue:
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)

		default:
			c.workqueue.Forget(key)
		}

		return err
	}(obj)

	if err != nil {
		c.logger.Errorf("ModuleSource reconcile error: %s", err.Error())
		return true
	}

	return true
}

type moduleChecksum map[string]string

type sourceChecksum map[string]moduleChecksum

func (c *Controller) getModuleSourceChecksum(msName string) moduleChecksum {
	c.rwlock.RLock()
	defer c.rwlock.RUnlock()

	res, ok := c.moduleSourcesChecksum[msName]
	if ok {
		return res
	}

	return make(moduleChecksum)
}

const (
	defaultScanInterval = 3 * time.Minute
)

var (
	ErrNoPolicyFound = errors.New("no matching update policy found")

	checksumAnnotation = "modules.deckhouse.io/registry-spec-checksum"
)

func (c *Controller) createOrUpdateReconcile(ctx context.Context, roMS *v1alpha1.ModuleSource) (ctrl.Result, error) {
	modulesErrorsMap := make(map[string]string)
	for _, moduleError := range roMS.Status.ModuleErrors {
		modulesErrorsMap[moduleError.Name] = moduleError.Error
	}

	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	ms := roMS.DeepCopy()

	ms.Status.Msg = ""
	ms.Status.ModuleErrors = make([]v1alpha1.ModuleError, 0)

	opts := controllerUtils.GenerateRegistryOptions(ms)

	regCli, err := cr.NewClient(ms.Spec.Registry.Repo, opts...)
	if err != nil {
		ms.Status.Msg = err.Error()
		if e := c.updateModuleSourceStatus(ms); e != nil {
			return ctrl.Result{Requeue: true}, e
		}

		// error can occur on wrong auth only, we don't want to requeue the source until auth is fixed
		return ctrl.Result{Requeue: false}, err
	}

	moduleNames, err := regCli.ListTags()
	if err != nil {
		ms.Status.Msg = err.Error()
		if e := c.updateModuleSourceStatus(ms); e != nil {
			return ctrl.Result{Requeue: true}, e
		}
		return ctrl.Result{Requeue: true}, err
	}

	// check, by means of comparing registry settings to the checkSum annotation, if new registry settings should be propagated to deployed module release
	updateNeeded, err := c.checkAndPropagateRegistrySettings(ms)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	// new registry settings checksum should be applied to module source
	if updateNeeded {
		if _, err := c.kubeClient.DeckhouseV1alpha1().ModuleSources().Update(context.TODO(), ms, metav1.UpdateOptions{}); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		// reque ms after modifying annotation
		return ctrl.Result{Requeue: true}, nil
	}

	sort.Strings(moduleNames)

	// form available modules structure
	availableModules := make([]v1alpha1.AvailableModule, 0, len(moduleNames))

	ms.Status.ModulesCount = len(moduleNames)

	modulesChecksums := c.getModuleSourceChecksum(ms.Name)

	md := downloader.NewModuleDownloader(c.externalModulesDir, ms, opts)

	// get all policies regardless of their labels
	policies, err := c.moduleUpdatePoliciesLister.List(labels.Everything())
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	for _, moduleName := range moduleNames {
		if moduleName == "modules" {
			c.logger.Warn("'modules' name for module is forbidden. Skip module.")
			continue
		}

		newChecksum, av, err := c.processSourceModule(ctx, md, ms, moduleName, modulesChecksums[moduleName], policies)
		availableModules = append(availableModules, av)
		if err != nil {
			modulesErrorsMap[moduleName] = err.Error()
			continue
		}

		delete(modulesErrorsMap, moduleName)

		if newChecksum != "" {
			modulesChecksums[moduleName] = newChecksum
		}
	}

	ms.Status.AvailableModules = availableModules

	if len(modulesErrorsMap) > 0 {
		ms.Status.Msg = "Some errors occurred. Inspect status for details"
		for moduleName, moduleError := range modulesErrorsMap {
			ms.Status.ModuleErrors = append(ms.Status.ModuleErrors, v1alpha1.ModuleError{Name: moduleName, Error: moduleError})
		}
	}

	err = c.updateModuleSourceStatus(ms)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// save checksums
	c.saveSourceChecksums(ms.Name, modulesChecksums)

	// everything is ok, check source on the other iteration
	return ctrl.Result{RequeueAfter: defaultScanInterval}, nil
}

func (c *Controller) processSourceModule(ctx context.Context, md *downloader.ModuleDownloader, ms *v1alpha1.ModuleSource, moduleName, moduleChecksum string, policies []*v1alpha1.ModuleUpdatePolicy) ( /*checksum*/ string, v1alpha1.AvailableModule, error) {
	av := v1alpha1.AvailableModule{
		Name:       moduleName,
		Policy:     "",
		Overridden: false,
	}
	// check if we have ModulePullOverride for source/module
	exists, err := c.isModulePullOverrideExists(ms.Name, moduleName)
	if err != nil {
		log.Warnf("Unexpected error on getting ModulePullOverride for %s/%s", ms.Name, moduleName)
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

func (c *Controller) isModulePullOverrideExists(sourceName, moduleName string) (bool, error) {
	res, err := c.modulePullOverridesLister.List(labels.SelectorFromValidatedSet(map[string]string{"source": sourceName, "module": moduleName}))
	if err != nil {
		return false, err
	}

	return len(res) > 0, nil
}

func (c *Controller) Reconcile(ctx context.Context, sourceName string) (ctrl.Result, error) {
	// Get the ModuleSource resource with this name
	ms, err := c.moduleSourcesLister.Get(sourceName)
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
		return c.deleteReconcile(ctx, ms)
	}

	return c.createOrUpdateReconcile(ctx, ms)
}

func (c *Controller) deleteReconcile(ctx context.Context, ms *v1alpha1.ModuleSource) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(ms, "modules.deckhouse.io/release-exists") {
		v := ms.GetAnnotations()["modules.deckhouse.io/force-delete"]

		if v != "true" {
			// check releases
			releases, err := c.moduleReleasesLister.List(labels.SelectorFromValidatedSet(map[string]string{"source": ms.Name, "status": "deployed"}))
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}

			if len(releases) > 0 {
				ms = ms.DeepCopy()
				ms.Status.Msg = "ModuleSource contains at least 1 Deployed release and cannot be deleted. Please delete ModuleRelease manually to continue"
				if err := c.updateModuleSourceStatus(ms); err != nil {
					return ctrl.Result{Requeue: true}, nil
				}

				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
		}

		ms = ms.DeepCopy()
		controllerutil.RemoveFinalizer(ms, "modules.deckhouse.io/release-exists")
		_, err := c.kubeClient.DeckhouseV1alpha1().ModuleSources().Update(ctx, ms, metav1.UpdateOptions{})
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	c.saveSourceChecksums(ms.Name, make(moduleChecksum))
	return ctrl.Result{}, nil
}

func (c *Controller) createModuleRelease(ctx context.Context, ms *v1alpha1.ModuleSource, moduleName, policyName string, result downloader.ModuleDownloadResult) error {
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

	_, err := c.kubeClient.DeckhouseV1alpha1().ModuleReleases().Create(ctx, rl, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			prev, err := c.kubeClient.DeckhouseV1alpha1().ModuleReleases().Get(ctx, rl.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			// seems weird to update already deployed/suspended release
			if prev.Status.Phase != v1alpha1.PhasePending {
				return nil
			}

			prev.Spec = rl.Spec
			_, err = c.kubeClient.DeckhouseV1alpha1().ModuleReleases().Update(ctx, prev, metav1.UpdateOptions{})
			return err
		}

		return err
	}
	return nil
}

func (c *Controller) saveSourceChecksums(msName string, checksums moduleChecksum) {
	c.rwlock.Lock()
	c.moduleSourcesChecksum[msName] = checksums
	c.rwlock.Unlock()
}

func (c *Controller) updateModuleSourceStatus(msCopy *v1alpha1.ModuleSource) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	msCopy.Status.SyncTime = metav1.NewTime(time.Now().UTC())

	_, err := c.kubeClient.DeckhouseV1alpha1().ModuleSources().UpdateStatus(context.TODO(), msCopy, metav1.UpdateOptions{})
	return err
}

// getReleasePolicy checks if any update policy matches the module release and if it's so - returns the policy and its release channel.
// if several policies match the module release labels, conflict=true is returned
func (c *Controller) getReleasePolicy(sourceName, moduleName string, policies []*v1alpha1.ModuleUpdatePolicy) (*v1alpha1.ModuleUpdatePolicy, error) {
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
				matchedPolicy = policy
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

// checkAndPropagateRegistrySettings checks if modules source registry settings were updated (comparing checksumAnnotation annotation and current registry spec)
// and update relevant module releases' openapi values files if it the case
func (c *Controller) checkAndPropagateRegistrySettings(msCopy *v1alpha1.ModuleSource) ( /* update required */ bool, error) {
	// get registry settings checksum
	currentChecksum := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s/%s", msCopy.Spec.Registry.Repo, msCopy.Spec.Registry.DockerCFG))))
	// if there is no annotations - only set the current checksum value
	if msCopy.ObjectMeta.Annotations == nil {
		msCopy.ObjectMeta.Annotations = make(map[string]string)
		msCopy.ObjectMeta.Annotations[checksumAnnotation] = currentChecksum
		return true, nil
	}

	// if the annotation matches current checksum - there is nothing to do here
	if msCopy.ObjectMeta.Annotations[checksumAnnotation] == currentChecksum {
		return false, nil
	}

	// get related releases
	moduleReleasesFromSource, err := c.moduleReleasesLister.List(labels.SelectorFromSet(labels.Set{"source": msCopy.Name}))
	if err != nil {
		return false, fmt.Errorf("could list module releases to update registry settings: %w", err)
	}

	for _, release := range moduleReleasesFromSource {
		if release.Status.Phase == v1alpha1.PhaseDeployed {
			ownerReferences := release.GetOwnerReferences()
			for _, ref := range ownerReferences {
				if ref.UID == msCopy.UID && ref.Name == msCopy.Name && ref.Kind == "ModuleSource" {
					err = downloader.InjectRegistryToModuleValues(filepath.Join(os.Getenv("EXTERNAL_MODULES_DIR"), release.Spec.ModuleName, fmt.Sprintf("v%s", release.Spec.Version)), msCopy)
					if err != nil {
						return false, fmt.Errorf("couldn't update module release %v registry settings: %w", release.Name, err)
					}
					break
				}
			}
		}
	}
	msCopy.ObjectMeta.Annotations[checksumAnnotation] = currentChecksum

	return true, nil
}
