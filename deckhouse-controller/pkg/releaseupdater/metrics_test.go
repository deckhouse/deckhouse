/*
Copyright 2026 Flant JSC

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

package releaseupdater

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// countingStorage counts the purges of the grouped storage.
type countingStorage struct {
	*metricsstorage.MetricStorage
	grouped *countingGrouped
}

func (s *countingStorage) Grouped() metricsstorage.GroupedStorage {
	return s.grouped
}

type countingGrouped struct {
	metricsstorage.GroupedStorage
	purges int
}

func (g *countingGrouped) ExpireGroupMetricByName(group, name string) {
	g.purges++
	g.GroupedStorage.ExpireGroupMetricByName(group, name)
}

func newCountingStorage() *countingStorage {
	storage := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry(), metricsstorage.WithLogger(log.NewNop()))

	return &countingStorage{
		MetricStorage: storage,
		grouped:       &countingGrouped{GroupedStorage: storage.Grouped()},
	}
}

// releaseSeries returns the label sets of the series of the release.
func releaseSeries(t *testing.T, storage *countingStorage, release string) []map[string]string {
	t.Helper()

	families, err := storage.Gather()
	require.NoError(t, err)

	var series []map[string]string
	for _, family := range families {
		if family.GetName() != ModuleReleaseBlockedMetricName {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := make(map[string]string)
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels[metrics.LabelName] == release {
				series = append(series, labels)
			}
		}
	}

	return series
}

func releaseLabels(release, queueDepth string) MetricLabels {
	labels := MetricLabels{metrics.LabelName: release, metrics.LabelModule: "echo"}
	labels.SetTrue(metrics.LabelManualApprovalRequired)
	labels[metrics.LabelReleaseQueueDepth] = queueDepth

	return labels
}

func TestUpdateReleaseMetric(t *testing.T) {
	t.Run("unchanged labels do not purge the series", func(t *testing.T) {
		storage := newCountingStorage()
		updater := NewMetricsUpdater(storage, ModuleReleaseBlockedMetricName)

		updater.UpdateReleaseMetric("echo-v1.1.0", releaseLabels("echo-v1.1.0", "1"))
		purges := storage.grouped.purges

		for range 3 {
			updater.UpdateReleaseMetric("echo-v1.1.0", releaseLabels("echo-v1.1.0", "1"))
		}

		assert.Equal(t, purges, storage.grouped.purges, "an unchanged series is purged")
		assert.Len(t, releaseSeries(t, storage, "echo-v1.1.0"), 1)
	})

	t.Run("changed labels replace the series", func(t *testing.T) {
		storage := newCountingStorage()
		updater := NewMetricsUpdater(storage, ModuleReleaseBlockedMetricName)

		updater.UpdateReleaseMetric("echo-v1.1.0", releaseLabels("echo-v1.1.0", "1"))
		updater.UpdateReleaseMetric("echo-v1.1.0", releaseLabels("echo-v1.1.0", "2"))

		series := releaseSeries(t, storage, "echo-v1.1.0")
		require.Len(t, series, 1, "the series with the old labels is kept")
		assert.Equal(t, "2", series[0][metrics.LabelReleaseQueueDepth])
	})

	t.Run("labels changed in place by the caller replace the series", func(t *testing.T) {
		storage := newCountingStorage()
		updater := NewMetricsUpdater(storage, ModuleReleaseBlockedMetricName)

		labels := releaseLabels("echo-v1.1.0", "1")
		updater.UpdateReleaseMetric("echo-v1.1.0", labels)
		labels[metrics.LabelReleaseQueueDepth] = "2"
		updater.UpdateReleaseMetric("echo-v1.1.0", labels)

		series := releaseSeries(t, storage, "echo-v1.1.0")
		require.Len(t, series, 1, "the series with the old labels is kept")
		assert.Equal(t, "2", series[0][metrics.LabelReleaseQueueDepth])
	})

	t.Run("purge removes the series of the release only", func(t *testing.T) {
		storage := newCountingStorage()
		updater := NewMetricsUpdater(storage, ModuleReleaseBlockedMetricName)

		updater.UpdateReleaseMetric("echo-v1.1.0", releaseLabels("echo-v1.1.0", "1"))
		updater.UpdateReleaseMetric("echo-v1.2.0", releaseLabels("echo-v1.2.0", "2"))
		updater.PurgeReleaseMetric("echo-v1.1.0")

		assert.Empty(t, releaseSeries(t, storage, "echo-v1.1.0"))
		assert.Len(t, releaseSeries(t, storage, "echo-v1.2.0"), 1)

		// the purged release is published again with the same labels
		updater.UpdateReleaseMetric("echo-v1.1.0", releaseLabels("echo-v1.1.0", "1"))
		assert.Len(t, releaseSeries(t, storage, "echo-v1.1.0"), 1)
	})
}
