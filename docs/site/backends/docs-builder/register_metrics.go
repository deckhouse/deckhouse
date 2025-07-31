package main

import (
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func RegisterMetrics(mStorage *metricsstorage.MetricStorage) error {
	// Counters: count of upload/build/delete requests (ok/fail)
	_, err := mStorage.RegisterCounter("docs_builder_upload_total", []string{"status"})
	if err != nil {
		return err
	}
	_, err = mStorage.RegisterCounter("docs_builder_build_total", []string{"status"})
	if err != nil {
		return err
	}
	_, err = mStorage.RegisterCounter("docs_builder_delete_total", []string{"status"})
	if err != nil {
		return err
	}

	// Histograms:time taken for upload/build/delete requests (ok/fail)
	defaultBuckets := []float64{0.1, 0.5, 1, 2.5, 5, 10}
	_, err = mStorage.RegisterHistogram("docs_builder_upload_duration_seconds", []string{"status"}, defaultBuckets)
	if err != nil {
		return err
	}
	_, err = mStorage.RegisterHistogram("docs_builder_build_duration_seconds", []string{"status"}, defaultBuckets)
	if err != nil {
		return err
	}
	_, err = mStorage.RegisterHistogram("docs_builder_delete_duration_seconds", []string{"status"}, defaultBuckets)
	if err != nil {
		return err
	}

	// Gauge: total number of loaded modules in the cache
	_, err = mStorage.RegisterGauge("docs_builder_cached_modules", nil)
	if err != nil {
		return err
	}

	return nil
}
