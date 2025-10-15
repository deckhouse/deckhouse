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

package watcher

import (
	"context"
	"log"
	"sync"

	met "extended-monitoring/metrics"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Watcher struct {
	clientSet  *kubernetes.Clientset
	mu         sync.Mutex
	nsWatchers map[string]context.CancelFunc
	metrics    *met.ExporterMetrics
}

func NewWatcher(clientSet *kubernetes.Clientset, metrics *met.ExporterMetrics) *Watcher {
	return &Watcher{
		clientSet:  clientSet,
		nsWatchers: make(map[string]context.CancelFunc),
		metrics:    metrics,
	}
}

// ---------------- Node Watcher ----------------

func (w *Watcher) StartNodeWatcher(ctx context.Context) {
	factory := informers.NewSharedInformerFactory(w.clientSet, 0)
	informer := factory.Core().V1().Nodes().Informer()
	runInformer[v1.Node](ctx, informer, w.updateNode, w.deleteNode, "NODE")
}

func (w *Watcher) updateNode(node *v1.Node) {
	w.updateMetrics(
		w.metrics.NodeEnabled.WithLabelValues,
		w.metrics.NodeThreshold.WithLabelValues,
		node.Labels,
		nodeThresholdMap,
		node.Name,
	)
	log.Printf("[NODE UPDATE] %s", node.Name)
	met.UpdateIsPopulated()
}

func (w *Watcher) deleteNode(node *v1.Node) {
	w.metrics.NodeEnabled.DeleteLabelValues(node.Name)
	for key := range nodeThresholdMap {
		w.metrics.NodeThreshold.DeleteLabelValues(node.Name, key)
	}
	log.Printf("[NODE DELETE] %s", node.Name)
	met.UpdateLastObserved()
	met.UpdateIsPopulated()
}

// ---------------- Namespace Watcher ----------------

func (w *Watcher) StartNamespaceWatcher(ctx context.Context) {
	factory := informers.NewSharedInformerFactory(w.clientSet, 0)
	informer := factory.Core().V1().Namespaces().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { w.addNamespace(ctx, obj.(*v1.Namespace)) },
		UpdateFunc: func(_, obj interface{}) { w.updateNamespace(ctx, obj.(*v1.Namespace)) },
		DeleteFunc: func(obj interface{}) { w.deleteNamespace(obj.(*v1.Namespace)) },
	})
	if err != nil {
		log.Printf("[NS] AddEventHandler failed: %v", err)
	}

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
	met.UpdateLastObserved()
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

		w.cleanupNamespaceResources(ns.Name)

		log.Printf("[NS DISABLED] %s watchers stopped and resource metrics cleaned", ns.Name)
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
	met.UpdateLastObserved()
}

func (w *Watcher) deleteNamespace(ns *v1.Namespace) {
	w.metrics.NamespacesEnabled.DeleteLabelValues(ns.Name)
	w.mu.Lock()
	if cancel, exists := w.nsWatchers[ns.Name]; exists {
		cancel()
		delete(w.nsWatchers, ns.Name)
		log.Printf("[NS DELETE] %s watchers stopped", ns.Name)
		met.UpdateLastObserved()
	}
	w.mu.Unlock()
}

// ---------------- Pod Watcher ----------------

func (w *Watcher) StartPodWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Core().V1().Pods().Informer()
	runInformer[v1.Pod](ctx, informer, w.updatePod, w.deletePod, "POD")
}

func (w *Watcher) updatePod(pod *v1.Pod) {
	w.updateMetrics(
		w.metrics.PodEnabled.WithLabelValues,
		w.metrics.PodThreshold.WithLabelValues,
		pod.Labels,
		podThresholdMap,
		pod.Namespace, pod.Name,
	)
	log.Printf("[POD UPDATE] %s/%s", pod.Namespace, pod.Name)
}

func (w *Watcher) deletePod(pod *v1.Pod) {
	w.metrics.PodEnabled.DeleteLabelValues(pod.Namespace, pod.Name)
	for key := range podThresholdMap {
		w.metrics.PodThreshold.DeleteLabelValues(pod.Namespace, pod.Name, key)
	}
	log.Printf("[POD DELETE] %s/%s", pod.Namespace, pod.Name)
	met.UpdateLastObserved()
}

// ---------------- DaemonSet Watcher ----------------

func (w *Watcher) StartDaemonSetWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Apps().V1().DaemonSets().Informer()
	runInformer[appsv1.DaemonSet](ctx, informer, w.updateDaemonSet, w.deleteDaemonSet, "DS")
}

func (w *Watcher) updateDaemonSet(ds *appsv1.DaemonSet) {
	w.updateMetrics(
		w.metrics.DaemonSetEnabled.WithLabelValues,
		w.metrics.DaemonSetThreshold.WithLabelValues,
		ds.Labels,
		daemonSetThresholdMap,
		ds.Namespace, ds.Name,
	)
	log.Printf("[DS UPDATE] %s/%s", ds.Namespace, ds.Name)
}

