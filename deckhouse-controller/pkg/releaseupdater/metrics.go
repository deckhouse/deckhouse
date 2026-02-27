/*
Copyright 2024 Flant JSC

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

package releaseupdater

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const D8ReleaseBlockedMetricName = "d8_release_info"
const ModuleReleaseBlockedMetricName = "d8_module_release_info"

func NewMetricsUpdater(metricStorage metricsstorage.Storage, metricName string) *MetricsUpdater {
	return &MetricsUpdater{
		metricStorage: metricStorage,
		metricName:    metricName,
	}
}

type MetricsUpdater struct {
	metricStorage metricsstorage.Storage
	metricName    string
}

func (mu *MetricsUpdater) UpdateReleaseMetric(name string, metricLabels MetricLabels) {
	mu.PurgeReleaseMetric(name)
	mu.metricStorage.Grouped().GaugeSet(name, mu.metricName, 1, metricLabels)
}

func (mu *MetricsUpdater) PurgeReleaseMetric(name string) {
	mu.metricStorage.Grouped().ExpireGroupMetricByName(name, mu.metricName)
}

type MetricLabels map[string]string

func NewReleaseMetricLabels(release v1alpha1.Release) MetricLabels {
	labels := make(MetricLabels, 9)

	labels[metrics.LabelName] = release.GetName()

	labels.SetFalse(metrics.LabelManualApprovalRequired)
	labels.SetFalse(metrics.LabelDisruptionApprovalRequired)
	labels.SetFalse(metrics.LabelRequirementsNotMet)
	labels.SetFalse(metrics.LabelNotificationNotSent)

	labels[metrics.LabelReleaseQueueDepth] = "nil"
	labels[metrics.LabelMajorReleaseDepth] = "nil"
	labels[metrics.LabelMajorReleaseName] = "nil"
	labels[metrics.LabelFromToName] = "nil"

	if _, ok := release.(*v1alpha1.ModuleRelease); ok {
		labels[metrics.LabelModule] = release.GetModuleName()
	}

	return labels
}

func (ml MetricLabels) SetTrue(key string) {
	ml[key] = "true"
}

func (ml MetricLabels) SetFalse(key string) {
	ml[key] = "false"
}
