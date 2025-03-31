/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"
	"github.com/deckhouse/deckhouse/pkg/log"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sync"
	"time"
)

type Watcher struct {
	mu sync.RWMutex
	es map[string]*discoveryv1.EndpointSlice
}

func NewWatcher() *Watcher {
	return &Watcher{
		es: make(map[string]*discoveryv1.EndpointSlice),
	}
}

func (w *Watcher) StartEndpointSliceWatcher(ctx context.Context, serviceName, namespace string) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}
	factory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		time.Minute,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = labels.SelectorFromSet(map[string]string{
				"kubernetes.io/service-name": fmt.Sprintf("%s", serviceName),
			}).String()
		}),
	)

	informer := factory.Discovery().V1().EndpointSlices().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			w.updateEndpointSlice(obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			w.updateEndpointSlice(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			w.deleteEndpointSlice(obj)
		},
	})

	go informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		panic("Failed to sync EndpointSlice informer cache")
	}

	<-ctx.Done()
	log.Info("Shutting down EndpointSlice informer")
}

func (w *Watcher) updateEndpointSlice(obj interface{}) {
	if endpointSlice, ok := obj.(*discoveryv1.EndpointSlice); ok {
		w.mu.Lock()
		w.es[endpointSlice.Name] = endpointSlice
		w.mu.Unlock()
	}
}
func (w *Watcher) deleteEndpointSlice(obj interface{}) {
	if endpointSlice, ok := obj.(*discoveryv1.EndpointSlice); ok {
		w.mu.Lock()
		delete(w.es, endpointSlice.Name)
		w.mu.Unlock()
	}
}

func (w *Watcher) GetIPsIstiodPods() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var ips []string
	for _, slice := range w.es {
		for _, endpoint := range slice.Endpoints {
			for _, addr := range endpoint.Addresses {
				ips = append(ips, addr)
			}
		}
	}
	return ips
}
