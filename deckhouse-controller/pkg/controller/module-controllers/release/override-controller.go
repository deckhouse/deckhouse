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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/flant/addon-operator/pkg/utils/logger"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned"
	d8informers "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/informers/externalversions/deckhouse.io/v1alpha1"
	d8listers "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/listers/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	sm "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/source_modules"
	deckhouseconfig "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
)

// ModulePullOverrideController is the controller implementation for ModulePullOverride resources
type ModulePullOverrideController struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface

	// d8ClientSet is a clientset for our own API group
	d8ClientSet versioned.Interface

	moduleSourcesLister d8listers.ModuleSourceLister
	moduleSourcesSynced cache.InformerSynced

	modulePullOverridesLister d8listers.ModulePullOverrideLister
	modulePullOverridesSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface

	logger logger.Logger

	modulesValidator   moduleValidator
	externalModulesDir string
	symlinksDir        string
	sourceModules      *sm.SourceModules
}

// NewModulePullOverrideController returns a new sample controller
func NewModulePullOverrideController(ks kubernetes.Interface,
	d8ClientSet versioned.Interface,
	moduleSourceInformer d8informers.ModuleSourceInformer,
	modulePullOverridesInformer d8informers.ModulePullOverrideInformer,
	modulesValidator moduleValidator,
	sourceModules *sm.SourceModules,
) *ModulePullOverrideController {
	ratelimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(500*time.Millisecond, 1000*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	lg := log.WithField("component", "ModulePullOverrideController")

	controller := &ModulePullOverrideController{
		kubeclientset:             ks,
		d8ClientSet:               d8ClientSet,
		moduleSourcesLister:       moduleSourceInformer.Lister(),
		moduleSourcesSynced:       moduleSourceInformer.Informer().HasSynced,
		modulePullOverridesLister: modulePullOverridesInformer.Lister(),
		modulePullOverridesSynced: modulePullOverridesInformer.Informer().HasSynced,

		workqueue: workqueue.NewRateLimitingQueue(ratelimiter),

		logger: lg,

		sourceModules: sourceModules,

		modulesValidator:   modulesValidator,
		externalModulesDir: os.Getenv("EXTERNAL_MODULES_DIR"),
		symlinksDir:        filepath.Join(os.Getenv("EXTERNAL_MODULES_DIR"), "modules"),
	}

	_, err := modulePullOverridesInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueModuleOverride,
		UpdateFunc: func(old, new interface{}) {
			newM := new.(*v1alpha1.ModulePullOverride)
			oldM := old.(*v1alpha1.ModulePullOverride)

			if newM.Spec == oldM.Spec {
				if _, ok := newM.Labels["renew"]; ok {
					controller.enqueueModuleOverride(newM)
				}
				return
			}

			controller.enqueueModuleOverride(newM)
		},
	})
	if err != nil {
		log.Fatalf("add event handler failed: %s", err)
	}

	return controller
}

func (c *ModulePullOverrideController) Run(ctx context.Context, workers int) {
	if c.externalModulesDir == "" {
		c.logger.Info("env: 'EXTERNAL_MODULES_DIR' is empty, we are not going to work with source modules")
		return
	}

	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Check if controller's dependencies have been initialized
	_ = wait.PollUntilContextCancel(ctx, utils.SyncedPollPeriod, false,
		func(context.Context) (bool, error) {
			return deckhouseconfig.IsServiceInited() && c.modulesValidator.AreModulesInited(), nil
		})

	// Start the informer factories to begin populating the informer caches
	c.logger.Info("Starting controller")

	// Wait for the caches to be synced before starting workers
	c.logger.Debug("Waiting for caches to sync")

	if ok := cache.WaitForCacheSync(ctx.Done(), c.moduleSourcesSynced, c.modulePullOverridesSynced); !ok {
		c.logger.Fatal("failed to wait for caches to sync")
	}

	c.logger.Infof("Starting workers count: %d", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	<-ctx.Done()
	c.logger.Info("Shutting down workers")
}

func (c *ModulePullOverrideController) enqueueModuleOverride(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.logger.Debugf("enqueue ModuleOverride: %s", key)
	c.workqueue.Add(key)
}

func (c *ModulePullOverrideController) runWorker(ctx context.Context) {
	for c.processNextModuleOverride(ctx) {
	}
}

func (c *ModulePullOverrideController) processNextModuleOverride(ctx context.Context) bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			c.logger.Errorf("expected string in workqueue but got %#v", obj)
			return nil
		}

		// run reconcile loop
		result, err := c.OverrideReconcile(ctx, key)
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
		c.logger.Errorf("ModulePullOverride reconcile error: %s", err.Error())
		return true
	}

	return true
}

