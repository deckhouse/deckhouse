package metrics

import (
	"fmt"
	"net/http"
	"time"

	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

const (
	RegistryRequestTimeMetric = "registry_request_time"
	RegistryRequestsCount     = "registry_requests_count"
)

func RegisterMetrics(ms *metricstorage.MetricStorage) error {
	_, err := ms.RegisterHistogram(RegistryRequestTimeMetric, []string{"status_code"}, []float64{0.5, 0.95, 0.99}, options.WithHelp("Checks request time to registry"))
	if err != nil {
		return fmt.Errorf("can not register registry_request_time: %w", err)
	}

	_, err = ms.RegisterGauge(RegistryRequestsCount, []string{}, options.WithHelp("Checks count of requests to registry"))
	if err != nil {
		return fmt.Errorf("can not register registry_requests_count: %w", err)
	}

	return nil
}

type MetricRoundTripper struct {
	Next http.RoundTripper
	MS   *metricstorage.MetricStorage
}

func (l MetricRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	// Before request
	timeBeforeRequest := time.Now().Unix()

	// Request
	resp, err := l.Next.RoundTrip(r)

	// After request
	requestTime := time.Now().Unix() - timeBeforeRequest
	l.MS.HistogramObserve(RegistryRequestTimeMetric, float64(requestTime), map[string]string{"status_code": resp.Status}, []float64{0.5, 0.95, 0.99})
	l.MS.GaugeAdd(RegistryRequestsCount, 1.0, map[string]string{})

	return resp, err
}
