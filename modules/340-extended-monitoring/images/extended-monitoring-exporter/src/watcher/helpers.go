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

func boolToFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func enabledOnNamespace(labels map[string]string) bool {
	_, ok := labels[namespacesEnabledLabel]
	return ok
}

func enabledLabel(labels map[string]string) bool {
	val, ok := labels[namespacesEnabledLabel]

	if !ok {
		return true
	}

	if b, err := strconv.ParseBool(val); err == nil {
		return b
	}

	return true
}

func thresholdValue(labels map[string]string, threshold string, def float64) float64 {
	if val, ok := labels[labelThresholdPrefix+threshold]; ok {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		log.Printf("[thresholdLabel] could not parse %s=%s", threshold, val)
	}
	return def
}

func eventLabels(labels map[string]string, deleteEvent bool) (string, map[string]string) {
	logLabel := "EVENT"
	if deleteEvent {
		logLabel = "DELETE EVENT"
		labels = map[string]string{namespacesEnabledLabel: "false"}
	}
	return logLabel, labels
}

func runInformer[T any](
	ctx context.Context,
	informer cache.SharedIndexInformer,
	eventHandler func(*T, bool),
	name string,
) {
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { eventHandler(obj.(*T), false) },
		UpdateFunc: func(_, obj any) { eventHandler(obj.(*T), false) },
		DeleteFunc: func(obj any) { eventHandler(obj.(*T), true) },
	}); err != nil {
		log.Printf("[%s] AddEventHandler failed: %v", name, err)
		return
	}

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
	log.Printf("[%s watcher started]", name)
}

func (w *Watcher) updateMetrics(
	enabledVec *prometheus.GaugeVec,
	thresholdVec *prometheus.GaugeVec,
	resourceLabels map[string]string,
	thresholds map[string]float64,
	labels prometheus.Labels,
) {
	enabled := enabledLabel(resourceLabels)
	enabledVec.With(labels).Set(boolToFloat64(enabled))

	if enabled {
		for key, defaultValue := range thresholds {
			labels["threshold"] = key
			thresholdVec.With(labels).Set(thresholdValue(labels, key, defaultValue))
		}
	} else {
		enabledVec.DeletePartialMatch(labels)
		thresholdVec.DeletePartialMatch(labels)
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
