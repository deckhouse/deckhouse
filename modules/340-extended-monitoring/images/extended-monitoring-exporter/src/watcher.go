/*
Copyright 2025 Flant JSC

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
	"context"
	batchv1 "k8s.io/api/batch/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"log"
	"strconv"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

)

const (
	labelThresholdPrefix   = "threshold.extended-monitoring.deckhouse.io/"
	namespacesEnabledLabel = "extended-monitoring.deckhouse.io/enabled"
)

// ---------------- Watcher ----------------

type Watcher struct {
	clientSet *kubernetes.Clientset
	mu        sync.Mutex
	nsWatchers map[string]context.CancelFunc
	metrics   *PrometheusExporterMetrics
}

func NewWatcher(clientSet *kubernetes.Clientset, metrics *PrometheusExporterMetrics) *Watcher {
	return &Watcher{
		clientSet:  clientSet,
		nsWatchers: make(map[string]context.CancelFunc),
		metrics:    metrics,
	}
}

// ---------------- Helpers ----------------

func enabledLabel(labels map[string]string) float64 {
	if val, ok := labels[namespacesEnabledLabel]; ok {
		if b, err := strconv.ParseBool(val); err == nil && !b {
			return 0
		}
	}
	return 1
}

func thresholdLabel(labels map[string]string, threshold string, def float64) float64 {
	if val, ok := labels[labelThresholdPrefix+threshold]; ok {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		log.Printf("[thresholdLabel] could not parse %s=%s", threshold, val)
	}
	return def
}

// ---------------- Node Watcher ----------------

var nodeThresholdMap = map[string]float64{
	"disk-bytes-warning":             70,
	"disk-bytes-critical":            80,
	"disk-inodes-warning":            90,
	"disk-inodes-critical":           95,
	"load-average-per-core-warning":  3,
	"load-average-per-core-critical": 10,
}

func (w *Watcher) StartNodeWatcher(ctx context.Context) {
	factory := informers.NewSharedInformerFactory(w.clientSet, 5*time.Second)
	informer := factory.Core().V1().Nodes().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { w.updateNode(obj.(*v1.Node)) },
		UpdateFunc: func(_, obj interface{}) { w.updateNode(obj.(*v1.Node)) },
		DeleteFunc: func(obj interface{}) { w.deleteNode(obj.(*v1.Node)) },
	})

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
}

func (w *Watcher) updateNode(node *v1.Node) {
	enabled := enabledLabel(node.Labels)
	w.metrics.NodeEnabled.WithLabelValues(node.Name).Set(enabled)
	for key, def := range nodeThresholdMap {
		w.metrics.NodeThreshold.WithLabelValues(node.Name, key).Set(thresholdLabel(node.Labels, key, def))
	}
	log.Printf("[NODE UPDATE] %s", node.Name)
	lastObserved = time.Now()
}

func (w *Watcher) deleteNode(node *v1.Node) {
	w.metrics.NodeEnabled.DeleteLabelValues(node.Name)
	for key := range nodeThresholdMap {
		w.metrics.NodeThreshold.DeleteLabelValues(node.Name, key)
	}
	log.Printf("[NODE DELETE] %s", node.Name)
	lastObserved = time.Now()
}

// ---------------- Namespace Watcher ----------------

func (w *Watcher) StartNamespaceWatcher(ctx context.Context) {
	factory := informers.NewSharedInformerFactory(w.clientSet, 5*time.Second)
	informer := factory.Core().V1().Namespaces().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { w.addNamespace(ctx, obj.(*v1.Namespace)) },
		UpdateFunc: func(_, obj interface{}) { w.updateNamespace(ctx, obj.(*v1.Namespace)) },
		DeleteFunc: func(obj interface{}) { w.deleteNamespace(obj.(*v1.Namespace)) },
	})

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
}

func (w *Watcher) addNamespace(ctx context.Context, ns *v1.Namespace) {
	enabled := enabledLabel(ns.Labels)
	w.metrics.NamespacesEnabled.WithLabelValues(ns.Name).Set(enabled)
	log.Printf("[NS ADD] %s", ns.Name)

	if enabled == 1 {
		nsCtx, cancel := context.WithCancel(ctx)
		w.mu.Lock()
		w.nsWatchers[ns.Name] = cancel
		w.mu.Unlock()

		go w.StartPodWatcher(nsCtx, ns.Name)
		go w.StartDaemonSetWatcher(nsCtx, ns.Name)
		go w.StartStatefulSetWatcher(nsCtx, ns.Name)
		go w.StartDeploymentWatcher(nsCtx, ns.Name)
		go w.StartIngressWatcher(nsCtx, ns.Name)
		go w.StartCronJobWatcher(nsCtx, ns.Name)
	}
	lastObserved = time.Now()
}

func (w *Watcher) updateNamespace(ctx context.Context, ns *v1.Namespace) {
	enabled := enabledLabel(ns.Labels)
	w.metrics.NamespacesEnabled.WithLabelValues(ns.Name).Set(enabled)
	log.Printf("[NS UPDATE] %s", ns.Name)

	w.mu.Lock()
	cancel, exists := w.nsWatchers[ns.Name]
	w.mu.Unlock()

	if enabled == 0 && exists {
		cancel()
		w.mu.Lock()
		delete(w.nsWatchers, ns.Name)
		w.mu.Unlock()
		log.Printf("[NS DISABLED] %s watchers stopped", ns.Name)
	}

	if enabled == 1 && !exists {
		nsCtx, cancel := context.WithCancel(ctx)
		w.mu.Lock()
		w.nsWatchers[ns.Name] = cancel
		w.mu.Unlock()

		go w.StartPodWatcher(nsCtx, ns.Name)
		go w.StartDaemonSetWatcher(nsCtx, ns.Name)
		go w.StartStatefulSetWatcher(nsCtx, ns.Name)
		go w.StartDeploymentWatcher(nsCtx, ns.Name)
		go w.StartIngressWatcher(nsCtx, ns.Name)
		go w.StartCronJobWatcher(nsCtx, ns.Name)

		log.Printf("[NS ENABLED] %s watchers started", ns.Name)
	}
	lastObserved = time.Now()
}


func (w *Watcher) deleteNamespace(ns *v1.Namespace) {
	w.metrics.NamespacesEnabled.DeleteLabelValues(ns.Name)
	w.mu.Lock()
	if cancel, exists := w.nsWatchers[ns.Name]; exists {
		cancel()
		delete(w.nsWatchers, ns.Name)
		log.Printf("[NS DELETE] %s watchers stopped", ns.Name)
		lastObserved = time.Now()
	}
	w.mu.Unlock()
}

// ---------------- Pod Watcher ----------------

var podThresholdMap = map[string]float64{
	"disk-bytes-warning":            85,
	"disk-bytes-critical":           95,
	"disk-inodes-warning":           85,
	"disk-inodes-critical":          90,
}

func (w *Watcher) StartPodWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet,
		5*time.Second,
		informers.WithNamespace(namespace),
	)
	informer := factory.Core().V1().Pods().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { w.updatePod(obj.(*v1.Pod)) },
		UpdateFunc: func(_, obj interface{}) { w.updatePod(obj.(*v1.Pod)) },
		DeleteFunc: func(obj interface{}) { w.deletePod(obj.(*v1.Pod)) },
	})

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
}

func (w *Watcher) updatePod(pod *v1.Pod) {
	enabled := enabledLabel(pod.Labels)
	w.metrics.PodEnabled.WithLabelValues(pod.Namespace, pod.Name).Set(enabled)
	lastObserved = time.Now()

	if enabled == 1 {
		for key, def := range podThresholdMap {
			w.metrics.PodThreshold.WithLabelValues(pod.Namespace, pod.Name, key).Set(thresholdLabel(pod.Labels, key, def))
		}
	}
	log.Printf("[POD UPDATE] %s/%s", pod.Namespace, pod.Name)
	lastObserved = time.Now()
}

func (w *Watcher) deletePod(pod *v1.Pod) {
	w.metrics.PodEnabled.DeleteLabelValues(pod.Namespace, pod.Name)
	for key := range podThresholdMap {
		w.metrics.PodThreshold.DeleteLabelValues(pod.Namespace, pod.Name, key)
	}
	log.Printf("[POD DELETE] %s/%s", pod.Namespace, pod.Name)
	lastObserved = time.Now()
}

// ---------------- Daemon Set Watcher ----------------

var deamonSetThresholdMap = map[string]float64{
	"replicas-not-ready": 0,
}

func (w *Watcher) StartDaemonSetWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet,
		5*time.Second,
		informers.WithNamespace(namespace),
	)
	informer := factory.Apps().V1().DaemonSets().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { w.updateDaemonSet(obj.(*appsv1.DaemonSet)) },
		UpdateFunc: func(_, obj interface{}) { w.updateDaemonSet(obj.(*appsv1.DaemonSet)) },
		DeleteFunc: func(obj interface{}) { w.deleteDaemonSet(obj.(*appsv1.DaemonSet)) },
	})

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
}

func (w *Watcher) updateDaemonSet(ds *appsv1.DaemonSet) {
	enabled := enabledLabel(ds.Labels)
	w.metrics.DaemonSetEnabled.WithLabelValues(ds.Namespace, ds.Name).Set(enabled)

	if enabled == 1 {
		for key, def := range deamonSetThresholdMap {
			w.metrics.DaemonSetThreshold.WithLabelValues(ds.Namespace, ds.Name, key).
				Set(thresholdLabel(ds.Labels, key, def))
		}
	}
	log.Printf("[DS UPDATE] %s/%s", ds.Namespace, ds.Name)
	lastObserved = time.Now()
}

func (w *Watcher) deleteDaemonSet(ds *appsv1.DaemonSet) {
	w.metrics.DaemonSetEnabled.DeleteLabelValues(ds.Namespace, ds.Name)
	for key := range deamonSetThresholdMap {
		w.metrics.DaemonSetThreshold.DeleteLabelValues(ds.Namespace, ds.Name, key)
	}
	log.Printf("[DS DELETE] %s/%s", ds.Namespace, ds.Name)
	lastObserved = time.Now()
}

// ---------------- Stateful Set Watcher ----------------

var statefulSetThresholdMap = map[string]float64{
	"replicas-not-ready": 0,
}

func (w *Watcher) StartStatefulSetWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet,
		5*time.Second,
		informers.WithNamespace(namespace),
	)
	informer := factory.Apps().V1().StatefulSets().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { w.updateStatefulSet(obj.(*appsv1.StatefulSet)) },
		UpdateFunc: func(_, obj interface{}) { w.updateStatefulSet(obj.(*appsv1.StatefulSet)) },
		DeleteFunc: func(obj interface{}) { w.deleteStatefulSet(obj.(*appsv1.StatefulSet)) },
	})

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
}

func (w *Watcher) updateStatefulSet(ds *appsv1.StatefulSet) {
	enabled := enabledLabel(ds.Labels)
	w.metrics.StatefulSetEnabled.WithLabelValues(ds.Namespace, ds.Name).Set(enabled)

	if enabled == 1 {
		for key, def := range statefulSetThresholdMap {
			w.metrics.StatefulSetThreshold.WithLabelValues(ds.Namespace, ds.Name, key).
				Set(thresholdLabel(ds.Labels, key, def))
		}
	}
	log.Printf("[STS UPDATE] %s/%s", ds.Namespace, ds.Name)
	lastObserved = time.Now()
}

func (w *Watcher) deleteStatefulSet(ds *appsv1.StatefulSet) {
	w.metrics.StatefulSetEnabled.DeleteLabelValues(ds.Namespace, ds.Name)
	for key := range statefulSetThresholdMap {
		w.metrics.StatefulSetThreshold.DeleteLabelValues(ds.Namespace, ds.Name, key)
	}
	log.Printf("[STS DELETE] %s/%s", ds.Namespace, ds.Name)
	lastObserved = time.Now()
}

// ---------------- Deployment Watcher ----------------

var deploymentThresholdMap = map[string]float64{
	"replicas-not-ready": 0,
}

func (w *Watcher) StartDeploymentWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet,
		5*time.Second,
		informers.WithNamespace(namespace),
	)
	informer := factory.Apps().V1().Deployments().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { w.updateDeployment(obj.(*appsv1.Deployment)) },
		UpdateFunc: func(_, obj interface{}) { w.updateDeployment(obj.(*appsv1.Deployment)) },
		DeleteFunc: func(obj interface{}) { w.deleteDeployment(obj.(*appsv1.Deployment)) },
	})

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
}

func (w *Watcher) updateDeployment(dep *appsv1.Deployment) {
	enabled := enabledLabel(dep.Labels)
	w.metrics.DeploymentEnabled.WithLabelValues(dep.Namespace, dep.Name).Set(enabled)
	if enabled == 1 {
		for key, def := range deploymentThresholdMap {
			w.metrics.DeploymentThreshold.WithLabelValues(dep.Namespace, dep.Name, key).Set(thresholdLabel(dep.Labels, key, def))
		}
	}
	log.Printf("[DEP UPDATE] %s/%s", dep.Namespace, dep.Name)
	lastObserved = time.Now()
}

func (w *Watcher) deleteDeployment(dep *appsv1.Deployment) {
	w.metrics.DeploymentEnabled.DeleteLabelValues(dep.Namespace, dep.Name)
	for key := range deploymentThresholdMap {
		w.metrics.DeploymentThreshold.DeleteLabelValues(dep.Namespace, dep.Name, key)
	}
	log.Printf("[DEP DELETE] %s/%s", dep.Namespace, dep.Name)
	lastObserved = time.Now()
}

// ---------------- Ingress Watcher ----------------

var ingressThresholdMap = map[string]float64{
	"5xx-warning":  10,
	"5xx-critical": 20,
}

func (w *Watcher) StartIngressWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet,
		5*time.Second,
		informers.WithNamespace(namespace),
	)
	informer := factory.Networking().V1().Ingresses().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { w.updateIngress(obj.(*networkingv1.Ingress)) },
		UpdateFunc: func(_, obj interface{}) { w.updateIngress(obj.(*networkingv1.Ingress)) },
		DeleteFunc: func(obj interface{}) { w.deleteIngress(obj.(*networkingv1.Ingress)) },
	})

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
}

func (w *Watcher) updateIngress(ing *networkingv1.Ingress) {
	enabled := enabledLabel(ing.Labels)
	w.metrics.IngressEnabled.WithLabelValues(ing.Namespace, ing.Name).Set(enabled)
	if enabled == 1 {
		for key, def := range ingressThresholdMap {
			w.metrics.IngressThreshold.WithLabelValues(ing.Namespace, ing.Name, key).Set(thresholdLabel(ing.Labels, key, def))
		}
	}
	log.Printf("[ING UPDATE] %s/%s", ing.Namespace, ing.Name)
	lastObserved = time.Now()
}

func (w *Watcher) deleteIngress(ing *networkingv1.Ingress) {
	w.metrics.IngressEnabled.DeleteLabelValues(ing.Namespace, ing.Name)
	for key := range ingressThresholdMap {
		w.metrics.IngressThreshold.DeleteLabelValues(ing.Namespace, ing.Name, key)
	}
	log.Printf("[ING DELETE] %s/%s", ing.Namespace, ing.Name)
	lastObserved = time.Now()
}

// ---------------- CronJob Watcher ----------------

func (w *Watcher) StartCronJobWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet,
		5*time.Second,
		informers.WithNamespace(namespace),
	)
	informer := factory.Batch().V1().CronJobs().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { w.updateCronJob(obj.(*batchv1.CronJob)) },
		UpdateFunc: func(_, obj interface{}) { w.updateCronJob(obj.(*batchv1.CronJob)) },
		DeleteFunc: func(obj interface{}) { w.deleteCronJob(obj.(*batchv1.CronJob)) },
	})

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
}

func (w *Watcher) updateCronJob(job *batchv1.CronJob) {
	enabled := enabledLabel(job.Labels)
	w.metrics.CronJobEnabled.WithLabelValues(job.Namespace, job.Name).Set(enabled)
	log.Printf("[CRONJOB UPDATE] %s/%s", job.Namespace, job.Name)
	lastObserved = time.Now()
}


func (w *Watcher) deleteCronJob(job *batchv1.CronJob) {
	w.metrics.CronJobEnabled.DeleteLabelValues(job.Namespace, job.Name)
	log.Printf("[CRONJOB DELETE] %s/%s", job.Namespace, job.Name)
	lastObserved = time.Now()
}
