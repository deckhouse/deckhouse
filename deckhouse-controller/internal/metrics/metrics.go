package metrics

import "github.com/flant/shell-operator/pkg/metric"

func RegisterMetrics(metricStorage metric.Storage) {
	registerModuleMetrics(metricStorage)
}

const (
	MigratedModuleNotFoundMetricName = "d8_migrated_module_not_found"
)

func registerModuleMetrics(metricStorage metric.Storage) {
	metricStorage.RegisterGauge(MigratedModuleNotFoundMetricName, map[string]string{
		"module_name": "",
	})
}
