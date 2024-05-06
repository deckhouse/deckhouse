/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package docs

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/utils/logger"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coordination_informers_v1 "k8s.io/client-go/informers/coordination/v1"
	coordination_listers_v1 "k8s.io/client-go/listers/coordination/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/module"
	docs_builder "github.com/deckhouse/deckhouse/go_lib/module/docs-builder"
)

type moduleReleaseGetter interface {
	GetModuleName() string
	GetReleaseVersion() string
	GetModuleSource() string
	GetWeight() uint32
}

type Updater struct {
	leasesInformer cache.SharedIndexInformer
	leasesLister   coordination_listers_v1.LeaseLister
	leasesSynced   cache.InformerSynced

	leaseWorkqueue workqueue.RateLimitingInterface

	client client.Client

	externalModulesDir string

	docsBuilder *docs_builder.Client
	dc          dependency.Container

	logger    logger.Logger
	apiCallMu sync.Mutex
}

func NewUpdater(
	leasesInformer coordination_informers_v1.LeaseInformer,
	client client.Client,
	dc dependency.Container,
) *Updater {
	ratelimiter := workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(500*time.Millisecond, 1000*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	lg := log.WithField("component", "ModuleDocumentationUpdater")

	updater := &Updater{
		leasesInformer: leasesInformer.Informer(),
		leasesLister:   leasesInformer.Lister(),
		leasesSynced:   leasesInformer.Informer().HasSynced,
		client:         client,

		leaseWorkqueue: workqueue.NewRateLimitingQueue(ratelimiter),

		logger:             lg,
		externalModulesDir: os.Getenv("EXTERNAL_MODULES_DIR"),

		docsBuilder: docs_builder.NewClient(dc.GetHTTPClient()),
	}

	_, err := updater.leasesInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: updater.enqueueLease,
	})
	if err != nil {
		updater.logger.Fatalf("add event handler failed: %s", err)
	}

	return updater
}

func (d *Updater) RunLeaseInformer(stopCh <-chan struct{}) {
	go d.leasesInformer.Run(stopCh)
}

func (d *Updater) Run(ctx context.Context) {
	defer d.leaseWorkqueue.ShutDown()

	go wait.UntilWithContext(ctx, d.runLeaseWorker, time.Second)

	<-ctx.Done()
}

func (d *Updater) RunPreflightCheck(ctx context.Context) error {
	if d.externalModulesDir == "" {
		return nil
	}

	if ok := cache.WaitForCacheSync(ctx.Done(), d.leasesSynced); !ok {
		d.logger.Fatal("failed to wait for caches to sync")
	}
	d.logger.Info("Documentation builder's object cache synced")

	return nil
}

func (d *Updater) SendDocumentation(ctx context.Context, m moduleReleaseGetter) error {
	d.apiCallMu.Lock()
	defer d.apiCallMu.Unlock()

	moduleName := m.GetModuleName()
	moduleVersion := m.GetReleaseVersion()
	d.logger.Infof("Updating documentation for %s module", moduleName)
	addrs, err := d.getDocsBuilderAddresses(ctx)
	if err != nil {
		return fmt.Errorf("get docs builder addresses: %w", err)
	}

	if len(addrs) == 0 {
		return nil
	}

	ms := new(v1alpha1.ModuleSource)
	err = d.client.Get(ctx, types.NamespacedName{Name: m.GetModuleSource()}, ms)
	if err != nil {
		return fmt.Errorf("get module source: %w", err)
	}

	md := downloader.NewModuleDownloader(d.dc, d.externalModulesDir, ms, utils.GenerateRegistryOptions(ms))
	for _, addr := range addrs {
		err = func() error {
			// Trying to get the documentation from the module's dir
			d.logger.Debugf("Getting the %s module's documentation locally", moduleName)
			docsArchive, err := d.getDocumentationFromModuleDir(m)
			if err != nil {
				d.logger.Infof("Failed to get %s module documentation from local directory with error: %v", moduleName, err)

				// Trying to get the documentation from the registry
				docsArchive, err = md.GetDocumentationArchive(moduleName, moduleVersion)
				if err != nil {
					return fmt.Errorf("get documentation archive: %w", err)
				}
			}
			defer docsArchive.Close()

			err = d.buildDocumentation(docsArchive, addr, moduleName, moduleVersion)
			if err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Updater) getDocumentationFromModuleDir(m moduleReleaseGetter) (io.ReadCloser, error) {
	moduleName := m.GetModuleName()
	weight := m.GetWeight()
	moduleDir := path.Join(d.externalModulesDir, "/modules/", fmt.Sprintf("%d-%s", weight, moduleName)) + "/"

	dir, err := os.Stat(moduleDir)
	if err != nil {
		return nil, err
	}

	if !dir.IsDir() {
		return nil, fmt.Errorf("%s of the %s module isn't a directory", moduleDir, moduleName)
	}

	pr, pw := io.Pipe()

	go func() {
		tw := tar.NewWriter(pw)
		defer tw.Close()

		pw.CloseWithError(filepath.Walk(moduleDir, func(file string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !module.IsDocsPath(strings.TrimPrefix(file, moduleDir)) {
				return nil
			}

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			header.Name = strings.TrimPrefix(file, moduleDir)

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(tw, f); err != nil {
				return err
			}

			return nil
		}))
	}()

	return pr, nil
}

func (d *Updater) getDocsBuilderAddresses(_ context.Context) (addresses []string, err error) {
	list, err := d.leasesLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list leases: %w", err)
	}

	for _, lease := range list {
		if lease.Spec.HolderIdentity == nil {
			continue
		}

		// a stale lease found
		if lease.Spec.RenewTime.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second).Before(time.Now()) {
			continue
		}

		addresses = append(addresses, "http://"+*lease.Spec.HolderIdentity)
	}

	return
}

