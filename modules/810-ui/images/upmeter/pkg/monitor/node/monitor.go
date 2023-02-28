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

package node

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
)

type Monitor struct {
	informer cache.SharedInformer
	stopCh   chan struct{}

	logger *log.Entry
}

func NewMonitor(kubeClient kube.Client, logger *log.Entry) *Monitor {
	var (
		gvr          = schema.GroupVersionResource{Version: "v1", Resource: "nodes"}
		indexers     = cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
		resyncPeriod = 5 * time.Minute

		tweakListOptions dynamicinformer.TweakListOptionsFunc = nil
	)

	informer := dynamicinformer.NewFilteredDynamicInformer(
		kubeClient.Dynamic(), gvr, corev1.NamespaceAll, resyncPeriod, indexers, tweakListOptions)

	return &Monitor{
		informer: informer.Informer(),
		stopCh:   make(chan struct{}),
		logger:   logger.WithField("component", "node-monitor"),
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

func (m *Monitor) getLogger() *log.Entry {
	return m.logger
}

func (m *Monitor) List() ([]*corev1.Node, error) {
	list := make([]*corev1.Node, 0)
	for _, obj := range m.informer.GetStore().List() {
		node, err := convert(obj)
		if err != nil {
			return nil, err
		}

		list = append(list, node)
	}
	return list, nil
}

func convert(o interface{}) (*corev1.Node, error) {
	var node corev1.Node
	unstrObj, ok := o.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("cannot convert object to *unstructured.Unstructured: %v", o)
	}

	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstrObj.UnstructuredContent(), &node)
	if err != nil {
		return nil, fmt.Errorf("cannot convert unstructured to core/v1 Node: %v", err)
	}
	return &node, nil
}

type Handler interface {
	OnAdd(*corev1.Node)
	OnModify(*corev1.Node)
	OnDelete(*corev1.Node)
}

type Lister interface {
	List() ([]*corev1.Node, error)
}
