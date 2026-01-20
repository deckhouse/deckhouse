// Copyright 2025 Flant JSC
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

package main

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/golang/glog"
)

func (a *app) startPodWatcher(ctx context.Context, clientset *kubernetes.Clientset) cache.SharedIndexInformer {
	factory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		5*time.Minute,
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			if a.nodeName != "" {
				opts.FieldSelector = "spec.nodeName=" + a.nodeName
			}
		}),
	)

	podInformer := factory.Core().V1().Pods().Informer()
	if err := podInformer.AddIndexers(cache.Indexers{
		podUIDIndex: func(obj interface{}) ([]string, error) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return nil, nil
			}
			return []string{string(pod.UID)}, nil
		},
	}); err != nil {
		glog.Fatal(err)
	}

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			a.kubeAPIOK.Store(true)
			a.syncPod(obj)
		},
		UpdateFunc: func(_, newObj interface{}) {
			a.kubeAPIOK.Store(true)
			a.syncPod(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			a.kubeAPIOK.Store(true)
			a.deletePod(obj)
		},
	})

	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced) {
		glog.Fatal("pod informer cache sync failed")
	}

	a.kubeAPIOK.Store(true)
	a.isReady.Store(true)
	return podInformer
}

func (a *app) syncPod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok || pod == nil {
		return
	}

	containerIDs := getPodContainerIDs(pod)
	removedIDs := a.updatePodContainers(string(pod.UID), containerIDs)
	for _, containerID := range removedIDs {
		if labels, ok := a.getTrackedLabels(containerID); ok {
			a.kubernetesCounterVec.Delete(a.buildPrometheusLabels(labels))
			a.deleteTrackedLabels(containerID)
		}
	}

	for containerID, containerName := range containerIDs {
		labels := buildContainerLabelsFromPod(pod, containerName)
		a.trackContainerLabels(containerID, labels)
		a.prometheusEnsureSeries(labels)
	}
}

func (a *app) deletePod(obj interface{}) {
	pod := extractPod(obj)
	if pod == nil {
		return
	}

	for _, containerID := range a.deletePodContainers(string(pod.UID)) {
		if labels, ok := a.getTrackedLabels(containerID); ok {
			a.kubernetesCounterVec.Delete(a.buildPrometheusLabels(labels))
			a.deleteTrackedLabels(containerID)
		}
	}
}

func (a *app) updatePodContainers(podUID string, containerIDs map[string]string) []string {
	a.podsMu.Lock()
	defer a.podsMu.Unlock()

	if a.containerIDsByPod == nil {
		a.containerIDsByPod = make(map[string]map[string]struct{})
	}

	current := make(map[string]struct{}, len(containerIDs))
	for containerID := range containerIDs {
		current[containerID] = struct{}{}
	}

	prev := a.containerIDsByPod[podUID]
	var removed []string
	for containerID := range prev {
		if _, ok := current[containerID]; !ok {
			removed = append(removed, containerID)
		}
	}

	a.containerIDsByPod[podUID] = current
	return removed
}

func (a *app) deletePodContainers(podUID string) []string {
	a.podsMu.Lock()
	defer a.podsMu.Unlock()

	prev := a.containerIDsByPod[podUID]
	delete(a.containerIDsByPod, podUID)

	removed := make([]string, 0, len(prev))
	for containerID := range prev {
		removed = append(removed, containerID)
	}
	return removed
}

func (a *app) getLabelsFromPodUID(podUID, containerID string) map[string]string {
	if a.podIndexer == nil {
		return nil
	}

	objects, err := a.podIndexer.ByIndex(podUIDIndex, podUID)
	if err != nil || len(objects) == 0 {
		return nil
	}

	pod, ok := objects[0].(*corev1.Pod)
	if !ok || pod == nil {
		return nil
	}

	containerIDs := getPodContainerIDs(pod)
	if containerName, ok := containerIDs[containerID]; ok {
		return buildContainerLabelsFromPod(pod, containerName)
	}
	return nil
}

func extractPod(obj interface{}) *corev1.Pod {
	switch t := obj.(type) {
	case *corev1.Pod:
		return t
	case cache.DeletedFinalStateUnknown:
		if pod, ok := t.Obj.(*corev1.Pod); ok {
			return pod
		}
	}
	return nil
}
