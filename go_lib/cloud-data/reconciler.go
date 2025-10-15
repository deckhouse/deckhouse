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

package clouddata

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	CheckCloudConditions(ctx context.Context) ([]v1alpha1.CloudCondition, error)
}

type Reconciler struct {
	cloudRequestErrorMetric    *prometheus.GaugeVec
	updateResourceErrorMetric  *prometheus.GaugeVec
	orphanedDiskMetric         *prometheus.GaugeVec
	cloudConditionsErrorMetric *prometheus.GaugeVec

	discoverer       Discoverer
	checkInterval    time.Duration
	listenAddress    string
	logger           *log.Logger
	k8sDynamicClient dynamic.Interface
	k8sClient        *kubernetes.Clientset
	probe            bool
	probeLock        sync.RWMutex
}

func NewReconciler(
	discoverer Discoverer,
	listenAddress string,
	interval time.Duration,
	logger *log.Logger,
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
		probe:            true,
	}
}

func (c *Reconciler) Start() {
	defer c.logger.Info("Stop cloud data discoverer fully")

	c.logger.Info("Start cloud data discoverer")
	c.logger.Info("Address:", "address", c.listenAddress)
	c.logger.Info("Checks interval:", "checks_interval", c.checkInterval)

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
		c.logger.Info("Waiting for stop reconcile loop...")
		<-doneCh

		ctx, cancel := context.WithTimeout(rootCtx, 10*time.Second)
		defer cancel()

		c.logger.Info("Shutdown ...")

		err := httpServer.Shutdown(ctx)
		if err != nil {
			c.logger.Fatalf("Error occurred while closing the server: %v\n", err)
		}
		os.Exit(0)
	}()

	go c.reconcileLoop(rootCtx, doneCh)

	err := httpServer.ListenAndServe()
	if err != http.ErrServerClosed {
		c.logger.Error("http server error", err)
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

	c.cloudConditionsErrorMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "cloud_data",
		Subsystem: "discovery",
		Name:      "cloud_conditions_error",
		Help:      "Indicates that there are unmet cloud conditions in the cluster",
	},
		[]string{"name", "message"},
	)
	prometheus.MustRegister(c.cloudConditionsErrorMetric)
}

func (c *Reconciler) setProbe(probe bool) {
	c.probeLock.Lock()
	defer c.probeLock.Unlock()
	c.probe = probe
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
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		c.probeLock.RLock()
		defer c.probeLock.RUnlock()
		if c.probe {
			_, _ = w.Write([]byte("ok"))
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("false"))
		c.logger.Error("Probe failed")
	})
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(indexPageContent))
	})

	return &http.Server{Addr: c.listenAddress, Handler: router, ReadHeaderTimeout: 30 * time.Second}
}

func (c *Reconciler) reconcile(ctx context.Context) {
	c.logger.Info("Start next data discovery")
	defer c.logger.Info("Finish data discovery")

	c.checkCloudConditions(ctx)
	c.instanceTypesReconcile(ctx)
	c.discoveryDataReconcile(ctx)
	c.orphanedDisksReconcile(ctx)
}

func (c *Reconciler) checkCloudConditions(ctx context.Context) {
	c.logger.Info("Start checking cloud conditions")
	defer c.logger.Info("Finish checking cloud conditions")

	conditions, err := c.discoverer.CheckCloudConditions(ctx)
	if err != nil {
		c.logger.Errorf("Error occurred while checking cloud conditions: %v", err)
		return
	}

	c.cloudConditionsErrorMetric.Reset()
	for i := range conditions {
		c.logger.Infof("Condition (%s) message: %s, ok: %t\n", conditions[i].Name, conditions[i].Message, conditions[i].Ok)
		if !conditions[i].Ok {
			c.cloudConditionsErrorMetric.WithLabelValues(conditions[i].Name, conditions[i].Message).Set(1.0)
		}
	}

	if len(conditions) == 0 {
		c.logger.Infof("Got 0 conditions")

		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, err = c.k8sClient.CoreV1().ConfigMaps("kube-system").Get(cctx, "d8-cloud-provider-conditions", metav1.GetOptions{})
		cancel()

		if errors.IsNotFound(err) {
			// don't create empty configmap if we don't have an existing one
			return
		}
	}

	jsonConditions, err := json.Marshal(conditions)
	if err != nil {
		c.logger.Errorf("failed to marshal conditions: %v", err)
		return
	}

	if err = retryFunc(15, 3*time.Second, 30*time.Second, c.logger, func() error {
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		configMap, err1 := c.k8sClient.CoreV1().ConfigMaps("kube-system").Get(cctx, "d8-cloud-provider-conditions", metav1.GetOptions{})
		cancel()
		if errors.IsNotFound(err1) {
			configMap = &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "d8-cloud-provider-conditions",
					Namespace: "kube-system",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				Data: map[string]string{"conditions": string(jsonConditions)},
			}

			cctx, cancel = context.WithTimeout(ctx, 10*time.Second)
			_, err1 = c.k8sClient.CoreV1().ConfigMaps("kube-system").Create(cctx, configMap, metav1.CreateOptions{})
			cancel()

			if err1 != nil {
				return fmt.Errorf("Cannot create d8-cloud-provider-conditions configMap: %v", err)
			}
		} else if err1 != nil {
			return fmt.Errorf("Cannot check d8-cloud-provider-conditions configMap before creating it: %v", err1)
		} else {
			configMap.Data["conditions"] = string(jsonConditions)

			cctx, cancel = context.WithTimeout(ctx, 10*time.Second)
			_, err1 = c.k8sClient.CoreV1().ConfigMaps("kube-system").Update(cctx, configMap, metav1.UpdateOptions{})
			cancel()

			if err1 != nil {
				return fmt.Errorf("Cannot update d8-cloud-provider-conditions configMap: %v", err)
			}
		}
		return nil
	}); err != nil {
		c.updateResourceErrorMetric.WithLabelValues().Set(1.0)
		c.logger.Error("Cannot update d8-cloud-provider-conditions configMap. Timed out. See error messages below.")
		c.setProbe(false)
	}
}

