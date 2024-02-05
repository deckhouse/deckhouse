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

package release

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/utils/logger"
	"github.com/flant/addon-operator/pkg/values/validation"
	"github.com/flant/shell-operator/pkg/metric_storage"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned"
	d8informers "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/informers/externalversions/deckhouse.io/v1alpha1"
	d8listers "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/listers/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/models"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/docs"
	deckhouseconfig "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

// Controller is the controller implementation for ModuleRelease resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface

	// d8ClientSet is a clientset for our own API group
	d8ClientSet versioned.Interface

	moduleReleasesLister       d8listers.ModuleReleaseLister
	moduleReleasesSynced       cache.InformerSynced
	moduleSourcesLister        d8listers.ModuleSourceLister
	moduleSourcesSynced        cache.InformerSynced
	moduleUpdatePoliciesLister d8listers.ModuleUpdatePolicyLister
	moduleUpdatePoliciesSynced cache.InformerSynced
	modulePullOverridesLister  d8listers.ModulePullOverrideLister
	modulePullOverridesSynced  cache.InformerSynced
	metricStorage              *metric_storage.MetricStorage

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	logger logger.Logger

	// <module-name>: <module-source>
	sourceModules map[string]string

	modulesValidator   moduleValidator
	externalModulesDir string
	symlinksDir        string

	deckhouseEmbeddedPolicy *v1alpha1.ModuleUpdatePolicySpec

	m             sync.Mutex
	delayTimer    *time.Timer
	restartReason string

	documentationUpdater *docs.Updater
}

const (
	UpdatePolicyLabel  = "modules.deckhouse.io/update-policy"
	approvalAnnotation = "modules.deckhouse.io/approved"

	defaultCheckInterval   = 15 * time.Second
	fsReleaseFinalizer     = "modules.deckhouse.io/exist-on-fs"
	sourceReleaseFinalizer = "modules.deckhouse.io/release-exists"
	manualApprovalRequired = `Waiting for manual approval (annotation modules.deckhouse.io/approved="true" required)`
	disabledByIgnorePolicy = `Update disabled by 'Ignore' update policy`
	waitingForWindow       = "Release is waiting for the update window: %s"
)

// NewController returns a new sample controller
func NewController(ks kubernetes.Interface,
	d8ClientSet versioned.Interface,
	moduleReleaseInformer d8informers.ModuleReleaseInformer,
	moduleSourceInformer d8informers.ModuleSourceInformer,
	moduleUpdatePolicyInformer d8informers.ModuleUpdatePolicyInformer,
	modulePullOverridesInformer d8informers.ModulePullOverrideInformer,
	mv moduleValidator,
	metricStorage *metric_storage.MetricStorage,
	embeddedPolicy *v1alpha1.ModuleUpdatePolicySpec,
	documentationUpdater *docs.Updater,
) *Controller {
	ratelimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(500*time.Millisecond, 1000*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	lg := log.WithField("component", "ModuleReleaseController")

	controller := &Controller{
		kubeclientset:              ks,
		d8ClientSet:                d8ClientSet,
		moduleReleasesLister:       moduleReleaseInformer.Lister(),
		moduleReleasesSynced:       moduleReleaseInformer.Informer().HasSynced,
		moduleSourcesLister:        moduleSourceInformer.Lister(),
		moduleSourcesSynced:        moduleSourceInformer.Informer().HasSynced,
		moduleUpdatePoliciesLister: moduleUpdatePolicyInformer.Lister(),
		moduleUpdatePoliciesSynced: moduleUpdatePolicyInformer.Informer().HasSynced,
		modulePullOverridesLister:  modulePullOverridesInformer.Lister(),
		modulePullOverridesSynced:  modulePullOverridesInformer.Informer().HasSynced,
		metricStorage:              metricStorage,
		workqueue:                  workqueue.NewRateLimitingQueue(ratelimiter),
		logger:                     lg,

		sourceModules: make(map[string]string),

		modulesValidator:        mv,
		externalModulesDir:      os.Getenv("EXTERNAL_MODULES_DIR"),
		symlinksDir:             filepath.Join(os.Getenv("EXTERNAL_MODULES_DIR"), "modules"),
		deckhouseEmbeddedPolicy: embeddedPolicy,

		delayTimer: time.NewTimer(3 * time.Second),

		documentationUpdater: documentationUpdater,
	}

	// Set up an event handler for when ModuleRelease resources change
	_, err := moduleReleaseInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueModuleRelease,
		UpdateFunc: func(old, new interface{}) {
			newMS := new.(*v1alpha1.ModuleRelease)
			oldMS := old.(*v1alpha1.ModuleRelease)

			if newMS.ResourceVersion == oldMS.ResourceVersion {
				// Periodic resync will send update events for all known ModuleRelease.
				return
			}

			controller.enqueueModuleRelease(new)
		},
		DeleteFunc: controller.enqueueModuleRelease,
	})
	if err != nil {
		log.Fatalf("add event handler failed: %s", err)
	}

	return controller
}