func (c *ModulePullOverrideController) OverrideReconcile(ctx context.Context, key string) (ctrl.Result, error) {
	mo, err := c.modulePullOverridesLister.Get(key)
	if err != nil {
		// The ModulePullOverride resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	return c.moduleOverrideReconcile(ctx, mo)
}

func (c *ModulePullOverrideController) moduleOverrideReconcile(ctx context.Context, moRO *v1alpha1.ModulePullOverride) (ctrl.Result, error) {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	mo := moRO.DeepCopy()

	// add labels if empty
	// source and release controllers are looking for this labels
	if _, ok := mo.Labels["module"]; !ok {
		if len(mo.Labels) > 0 {
			mo.Labels["module"] = mo.Name
			mo.Labels["source"] = mo.Spec.Source
		} else {
			mo.SetLabels(map[string]string{"module": mo.Name, "source": mo.Spec.Source})
		}
		_, err := c.d8ClientSet.DeckhouseV1alpha1().ModulePullOverrides().Update(ctx, mo, metav1.UpdateOptions{})
		return ctrl.Result{RequeueAfter: 500 * time.Millisecond}, err
	}

	ms, err := c.moduleSourcesLister.Get(mo.Spec.Source)
	if err != nil {
		if apierrors.IsNotFound(err) {
			mo.Status.Message = fmt.Sprintf("ModuleSource %q not found", mo.Spec.Source)
			if e := c.updateModulePullOverrideStatus(ctx, mo); e != nil {
				return ctrl.Result{Requeue: true}, e
			}
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	c.sourceModules.SetSource(mo.Name, mo.Spec.Source)

	md := downloader.NewModuleDownloader(c.externalModulesDir, ms, utils.GenerateRegistryOptions(ms))
	newChecksum, moduleDef, err := md.DownloadDevImageTag(mo.Name, mo.Spec.ImageTag, mo.Status.ImageDigest)
	if err != nil {
		mo.Status.Message = err.Error()
		if e := c.updateModulePullOverrideStatus(ctx, mo); e != nil {
			return ctrl.Result{Requeue: true}, e
		}
		return ctrl.Result{RequeueAfter: mo.Spec.ScanInterval.Duration}, nil
	}

	if newChecksum == "" {
		// module is up-to-date
		if mo.Status.Message != "" {
			// drop error message, if exists
			mo.Status.Message = ""
			if e := c.updateModulePullOverrideStatus(ctx, mo); e != nil {
				return ctrl.Result{Requeue: true}, e
			}
		}
		return ctrl.Result{RequeueAfter: mo.Spec.ScanInterval.Duration}, nil
	}

	// what if it is nil?
	if moduleDef != nil {
		err = validateModule(c.modulesValidator, *moduleDef)
		if err != nil {
			mo.Status.Message = fmt.Sprintf("validation failed: %s", err)
			if e := c.updateModulePullOverrideStatus(ctx, mo); e != nil {
				return ctrl.Result{Requeue: true}, e
			}

			return ctrl.Result{RequeueAfter: mo.Spec.ScanInterval.Duration}, nil
		}
	}

	symlinkPath := filepath.Join(c.symlinksDir, fmt.Sprintf("%d-%s", moduleDef.Weight, mo.Name))
	err = c.enableModule(mo.Name, symlinkPath)
	if err != nil {
		mo.Status.Message = err.Error()
		if e := c.updateModulePullOverrideStatus(ctx, mo); e != nil {
			return ctrl.Result{Requeue: true}, e
		}

		return ctrl.Result{Requeue: true}, err
	}

	mo.Status.Message = ""
	mo.Status.ImageDigest = newChecksum

	if err := c.updateModulePullOverrideStatus(ctx, mo); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if _, ok := mo.Labels["renew"]; ok {
		delete(mo.Labels, "renew")
		_, _ = c.d8ClientSet.DeckhouseV1alpha1().ModulePullOverrides().Update(ctx, mo, metav1.UpdateOptions{})
	}

	// register/reload module
	err = c.modulesValidator.RegisterModule(mo.Spec.Source, symlinkPath)
	if err != nil {
		c.logger.Errorf("Module %q registration/reload failed: %v", mo.Name, err)
		return ctrl.Result{Requeue: true}, err
	}
	c.logger.Infof("Module %s was reloaded because its ModulePullOverride image was updated", mo.Name)

	return ctrl.Result{RequeueAfter: mo.Spec.ScanInterval.Duration}, nil
}

func (c *ModulePullOverrideController) enableModule(moduleName, symlinkPath string) error {
	currentModuleSymlink, err := findExistingModuleSymlink(c.symlinksDir, moduleName)
	if err != nil {
		currentModuleSymlink = "900-" + moduleName // fallback
	}

	return enableModule(c.externalModulesDir, currentModuleSymlink, symlinkPath, path.Join("../", moduleName, "dev"))
}

func (c *ModulePullOverrideController) updateModulePullOverrideStatus(ctx context.Context, mo *v1alpha1.ModulePullOverride) error {
	mo.Status.UpdatedAt = metav1.Now()
	_, err := c.d8ClientSet.DeckhouseV1alpha1().ModulePullOverrides().UpdateStatus(ctx, mo, metav1.UpdateOptions{})

	return err
}
