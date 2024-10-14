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

import (
	"github.com/flant/shell-operator/pkg/metric"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

const metricReleasesGroup = "d8_releases"

func newMetricUpdater(metricStorage metric.Storage) *metricUpdater {
	return &metricUpdater{
		metricStorage: metricStorage,
	}
}

type metricUpdater struct {
	metricStorage metric.Storage
}

func (mu metricUpdater) WaitingManual(release *v1alpha1.DeckhouseRelease, totalPendingManualReleases float64) {
	mu.metricStorage.GaugeSet(
		"d8_module_release_waiting_manual",
		totalPendingManualReleases,
		map[string]string{
			"name":    release.GetName(),
			"kind":    "deckhouse",
			"version": release.Spec.Version,
		})
}

func (mu metricUpdater) ReleaseBlocked(name, reason string) {
	mu.metricStorage.Grouped().GaugeSet(metricReleasesGroup, "d8_release_blocked", 1, map[string]string{"name": name, "reason": reason})
}