func (d *Updater) buildDocumentation(docsArchive io.ReadCloser, baseAddr, moduleName, moduleVersion string) error {
	err := d.docsBuilder.SendDocumentation(baseAddr, moduleName, moduleVersion, docsArchive)
	if err != nil {
		return fmt.Errorf("send documentation: %w", err)
	}

	err = d.docsBuilder.BuildDocumentation(baseAddr)
	if err != nil {
		return fmt.Errorf("build documentation: %w", err)
	}

	return nil
}

func (d *Updater) enqueueLease(obj interface{}) {
	var key cache.ObjectName
	var err error
	if key, err = cache.ObjectToName(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	d.logger.Debugf("enqueue Lease: %s", key)
	d.leaseWorkqueue.Add(key)
}

func (d *Updater) runLeaseWorker(ctx context.Context) {
	for d.processNextLease(ctx) {
	}
}

func (d *Updater) processNextLease(ctx context.Context) bool {
	obj, shutdown := d.leaseWorkqueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer d.leaseWorkqueue.Done(obj)
		var key cache.ObjectName
		var ok bool
		var req ctrl.Request

		if key, ok = obj.(cache.ObjectName); !ok {
			d.leaseWorkqueue.Forget(obj)
			d.logger.Errorf("expected cache.ObjectName in workqueue but got %#v", obj)
			return nil
		}

		req.Namespace, req.Name = key.Parts()
		result, err := d.leaseCreateReconcile(ctx, req)
		switch {
		case result.RequeueAfter != 0:
			d.leaseWorkqueue.AddAfter(key, result.RequeueAfter)

		case result.Requeue:
			d.leaseWorkqueue.AddRateLimited(key)

		default:
			d.leaseWorkqueue.Forget(key)
		}

		return err
	}(obj)
	if err != nil {
		d.logger.Errorf("Lease reconcile error: %s", err.Error())
		return true
	}

	return true
}

func (d *Updater) leaseCreateReconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	d.logger.Infof("Rebuilding documentation for all deployed modules")
	var releasesList v1alpha1.ModuleReleaseList
	err := d.client.List(ctx, &releasesList)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("fetch ModuleReleases failed: %w", err)
	}

	for _, release := range releasesList.Items {
		// check if ModulePullOverride exists
		exists, err := d.isModulePullOverrideExists(ctx, release.GetModuleSource(), release.Spec.ModuleName)
		if err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("fetch ModulePullOverride for %s failed: %w", release.Spec.ModuleName, err)
		}

		if exists {
			continue
		}

		if release.Status.Phase != v1alpha1.PhaseDeployed {
			continue
		}

		err = d.SendDocumentation(ctx, &release)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	var mposList v1alpha1.ModulePullOverrideList
	err = d.client.List(ctx, &mposList)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("fetch ModulePullOverrides failed: %w", err)
	}

	for _, mpo := range mposList.Items {
		err := d.SendDocumentation(ctx, &mpo)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	return ctrl.Result{}, nil
}

func (d *Updater) isModulePullOverrideExists(ctx context.Context, sourceName, moduleName string) (bool, error) {
	var res v1alpha1.ModulePullOverrideList
	err := d.client.List(ctx, &res, client.MatchingLabels{"source": sourceName, "module": moduleName})
	if err != nil {
		return false, err
	}

	return len(res.Items) > 0, nil
}
