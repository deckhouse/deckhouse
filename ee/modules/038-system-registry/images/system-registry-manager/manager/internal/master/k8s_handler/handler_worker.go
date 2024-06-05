/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package k8shandler

import (
	"context"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"sync"
)

type WorkerDaemonsetResource struct {
	data          *appsv1.DaemonSet
	daemonsetName string
	mu            sync.Mutex
}

func NewWorkerDaemonsetResource(daemonsetName string) *WorkerDaemonsetResource {
	return &WorkerDaemonsetResource{
		daemonsetName: daemonsetName,
	}
}

func (r *WorkerDaemonsetResource) Filter(obj interface{}) bool {
	if daemonSet, ok := obj.(*appsv1.DaemonSet); ok {
		return daemonSet.Name == r.daemonsetName
	}
	return false
}

func (r *WorkerDaemonsetResource) OnAdd(obj interface{}, isInInitialList bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if daemonset, ok := obj.(*appsv1.DaemonSet); ok {
		r.data = daemonset
	} else {
		log.Error("Pars pod error")
	}
}
func (r *WorkerDaemonsetResource) OnUpdate(oldObj, newObj interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if daemonset, ok := newObj.(*appsv1.DaemonSet); ok {
		r.data = daemonset
	} else {
		log.Error("Pars pod error")
	}
}
func (r *WorkerDaemonsetResource) OnDelete(obj interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := obj.(*appsv1.DaemonSet); ok {
		r.data = nil
	} else {
		log.Error("Pars pod error")
	}
}

func (r *WorkerDaemonsetResource) GetGroupVersionResourse() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "daemonsets",
	}
}

func (r *WorkerDaemonsetResource) GetData() *appsv1.DaemonSet {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.data
}

type WorkerEndpointResource struct {
	data         *corev1.Endpoints
	endpointName string
	mu           sync.Mutex
}

func NewWorkerEndpointResource(endpointName string) *WorkerEndpointResource {
	return &WorkerEndpointResource{
		endpointName: endpointName,
	}
}

func (r *WorkerEndpointResource) Filter(obj interface{}) bool {
	if endpoint, ok := obj.(*corev1.Endpoints); ok {
		return endpoint.Name == r.endpointName
	}
	return false
}

func (r *WorkerEndpointResource) OnAdd(obj interface{}, isInInitialList bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if endpoints, ok := obj.(*corev1.Endpoints); ok {
		r.data = endpoints
	} else {
		log.Error("Pars endpoints error")
	}
}
func (r *WorkerEndpointResource) OnUpdate(oldObj, newObj interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if endpoints, ok := newObj.(*corev1.Endpoints); ok {
		r.data = endpoints
	} else {
		log.Error("Pars endpoints error")
	}
}
func (r *WorkerEndpointResource) OnDelete(obj interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := obj.(*corev1.Endpoints); ok {
		r.data = nil
	} else {
		log.Error("Pars endpoints error")
	}
}

func (r *WorkerEndpointResource) Update(clientSet *kubernetes.Clientset, namespace string) {
	log.Info("Update WorkerEndpointResource")
	data, err := clientSet.CoreV1().Endpoints(namespace).Get(context.TODO(), r.endpointName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("error getting Endpoint: %s", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = data
}

func (r *WorkerEndpointResource) GetGroupVersionResourse() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "endpoints",
	}
}

func (r *WorkerEndpointResource) GetData() *corev1.Endpoints {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.data
}
