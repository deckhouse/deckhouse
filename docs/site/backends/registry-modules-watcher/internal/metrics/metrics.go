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
	"log/slog"
	"net/http"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const (
	RegistryRequestMillisecondsMetric      = "registry_request_milliseconds"
	RegistryRequestsCountMetric            = "registry_requests_count"
	RegistryScannerCacheLengthMetric       = "registry_scanner_cache_length"
	RegistryWatcherBackendsCountMetric     = "registry_watcher_backends_count"
	SenderUploadRequestsCountMetric        = "sender_upload_requests_count"
	SenderUploadRequestsMillisecondsMetric = "sender_upload_requests_milliseconds"
	SenderBuildRequestsCountMetric         = "sender_build_requests_count"
	SenderBuildRequestsMillisecondsMetric  = "sender_build_requests_milliseconds"
	SenderDeleteRequestsCountMetric        = "sender_delete_requests_count"
	SenderDeleteRequestsMillisecondsMetric = "sender_delete_requests_milliseconds"
)

func RegisterMetrics(ms *metricstorage.MetricStorage, logger *log.Logger) error {
	logger.Info("register metric", slog.String("metric", RegistryRequestMillisecondsMetric))
	_, err := ms.RegisterHistogram(RegistryRequestMillisecondsMetric, []string{"status_code"}, []float64{0.5, 0.95, 0.99}, options.WithHelp("Checks request time in milliseconds to registry"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", RegistryRequestMillisecondsMetric, err)
	}

	logger.Info("register metric", slog.String("metric", RegistryRequestsCountMetric))
	_, err = ms.RegisterGauge(RegistryRequestsCountMetric, []string{}, options.WithHelp("Checks count of requests to registry"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", RegistryRequestsCountMetric, err)
	}

	logger.Info("register metric", slog.String("metric", RegistryScannerCacheLengthMetric))
	_, err = ms.RegisterGauge(RegistryScannerCacheLengthMetric, []string{"registry"}, options.WithHelp("Checks length of cache by registry"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", RegistryScannerCacheLengthMetric, err)
	}

	logger.Info("register metric", slog.String("metric", RegistryWatcherBackendsCountMetric))
	_, err = ms.RegisterGauge(RegistryWatcherBackendsCountMetric, []string{}, options.WithHelp("Checks watcher backends count"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", RegistryWatcherBackendsCountMetric, err)
	}

	logger.Info("register metric", slog.String("metric", SenderUploadRequestsCountMetric))
	_, err = ms.RegisterHistogram(SenderUploadRequestsCountMetric, []string{"status_code"}, []float64{0.5, 0.95, 0.99}, options.WithHelp(""))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", SenderUploadRequestsCountMetric, err)
	}

	logger.Info("register metric", slog.String("metric", SenderUploadRequestsMillisecondsMetric))
	_, err = ms.RegisterHistogram(SenderUploadRequestsMillisecondsMetric, []string{"status_code"}, []float64{0.5, 0.95, 0.99}, options.WithHelp(""))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", SenderUploadRequestsMillisecondsMetric, err)
	}

	logger.Info("register metric", slog.String("metric", SenderBuildRequestsCountMetric))
	_, err = ms.RegisterHistogram(SenderBuildRequestsCountMetric, []string{"status_code"}, []float64{0.5, 0.95, 0.99}, options.WithHelp(""))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", SenderBuildRequestsCountMetric, err)
	}

	logger.Info("register metric", slog.String("metric", SenderBuildRequestsMillisecondsMetric))
	_, err = ms.RegisterHistogram(SenderBuildRequestsMillisecondsMetric, []string{"status_code"}, []float64{0.5, 0.95, 0.99}, options.WithHelp(""))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", SenderBuildRequestsMillisecondsMetric, err)
	}

	logger.Info("register metric", slog.String("metric", SenderDeleteRequestsCountMetric))
	_, err = ms.RegisterHistogram(SenderDeleteRequestsCountMetric, []string{"status_code"}, []float64{0.5, 0.95, 0.99}, options.WithHelp(""))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", SenderDeleteRequestsCountMetric, err)
	}

	logger.Info("register metric", slog.String("metric", SenderDeleteRequestsMillisecondsMetric))
	_, err = ms.RegisterHistogram(SenderDeleteRequestsMillisecondsMetric, []string{"status_code"}, []float64{0.5, 0.95, 0.99}, options.WithHelp(""))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", SenderDeleteRequestsMillisecondsMetric, err)
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
	timeBeforeRequest := time.Now().UnixMilli()

	// Request
	resp, err := l.Next.RoundTrip(r)

	// After request
	requestTime := time.Now().UnixMilli() - timeBeforeRequest
	l.MetricStorage.HistogramObserve(RegistryRequestMillisecondsMetric, float64(requestTime), map[string]string{"status_code": resp.Status}, []float64{0.5, 0.95, 0.99})
	l.MetricStorage.GaugeAdd(RegistryRequestsCountMetric, 1.0, map[string]string{})

	return resp, err
}