func (c *Controller) enqueueModuleRelease(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.logger.Debugf("enqueue ModuleRelease: %s", key)
	c.workqueue.Add(key)
}

func (c *Controller) emitRestart(msg string) {
	c.m.Lock()
	c.delayTimer.Reset(3 * time.Second)
	c.restartReason = msg
	c.m.Unlock()
}

func (c *Controller) restartLoop(ctx context.Context) {
	for {
		c.m.Lock()
		select {
		case <-c.delayTimer.C:
			if c.restartReason != "" {
				c.logger.Infof("Restarting Deckhouse because %s", c.restartReason)

				err := syscall.Kill(1, syscall.SIGUSR2)
				if err != nil {
					c.logger.Fatalf("Send SIGUSR2 signal failed: %s", err)
				}
			}
			c.delayTimer.Reset(3 * time.Second)

		case <-ctx.Done():
			return
		}

		c.m.Unlock()
	}
}

func (c *Controller) Run(ctx context.Context, workers int) {
	if c.externalModulesDir == "" {
		c.logger.Info("env: 'EXTERNAL_MODULES_DIR' is empty, we are not going to work with source modules")
		return
	}

	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Check if controller's dependencies have been initialized
	_ = wait.PollUntilContextCancel(ctx, utils.SyncedPollPeriod, false,
		func(context.Context) (bool, error) {
			// TODO: add modulemanager initialization check c.modulesValidator.AreModulesInited() (required for reloading modules without restarting deckhouse)
			return deckhouseconfig.IsServiceInited(), nil
		})

	// Start the informer factories to begin populating the informer caches
	c.logger.Info("Starting ModuleRelease controller")

	// Wait for the caches to be synced before starting workers
	c.logger.Debug("Waiting for ModuleReleaseInformer caches to sync")

	go c.restartLoop(ctx)

	if ok := cache.WaitForCacheSync(ctx.Done(), c.moduleReleasesSynced, c.moduleSourcesSynced,
		c.moduleUpdatePoliciesSynced, c.modulePullOverridesSynced); !ok {
		c.logger.Fatal("failed to wait for caches to sync")
	}

	err := c.registerMetrics()
	if err != nil {
		c.logger.Errorf("register metrics: %v", err)
	}

	c.logger.Infof("Starting workers count: %d", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	<-ctx.Done()
	c.logger.Info("Shutting down workers")
}

func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

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
		c.logger.Errorf("ModuleRelease reconcile error: %s", err.Error())
		return true
	}

	return true
}

