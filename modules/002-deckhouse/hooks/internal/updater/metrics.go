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
