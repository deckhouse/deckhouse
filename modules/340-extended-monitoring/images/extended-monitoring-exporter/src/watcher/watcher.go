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

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	met "extended-monitoring/metrics"
)

type Watcher struct {
	clientSet  *kubernetes.Clientset
	mu         sync.Mutex
	nsWatchers map[string]context.CancelFunc
	nsLabels   map[string]map[string]string
	metrics    *met.ExporterMetrics
}

func NewWatcher(clientSet *kubernetes.Clientset, metrics *met.ExporterMetrics) *Watcher {
	return &Watcher{
		clientSet:  clientSet,
		nsWatchers: make(map[string]context.CancelFunc),
		nsLabels:   make(map[string]map[string]string),
		metrics:    metrics,
	}
}

// ---------------- Node Watcher ----------------

func (w *Watcher) StartNodeWatcher(ctx context.Context) {
	factory := informers.NewSharedInformerFactory(w.clientSet, 0)
	informer := factory.Core().V1().Nodes().Informer()
	runInformer[v1.Node](ctx, informer, w.updateNode, "NODE")
}

func (w *Watcher) updateNode(node *v1.Node, deleted bool) {
	logLabel, labels := eventLabels(node.Labels, deleted)

	log.Printf("[NODE %s] %s", logLabel, node.Name)

	w.updateMetrics(
		w.metrics.NodeEnabled,
		w.metrics.NodeThreshold,
		labels,
		nil,
		nodeThresholdMap,
		prometheus.Labels{"node": node.Name},
	)
	met.UpdateIsPopulated()
}

// ---------------- Namespace Watcher ----------------

func (w *Watcher) StartNamespaceWatcher(ctx context.Context) {
	factory := informers.NewSharedInformerFactory(w.clientSet, 0)
	informer := factory.Core().V1().Namespaces().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { w.addNamespace(ctx, obj.(*v1.Namespace)) },
		UpdateFunc: func(_, obj any) { w.updateNamespace(ctx, obj.(*v1.Namespace)) },
		DeleteFunc: func(obj any) { w.deleteNamespace(obj.(*v1.Namespace)) },
	})
	if err != nil {
		log.Printf("[NAMESPACE] AddEventHandler failed: %v", err)
	}

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
}

func (w *Watcher) addNamespace(ctx context.Context, ns *v1.Namespace) {
	enabled := enabledOnNamespace(ns.Labels)
	w.metrics.NamespacesEnabled.WithLabelValues(ns.Name).Set(boolToFloat64(enabled))
	log.Printf("[NAMESPACE ADDED] %s", ns.Name)

	w.mu.Lock()
	w.nsLabels[ns.Name] = ns.Labels
	w.mu.Unlock()

	if enabled {
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
	enabled := enabledOnNamespace(ns.Labels)
	w.metrics.NamespacesEnabled.WithLabelValues(ns.Name).Set(boolToFloat64(enabled))
	log.Printf("[NAMESPACE UPDATE] %s", ns.Name)

	w.mu.Lock()
	oldLabels := w.nsLabels[ns.Name]
	podThresholdsChanged := thresholdLabelsChangedForMap(oldLabels, ns.Labels, podThresholdMap)
	ingressThresholdsChanged := thresholdLabelsChangedForMap(oldLabels, ns.Labels, ingressThresholdMap)
	replicasThresholdChanged := thresholdLabelsChangedForMap(oldLabels, ns.Labels, daemonSetThresholdMap)
	w.nsLabels[ns.Name] = ns.Labels
	w.mu.Unlock()

	w.mu.Lock()
	cancel, exists := w.nsWatchers[ns.Name]
	w.mu.Unlock()

	if !enabled && exists {
		cancel()
		w.mu.Lock()
		delete(w.nsWatchers, ns.Name)
		w.mu.Unlock()

		w.cleanupNamespaceResources(ns.Name)

		log.Printf("[NAMESPACE DISABLED] %s watchers stopped and resource metrics cleaned", ns.Name)
	}

	if enabled && !exists {
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

		log.Printf("[NAMESPACE ENABLED] %s watchers started", ns.Name)
	}

	if enabled && exists {
		if podThresholdsChanged {
			log.Printf("[NAMESPACE UPDATE] %s pod threshold labels changed, refreshing pods", ns.Name)
			w.refreshPods(ctx, ns.Name)
		}
		if ingressThresholdsChanged {
			log.Printf("[NAMESPACE UPDATE] %s ingress threshold labels changed, refreshing ingresses", ns.Name)
			w.refreshIngresses(ctx, ns.Name)
		}
		if replicasThresholdChanged {
			log.Printf("[NAMESPACE UPDATE] %s replicas-not-ready threshold changed, refreshing workload resources", ns.Name)
			w.refreshDaemonSets(ctx, ns.Name)
			w.refreshStatefulSets(ctx, ns.Name)
			w.refreshDeployments(ctx, ns.Name)
		}
	}

	met.UpdateLastObserved()
}

func (w *Watcher) deleteNamespace(ns *v1.Namespace) {
	w.metrics.NamespacesEnabled.DeleteLabelValues(ns.Name)
	w.mu.Lock()
	if cancel, exists := w.nsWatchers[ns.Name]; exists {
		cancel()
		delete(w.nsWatchers, ns.Name)
		delete(w.nsLabels, ns.Name)
		log.Printf("[NAMESPACE DELETED] %s watchers stopped", ns.Name)
		met.UpdateLastObserved()
	}
	w.mu.Unlock()
}

func (w *Watcher) refreshPods(ctx context.Context, namespace string) {
	podList, err := w.clientSet.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[NAMESPACE REFRESH] Failed to list pods in %s: %v", namespace, err)
		return
	}

	for i := range podList.Items {
		w.updatePod(&podList.Items[i], false)
	}
	log.Printf("[NAMESPACE REFRESH] Updated %d pods in %s", len(podList.Items), namespace)
}