// only ModuleRelease with active finalizer can get here, we have to remove the module on filesystem and remove the finalizer
func (c *Controller) deleteReconcile(ctx context.Context, roMR *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	// deleted release
	// also cleanup the filesystem
	modulePath := path.Join(c.externalModulesDir, roMR.Spec.ModuleName, "v"+roMR.Spec.Version.String())

	err := os.RemoveAll(modulePath)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if roMR.Status.Phase == v1alpha1.PhaseDeployed {
		symlinkPath := filepath.Join(c.externalModulesDir, "modules", fmt.Sprintf("%d-%s", roMR.Spec.Weight, roMR.Spec.ModuleName))
		err := os.RemoveAll(symlinkPath)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	if !controllerutil.ContainsFinalizer(roMR, fsReleaseFinalizer) {
		return ctrl.Result{}, nil
	}

	mr := roMR.DeepCopy()
	controllerutil.RemoveFinalizer(mr, fsReleaseFinalizer)
	_, err = c.d8ClientSet.DeckhouseV1alpha1().ModuleReleases().Update(ctx, mr, metav1.UpdateOptions{})
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

func (c *Controller) createOrUpdateReconcile(ctx context.Context, roMR *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	mr := roMR.DeepCopy()

	switch mr.Status.Phase {
	case "":
		mr.Status.Phase = v1alpha1.PhasePending
		mr.Status.TransitionTime = metav1.NewTime(time.Now().UTC())
		if e := c.updateModuleReleaseStatus(ctx, mr); e != nil {
			return ctrl.Result{Requeue: true}, e
		}

		return ctrl.Result{}, nil

	case v1alpha1.PhaseSuperseded, v1alpha1.PhaseSuspended:
		// update labels
		addLabels(mr, map[string]string{"status": strings.ToLower(mr.Status.Phase)})
		if err := c.updateModuleRelease(ctx, mr); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{}, nil

	case v1alpha1.PhaseDeployed:
		err := c.documentationUpdater.SendDocumentation(ctx, mr)
		if err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("send documentation: %w", err)
		}

		// add finalizer and status label
		if !controllerutil.ContainsFinalizer(mr, fsReleaseFinalizer) {
			controllerutil.AddFinalizer(mr, fsReleaseFinalizer)
		}

		addLabels(mr, map[string]string{"status": strings.ToLower(v1alpha1.PhaseDeployed)})
		if e := c.updateModuleRelease(ctx, mr); e != nil {
			return ctrl.Result{Requeue: true}, c.updateModuleRelease(ctx, mr)
		}

		// at least one release for module source is deployed, add finalizer to prevent module source deletion
		ms, err := c.moduleSourcesLister.Get(mr.GetModuleSource())
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		if !controllerutil.ContainsFinalizer(ms, sourceReleaseFinalizer) {
			ms = ms.DeepCopy()
			controllerutil.AddFinalizer(ms, sourceReleaseFinalizer)
			_, err = c.d8ClientSet.DeckhouseV1alpha1().ModuleSources().Update(ctx, ms, metav1.UpdateOptions{})
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// if ModulePullOverride is set, don't process pending release, to avoid fs override
	exists, err := c.isModulePullOverrideExists(mr.GetModuleSource(), mr.Spec.ModuleName)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if exists {
		c.logger.Infof("ModulePullOverride for module %q exists. Skipping release processing", mr.Spec.ModuleName)
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	// process only pending releases
	return c.reconcilePendingRelease(ctx, mr)
}

func (c *Controller) isModulePullOverrideExists(sourceName, moduleName string) (bool, error) {
	res, err := c.modulePullOverridesLister.List(labels.SelectorFromValidatedSet(map[string]string{"source": sourceName, "module": moduleName}))
	if err != nil {
		return false, err
	}

	return len(res) > 0, nil
}

func (c *Controller) reconcilePendingRelease(ctx context.Context, mr *v1alpha1.ModuleRelease) (ctrl.Result, error) {
	moduleName := mr.Spec.ModuleName

	otherReleases, err := c.moduleReleasesLister.List(labels.SelectorFromValidatedSet(map[string]string{"module": moduleName}))
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	otherReleases = deepCopyList(otherReleases)

	// search symlink for module by regexp
	// module weight for a new version of the module may be different from the old one,
	// we need to find a symlink that contains the module name without looking at the weight prefix.
	currentModuleSymlink, err := findExistingModuleSymlink(c.symlinksDir, moduleName)
	if err != nil {
		currentModuleSymlink = "900-" + moduleName // fallback
	}

	var modulesChangedReason string
	defer func() {
		if modulesChangedReason != "" {
			c.emitRestart(modulesChangedReason)
		}
	}()

	nConfig, err := c.parseNotificationConfig()
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("parse notification config: %w", err)
	}

	kubeAPI := newKubeAPI(c.logger, c.d8ClientSet, c.moduleSourcesLister, c.externalModulesDir, c.symlinksDir, c.modulesValidator)
	releaseUpdater := newModuleUpdater(c.logger, nConfig, kubeAPI)

	releaseUpdater.PrepareReleases(otherReleases)
	if releaseUpdater.ReleasesCount() == 0 {
		return ctrl.Result{}, nil
	}

	releaseUpdater.PredictNextRelease()

	if releaseUpdater.LastReleaseDeployed() {
		// latest release deployed
		deployedRelease := otherReleases[releaseUpdater.GetCurrentDeployedReleaseIndex()]
		deckhouseconfig.Service().AddModuleNameToSource(deployedRelease.Spec.ModuleName, deployedRelease.GetModuleSource())
		c.sourceModules[deployedRelease.Spec.ModuleName] = deployedRelease.GetModuleSource()

		// check symlink exists on FS, relative symlink
		modulePath := generateModulePath(moduleName, deployedRelease.Spec.Version.String())
		if !isModuleExistsOnFS(c.symlinksDir, currentModuleSymlink, modulePath) {
			newModuleSymlink := path.Join(c.symlinksDir, fmt.Sprintf("%d-%s", deployedRelease.Spec.Weight, moduleName))
			c.logger.Debugf("Module %q is not exists on the filesystem. Restoring", moduleName)
			err = enableModule(c.externalModulesDir, currentModuleSymlink, newModuleSymlink, modulePath)
			if err != nil {
				c.logger.Errorf("Module restore failed: %v", err)
				if e := c.suspendModuleVersionForRelease(ctx, deployedRelease, err); e != nil {
					return ctrl.Result{Requeue: true}, e
				}

				return ctrl.Result{Requeue: true}, err
			}
			// defer restart
			modulesChangedReason = "one of modules is not enabled"
		}

		return ctrl.Result{}, nil
	}

	if releaseUpdater.GetPredictedReleaseIndex() == -1 {
		return ctrl.Result{}, nil
	}

	release := otherReleases[releaseUpdater.GetPredictedReleaseIndex()]
	var policy *v1alpha1.ModuleUpdatePolicy
	// if release has associated update policy
	if policyName, found := release.ObjectMeta.Labels[UpdatePolicyLabel]; found {
		if policyName == "" {
			policy = &v1alpha1.ModuleUpdatePolicy{
				TypeMeta: metav1.TypeMeta{
					Kind:       v1alpha1.ModuleUpdatePolicyGVK.Kind,
					APIVersion: v1alpha1.ModuleUpdatePolicyGVK.GroupVersion().String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
				Spec: *c.deckhouseEmbeddedPolicy,
			}
		} else {
			// get policy spec
			policy, err = c.moduleUpdatePoliciesLister.Get(policyName)
			if err != nil {
				if e := c.updateModuleReleaseStatusMessage(ctx, release, fmt.Sprintf("Update policy %s not found", policyName)); e != nil {
					return ctrl.Result{Requeue: true}, e
				}
				return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
			}
		}

		if policy.Spec.Update.Mode == "Ignore" {
			if e := c.updateModuleReleaseStatusMessage(ctx, release, disabledByIgnorePolicy); e != nil {
				return ctrl.Result{Requeue: true}, e
			}
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil

			//TODO: remove next block because it handled in release updater
			//case "Manual":
			//	if !release.GetApproved() {
			//		if e := c.updateModuleReleaseStatusMessage(ctx, release, manualApprovalRequired); e != nil {
			//			return ctrl.Result{Requeue: true}, e
			//		}
			//		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
			//	}
			//	release.SetApprovedStatus(true)
			//
			//case "Auto":
			//	if !policy.Spec.Update.Windows.IsAllowed(ts) {
			//		if e := c.updateModuleReleaseStatusMessage(ctx, release, fmt.Sprintf(waitingForWindow, policy.Spec.Update.Windows.NextAllowedTime(ts))); e != nil {
			//			return ctrl.Result{Requeue: true}, e
			//		}
			//		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
			//	}
		}

		//TODO: remove next block as it moved to kubeAPI.DeployRelease
		// download desired module version
		//ms, err := c.moduleSourcesLister.Get(mr.GetModuleSource())
		//if err != nil {
		//	return ctrl.Result{Requeue: true}, err
		//}
		//
		//md := downloader.NewModuleDownloader(c.externalModulesDir, ms, utils.GenerateRegistryOptions(ms))
		//ds, err := md.DownloadByModuleVersion(release.Spec.ModuleName, release.Spec.Version.String())
		//if err != nil {
		//	return ctrl.Result{RequeueAfter: defaultCheckInterval}, err
		//}
		//
		//release, err = c.updateModuleReleaseDownloadStatistic(ctx, release, ds)
		//if err != nil {
		//	return ctrl.Result{Requeue: true}, fmt.Errorf("update module release download statistic: %w", err)
		//}
		//
		//moduleVersionPath := path.Join(c.externalModulesDir, moduleName, "v"+release.Spec.Version.String())
		//relativeModulePath := generateModulePath(moduleName, release.Spec.Version.String())
		//newModuleSymlink := path.Join(c.symlinksDir, fmt.Sprintf("%d-%s", release.Spec.Weight, moduleName))
		//
		//def := models.DeckhouseModuleDefinition{
		//	Name:   moduleName,
		//	Weight: release.Spec.Weight,
		//	Path:   moduleVersionPath,
		//}
		//err = validateModule(c.modulesValidator, def)
		//if err != nil {
		//	c.logger.Errorf("Module '%s:v%s' validation failed: %s", moduleName, release.Spec.Version.String(), err)
		//	release.Status.Phase = v1alpha1.PhaseSuspended
		//	if e := c.updateModuleReleaseStatusMessage(ctx, release, "validation failed: "+err.Error()); e != nil {
		//		return ctrl.Result{Requeue: true}, e
		//	}
		//
		//	return ctrl.Result{}, nil
		//}
		//
		//err = enableModule(c.externalModulesDir, currentModuleSymlink, newModuleSymlink, relativeModulePath)
		//if err != nil {
		//	c.logger.Errorf("Module deploy failed: %v", err)
		//	if e := c.suspendModuleVersionForRelease(ctx, release, err); e != nil {
		//		return ctrl.Result{Requeue: true}, e
		//	}
		//}
		//// disable target module hooks so as not to invoke them before restart
		//if c.modulesValidator.GetModule(moduleName) != nil {
		//	c.modulesValidator.DisableModuleHooks(moduleName)
		//}
		//// after deploying a new release, mark previous one (if any) as superseded
		//if releaseUpdater.GetCurrentDeployedReleaseIndex() >= 0 {
		//	release := otherReleases[releaseUpdater.GetCurrentDeployedReleaseIndex()]
		//	release.Status.Phase = v1alpha1.PhaseSuperseded
		//	release.Status.Message = ""
		//	release.Status.TransitionTime = metav1.NewTime(time.Now().UTC())
		//	if e := c.updateModuleReleaseStatus(ctx, release); e != nil {
		//		return ctrl.Result{Requeue: true}, e
		//	}
		//}
		//
		//// defer restart
		//if modulesChangedReason == "" {
		//	modulesChangedReason = "a new module release found"
		//}
		//
		//release.Status.Phase = v1alpha1.PhaseDeployed
		//release.Status.Message = ""
		//release.Status.TransitionTime = metav1.NewTime(time.Now().UTC())
		//if e := c.updateModuleReleaseStatus(ctx, release); e != nil {
		//	return ctrl.Result{Requeue: true}, e
		//}

		releaseUpdater.SetMode(policy.Spec.Update.Mode)

		if releaseUpdater.PredictedReleaseIsPatch() {
			// patch release does not respect update windows or ManualMode
			if !releaseUpdater.ApplyPredictedRelease(nil) {
				return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
			}

			err = releaseUpdater.ChangeUpdatingFlag(false)
			if err != nil {
				return ctrl.Result{Requeue: true}, fmt.Errorf("change updating flag: %w", err)
			}

			return ctrl.Result{}, nil
		}

		var windows update.Windows
		if !releaseUpdater.InManualMode() {
			windows = policy.Spec.Update.Windows
		}

		if !releaseUpdater.ApplyPredictedRelease(windows) {
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}

		err = releaseUpdater.ChangeUpdatingFlag(false)
		if err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("change updating flag: %w", err)
		}

		modulesChangedReason = "a new module release found"

	} else {
		if e := c.updateModuleReleaseStatusMessage(ctx, mr, fmt.Sprintf("Update policy not set. Create a ModuleUpdatePolicy object and label the release '%s=<policy_name>'", UpdatePolicyLabel)); e != nil {
			return ctrl.Result{Requeue: true}, e
		}
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	releaseUpdater.SetMode(policy.Spec.Update.Mode)

	if releaseUpdater.PredictedReleaseIsPatch() {
		// patch release does not respect update windows or ManualMode
		if !releaseUpdater.ApplyPredictedRelease(nil) {
			return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
		}
		return ctrl.Result{}, nil
	}

	var windows update.Windows
	if !releaseUpdater.InManualMode() {
		windows = policy.Spec.Update.Windows
	}

	if !releaseUpdater.ApplyPredictedRelease(windows) {
		return ctrl.Result{RequeueAfter: defaultCheckInterval}, nil
	}

	modulesChangedReason = "a new module release found"

	return ctrl.Result{}, nil
}

func (c *Controller) Reconcile(ctx context.Context, releaseName string) (ctrl.Result, error) {
	// Get the ModuleRelease resource with this name
	mr, err := c.moduleReleasesLister.Get(releaseName)
	if err != nil {
		// The ModuleRelease resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	if !mr.DeletionTimestamp.IsZero() {
		return c.deleteReconcile(ctx, mr)
	}

	return c.createOrUpdateReconcile(ctx, mr)
}

func (c *Controller) suspendModuleVersionForRelease(ctx context.Context, release *v1alpha1.ModuleRelease, err error) error {
	if os.IsNotExist(err) {
		err = errors.New("not found")
	}

	release.Status.Phase = v1alpha1.PhaseSuspended
	release.Status.Message = fmt.Sprintf("Desired version of the module met problems: %s", err)
	release.Status.TransitionTime = metav1.NewTime(time.Now().UTC())

	return c.updateModuleReleaseStatus(ctx, release)
}

func enableModule(externalModulesDir, oldSymlinkPath, newSymlinkPath, modulePath string) error {
	if oldSymlinkPath != "" {
		if _, err := os.Lstat(oldSymlinkPath); err == nil {
			err = os.Remove(oldSymlinkPath)
			if err != nil {
				return err
			}
		}
	}

	if _, err := os.Lstat(newSymlinkPath); err == nil {
		err = os.Remove(newSymlinkPath)
		if err != nil {
			return err
		}
	}

	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(externalModulesDir, strings.TrimPrefix(modulePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return err
	}

	return os.Symlink(modulePath, newSymlinkPath)
}

func findExistingModuleSymlink(rootPath, moduleName string) (string, error) {
	var symlinkPath string

	moduleRegexp := regexp.MustCompile(`^(([0-9]+)-)?(` + moduleName + `)$`)
	walkDir := func(path string, d os.DirEntry, err error) error {
		if !moduleRegexp.MatchString(d.Name()) {
			return nil
		}

		symlinkPath = path
		return filepath.SkipDir
	}

	err := filepath.WalkDir(rootPath, walkDir)

	return symlinkPath, err
}

func generateModulePath(moduleName, version string) string {
	return path.Join("../", moduleName, "v"+version)
}

func isModuleExistsOnFS(symlinksDir, symlinkPath, modulePath string) bool {
	targetPath, err := filepath.EvalSymlinks(symlinkPath)
	if err != nil {
		return false
	}

	if filepath.IsAbs(targetPath) {
		targetPath, err = filepath.Rel(symlinksDir, targetPath)
		if err != nil {
			return false
		}
	}

	return targetPath == modulePath
}

func addLabels(mr *v1alpha1.ModuleRelease, labels map[string]string) {
	lb := mr.GetLabels()
	if len(lb) == 0 {
		mr.SetLabels(labels)
	} else {
		for l, v := range labels {
			lb[l] = v
		}
	}
}

// updateModuleReleaseStatusMessage updates module release's `.status.message field
func (c *Controller) updateModuleReleaseStatusMessage(ctx context.Context, mrCopy *v1alpha1.ModuleRelease, message string) error {
	if mrCopy.Status.Message == message {
		return nil
	}

	mrCopy.Status.Message = message

	err := c.updateModuleReleaseStatus(ctx, mrCopy)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) updateModuleReleaseStatus(ctx context.Context, mrCopy *v1alpha1.ModuleRelease) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	_, err := c.d8ClientSet.DeckhouseV1alpha1().ModuleReleases().UpdateStatus(ctx, mrCopy, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) updateModuleRelease(ctx context.Context, mrCopy *v1alpha1.ModuleRelease) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	_, err := c.d8ClientSet.DeckhouseV1alpha1().ModuleReleases().Update(ctx, mrCopy, metav1.UpdateOptions{})
	return err
}

// RunPreflightCheck start a few checks and synchronize deckhouse filesystem with ModuleReleases
//   - Download modules, which have status=deployed on ModuleRelease but have no files on Filesystem
//   - Delete modules, that don't have ModuleRelease presented in the cluster
func (c *Controller) RunPreflightCheck(ctx context.Context) error {
	if c.externalModulesDir == "" {
		return nil
	}

	if ok := cache.WaitForCacheSync(ctx.Done(), c.moduleReleasesSynced, c.moduleSourcesSynced, c.moduleUpdatePoliciesSynced, c.modulePullOverridesSynced); !ok {
		c.logger.Fatal("failed to wait for caches to sync")
	}
	c.logger.Info("Release controller's object cache synced")

	err := c.restoreAbsentSourceModules()
	if err != nil {
		return fmt.Errorf("modules restoration failed: %w", err)
	}

	return c.deleteModulesWithAbsentRelease()
}

func (c *Controller) deleteModulesWithAbsentRelease() error {
	symlinksDir := filepath.Join(c.externalModulesDir, "modules")

	fsModulesLinks, err := c.readModulesFromFS(symlinksDir)
	if err != nil {
		return fmt.Errorf("read source modules from the filesystem failed: %w", err)
	}

	releases, err := c.moduleReleasesLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("fetch ModuleReleases failed: %w", err)
	}

	c.logger.Debugf("%d ModuleReleases found", len(releases))

	for _, release := range releases {
		c.sourceModules[release.Spec.ModuleName] = release.GetModuleSource()
		delete(fsModulesLinks, release.Spec.ModuleName)
	}

	for module, moduleLinkPath := range fsModulesLinks {
		_, err = c.modulePullOverridesLister.Get(module)
		if err != nil && apierrors.IsNotFound(err) {
			c.logger.Warnf("Module %q has neither ModuleRelease nor ModuleOverride. Purging from FS", module)
			_ = os.RemoveAll(moduleLinkPath)
		}
	}

	return nil
}

func (c *Controller) GetModuleSources() map[string]string {
	return c.sourceModules
}

func (c *Controller) readModulesFromFS(dir string) (map[string]string, error) {
	moduleLinks, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	modules := make(map[string]string, len(moduleLinks))

	for _, moduleLink := range moduleLinks {
		index := strings.Index(moduleLink.Name(), "-")
		if index == -1 {
			continue
		}

		moduleName := moduleLink.Name()[index+1:]
		modules[moduleName] = path.Join(dir, moduleLink.Name())
	}

	return modules, nil
}

// restoreAbsentSourceModules checks ModuleReleases with Deployed status and restore them on the FS
func (c *Controller) restoreAbsentSourceModules() error {
	releaseList, err := c.d8ClientSet.DeckhouseV1alpha1().ModuleReleases().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// TODO: add labels to list only Deployed releases
	for _, item := range releaseList.Items {
		if item.Status.Phase != "Deployed" {
			continue
		}

		moduleWeight := item.Spec.Weight
		moduleVersion := "v" + item.Spec.Version.String()
		moduleName := item.Spec.ModuleName
		moduleSource := item.GetModuleSource()

		// if ModulePullOverride is set, don't check and restore overridden release
		exists, err := c.isModulePullOverrideExists(moduleSource, moduleName)
		if err != nil {
			c.logger.Errorf("Couldn't check module pull override for module %s: %s", moduleName, err)
		}

		if exists {
			c.logger.Infof("ModulePullOverride for module %q exists. Skipping release restore", moduleName)
			continue
		}

		moduleDir := filepath.Join(c.symlinksDir, fmt.Sprintf("%d-%s", item.Spec.Weight, item.Spec.ModuleName))
		_, err = os.Stat(moduleDir)
		if err != nil {
			// module dir not found
			if os.IsNotExist(err) {
				err := c.createModuleSymlink(moduleName, moduleVersion, moduleSource, moduleWeight)
				if err != nil {
					return fmt.Errorf("couldn't create module symlink: %s", err)
				}
				// some other error
			} else {
				return fmt.Errorf("module %s check error: %s", moduleName, err)
			}
			// check if module versions is up to date
		} else {
			dstDir, err := filepath.EvalSymlinks(moduleDir)
			if err != nil {
				return fmt.Errorf("couldn't evaluate module %s symlink %s: %s", moduleName, moduleDir, err)
			}

			// module version on file system doesn't equal to the deployed module release
			if filepath.Base(dstDir) != moduleVersion {
				if err := os.Remove(moduleDir); err != nil {
					return fmt.Errorf("couldn't delete stale symlink %s for module %s: %s", moduleDir, moduleName, err)
				}
				if err := c.createModuleSymlink(moduleName, moduleVersion, moduleSource, moduleWeight); err != nil {
					return fmt.Errorf("couldn't create module symlink: %s", err)
				}
			}
		}
	}

	// restoring modules from MPO
	mpoList, err := c.d8ClientSet.DeckhouseV1alpha1().ModulePullOverrides().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, item := range mpoList.Items {
		moduleName := item.Name
		moduleSource := item.Spec.Source
		moduleImageTag := item.Spec.ImageTag

		ms, err := c.moduleSourcesLister.Get(moduleSource)
		if err != nil {
			return fmt.Errorf("ModuleSource %s is absent. Skipping restoration of the module %s with pull override", moduleSource, moduleName)
		}

		md := downloader.NewModuleDownloader(c.externalModulesDir, ms, utils.GenerateRegistryOptions(ms))
		_, moduleDef, err := md.DownloadDevImageTag(moduleName, moduleImageTag, "")
		if err != nil {
			return fmt.Errorf("couldn't get module %s pull override definition: %s", moduleName, err)
		}

		if moduleDef == nil {
			return fmt.Errorf("module definition for module %s pull override is nil. Ignore", moduleName)
		}

		moduleWeight := moduleDef.Weight
		moduleDir := filepath.Join(c.symlinksDir, fmt.Sprintf("%d-%s", moduleWeight, moduleName))
		_, err = os.Stat(moduleDir)
		if err != nil {
			// module dir not found
			if os.IsNotExist(err) {
				err := c.deleteStaleSymlink(moduleName)
				if err != nil {
					c.logger.Warnf("%s", err)
				}

				// restore symlink
				moduleRelativePath := filepath.Join("../", moduleName, "dev")
				symlinkPath := filepath.Join(c.symlinksDir, fmt.Sprintf("%d-%s", moduleWeight, moduleName))
				err = restoreModuleSymlink(c.externalModulesDir, symlinkPath, moduleRelativePath)
				if err != nil {
					return fmt.Errorf("create symlink for module %s failed: %s", moduleName, err)
				}

				log.Infof("Module %s with pull override restored", moduleName)
				// some other error
			} else {
				return fmt.Errorf("module %s with pull override check error: %s", moduleName, err)
			}
		}
	}
	return nil
}

// deleteStaleSymlink checks if there is a symlink for the module with different weight in the symlink folder
// and deletes it
func (c *Controller) deleteStaleSymlink(moduleName string) error {
	anotherModuleSymlink, err := findExistingModuleSymlink(c.symlinksDir, moduleName)
	if err != nil {
		return fmt.Errorf("Couldn't check if there are any other symlinks for module %v: %w", moduleName, err)
	}
	if len(anotherModuleSymlink) > 0 {
		if err := os.Remove(anotherModuleSymlink); err != nil {
			return fmt.Errorf("Couldn't delete stale symlink %v for module %v: %w", anotherModuleSymlink, moduleName, err)
		}
	}

	return nil
}

// createModuleSymlink checks if there is a stale symlink for a module in the symlink dir and deletes it before
// attempting to download current version of the module and creating correct symlink
func (c *Controller) createModuleSymlink(moduleName, moduleVersion, moduleSource string, moduleWeight uint32) error {
	log.Infof("Module %q is absent on file system. Restoring it from source %q", moduleName, moduleSource)

	err := c.deleteStaleSymlink(moduleName)
	if err != nil {
		return err
	}

	ms, err := c.moduleSourcesLister.Get(moduleSource)
	if err != nil {
		return fmt.Errorf("ModuleSource %v is absent. Skipping restoration of the module %v", moduleSource, moduleName)
	}

	md := downloader.NewModuleDownloader(c.externalModulesDir, ms, utils.GenerateRegistryOptions(ms))
	_, err = md.DownloadByModuleVersion(moduleName, moduleVersion)
	if err != nil {
		return fmt.Errorf("Download module %v with version %v failed: %w. Skipping", moduleName, moduleVersion, err)
	}

	// restore symlink
	moduleRelativePath := filepath.Join("../", moduleName, moduleVersion)
	symlinkPath := filepath.Join(c.symlinksDir, fmt.Sprintf("%d-%s", moduleWeight, moduleName))
	err = restoreModuleSymlink(c.externalModulesDir, symlinkPath, moduleRelativePath)
	if err != nil {
		return fmt.Errorf("Create symlink for module %v failed: %w", moduleName, err)
	}
	log.Infof("Module %s:%s restored", moduleName, moduleVersion)

	return nil
}

func (c *Controller) parseNotificationConfig() (*updater.NotificationConfig, error) {
	secret, err := c.kubeclientset.CoreV1().Secrets("d8-system").Get(context.Background(), "deckhouse-discovery", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get secret: %w", err)
	}

	jsonSettings, ok := secret.Data["updateSettings.json"]
	if !ok {
		return new(updater.NotificationConfig), nil
	}

	var settings struct {
		NotificationConfig *updater.NotificationConfig `json:"notification"`
	}

	err = json.Unmarshal(jsonSettings, &settings)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}

	return settings.NotificationConfig, nil
}

func validateModule(validator moduleValidator, def models.DeckhouseModuleDefinition) error {
	if def.Weight < 900 || def.Weight > 999 {
		return fmt.Errorf("external module weight must be between 900 and 999")
	}

	if def.Path == "" {
		return fmt.Errorf("cannot validate module without path. Path is required to load openapi specs")
	}

	dm := models.NewDeckhouseModule(def, addonutils.Values{}, validator.GetValuesValidator())
	err := validator.ValidateModule(dm.GetBasicModule())
	if err != nil {
		return err
	}

	return nil
}

func restoreModuleSymlink(externalModulesDir, symlinkPath, moduleRelativePath string) error {
	// make absolute path for versioned module
	moduleAbsPath := filepath.Join(externalModulesDir, strings.TrimPrefix(moduleRelativePath, "../"))
	// check that module exists on a disk
	if _, err := os.Stat(moduleAbsPath); os.IsNotExist(err) {
		return err
	}

	return os.Symlink(moduleRelativePath, symlinkPath)
}

type moduleValidator interface {
	ValidateModule(m *addonmodules.BasicModule) error
	GetValuesValidator() *validation.ValuesValidator
	DisableModuleHooks(moduleName string)
	GetModule(moduleName string) *addonmodules.BasicModule
}

func (c *Controller) updateModuleReleaseDownloadStatistic(ctx context.Context, release *v1alpha1.ModuleRelease,
	ds *downloader.DownloadStatistic) (*v1alpha1.ModuleRelease, error) {
	release.Status.Size = ds.Size
	release.Status.PullDuration = metav1.Duration{Duration: ds.PullDuration}

	return c.d8ClientSet.DeckhouseV1alpha1().ModuleReleases().UpdateStatus(ctx, release, metav1.UpdateOptions{})
}

func (c *Controller) registerMetrics() error {
	releases, err := c.moduleReleasesLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("list module releases: %w", err)
	}

	for _, release := range releases {
		l := map[string]string{
			"version": release.Spec.Version.String(),
			"module":  release.Spec.ModuleName,
		}

		c.metricStorage.GaugeSet("{PREFIX}module_pull_seconds_total", release.Status.PullDuration.Seconds(), l)
		c.metricStorage.GaugeSet("{PREFIX}module_size_bytes_total", float64(release.Status.Size), l)
	}

	return nil
}

type deepCopier[T any] interface {
	DeepCopy() T
}

func deepCopyList[T deepCopier[T]](list []T) []T {
	result := make([]T, len(list))

	for i := range list {
		result[i] = list[i].DeepCopy()
	}

	return result
}
