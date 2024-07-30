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

package cloud_data

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type Discoverer interface {
	InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error)
	DiscoveryData(ctx context.Context, cloudProviderDiscoveryData []byte) ([]byte, error)
	DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error)
}

type Reconciler struct {
	cloudRequestErrorMetric   *prometheus.GaugeVec
	updateResourceErrorMetric *prometheus.GaugeVec
	orphanedDiskMetric        *prometheus.GaugeVec

	discoverer       Discoverer
	checkInterval    time.Duration
	listenAddress    string
	logger           *log.Entry
	k8sDynamicClient dynamic.Interface
	k8sClient        *kubernetes.Clientset
}

func NewReconciler(
	discoverer Discoverer,
	listenAddress string,
	interval time.Duration,
	logger *log.Entry,
	k8sClient *kubernetes.Clientset,
	k8sDynamicClient dynamic.Interface,
) *Reconciler {
	return &Reconciler{
		checkInterval:    interval,
		listenAddress:    listenAddress,
		discoverer:       discoverer,
		logger:           logger,
		k8sClient:        k8sClient,
		k8sDynamicClient: k8sDynamicClient,
	}
}

func (c *Reconciler) Start() {
	defer c.logger.Infoln("Stop cloud data discoverer fully")

	c.logger.Infoln("Start cloud data discoverer")
	c.logger.Infoln("Address:", c.listenAddress)
	c.logger.Infoln("Checks interval:", c.checkInterval)

	// channels to stop converge loop
	doneCh := make(chan struct{})

	c.registerMetrics()

	httpServer := c.getHTTPServer()

	rootCtx, cancel := context.WithCancel(context.Background())

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		c.logger.Infof("Signal received: %v. Exiting.\n", <-signalChan)
		cancel()
		c.logger.Infoln("Waiting for stop reconcile loop...")
		<-doneCh

		ctx, cancel := context.WithTimeout(rootCtx, 10*time.Second)
		defer cancel()

		c.logger.Infoln("Shutdown ...")

		err := httpServer.Shutdown(ctx)
		if err != nil {
			c.logger.Fatalf("Error occurred while closing the server: %v\n", err)
		}
		os.Exit(0)
	}()

	go c.reconcileLoop(rootCtx, doneCh)

	err := httpServer.ListenAndServe()
	if err != http.ErrServerClosed {
		c.logger.Fatal(err)
	}
}
func (c *Reconciler) registerMetrics() {
	c.cloudRequestErrorMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "cloud_data",
		Subsystem: "discovery",
		Name:      "cloud_request_error",
		Help:      "Indicate that last cloud discovery data request failed with error",
	},
		[]string{"type"},
	)
	prometheus.MustRegister(c.cloudRequestErrorMetric)

	c.updateResourceErrorMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "cloud_data",
		Subsystem: "discovery",
		Name:      "update_resource_error",
		Help:      "Indicate that last updating clooud-data resource failed with error",
	},
		make([]string, 0),
	)
	prometheus.MustRegister(c.updateResourceErrorMetric)

	c.orphanedDiskMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "cloud_data",
		Subsystem: "discovery",
		Name:      "orphaned_disk_info",
		Help:      "Indicates that there is a disk in the cloud for which there is no PersistentVolume in the cluster",
	},
		[]string{"id", "name"},
	)
	prometheus.MustRegister(c.orphanedDiskMetric)
}

func (c *Reconciler) reconcileLoop(ctx context.Context, doneCh chan<- struct{}) {
	c.reconcile(ctx)

	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.reconcile(ctx)
		case <-ctx.Done():
			doneCh <- struct{}{}
			return
		}
	}
}

func (c *Reconciler) getHTTPServer() *http.Server {
	indexPageContent := fmt.Sprintf(`<html>
             <head><title>Cloud data discoverer </title></head>
             <body>
             <h1> Discovery data every %s</h1>
             </body>
             </html>`, c.checkInterval.String())

	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.Handler())
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(indexPageContent))
	})

	return &http.Server{Addr: c.listenAddress, Handler: router, ReadHeaderTimeout: 30 * time.Second}
}

func (c *Reconciler) reconcile(ctx context.Context) {
	c.logger.Infoln("Start next data discovery")
	defer c.logger.Infoln("Finish data discovery")

	c.instanceTypesReconcile(ctx)
	c.discoveryDataReconcile(ctx)
	c.orphanedDisksReconcile(ctx)
}

