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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
)

const (
	metricReleasesGroup = "d8_releases"
)

func newMetricsUpdater(input *go_hook.HookInput) *metricsUpdater {
	return &metricsUpdater{input.MetricsCollector}
}

type metricsUpdater struct {
	MetricsCollector go_hook.MetricsCollector
}

func (mu metricsUpdater) WaitingManual(name string, totalPendingManualReleases float64) {
	mu.MetricsCollector.Set("d8_release_waiting_manual", totalPendingManualReleases, map[string]string{"name": name}, metrics.WithGroup(metricReleasesGroup))
}

func (mu metricsUpdater) ReleaseBlocked(name, reason string) {
	mu.MetricsCollector.Set("d8_release_blocked", 1, map[string]string{"name": name, "reason": reason}, metrics.WithGroup(metricReleasesGroup))
}
