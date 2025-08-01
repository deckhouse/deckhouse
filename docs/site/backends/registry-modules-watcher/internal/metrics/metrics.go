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
	"strconv"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

const (
	RegistryRequestSecondsMetric          = "{PREFIX}_registry_request_seconds"
	RegistryRequestsCountMetric           = "{PREFIX}_registry_requests_count"
	RegistryPullSecondsMetric             = "{PREFIX}_registry_pull_seconds"
	RegistryScannerCacheLengthMetric      = "{PREFIX}_registry_scanner_cache_length"
	RegistryWatcherBackendsTotalMetric    = "{PREFIX}_registry_watcher_backends_total"
	RegistryWatcherNewBackendsTotalMetric = "{PREFIX}_registry_watcher_new_backends_total"
	SenderUploadRequestsCountMetric       = "{PREFIX}_sender_upload_requests_count"
	SenderUploadRequestsSecondsMetric     = "{PREFIX}_sender_upload_requests_seconds"
	SenderBuildRequestsCountMetric        = "{PREFIX}_sender_build_requests_count"
	SenderBuildRequestsSecondsMetric      = "{PREFIX}_sender_build_requests_seconds"
	SenderDeleteRequestsCountMetric       = "{PREFIX}_sender_delete_requests_count"
	SenderDeleteRequestsSecondsMetric     = "{PREFIX}_sender_delete_requests_seconds"
)

func RegisterMetrics(ms *metricstorage.MetricStorage, logger *log.Logger) error {
	defaultSecondsBuckets := []float64{
		0.0,
		0.02, 0.05, // 20,50 milliseconds
		0.1, 0.2, 0.5, // 100,200,500 milliseconds
		1, 2, 5, // 1,2,5 seconds
		10, 20, 50, // 10,20,50 seconds
		100, 200, 500, // 100,200,500 seconds
	}

	logger.Info("register metric", slog.String("metric", RegistryRequestSecondsMetric))
	_, err := ms.RegisterHistogram(RegistryRequestSecondsMetric, []string{"status_code"}, defaultSecondsBuckets, options.WithHelp("Request time to the registry in seconds"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", RegistryRequestSecondsMetric, err)
	}

	logger.Info("register metric", slog.String("metric", RegistryRequestsCountMetric))
	_, err = ms.RegisterCounter(RegistryRequestsCountMetric, []string{"status_code"}, options.WithHelp("Number of requests to the registry"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", RegistryRequestsCountMetric, err)
	}

	logger.Info("register metric", slog.String("metric", RegistryPullSecondsMetric))
	_, err = ms.RegisterHistogram(RegistryPullSecondsMetric, []string{}, defaultSecondsBuckets, options.WithHelp("Image pull time from registry registry in seconds"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", RegistryPullSecondsMetric, err)
	}

	logger.Info("register metric", slog.String("metric", RegistryScannerCacheLengthMetric))
	_, err = ms.RegisterGauge(RegistryScannerCacheLengthMetric, []string{"registry"}, options.WithHelp("Checks length of cache by registry"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", RegistryScannerCacheLengthMetric, err)
	}

	logger.Info("register metric", slog.String("metric", RegistryWatcherBackendsTotalMetric))
	_, err = ms.RegisterGauge(RegistryWatcherBackendsTotalMetric, []string{}, options.WithHelp("Count of watcher backends"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", RegistryWatcherBackendsTotalMetric, err)
	}

	logger.Info("register metric", slog.String("metric", RegistryWatcherNewBackendsTotalMetric))
	_, err = ms.RegisterGauge(RegistryWatcherNewBackendsTotalMetric, []string{}, options.WithHelp("Count of new watcher backends"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", RegistryWatcherNewBackendsTotalMetric, err)
	}

	logger.Info("register metric", slog.String("metric", SenderUploadRequestsCountMetric))
	_, err = ms.RegisterCounter(SenderUploadRequestsCountMetric, []string{"status_code"}, options.WithHelp("Number of the sender requests for uploading"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", SenderUploadRequestsCountMetric, err)
	}

	logger.Info("register metric", slog.String("metric", SenderUploadRequestsSecondsMetric))
	_, err = ms.RegisterHistogram(SenderUploadRequestsSecondsMetric, []string{"status_code"}, defaultSecondsBuckets, options.WithHelp("Sender upload request time in seconds"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", SenderUploadRequestsSecondsMetric, err)
	}

	logger.Info("register metric", slog.String("metric", SenderBuildRequestsCountMetric))
	_, err = ms.RegisterCounter(SenderBuildRequestsCountMetric, []string{"status_code"}, options.WithHelp("Number of the sender requests for build"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", SenderBuildRequestsCountMetric, err)
	}

	logger.Info("register metric", slog.String("metric", SenderBuildRequestsSecondsMetric))
	_, err = ms.RegisterHistogram(SenderBuildRequestsSecondsMetric, []string{"status_code"}, defaultSecondsBuckets, options.WithHelp("Sender build request time in seconds"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", SenderBuildRequestsSecondsMetric, err)
	}

	logger.Info("register metric", slog.String("metric", SenderDeleteRequestsCountMetric))
	_, err = ms.RegisterCounter(SenderDeleteRequestsCountMetric, []string{"status_code"}, options.WithHelp("Number of the sender requests for delete"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", SenderDeleteRequestsCountMetric, err)
	}

	logger.Info("register metric", slog.String("metric", SenderDeleteRequestsSecondsMetric))
	_, err = ms.RegisterHistogram(SenderDeleteRequestsSecondsMetric, []string{"status_code"}, defaultSecondsBuckets, options.WithHelp("Sender delete request time in seconds"))
	if err != nil {
		return fmt.Errorf("can not register %s: %w", SenderDeleteRequestsSecondsMetric, err)
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
	timeBeforeRequest := time.Now()

	// Request
	resp, err := l.Next.RoundTrip(r)

	// After request
	requestTime := time.Since(timeBeforeRequest).Seconds()
	labels := map[string]string{"status_code": strconv.Itoa(resp.StatusCode)}
	l.MetricStorage.HistogramObserve(RegistryRequestSecondsMetric, requestTime, labels, nil)
	l.MetricStorage.CounterAdd(RegistryRequestsCountMetric, 1.0, labels)

	return resp, err
}
