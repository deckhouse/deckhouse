package metrics

import (
	"fmt"

	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

const (
	RegistryRequestTimeMetric = "registry_request_time"
	RegistryRequestsCount     = "registry_requests_count"
)

func RegisterMetrics(ms *metricstorage.MetricStorage) error {
	_, err := ms.RegisterHistogram(RegistryRequestTimeMetric, []string{}, []float64{0.5, 0.95, 0.99}, options.WithHelp("Checks request time to registry"))
	if err != nil {
		return fmt.Errorf("can not register registry_request_time: %w", err)
	}

	_, err = ms.RegisterGauge(RegistryRequestsCount, []string{}, options.WithHelp("Checks count of requests to registry"))
	if err != nil {
		return fmt.Errorf("can not register registry_requests_count: %w", err)
	}

	return nil
}