func (c *Reconciler) instanceTypesReconcile(ctx context.Context) {
	c.logger.Info("Start instance type discovery step")
	defer c.logger.Info("Finish instance type discovery step")

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

	err = retryFunc(15, 3*time.Second, 30*time.Second, c.logger, func() error {
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
		c.logger.Error("Cannot update cloud data resource. Timed out. See error messages below.")
		c.setProbe(false)
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
	c.logger.Info("Start cloud data discovery step")
	defer c.logger.Info("Finish cloud data discovery step")

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var cloudDiscoveryData []byte

	err := retryFunc(15, 3*time.Second, 30*time.Second, c.logger, func() error {
		secret, err := c.k8sClient.CoreV1().Secrets("kube-system").Get(cctx, "d8-provider-cluster-configuration", metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// d8-provider-cluster-configuration can not be exist in hybrid clusters
				return nil
			}
			return fmt.Errorf("failed to get 'd8-provider-cluster-configuration' secret: %v", err)
		}
		cloudDiscoveryData = secret.Data["cloud-provider-discovery-data.json"]
		return nil
	})
	if err != nil {
		c.cloudRequestErrorMetric.WithLabelValues("discovery_data").Set(1.0)
		c.logger.Error("Cannot get 'd8-provider-cluster-configuration' secret. Timed out. See error messages below.")
		c.setProbe(false)
		return
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

	err = retryFunc(15, 3*time.Second, 30*time.Second, c.logger, func() error {
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
			c.updateResourceErrorMetric.WithLabelValues().Set(0.0)
			return nil

		} else if errGetting != nil {
			return fmt.Errorf("Cannot check d8-cloud-provider-discovery-data secret before creating it: %v", errGetting)
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
		c.logger.Error("Cannot update cloud data resource. Timed out. See error messages below.")
		c.setProbe(false)
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
	c.logger.Info("Start orphaned disks discovery step")
	defer c.logger.Info("Finish orphaned disks discovery step")

	err := retryFunc(15, 3*time.Second, 30*time.Second, c.logger, func() error {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		disksMeta, err := c.discoverer.DisksMeta(cctx)
		if err != nil {
			c.cloudRequestErrorMetric.WithLabelValues("disks_meta").Set(1.0)
			return fmt.Errorf("Getting disks meta error: %v", err)
		}

		if len(disksMeta) == 0 {
			c.logger.Info("No disks found")
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
		c.logger.Error("Cannot update cloud data resource. Timed out. See error messages below.")
		c.setProbe(false)
	}
}

type retryable func() error

var errMaxRetriesReached = fmt.Errorf("exceeded retry limit")

func retryFunc(attempts int, initialSleep time.Duration, maxSleep time.Duration, logger *log.Logger, fn retryable) error {
	var err error
	sleep := initialSleep

	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		logger.Errorf("Attempt %d of %d. %v", i+1, attempts, err)

		if i < attempts-1 {
			jitter := time.Duration(rand.Int63n(int64(sleep / 2)))
			sleepTime := sleep + jitter

			logger.Infof("Waiting %v before next attempt", sleepTime)
			time.Sleep(sleepTime)
			sleep *= 2
			if sleep > maxSleep {
				sleep = maxSleep
			}
		}
	}

	return fmt.Errorf("%v: %w", err, errMaxRetriesReached)
}
