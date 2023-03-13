/*
Copyright 2023 Flant JSC

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

package main

import (
	"sync"
	"time"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"
	"github.com/flant/constraint_exporter/pkg/kinds"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Exporter struct {
	client     *kubernetes.Clientset
	kubeConfig *rest.Config

	kindTracker *kinds.KindTracker

	metrics []prometheus.Metric
}

func NewExporter() *Exporter {
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Create kubernetes config failed: %+v\n", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Create kubernetes client failed: %+v\n", err)
	}

	return &Exporter{
		client:     client,
		kubeConfig: config,
		metrics:    make([]prometheus.Metric, 0),
	}
}

func (e *Exporter) initKindTracker(cmNS, cmName string) error {
	kt := kinds.NewKindTracker(e.client, cmNS, cmName)
	err := kt.FindInitialChecksum()
	if err != nil {
		return err
	}

	e.kindTracker = kt

	return nil
}

func (e *Exporter) createKubeClientGroupVersion() (controllerClient.Client, error) {
	client, err := controllerClient.New(e.kubeConfig, controllerClient.Options{})
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- gatekeeper.Up
	ch <- gatekeeper.ConstraintViolation
	ch <- gatekeeper.ConstraintInformation
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		gatekeeper.Up, prometheus.GaugeValue, 1,
	)
	for _, m := range e.metrics {
		ch <- m
	}
}

func (e *Exporter) startScheduled(clientGVR controllerClient.Client, t time.Duration) {
	ticker = time.NewTicker(t)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			var (
				constraints []gatekeeper.Constraint
				mutations   []gatekeeper.Mutation
				wg          sync.WaitGroup
			)

			wg.Add(1)
			go func() {
				var err error
				constraints, err = e.fetchConstraints(clientGVR)
				if err != nil {
					klog.Warningf("Get constraints failed: %+v\n", err)
				}
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				var err error
				mutations, err = gatekeeper.GetMutations(clientGVR, e.client)
				if err != nil {
					klog.Warningf("Get mutations failed: %+v\n", err)
				}
				wg.Done()
			}()
			wg.Wait()

			if e.kindTracker != nil {
				go e.kindTracker.UpdateTrackedObjects(constraints, mutations)
			}
		}
	}
}

// fetch constraints and fill metrics
func (e *Exporter) fetchConstraints(clientGVR controllerClient.Client) ([]gatekeeper.Constraint, error) {
	constraints, err := gatekeeper.GetConstraints(clientGVR, e.client)
	if err != nil {
		return nil, err
	}

	allMetrics := make([]prometheus.Metric, 0)
	violationMetrics := gatekeeper.ExportViolations(constraints)
	allMetrics = append(allMetrics, violationMetrics...)

	constraintInformationMetrics := gatekeeper.ExportConstraintInformation(constraints)
	allMetrics = append(allMetrics, constraintInformationMetrics...)

	e.metrics = allMetrics

	return constraints, nil
}
