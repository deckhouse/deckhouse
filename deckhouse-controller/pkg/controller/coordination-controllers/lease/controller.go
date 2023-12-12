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
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/utils/logger"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/iancoleman/strcase"
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
	controllerUtils "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/go_lib/module"
)

const (
	leaseLabel    = "deckhouse.io/documentation-builder-sync"
	namespace     = "d8-system"
	resyncTimeout = 15 * time.Minute
)

type Controller struct {
	kubeclientset kubernetes.Interface
	d8ClientSet   versioned.Interface
	workqueue     workqueue.RateLimitingInterface
	lister        coordinationv1.LeaseLister
	informer      cache.SharedIndexInformer
	httpClient    d8http.Client
	logger        logger.Logger
}

func NewController(ks kubernetes.Interface, d8ClientSet versioned.Interface) *Controller {
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

	httpClient := d8http.NewClient(d8http.WithTimeout(3 * time.Minute))

	controller := &Controller{
		kubeclientset: ks,
		d8ClientSet:   d8ClientSet,
		workqueue:     workqueue.NewRateLimitingQueue(ratelimiter),
		lister:        lister,
		informer:      informer,
		httpClient:    httpClient,
		logger:        lg,
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
			// Put the item back on the workqueue to handle any transient errors.
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

func (c *Controller) createReconcile(ctx context.Context, _ *coordination.Lease) (ctrl.Result, error) {
	list, err := c.d8ClientSet.DeckhouseV1alpha1().ModuleSources().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("list: %w", err)
	}

	addrs, err := c.getDocsBuilderAddresses(ctx)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("get builder addresses: %w", err)
	}

	if len(addrs) == 0 {
		return ctrl.Result{}, nil
	}

	for _, item := range list.Items {
		err = errors.Join(err, c.processModuleSource(item, addrs))
	}
	if err != nil {
		c.logger.Warnf("process module source error: %w", err)
	}

	for _, addr := range addrs {
		err = c.buildDocumentation(addr)
		if err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("build documentation %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (c *Controller) processModuleSource(ms deckhouseiov1alpha1.ModuleSource, addrs []string) error {
	opts := controllerUtils.GenerateRegistryOptions(&ms)

	regCli, err := cr.NewClient(ms.Spec.Registry.Repo, opts...)
	if err != nil {
		return fmt.Errorf("get regestry client: %w", err)
	}

	tags, err := regCli.ListTags()
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}

	sort.Strings(tags)
	for _, moduleName := range tags {
		regCli, err := cr.NewClient(path.Join(ms.Spec.Registry.Repo, moduleName), opts...)
		if err != nil {
			return fmt.Errorf("fetch module %s: %v", moduleName, err)
		}

		moduleVersion, err := fetchModuleVersion(ms.Spec.ReleaseChannel, ms.Spec.Registry.Repo, moduleName, opts)
		if err != nil {
			return fmt.Errorf("fetch module version: %w", err)
		}

		for _, addr := range addrs {
			img, err := regCli.Image(moduleVersion)
			if err != nil {
				return fmt.Errorf("fetch module %s %s image: %v", moduleName, moduleVersion, err)
			}

			err = c.sendDocumentation(addr, img, moduleName, moduleVersion)
			if err != nil {
				return fmt.Errorf("send documentation for %s %s: %w", moduleName, moduleVersion, err)
			}
		}
	}
	return nil
}

func fetchModuleVersion(releaseChannel, repo, moduleName string, registryOptions []cr.Option) (moduleVersion string, err error) {
	regCli, err := cr.NewClient(path.Join(repo, moduleName, "release"), registryOptions...)
	if err != nil {
		return "", fmt.Errorf("fetch release image error: %v", err)
	}

	img, err := regCli.Image(strcase.ToKebab(releaseChannel))
	if err != nil {
		return "", fmt.Errorf("fetch image error: %v", err)
	}

	moduleMetadata, err := fetchModuleReleaseMetadata(img)
	if err != nil {
		return "", fmt.Errorf("fetch release metadata error: %v", err)
	}

	return "v" + moduleMetadata.Version.String(), nil
}

type moduleReleaseMetadata struct {
	Version *semver.Version `json:"version"`
}

func fetchModuleReleaseMetadata(img v1.Image) (moduleReleaseMetadata, error) {
	buf := bytes.NewBuffer(nil)
	var meta moduleReleaseMetadata

	layers, err := img.Layers()
	if err != nil {
		return meta, err
	}

	for _, layer := range layers {
		size, err := layer.Size()
		if err != nil {
			// dcr.logger.Warnf("couldn't calculate layer size")
			return meta, err
		}
		if size == 0 {
			// skip some empty werf layers
			continue
		}
		rc, err := layer.Uncompressed()
		if err != nil {
			return meta, err
		}

		err = untarMetadata(rc, buf)
		if err != nil {
			return meta, err
		}

		rc.Close()
	}

	err = json.Unmarshal(buf.Bytes(), &meta)

	return meta, err
}

func untarMetadata(rc io.ReadCloser, rw io.Writer) error {
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return nil
		}
		if err != nil {
			return err
		}
		if strings.HasPrefix(hdr.Name, ".werf") {
			continue
		}

		switch hdr.Name {
		case "version.json":
			_, err = io.Copy(rw, tr)
			if err != nil {
				return err
			}
			return nil

		default:
			continue
		}
	}
}

func (c *Controller) getDocsBuilderAddresses(ctx context.Context) (addresses []string, err error) {
	list, err := c.kubeclientset.DiscoveryV1().EndpointSlices("d8-system").List(ctx, metav1.ListOptions{LabelSelector: "app=documentation"})
	if err != nil {
		return nil, fmt.Errorf("list endpoint slices: %w", err)
	}

	for _, eps := range list.Items {
		var port int32
		for _, p := range eps.Ports {
			if p.Name != nil && *p.Name == "builder-http" {
				port = *p.Port
			}
		}

		if port == 0 {
			continue
		}
		for _, ep := range eps.Endpoints {
			for _, addr := range ep.Addresses {
				addresses = append(addresses, fmt.Sprintf("http://%s:%d", addr, port))
			}
		}
	}

	return
}

func (c *Controller) sendDocumentation(docsBuilderBasePath string, img v1.Image, moduleName, moduleVersion string) error {
	rc := module.ExtractDocs(img)
	defer rc.Close()

	url := fmt.Sprintf("%s/loadDocArchive/%s/%s", docsBuilderBasePath, moduleName, moduleVersion)
	response, statusCode, err := c.httpPost(url, rc)
	if err != nil {
		return fmt.Errorf("POST %q return %d %q: %w", url, statusCode, response, err)
	}

	return nil
}

func (c *Controller) buildDocumentation(docsBuilderBasePath string) error {
	url := fmt.Sprintf("%s/build", docsBuilderBasePath)
	response, statusCode, err := c.httpPost(url, nil)
	if err != nil {
		return fmt.Errorf("POST %q return %d %q: %w", url, statusCode, response, err)
	}

	return nil
}

func (c *Controller) httpPost(url string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, 0, err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	dataBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, 0, err
	}

	return dataBytes, res.StatusCode, nil
}
