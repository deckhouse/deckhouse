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
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"
	"github.com/flant/constraint_exporter/pkg/kinds"
)

type Exporter struct {
	client     *kubernetes.Clientset
	kubeConfig *rest.Config

	kindTracker *kinds.KindTracker

	metricsMu sync.RWMutex
	metrics   []prometheus.Metric
}

func NewExporter() *Exporter {
	config, err := rest.InClusterConfig()
	if err != nil {
		fatal("create kubernetes config failed", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		fatal("create kubernetes client failed", err)
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
	ch <- gatekeeper.ConstraintViolationsTruncated
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(
		gatekeeper.Up, prometheus.GaugeValue, 1,
	)

	e.metricsMu.RLock()
	metrics := append(make([]prometheus.Metric, 0, len(e.metrics)), e.metrics...)
	e.metricsMu.RUnlock()

	for _, m := range metrics {
		ch <- m
	}
}

func (e *Exporter) startScheduled(clientGVR controllerClient.Client, t time.Duration) {
	ticker = time.NewTicker(t)

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			var (
				constraints    []gatekeeper.Constraint
				mutations      []gatekeeper.Mutation
				constraintsErr error
				mutationsErr   error
				wg             sync.WaitGroup
			)

			wg.Add(1)
			go func() {
				constraints, constraintsErr = e.fetchConstraints(clientGVR)
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				mutations, mutationsErr = gatekeeper.GetMutations(clientGVR, e.client)
				wg.Done()
			}()
			wg.Wait()

			if constraintsErr != nil || mutationsErr != nil {
				if constraintsErr != nil {
					slog.Warn("get constraints failed", "error", constraintsErr)
				}
				if mutationsErr != nil {
					slog.Warn("get mutations failed", "error", mutationsErr)
				}
				slog.Warn("skip tracked objects update due to stale gatekeeper data")
				continue
			}

			if e.kindTracker != nil {
				e.kindTracker.UpdateTrackedObjects(constraints, mutations)
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

	// Preallocate: at least 1 metric per constraint (constraint info) plus some violations.
	allMetrics := make([]prometheus.Metric, 0, len(constraints))
	violationMetrics := gatekeeper.ExportViolations(constraints)
	allMetrics = append(allMetrics, violationMetrics...)

	constraintInformationMetrics := gatekeeper.ExportConstraintInformation(constraints)
	allMetrics = append(allMetrics, constraintInformationMetrics...)

	truncatedMetrics := gatekeeper.ExportViolationsTruncated(constraints)
	allMetrics = append(allMetrics, truncatedMetrics...)

	e.metricsMu.Lock()
	e.metrics = allMetrics
	e.metricsMu.Unlock()

	return constraints, nil
}