func (w *Watcher) refreshIngresses(ctx context.Context, namespace string) {
	ingressList, err := w.clientSet.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[NAMESPACE REFRESH] Failed to list ingresses in %s: %v", namespace, err)
		return
	}

	for i := range ingressList.Items {
		w.updateIngress(&ingressList.Items[i], false)
	}
	log.Printf("[NAMESPACE REFRESH] Updated %d ingresses in %s", len(ingressList.Items), namespace)
}

func (w *Watcher) refreshDaemonSets(ctx context.Context, namespace string) {
	dsList, err := w.clientSet.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[NAMESPACE REFRESH] Failed to list daemonsets in %s: %v", namespace, err)
		return
	}

	for i := range dsList.Items {
		w.updateDaemonSet(&dsList.Items[i], false)
	}
	log.Printf("[NAMESPACE REFRESH] Updated %d daemonsets in %s", len(dsList.Items), namespace)
}

func (w *Watcher) refreshStatefulSets(ctx context.Context, namespace string) {
	stsList, err := w.clientSet.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[NAMESPACE REFRESH] Failed to list statefulsets in %s: %v", namespace, err)
		return
	}

	for i := range stsList.Items {
		w.updateStatefulSet(&stsList.Items[i], false)
	}
	log.Printf("[NAMESPACE REFRESH] Updated %d statefulsets in %s", len(stsList.Items), namespace)
}

func (w *Watcher) refreshDeployments(ctx context.Context, namespace string) {
	depList, err := w.clientSet.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[NAMESPACE REFRESH] Failed to list deployments in %s: %v", namespace, err)
		return
	}

	for i := range depList.Items {
		w.updateDeployment(&depList.Items[i], false)
	}
	log.Printf("[NAMESPACE REFRESH] Updated %d deployments in %s", len(depList.Items), namespace)
}

// ---------------- Pod Watcher ----------------

func (w *Watcher) StartPodWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Core().V1().Pods().Informer()
	runInformer[v1.Pod](ctx, informer, w.updatePod, "POD")
}

func (w *Watcher) updatePod(pod *v1.Pod, deleted bool) {
	logLabel, labels := eventLabels(pod.Labels, deleted)

	log.Printf("[POD %s] %s", logLabel, pod.Name)

	w.mu.Lock()
	nsLabels := w.nsLabels[pod.Namespace]
	w.mu.Unlock()

	labelset := prometheus.Labels{"namespace": pod.Namespace, "pod": pod.Name}

	if deleted {
		w.deleteMetrics(labelset, w.metrics.PodEnabled, w.metrics.PodThreshold)
		return
	}

	w.updateMetrics(
		w.metrics.PodEnabled,
		w.metrics.PodThreshold,
		labels,
		nsLabels,
		podThresholdMap,
		labelset,
	)
}

// ---------------- DaemonSet Watcher ----------------

func (w *Watcher) StartDaemonSetWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Apps().V1().DaemonSets().Informer()
	runInformer[appsv1.DaemonSet](ctx, informer, w.updateDaemonSet, "DAEMONSET")
}