func (w *Watcher) deleteDaemonSet(ds *appsv1.DaemonSet) {
	w.metrics.DaemonSetEnabled.DeleteLabelValues(ds.Namespace, ds.Name)
	for key := range daemonSetThresholdMap {
		w.metrics.DaemonSetThreshold.DeleteLabelValues(ds.Namespace, ds.Name, key)
	}
	log.Printf("[DS DELETE] %s/%s", ds.Namespace, ds.Name)
	met.UpdateLastObserved()
}

// ---------------- StatefulSet Watcher ----------------

func (w *Watcher) StartStatefulSetWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Apps().V1().StatefulSets().Informer()
	runInformer[appsv1.StatefulSet](ctx, informer, w.updateStatefulSet, w.deleteStatefulSet, "STS")
}

func (w *Watcher) updateStatefulSet(sts *appsv1.StatefulSet) {
	w.updateMetrics(
		w.metrics.StatefulSetEnabled.WithLabelValues,
		w.metrics.StatefulSetThreshold.WithLabelValues,
		sts.Labels,
		statefulSetThresholdMap,
		sts.Namespace, sts.Name,
	)
	log.Printf("[STS UPDATE] %s/%s", sts.Namespace, sts.Name)
}

func (w *Watcher) deleteStatefulSet(sts *appsv1.StatefulSet) {
	w.metrics.StatefulSetEnabled.DeleteLabelValues(sts.Namespace, sts.Name)
	for key := range statefulSetThresholdMap {
		w.metrics.StatefulSetThreshold.DeleteLabelValues(sts.Namespace, sts.Name, key)
	}
	log.Printf("[STS DELETE] %s/%s", sts.Namespace, sts.Name)
	met.UpdateLastObserved()
}

// ---------------- Deployment Watcher ----------------

func (w *Watcher) StartDeploymentWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Apps().V1().Deployments().Informer()
	runInformer[appsv1.Deployment](ctx, informer, w.updateDeployment, w.deleteDeployment, "DEP")
}

func (w *Watcher) updateDeployment(dep *appsv1.Deployment) {
	w.updateMetrics(
		w.metrics.DeploymentEnabled.WithLabelValues,
		w.metrics.DeploymentThreshold.WithLabelValues,
		dep.Labels,
		deploymentThresholdMap,
		dep.Namespace, dep.Name,
	)
	log.Printf("[DEP UPDATE] %s/%s", dep.Namespace, dep.Name)
}

func (w *Watcher) deleteDeployment(dep *appsv1.Deployment) {
	w.metrics.DeploymentEnabled.DeleteLabelValues(dep.Namespace, dep.Name)
	for key := range deploymentThresholdMap {
		w.metrics.DeploymentThreshold.DeleteLabelValues(dep.Namespace, dep.Name, key)
	}
	log.Printf("[DEP DELETE] %s/%s", dep.Namespace, dep.Name)
	met.UpdateLastObserved()
}

// ---------------- Ingress Watcher ----------------

func (w *Watcher) StartIngressWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Networking().V1().Ingresses().Informer()
	runInformer[networkingv1.Ingress](ctx, informer, w.updateIngress, w.deleteIngress, "ING")
}

func (w *Watcher) updateIngress(ing *networkingv1.Ingress) {
	w.updateMetrics(
		w.metrics.IngressEnabled.WithLabelValues,
		w.metrics.IngressThreshold.WithLabelValues,
		ing.Labels,
		ingressThresholdMap,
		ing.Namespace, ing.Name,
	)
	log.Printf("[ING UPDATE] %s/%s", ing.Namespace, ing.Name)
}

func (w *Watcher) deleteIngress(ing *networkingv1.Ingress) {
	w.metrics.IngressEnabled.DeleteLabelValues(ing.Namespace, ing.Name)
	for key := range ingressThresholdMap {
		w.metrics.IngressThreshold.DeleteLabelValues(ing.Namespace, ing.Name, key)
	}
	log.Printf("[ING DELETE] %s/%s", ing.Namespace, ing.Name)
	met.UpdateLastObserved()
}

// ---------------- CronJob Watcher ----------------

func (w *Watcher) StartCronJobWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Batch().V1().CronJobs().Informer()
	runInformer[batchv1.CronJob](ctx, informer, w.updateCronJob, w.deleteCronJob, "CRONJOB")
}

func (w *Watcher) updateCronJob(job *batchv1.CronJob) {
	enabled := enabledLabel(job.Labels)
	w.metrics.CronJobEnabled.WithLabelValues(job.Namespace, job.Name).Set(enabled)
	log.Printf("[CRONJOB UPDATE] %s/%s", job.Namespace, job.Name)
	met.UpdateLastObserved()
}

func (w *Watcher) deleteCronJob(job *batchv1.CronJob) {
	w.metrics.CronJobEnabled.DeleteLabelValues(job.Namespace, job.Name)
	log.Printf("[CRONJOB DELETE] %s/%s", job.Namespace, job.Name)
	met.UpdateLastObserved()
}
