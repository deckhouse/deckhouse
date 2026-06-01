/*
Copyright 2026 Flant JSC

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

package common

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/cache"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func init() {
	cache.SetReflectorMetricsProvider(reflectorMetricsProvider{})
}

var (
	listsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "reflector_lists_total",
		Help: "Total number of API lists done by the reflectors",
	}, []string{"name"})

	listsDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: "reflector_list_duration_seconds",
		Help: "How long an API list takes to return and decode for the reflectors",
	}, []string{"name"})

	itemsPerList = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: "reflector_items_per_list",
		Help: "How many items an API list returns to the reflectors",
	}, []string{"name"})

	watchesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "reflector_watches_total",
		Help: "Total number of API watches done by the reflectors",
	}, []string{"name"})

	shortWatchesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "reflector_short_watches_total",
		Help: "Total number of short API watches done by the reflectors",
	}, []string{"name"})

	watchDuration = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: "reflector_watch_duration_seconds",
		Help: "How long an API watch takes to return and decode for the reflectors",
	}, []string{"name"})

	itemsPerWatch = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: "reflector_items_per_watch",
		Help: "How many items an API watch returns to the reflectors",
	}, []string{"name"})

	lastResourceVersion = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "reflector_last_resource_version",
		Help: "Last resource version seen for the reflectors",
	}, []string{"name"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(
		listsTotal,
		listsDuration,
		itemsPerList,
		watchesTotal,
		shortWatchesTotal,
		watchDuration,
		itemsPerWatch,
		lastResourceVersion,
	)
}

type reflectorMetricsProvider struct{}

func (reflectorMetricsProvider) NewListsMetric(name string) cache.CounterMetric {
	return listsTotal.WithLabelValues(name)
}

func (reflectorMetricsProvider) NewListDurationMetric(name string) cache.SummaryMetric {
	return listsDuration.WithLabelValues(name)
}

func (reflectorMetricsProvider) NewItemsInListMetric(name string) cache.SummaryMetric {
	return itemsPerList.WithLabelValues(name)
}

func (reflectorMetricsProvider) NewWatchesMetric(name string) cache.CounterMetric {
	return watchesTotal.WithLabelValues(name)
}

func (reflectorMetricsProvider) NewShortWatchesMetric(name string) cache.CounterMetric {
	return shortWatchesTotal.WithLabelValues(name)
}

func (reflectorMetricsProvider) NewWatchDurationMetric(name string) cache.SummaryMetric {
	return watchDuration.WithLabelValues(name)
}

func (reflectorMetricsProvider) NewItemsInWatchMetric(name string) cache.SummaryMetric {
	return itemsPerWatch.WithLabelValues(name)
}

func (reflectorMetricsProvider) NewLastResourceVersionMetric(name string) cache.GaugeMetric {
	return lastResourceVersion.WithLabelValues(name)
}
