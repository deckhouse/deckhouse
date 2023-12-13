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

package lease

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/flant/addon-operator/pkg/utils/logger"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	coordination "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	coordinationv1 "k8s.io/client-go/listers/coordination/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"

	deckhouseiov1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	controllerUtils "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
	docs_builder "github.com/deckhouse/deckhouse/go_lib/module/docs-builder"
)

const (
	leaseLabel    = "deckhouse.io/documentation-builder-sync"
	namespace     = "d8-system"
	resyncTimeout = 15 * time.Minute
)

type Controller struct {
	kubeclientset kubernetes.Interface
	d8ClientSet   versioned.Interface
	docsBuilder   *docs_builder.Client
	workqueue     workqueue.RateLimitingInterface
	lister        coordinationv1.LeaseLister
	informer      cache.SharedIndexInformer
	logger        logger.Logger

	externalModulesDir string
}

func NewController(ks kubernetes.Interface, d8ClientSet versioned.Interface, httpClient d8http.Client) *Controller {
	ratelimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(500*time.Millisecond, 1000*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	lg := log.WithField("component", "LeaseController")

	factory := informers.NewSharedInformerFactoryWithOptions(
		ks,
		resyncTimeout,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = leaseLabel
		}),
	)
	leaseInformer := factory.Coordination().V1().Leases()
	lister := leaseInformer.Lister()
	informer := leaseInformer.Informer()

	controller := &Controller{
		kubeclientset:      ks,
		d8ClientSet:        d8ClientSet,
		workqueue:          workqueue.NewRateLimitingQueue(ratelimiter),
		lister:             lister,
		informer:           informer,
		docsBuilder:        docs_builder.NewClient(httpClient),
		logger:             lg,
		externalModulesDir: os.Getenv("EXTERNAL_MODULES_DIR"),
	}

	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueLease,
	})

	return controller
}

func (c *Controller) enqueueLease(obj interface{}) {
	var key cache.ObjectName
	var err error
	if key, err = cache.ObjectToName(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.logger.Debugf("enqueue Lease: %s", key)
	c.workqueue.Add(key)
}

func (c *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	c.logger.Info("Starting Lease controller")

	c.logger.Debug("Waiting for lease caches to sync")
	go c.informer.Run(ctx.Done())
	if ok := cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced); !ok {
		c.logger.Fatal("failed to wait for caches to sync")
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

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key cache.ObjectName
		var ok bool
		var req ctrl.Request

		if key, ok = obj.(cache.ObjectName); !ok {
			c.workqueue.Forget(obj)
			c.logger.Errorf("expected cache.ObjectName in workqueue but got %#v", obj)
			return nil
		}

		req.Namespace, req.Name = key.Parts()
		result, err := c.Reconcile(ctx, req)
		switch {
		case result.RequeueAfter != 0:
			c.workqueue.AddAfter(key, result.RequeueAfter)

		case result.Requeue:
			c.workqueue.AddRateLimited(key)

		default:
			c.workqueue.Forget(key)
		}

		return err
	}(obj)
	if err != nil {
		c.logger.Errorf("Lease reconcile error: %s", err.Error())
		return true
	}

	return true
}

func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lease, err := c.lister.Leases(req.Namespace).Get(req.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	return c.createReconcile(ctx, lease)
}

func (c *Controller) createReconcile(ctx context.Context, lease *coordination.Lease) (ctrl.Result, error) {
	if lease == nil || lease.Spec.HolderIdentity == nil {
		return ctrl.Result{}, nil
	}
	addr := "http://" + *lease.Spec.HolderIdentity

	list, err := c.d8ClientSet.DeckhouseV1alpha1().ModuleSources().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("list: %w", err)
	}

	for _, ms := range list.Items {
		md := downloader.NewModuleDownloader(c.externalModulesDir, &ms, controllerUtils.GenerateRegistryOptions(&ms))
		versions, err := c.fetchModuleVersions(ms, md)
		if err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("process module source %s error: %v", ms.Name, err)
		}

		for moduleName, moduleVersion := range versions {
			err = c.sendDocumentation(addr, md, moduleName, moduleVersion)
			if err != nil {
				return ctrl.Result{Requeue: true}, fmt.Errorf("send documentation for %s %s: %w", moduleName, moduleVersion, err)
			}
		}
	}

	err = c.docsBuilder.BuildDocumentation(addr)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("build documentation %w", err)
	}

	return ctrl.Result{}, nil
}

func (c *Controller) fetchModuleVersions(ms deckhouseiov1alpha1.ModuleSource, md *downloader.ModuleDownloader) (map[string]string, error) {
	versions := make(map[string]string)
	modules, err := md.ListModules()
	if err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}

	for _, moduleName := range modules {
		moduleVersion, err := md.FetchModuleVersionFromReleaseChannel(moduleName, ms.Spec.ReleaseChannel)
		if err != nil {
			c.logger.Warnf("fetch module '%s' version: %v", moduleName, err)
			continue
		}

		versions[moduleName] = moduleVersion
	}

	return versions, nil
}

func (c *Controller) sendDocumentation(baseAddr string, md *downloader.ModuleDownloader, moduleName, moduleVersion string) error {
	docsArchive, err := md.GetDocumentationArchive(moduleName, moduleVersion)
	if err != nil {
		return fmt.Errorf("get documentation archive: %w", err)
	}
	defer docsArchive.Close()

	return c.docsBuilder.SendDocumentation(baseAddr, moduleName, moduleVersion, docsArchive)
}
