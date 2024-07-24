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

package d8updater

import "github.com/flant/shell-operator/pkg/metric_storage"

const metricReleasesGroup = "d8_releases"

func newMetricUpdater(metricStorage *metric_storage.MetricStorage) *metricUpdater {
	return &metricUpdater{
		metricStorage: metricStorage,
	}
}

type metricUpdater struct {
	metricStorage *metric_storage.MetricStorage
}

func (mu metricUpdater) WaitingManual(name string, totalPendingManualReleases float64) {
	mu.metricStorage.GroupedVault.GaugeSet(metricReleasesGroup, "d8_release_waiting_manual", totalPendingManualReleases, map[string]string{"name": name})
}

func (mu metricUpdater) ReleaseBlocked(name, reason string) {
	mu.metricStorage.GroupedVault.GaugeSet(metricReleasesGroup, "d8_release_blocked", 1, map[string]string{"name": name, "reason": reason})
}