func (w *Watcher) updateDaemonSet(ds *appsv1.DaemonSet, deleted bool) {
	logLabel, labels := eventLabels(ds.Labels, deleted)

	log.Printf("[DAEMONSET %s] %s", logLabel, ds.Name)

	w.mu.Lock()
	nsLabels := w.nsLabels[ds.Namespace]
	w.mu.Unlock()

	labelset := prometheus.Labels{"namespace": ds.Namespace, "daemonset": ds.Name}

	if deleted {
		w.deleteMetrics(labelset, w.metrics.DaemonSetEnabled, w.metrics.DaemonSetThreshold)
		return
	}

	w.updateMetrics(
		w.metrics.DaemonSetEnabled,
		w.metrics.DaemonSetThreshold,
		labels,
		nsLabels,
		daemonSetThresholdMap,
		labelset,
	)
}

// ---------------- StatefulSet Watcher ----------------

func (w *Watcher) StartStatefulSetWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Apps().V1().StatefulSets().Informer()
	runInformer[appsv1.StatefulSet](ctx, informer, w.updateStatefulSet, "STATEFULSET")
}

func (w *Watcher) updateStatefulSet(sts *appsv1.StatefulSet, deleted bool) {
	logLabel, labels := eventLabels(sts.Labels, deleted)

	log.Printf("[STATEFULSET %s] %s", logLabel, sts.Name)

	w.mu.Lock()
	nsLabels := w.nsLabels[sts.Namespace]
	w.mu.Unlock()

	labelset := prometheus.Labels{"namespace": sts.Namespace, "statefulset": sts.Name}

	if deleted {
		w.deleteMetrics(labelset, w.metrics.StatefulSetEnabled, w.metrics.StatefulSetThreshold)
		return
	}

	w.updateMetrics(
		w.metrics.StatefulSetEnabled,
		w.metrics.StatefulSetThreshold,
		labels,
		nsLabels,
		statefulSetThresholdMap,
		labelset,
	)
}

// ---------------- Deployment Watcher ----------------

func (w *Watcher) StartDeploymentWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Apps().V1().Deployments().Informer()
	runInformer[appsv1.Deployment](ctx, informer, w.updateDeployment, "DEPLOYMENT")
}

func (w *Watcher) updateDeployment(dep *appsv1.Deployment, deleted bool) {
	logLabel, labels := eventLabels(dep.Labels, deleted)

	log.Printf("[DEPLOYMENT %s] %s", logLabel, dep.Name)

	w.mu.Lock()
	nsLabels := w.nsLabels[dep.Namespace]
	w.mu.Unlock()

	labelset := prometheus.Labels{"namespace": dep.Namespace, "deployment": dep.Name}

	if deleted {
		w.deleteMetrics(labelset, w.metrics.DeploymentEnabled, w.metrics.DeploymentThreshold)
		return
	}

	w.updateMetrics(
		w.metrics.DeploymentEnabled,
		w.metrics.DeploymentThreshold,
		labels,
		nsLabels,
		deploymentThresholdMap,
		labelset,
	)
}

// ---------------- Ingress Watcher ----------------

func (w *Watcher) StartIngressWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Networking().V1().Ingresses().Informer()
	runInformer[networkingv1.Ingress](ctx, informer, w.updateIngress, "INGRESS")
}

func (w *Watcher) updateIngress(ing *networkingv1.Ingress, deleted bool) {
	logLabel, labels := eventLabels(ing.Labels, deleted)

	log.Printf("[INGRESS %s] %s", logLabel, ing.Name)

	w.mu.Lock()
	nsLabels := w.nsLabels[ing.Namespace]
	w.mu.Unlock()

	labelset := prometheus.Labels{"namespace": ing.Namespace, "ingress": ing.Name}

	if deleted {
		w.deleteMetrics(labelset, w.metrics.IngressEnabled, w.metrics.IngressThreshold)
		return
	}

	w.updateMetrics(
		w.metrics.IngressEnabled,
		w.metrics.IngressThreshold,
		labels,
		nsLabels,
		ingressThresholdMap,
		labelset,
	)
}

// ---------------- CronJob Watcher ----------------

func (w *Watcher) StartCronJobWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Batch().V1().CronJobs().Informer()
	runInformer[batchv1.CronJob](ctx, informer, w.updateCronJob, "CRONJOB")
}

func (w *Watcher) updateCronJob(job *batchv1.CronJob, deleted bool) {
	logLabel, labels := eventLabels(job.Labels, deleted)

	log.Printf("[CRONJOB %s] %s", logLabel, job.Name)

	labelset := prometheus.Labels{"namespace": job.Namespace, "cronjob": job.Name}

	if deleted {
		w.metrics.CronJobEnabled.DeletePartialMatch(labelset)
		met.UpdateLastObserved()
		return
	}

	w.metrics.CronJobEnabled.With(labelset).Set(boolToFloat64(enabledLabel(labels)))
	met.UpdateLastObserved()
}
