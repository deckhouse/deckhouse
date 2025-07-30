// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"fmt"
	"net/http"
	"time"

	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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

func RoundTripOption(ms *metricstorage.MetricStorage) remote.Option {
	return remote.WithTransport(MetricRoundTripper{
		Next:          remote.DefaultTransport,
		MetricStorage: ms,
	})
}

type MetricRoundTripper struct {
	Next          http.RoundTripper
	MetricStorage *metricstorage.MetricStorage
}

func (l MetricRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	// Before request
	timeBeforeRequest := time.Now().Unix()

	// Request
	resp, err := l.Next.RoundTrip(r)

	// After request
	requestTime := time.Now().Unix() - timeBeforeRequest
	l.MetricStorage.HistogramObserve(RegistryRequestTimeMetric, float64(requestTime), map[string]string{"status_code": resp.Status}, []float64{0.5, 0.95, 0.99})
	l.MetricStorage.GaugeAdd(RegistryRequestsCount, 1.0, map[string]string{})

	return resp, err
}
