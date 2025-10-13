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
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	met "extended-monitoring/metrics"

	"k8s.io/client-go/tools/cache"
)

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

func runInformer[T any](
	ctx context.Context,
	informer cache.SharedIndexInformer,
	update func(*T),
	delete func(*T),
	name string,
) {
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { update(obj.(*T)) },
		UpdateFunc: func(_, obj interface{}) { update(obj.(*T)) },
		DeleteFunc: func(obj interface{}) { delete(obj.(*T)) },
	}); err != nil {
		log.Printf("[%s] AddEventHandler failed: %v", name, err)
		return
	}

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
	log.Printf("[%s watcher started]", name)
}

func (w *Watcher) updateMetrics(
	enabledMetric func(...string) prometheus.Gauge,
	thresholdMetric func(...string) prometheus.Gauge,
	labels map[string]string,
	thresholds map[string]float64,
	labelValues ...string,
) {
	enabled := enabledLabel(labels)
	enabledMetric(labelValues...).Set(enabled)

	if enabled == 1 {
		for key, def := range thresholds {
			thresholdMetric(append(labelValues, key)...).
				Set(thresholdLabel(labels, key, def))
		}
	} else {
		for key := range thresholds {
			thresholdMetric(append(labelValues, key)...).Set(0)
		}
	}
	met.UpdateLastObserved()
}

func (w *Watcher) cleanupNamespaceResources(ns string) {
	w.metrics.PodEnabled.DeletePartialMatch(prometheus.Labels{"namespace": ns})
	w.metrics.PodThreshold.DeletePartialMatch(prometheus.Labels{"namespace": ns})

	w.metrics.DaemonSetEnabled.DeletePartialMatch(prometheus.Labels{"namespace": ns})
	w.metrics.DaemonSetThreshold.DeletePartialMatch(prometheus.Labels{"namespace": ns})

	w.metrics.StatefulSetEnabled.DeletePartialMatch(prometheus.Labels{"namespace": ns})
	w.metrics.StatefulSetThreshold.DeletePartialMatch(prometheus.Labels{"namespace": ns})

	w.metrics.DeploymentEnabled.DeletePartialMatch(prometheus.Labels{"namespace": ns})
	w.metrics.DeploymentThreshold.DeletePartialMatch(prometheus.Labels{"namespace": ns})

	w.metrics.IngressEnabled.DeletePartialMatch(prometheus.Labels{"namespace": ns})
	w.metrics.IngressThreshold.DeletePartialMatch(prometheus.Labels{"namespace": ns})

	w.metrics.CronJobEnabled.DeletePartialMatch(prometheus.Labels{"namespace": ns})
}
