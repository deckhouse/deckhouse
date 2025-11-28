/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type Watcher struct {
	mu   sync.RWMutex
	clientSet *kubernetes.Clientset
	metrics   *PrometheusExporterMetrics
	pods map[string]*IstioPodInfo
}

func NewWatcher(clientSet *kubernetes.Clientset, metrics *PrometheusExporterMetrics) *Watcher {
	return &Watcher{
		pods: make(map[string]*IstioPodInfo),
		clientSet: clientSet,
		metrics:   metrics,
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

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.updatePod,
		UpdateFunc: func(_, newObj interface{}) { w.updatePod(newObj) },
		DeleteFunc: w.deletePod,
	})
	if err != nil {
		log.Fatal(fmt.Sprintf("error adding pod event handler: %v", err))
	}

	go informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		panic("Failed to sync Pod informer cache")
	}

	<-ctx.Done()
	log.Info("Shutting down Pod informer")
}

func (w *Watcher) updatePod(obj interface{}) {
	if pod, ok := obj.(*v1.Pod); ok {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.pods[pod.Name] = &IstioPodInfo{
			pod.GetName(),
			pod.Status.PodIP,
			pod.Status.Phase,
		}
		log.Info(fmt.Sprintf("Pod updated: %s with IP: %s", pod.Name, pod.Status.PodIP))
	}
}

func (w *Watcher) deletePod(obj interface{}) {
	pod := podFromDeleteEvent(obj)
	if pod == nil {
		log.Error(fmt.Sprintf("Received unexpected object on pod delete event: %T", obj))
		return
	}

	w.mu.Lock()
	delete(w.pods, pod.Name)
	w.mu.Unlock()

	log.Info(fmt.Sprintf("Pod deleted: %s", pod.Name))
	if w.metrics != nil {
		w.metrics.DeleteIstiodMetrics(pod.Name)
	}
}

func podFromDeleteEvent(obj interface{}) *v1.Pod {
	switch t := obj.(type) {
	case *v1.Pod:
		return t
	case cache.DeletedFinalStateUnknown:
		if pod, ok := t.Obj.(*v1.Pod); ok {
			return pod
		}
	}

	return nil
}

func (w *Watcher) GetRunningIstiodPods() []IstioPodInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var pods []IstioPodInfo
	for _, pod := range w.pods {
		if pod.Status == v1.PodRunning && pod.IP != "" {
			pods = append(pods, IstioPodInfo{
				Name: pod.Name,
				IP:   pod.IP,
				Status: pod.Status,
			})
		}
	}
	return pods
}
