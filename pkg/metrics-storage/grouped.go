package metricsstorage

import (
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
	"github.com/prometheus/client_golang/prometheus"
)

type GroupedStorage interface {
	Registerer() prometheus.Registerer
	ExpireGroupMetrics(group string)
	ExpireGroupMetricByName(group, name string)
	GetOrCreateCounterCollector(name string, labelNames []string) (*collectors.ConstCounterCollector, error)
	GetOrCreateGaugeCollector(name string, labelNames []string) (*collectors.ConstGaugeCollector, error)
	CounterAdd(group string, name string, value float64, labels map[string]string)
	GaugeSet(group string, name string, value float64, labels map[string]string)
}