func (c *Reconciler) instanceTypesReconcile(ctx context.Context) {
	c.logger.Infoln("Start instance type discovery step")
	defer c.logger.Infoln("Finish instance type discovery step")

	instanceTypes, err := c.discoverer.InstanceTypes(ctx)
	if err != nil {
		c.logger.Errorf("Getting instance types error: %v\n", err)
		c.cloudRequestErrorMetric.WithLabelValues("instance_types").Set(1.0)
		return
	}
	c.cloudRequestErrorMetric.WithLabelValues("instance_types").Set(0.0)

	if instanceTypes == nil {
		c.updateResourceErrorMetric.WithLabelValues().Set(0.0)
		return
	}

	sort.SliceStable(instanceTypes, func(i, j int) bool {
		return instanceTypes[i].Name < instanceTypes[j].Name
	})

	err = retryFunc(3, 3, c.logger, func() error {
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		data, errGetting := c.k8sDynamicClient.Resource(v1alpha1.GVR).Get(cctx, v1alpha1.CloudDiscoveryDataResourceName, metav1.GetOptions{})
		cancel()

		if errors.IsNotFound(errGetting) {
			o, err := c.instanceTypesCloudDiscoveryUnstructured(nil, instanceTypes)
			if err != nil {
				// return because we have error in conversion
				return err
			}

			cctx, cancel = context.WithTimeout(ctx, 10*time.Second)
			_, err = c.k8sDynamicClient.Resource(v1alpha1.GVR).Create(cctx, o, metav1.CreateOptions{})
			cancel()

			if err != nil {
				return fmt.Errorf("Cannot create cloud data resource: %v", err)
			}

			errGetting = nil
		} else {
			o, err := c.instanceTypesCloudDiscoveryUnstructured(data, instanceTypes)
			if err != nil {
				// return because we have error in conversion
				return err
			}

			cctx, cancel = context.WithTimeout(ctx, 10*time.Second)
			_, err = c.k8sDynamicClient.Resource(v1alpha1.GVR).Update(cctx, o, metav1.UpdateOptions{})
			cancel()

			if err != nil {
				return fmt.Errorf("Cannot update cloud data resource: %v", err)
			}
		}

		if errGetting != nil {
			return fmt.Errorf("Cannot update cloud data resource: %v", errGetting)
		}

		c.updateResourceErrorMetric.WithLabelValues().Set(0.0)
		return nil
	})

	if err != nil {
		c.updateResourceErrorMetric.WithLabelValues().Set(1.0)
		c.logger.Errorln("Cannot update cloud data resource. Timed out. See error messages below.")
	}
}

func (c *Reconciler) instanceTypesCloudDiscoveryUnstructured(o *unstructured.Unstructured, instanceTypes []v1alpha1.InstanceType) (*unstructured.Unstructured, error) {
	data := v1alpha1.InstanceTypesCatalog{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.CloudDiscoveryDataResourceName,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "InstanceTypesCatalog",
			APIVersion: "deckhouse.io/v1alpha1",
		},
	}

	if o != nil {
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), &data)
		if err != nil {
			c.logger.Errorf("Failed to convert unstructured to data. Error: %v\n", err)
			c.updateResourceErrorMetric.WithLabelValues().Set(1.0)
			return nil, err
		}
	}

	data.InstanceTypes = instanceTypes

	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&data)
	if err != nil {
		c.logger.Errorf("Failed to convert data to unstructured. Error: %v\n", err)
		c.updateResourceErrorMetric.WithLabelValues().Set(1.0)
		return nil, err
	}

	u := &unstructured.Unstructured{Object: content}

	return u, nil
}

