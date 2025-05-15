/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sync"
)

type Watcher struct {
	mu   sync.RWMutex
	clientSet *kubernetes.Clientset
	pods map[string]*v1.Pod
}

func NewWatcher(clientSet *kubernetes.Clientset) *Watcher {
	return &Watcher{
		pods: make(map[string]*v1.Pod),
		clientSet: clientSet,
	}
}

func (w *Watcher) StartPodWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet,
		0,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = "istio=istiod,app=istiod"
		}),
	)

	informer := factory.Core().V1().Pods().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.addPod,
		UpdateFunc: func(_, newObj interface{}) { w.updatePod(newObj) },
		DeleteFunc: w.deletePod,
	})

	go informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		panic("Failed to sync Pod informer cache")
	}

	<-ctx.Done()
	log.Info("Shutting down Pod informer")
}

func (w *Watcher) addPod(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.pods[pod.Name] = pod
		log.Info("Pod added: %s with IP: %s", pod.Name, pod.Status.PodIP)
	}
}

func (w *Watcher) updatePod(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.pods[pod.Name] = pod
		log.Info("Pod updated: %s with IP: %s", pod.Name, pod.Status.PodIP)
	}
}

func (w *Watcher) deletePod(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		w.mu.Lock()
		defer w.mu.Unlock()
		delete(w.pods, pod.Name)
		log.Info("Pod deleted: %s", pod.Name)
	}
}

func (w *Watcher) GetRunningIstiodPods() []IstioPodInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var pods []IstioPodInfo
	for _, pod := range w.pods {
		if pod.Status.Phase == v1.PodRunning && pod.Status.PodIP != "" {
			pods = append(pods, IstioPodInfo{
				Name: pod.Name,
				IP:   pod.Status.PodIP,
			})
		}
	}
	return pods
}
