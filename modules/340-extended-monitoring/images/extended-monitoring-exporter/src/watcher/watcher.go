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

	"github.com/prometheus/client_golang/prometheus"
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
	runInformer[v1.Node](ctx, informer, w.updateNode, "NODE")
}

func (w *Watcher) updateNode(node *v1.Node, deleted bool) {
	logLabel, labels := eventLabels(node.Labels, deleted)

	log.Printf("[NODE %s] %s", logLabel, node.Name)

	w.updateMetrics(
		w.metrics.NodeEnabled,
		w.metrics.NodeThreshold,
		labels,
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
		log.Printf("[NS] AddEventHandler failed: %v", err)
	}

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
}

func (w *Watcher) addNamespace(ctx context.Context, ns *v1.Namespace) {
	enabled := enabledOnNamespace(ns.Labels)
	w.metrics.NamespacesEnabled.WithLabelValues(ns.Name).Set(boolToFloat64(enabled))
	log.Printf("[NS ADD] %s", ns.Name)

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
	enabled := enabledLabel(ns.Labels)
	w.metrics.NamespacesEnabled.WithLabelValues(ns.Name).Set(boolToFloat64(enabled))
	log.Printf("[NS UPDATE] %s", ns.Name)

	w.mu.Lock()
	cancel, exists := w.nsWatchers[ns.Name]
	w.mu.Unlock()

	if !enabled && exists {
		cancel()
		w.mu.Lock()
		delete(w.nsWatchers, ns.Name)
		w.mu.Unlock()

		w.cleanupNamespaceResources(ns.Name)

		log.Printf("[NS DISABLED] %s watchers stopped and resource metrics cleaned", ns.Name)
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
	runInformer[v1.Pod](ctx, informer, w.updatePod, "POD")
}

func (w *Watcher) updatePod(pod *v1.Pod, deleted bool) {
	logLabel, labels := eventLabels(pod.Labels, deleted)

	log.Printf("[POD %s] %s", logLabel, pod.Name)

	w.updateMetrics(
		w.metrics.PodEnabled,
		w.metrics.PodThreshold,
		labels,
		podThresholdMap,
		prometheus.Labels{"namespace": pod.Namespace, "pod": pod.Name},
	)
}

// ---------------- DaemonSet Watcher ----------------

func (w *Watcher) StartDaemonSetWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Apps().V1().DaemonSets().Informer()
	runInformer[appsv1.DaemonSet](ctx, informer, w.updateDaemonSet, "DS")
}

func (w *Watcher) updateDaemonSet(ds *appsv1.DaemonSet, deleted bool) {
	logLabel, labels := eventLabels(ds.Labels, deleted)

	log.Printf("[DAEMONSET %s] %s", logLabel, ds.Name)

	w.updateMetrics(
		w.metrics.DaemonSetEnabled,
		w.metrics.DaemonSetThreshold,
		labels,
		daemonSetThresholdMap,
		prometheus.Labels{"namespace": ds.Namespace, "daemonset": ds.Name},
	)
}

// ---------------- StatefulSet Watcher ----------------

func (w *Watcher) StartStatefulSetWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Apps().V1().StatefulSets().Informer()
	runInformer[appsv1.StatefulSet](ctx, informer, w.updateStatefulSet, "STS")
}

func (w *Watcher) updateStatefulSet(sts *appsv1.StatefulSet, deleted bool) {
	logLabel, labels := eventLabels(sts.Labels, deleted)

	log.Printf("[STATEFULSET %s] %s", logLabel, sts.Name)
	w.updateMetrics(
		w.metrics.StatefulSetEnabled,
		w.metrics.StatefulSetThreshold,
		labels,
		statefulSetThresholdMap,
		prometheus.Labels{"namespace": sts.Namespace, "statefulset": sts.Name},
	)
}

// ---------------- Deployment Watcher ----------------

func (w *Watcher) StartDeploymentWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Apps().V1().Deployments().Informer()
	runInformer[appsv1.Deployment](ctx, informer, w.updateDeployment, "DEP")
}

func (w *Watcher) updateDeployment(dep *appsv1.Deployment, deleted bool) {
	logLabel, labels := eventLabels(dep.Labels, deleted)

	log.Printf("[DEPLOYMENT %s] %s", logLabel, dep.Name)
	w.updateMetrics(
		w.metrics.DeploymentEnabled,
		w.metrics.DeploymentThreshold,
		labels,
		deploymentThresholdMap,
		prometheus.Labels{"namespace": dep.Namespace, "deployment": dep.Name},
	)
	log.Printf("[DEP UPDATE] %s/%s", dep.Namespace, dep.Name)
}

// ---------------- Ingress Watcher ----------------

func (w *Watcher) StartIngressWatcher(ctx context.Context, namespace string) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientSet, 0, informers.WithNamespace(namespace),
	)
	informer := factory.Networking().V1().Ingresses().Informer()
	runInformer[networkingv1.Ingress](ctx, informer, w.updateIngress, "ING")
}

func (w *Watcher) updateIngress(ing *networkingv1.Ingress, deleted bool) {
	logLabel, labels := eventLabels(ing.Labels, deleted)

	log.Printf("[INGRESS %s] %s", logLabel, ing.Name)
	w.updateMetrics(
		w.metrics.IngressEnabled,
		w.metrics.IngressThreshold,
		labels,
		ingressThresholdMap,
		prometheus.Labels{"namespace": ing.Namespace, "ingress": ing.Name},
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

	metricLabels := prometheus.Labels{"namespace": job.Namespace, "cronjob": job.Name}

	if deleted {
		w.metrics.CronJobEnabled.DeletePartialMatch(metricLabels)
		met.UpdateLastObserved()
		return
	}

	w.metrics.CronJobEnabled.With(metricLabels).Set(boolToFloat64(enabledLabel(labels)))
	met.UpdateLastObserved()
}