func (c *Reconciler) discoveryDataReconcile(ctx context.Context) {
	c.logger.Infoln("Start cloud data discovery step")
	defer c.logger.Infoln("Finish cloud data discovery step")

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var cloudDiscoveryData []byte

	secret, err := c.k8sClient.CoreV1().Secrets("kube-system").Get(cctx, "d8-provider-cluster-configuration", metav1.GetOptions{})
	// d8-provider-cluster-configuration can not be exist in hybrid clusters
	if err != nil {
		if !errors.IsNotFound(err) {
			c.logger.Errorf("Failed to get 'd8-provider-cluster-configuration' secret: %v\n", err)
			c.cloudRequestErrorMetric.WithLabelValues("discovery_data").Set(1.0)
			return
		}
	} else {
		cloudDiscoveryData = secret.Data["cloud-provider-discovery-data.json"]
	}

	discoveryData, err := c.discoverer.DiscoveryData(ctx, cloudDiscoveryData)
	if err != nil {
		c.logger.Errorf("Getting discovery data error: %v\n", err)
		c.cloudRequestErrorMetric.WithLabelValues("discovery_data").Set(1.0)
		return
	}
	c.cloudRequestErrorMetric.WithLabelValues("discovery_data").Set(0.0)

	if discoveryData == nil {
		c.updateResourceErrorMetric.WithLabelValues().Set(0.0)
		return
	}

	err = retryFunc(3, 3, c.logger, func() error {
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		secret, errGetting := c.k8sClient.CoreV1().Secrets("kube-system").Get(cctx, "d8-cloud-provider-discovery-data", metav1.GetOptions{})
		cancel()

		if errors.IsNotFound(errGetting) {
			cctx, cancel = context.WithTimeout(ctx, 10*time.Second)
			_, err = c.k8sClient.CoreV1().Secrets("kube-system").Create(cctx, c.createSecretWithDiscoveryData(discoveryData), metav1.CreateOptions{})
			cancel()

			if err != nil {
				return fmt.Errorf("Cannot create cloud data resource: %v", err)
			}

			errGetting = nil
		} else {
			secret.Data = map[string][]byte{
				"discovery-data.json": discoveryData,
			}

			cctx, cancel = context.WithTimeout(ctx, 10*time.Second)
			_, err = c.k8sClient.CoreV1().Secrets("kube-system").Update(cctx, secret, metav1.UpdateOptions{})
			cancel()

			if err != nil {
				return fmt.Errorf("Cannot update cloud data resource: %v", err)
			}
		}

		if errGetting != nil {
			return fmt.Errorf("Cannot get cloud data resource: %v", errGetting)
		}

		c.updateResourceErrorMetric.WithLabelValues().Set(0.0)
		return nil
	})

	if err != nil {
		c.updateResourceErrorMetric.WithLabelValues().Set(1.0)
		c.logger.Errorln("Cannot update cloud data resource. Timed out. See error messages below.")
	}
}

func (c *Reconciler) createSecretWithDiscoveryData(discoveryData []byte) *v1.Secret {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-cloud-provider-discovery-data",
			Namespace: "kube-system",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
	}

	secret.Data = map[string][]byte{
		"discovery-data.json": discoveryData,
	}

	return secret
}

type Set map[string]struct{}

func (s Set) Add(xs ...string) Set {
	for _, x := range xs {
		s[x] = struct{}{}
	}
	return s
}

func (s Set) Has(x string) bool {
	_, ok := s[x]
	return ok
}

func (c *Reconciler) orphanedDisksReconcile(ctx context.Context) {
	c.logger.Infoln("Start orphaned disks discovery step")
	defer c.logger.Infoln("Finish orphaned disks discovery step")

	err := retryFunc(3, 3, c.logger, func() error {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		disksMeta, err := c.discoverer.DisksMeta(cctx)
		if err != nil {
			c.cloudRequestErrorMetric.WithLabelValues("disks_meta").Set(1.0)
			return fmt.Errorf("Getting disks meta error: %v", err)
		}

		if len(disksMeta) == 0 {
			c.logger.Infoln("No disks found")
			c.cloudRequestErrorMetric.WithLabelValues("disks_meta").Set(0.0)
			c.updateResourceErrorMetric.WithLabelValues().Set(0.0)
			return nil
		}

		persistentVolumes, err := c.k8sClient.CoreV1().PersistentVolumes().List(cctx, metav1.ListOptions{})
		if err != nil {
			c.cloudRequestErrorMetric.WithLabelValues("disks_meta").Set(1.0)
			return fmt.Errorf("Failed to get PersistentVolumes from cluster: %v", err)
		}

		c.cloudRequestErrorMetric.WithLabelValues("disks_meta").Set(0.0)

		persistentVolumeNames := Set{}
		for _, pv := range persistentVolumes.Items {
			persistentVolumeNames.Add(pv.Name)
		}

		c.orphanedDiskMetric.Reset()
		for _, disk := range disksMeta {
			if !persistentVolumeNames.Has(disk.Name) {
				c.orphanedDiskMetric.WithLabelValues(disk.ID, disk.Name).Set(1.0)
			}
		}

		c.updateResourceErrorMetric.WithLabelValues().Set(0.0)
		return nil
	})

	if err != nil {
		c.updateResourceErrorMetric.WithLabelValues().Set(1.0)
		c.logger.Errorln("Cannot update cloud data resource. Timed out. See error messages below.")
	}
}

type retryable func() error

var errMaxRetriesReached = fmt.Errorf("exceeded retry limit")

func retryFunc(attempts int, sleep int, logger *log.Entry, fn retryable) error {
	var err error

	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		logger.Errorf("Attempt %d of %d. %v", i+1, attempts, err)

		if i < attempts-1 {
			logger.Infof("Waiting %d seconds before next attempt", sleep)
			time.Sleep(time.Duration(sleep) * time.Second)
		}
	}

	return fmt.Errorf("%v: %w", err, errMaxRetriesReached)
}
