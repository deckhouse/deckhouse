/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package k8shandler

import (
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
	"sync"
)

type SeaweedfsPodsResource struct {
	data          map[string]corev1.Pod
	labelSelector labels.Selector
	mu            sync.Mutex
}

func NewSeaweedfsPodsResource(labelSelector []string) (*SeaweedfsPodsResource, error) {
	selector, err := labels.Parse(strings.Join(labelSelector, ","))
	if err != nil {
		return nil, err
	}
	return &SeaweedfsPodsResource{
		data:          make(map[string]corev1.Pod),
		labelSelector: selector,
	}, nil
}

func (r *SeaweedfsPodsResource) Filter(obj interface{}) bool {
	if pod, ok := obj.(*corev1.Pod); ok {
		return r.labelSelector.Matches(labels.Set(pod.Labels))
	}
	return false
}

func (r *SeaweedfsPodsResource) OnAdd(obj interface{}, isInInitialList bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if pod, ok := obj.(*corev1.Pod); ok {
		r.data[pod.Name] = *pod
	} else {
		log.Error("Pars pod error")
	}
}
func (r *SeaweedfsPodsResource) OnUpdate(oldObj, newObj interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if pod, ok := newObj.(*corev1.Pod); ok {
		r.data[pod.Name] = *pod
	} else {
		log.Error("Pars pod error")
	}
}
func (r *SeaweedfsPodsResource) OnDelete(obj interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if pod, ok := obj.(*corev1.Pod); ok {
		delete(r.data, pod.Name)
	} else {
		log.Error("Pars pod error")
	}
}

func (r *SeaweedfsPodsResource) GetGroupVersionResourse() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
}

func (r *SeaweedfsPodsResource) GetData() map[string]corev1.Pod {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.data
}
