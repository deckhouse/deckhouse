// Copyright 2021 Flant JSC
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

package internal

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	"discoverer/internal/apis/discoverer/v1alpha1"
)

type Discoverer interface {
	InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error)
}

type Reconciler struct {
	cloudRequestErrorMetric   *prometheus.GaugeVec
	updateResourceErrorMetric *prometheus.GaugeVec

	discoverer    Discoverer
	checkInterval time.Duration
	listenAddress string
	logger        *log.Entry
	k8sClient     dynamic.Interface
}

func NewReconciler(
	discoverer Discoverer,
	listenAddress string,
	interval time.Duration,
	logger *log.Entry,
	k8sClient dynamic.Interface,
) *Reconciler {
	return &Reconciler{
		checkInterval: interval,
		listenAddress: listenAddress,
		discoverer:    discoverer,
		logger:        logger,
		k8sClient:     k8sClient,
	}
}

func (c *Reconciler) Start() {
	defer c.logger.Infoln("Stop cloud data discoverer fully")

	c.logger.Infoln("Start cloud data discoverer")
	c.logger.Infoln("Address: ", c.listenAddress)
	c.logger.Infoln("Checks interval: ", c.checkInterval)

	// channels to stop converge loop
	shutdownAllCh := make(chan struct{})
	doneCh := make(chan struct{})

	c.registerMetrics()

	httpServer := c.getHTTPServer()

	rootCtx := context.Background()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		c.logger.Infof("Signal received: %v. Exiting.\n", <-signalChan)
		c.logger.Infoln("Waiting for stop reconcile loop...")

		close(shutdownAllCh)
		<-doneCh

		ctx, cancel := context.WithTimeout(rootCtx, 10*time.Second)
		defer cancel()

		err := httpServer.Shutdown(ctx)
		if err != nil {
			c.logger.Fatalf("Error occurred while closing the server: %v\n", err)
		}
		os.Exit(0)
	}()

	go c.reconcileLoop(rootCtx, shutdownAllCh, doneCh)

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
}

func (c *Reconciler) reconcileLoop(ctx context.Context, shutdownCh <-chan struct{}, doneCh chan<- struct{}) {
	c.reconcile(ctx)

	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.reconcile(ctx)
		case <-shutdownCh:
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
	defer c.logger.Infoln("Finish next data discovery")

	instanceTypes, err := c.discoverer.InstanceTypes(ctx)
	if err != nil {
		c.logger.Errorln("Instance types error: %v", err)
		c.cloudRequestErrorMetric.WithLabelValues("instance_types").Set(1.0)
		return
	}
	c.cloudRequestErrorMetric.WithLabelValues("instance_types").Set(0.0)

	data := v1alpha1.NewCloudDiscoveryData(instanceTypes)

	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(data)
	if err != nil {
		c.logger.Errorln("Instance types error: %v", err)
		c.updateResourceErrorMetric.WithLabelValues().Set(1.0)
		return
	}

	u := &unstructured.Unstructured{Object: content}

	for i := 1; i <= 3; i++ {
		if i > 1 {
			c.logger.Infoln("Waiting 3 seconds before next attempt")
			time.Sleep(3 * time.Second)
		}

		updateCtx, updateCancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := c.k8sClient.Resource(v1alpha1.GRV).Update(updateCtx, u, metav1.UpdateOptions{})
		updateCancel()
		if errors.IsNotFound(err) {
			createCtx, createCancel := context.WithTimeout(ctx, 10*time.Second)
			_, err := c.k8sClient.Resource(v1alpha1.GRV).Create(createCtx, u, metav1.CreateOptions{})
			createCancel()
			if err != nil {
				c.logger.Errorf("Attempt %d. Cannot create cloud data resource: %v\n", i, err)
				continue
			}
		}

		if err != nil {
			c.logger.Errorf("Attempt %d. Cannot update cloud data resource: %v", i, err)
			continue
		}

		c.updateResourceErrorMetric.WithLabelValues().Set(0.0)
		return
	}

	c.updateResourceErrorMetric.WithLabelValues().Set(1.0)
}
