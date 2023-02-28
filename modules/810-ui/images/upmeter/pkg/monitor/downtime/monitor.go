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

package downtime

import (
	"context"
	"fmt"
	"time"

	kube "github.com/flant/kube-client/client"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"d8.io/upmeter/pkg/check"
)

type Monitor struct {
	informer cache.SharedInformer
	stopCh   chan struct{}

	logger *log.Entry
}

func NewMonitor(kubeClient kube.Client, logger *log.Entry) *Monitor {
	var (
		gvr = schema.GroupVersionResource{
			Group:    "deckhouse.io",
			Version:  "v1alpha1",
			Resource: "downtimes",
		}
		indexers     = cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
		resyncPeriod = 5 * time.Minute

		tweakListOptions dynamicinformer.TweakListOptionsFunc = nil
	)

	informer := dynamicinformer.NewFilteredDynamicInformer(
		kubeClient.Dynamic(), gvr, corev1.NamespaceAll, resyncPeriod, indexers, tweakListOptions)

	return &Monitor{
		informer: informer.Informer(),
		stopCh:   make(chan struct{}),
		logger:   logger.WithField("component", "downtime-monitor"),
	}
}

func (m *Monitor) Start(ctx context.Context) error {
	if err := m.informer.SetWatchErrorHandler(cache.DefaultWatchErrorHandler); err != nil {
		return fmt.Errorf("unable to set watch error handler: %w", err)
	}

	go m.informer.Run(m.stopCh)
	if !cache.WaitForCacheSync(ctx.Done(), m.informer.HasSynced) {
		return fmt.Errorf("unable to sync caches: %v", ctx.Err())
	}
	return nil
}

func (m *Monitor) Stop() {
	close(m.stopCh)
}

func (m *Monitor) List() ([]check.DowntimeIncident, error) {
	res := make([]check.DowntimeIncident, 0)
	for _, obj := range m.informer.GetStore().List() {
		incs, err := convert(obj)
		if err != nil {
			return nil, err
		}

		res = append(res, incs...)
	}
	return res, nil
}

func convert(o interface{}) ([]check.DowntimeIncident, error) {
	unstrObj, ok := o.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("cannot convert object to *unstructured.Unstructured: %v", o)
	}
	var incidentObj Downtime
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstrObj.UnstructuredContent(), &incidentObj)
	if err != nil {
		return nil, fmt.Errorf("cannot convert unstructured to Downtime: %v", err)
	}
	return incidentObj.GetDowntimeIncidents(), nil
}
