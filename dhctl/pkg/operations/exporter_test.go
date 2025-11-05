// Copyright 2021 Flant JSC
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

package operations

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func TestExporterGetStatistic(t *testing.T) {
	log.InitLogger("json")

	kubeCl := client.NewFakeKubernetesClient()
	retry.InTestEnvironment = true

	newTestConvergeExporter := func() *ConvergeExporter {
		return &ConvergeExporter{
			kubeCl:          kubeCl,
			ListenAddress:   "0.0.0.0",
			MetricsPath:     "/metrics",
			CheckInterval:   time.Second,
			existedEntities: newPreviouslyExistedEntities(),
			OneGaugeMetrics: make(map[string]prometheus.Gauge),
			GaugeMetrics:    make(map[string]*prometheus.GaugeVec),
			CounterMetrics:  make(map[string]*prometheus.CounterVec),
			isDebug:         false,
			logger:          log.GetDefaultLogger(),
			tmpDir:          t.TempDir(),
		}
	}
	exporter := newTestConvergeExporter()
	exporter.registerMetrics()

	t.Run("Should increment errors metric because nothing exists in a cluster", func(t *testing.T) {
		loggerProvider := func() log.Logger {
			return log.GetDefaultLogger()
		}
		dummyCleaner := cache.NewDummyTmpCleaner(loggerProvider, "")
		exporter.recordStatistic(exporter.getStatistic(context.Background(), dummyCleaner))

		errorsCounter, err := exporter.CounterMetrics["errors"].GetMetricWith(prometheus.Labels{})
		require.NoError(t, err)

		collected := io_prometheus_client.Metric{}
		errorsCounter.Write(&collected)
		require.Equal(t, float64(1), *collected.Counter.Value)
	})

	t.Run("Should increment only specified statuses", func(t *testing.T) {
		statistic := check.Statistics{
			Node: []check.NodeCheckResult{
				{Group: "test", Name: "test-0", Status: check.OKStatus},
				{Group: "test", Name: "test-1", Status: check.ChangedStatus},
			},
		}

		exporter.recordStatistic(&statistic, false)
		firstNodesStatus, err := exporter.GaugeMetrics["node_status"].GetMetricWith(prometheus.Labels{
			"node_group": "test",
			"name":       "test-0",
			"status":     check.OKStatus,
		})
		require.NoError(t, err)
		collected := io_prometheus_client.Metric{}
		firstNodesStatus.Write(&collected)
		require.Equal(t, float64(1), *collected.Gauge.Value)

		secondNodeStatus, err := exporter.GaugeMetrics["node_status"].GetMetricWith(prometheus.Labels{
			"node_group": "test",
			"name":       "test-1",
			"status":     check.ChangedStatus,
		})
		require.NoError(t, err)
		collected = io_prometheus_client.Metric{}
		secondNodeStatus.Write(&collected)
		require.Equal(t, float64(1), *collected.Gauge.Value)

		require.Equal(t, exporter.existedEntities.Nodes, map[string]string{"test-0": "test", "test-1": "test"})

		statisticWithoutOneNode := check.Statistics{
			Node: []check.NodeCheckResult{
				{Group: "test", Name: "test-0", Status: check.OKStatus},
			},
		}

		// if node disappears from statistic, we should mark its status as 0
		exporter.recordStatistic(&statisticWithoutOneNode, false)

		secondNodeStatus, err = exporter.GaugeMetrics["node_status"].GetMetricWith(prometheus.Labels{
			"node_group": "test",
			"name":       "test-1",
			"status":     check.ChangedStatus,
		})
		require.NoError(t, err)

		collected = io_prometheus_client.Metric{}
		secondNodeStatus.Write(&collected)
		require.Equal(t, float64(0), *collected.Gauge.Value)
		require.Equal(t, exporter.existedEntities.Nodes, map[string]string{"test-0": "test"})
	})
}
