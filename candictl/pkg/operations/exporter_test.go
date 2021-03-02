package operations

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/candictl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/candictl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/candictl/pkg/log"
	"github.com/deckhouse/deckhouse/candictl/pkg/util/retry"
)

func TestExporterGetStatistic(t *testing.T) {
	log.InitLogger("simple")

	kubeCl := client.NewFakeKubernetesClient()
	retry.InTestEnvironment = true

	newTestConvergeExporter := func() *ConvergeExporter {
		return &ConvergeExporter{
			kubeCl:          kubeCl,
			ListenAddress:   "0.0.0.0",
			MetricsPath:     "/metrics",
			CheckInterval:   time.Second,
			existedEntities: newPreviouslyExistedEntities(),
			GaugeMetrics:    make(map[string]*prometheus.GaugeVec),
			CounterMetrics:  make(map[string]*prometheus.CounterVec),
		}
	}
	exporter := newTestConvergeExporter()
	exporter.registerMetrics()

	t.Run("Should increment errors metric because nothing exists in a cluster", func(t *testing.T) {
		exporter.recordStatistic(exporter.getStatistic())

		errorsCounter, err := exporter.CounterMetrics["errors"].GetMetricWith(prometheus.Labels{})
		require.NoError(t, err)

		collected := io_prometheus_client.Metric{}
		errorsCounter.Write(&collected)
		require.Equal(t, float64(1), *collected.Counter.Value)
	})

	t.Run("Should increment only specified statuses", func(t *testing.T) {
		statistic := converge.Statistics{
			Node: []converge.NodeCheckResult{
				{Group: "test", Name: "test-0", Status: converge.OKStatus},
				{Group: "test", Name: "test-1", Status: converge.ChangedStatus},
			},
		}

		exporter.recordStatistic(&statistic)
		firstNodesStatus, err := exporter.GaugeMetrics["node_status"].GetMetricWith(prometheus.Labels{
			"node_group": "test",
			"name":       "test-0",
			"status":     converge.OKStatus,
		})
		require.NoError(t, err)
		collected := io_prometheus_client.Metric{}
		firstNodesStatus.Write(&collected)
		require.Equal(t, float64(1), *collected.Gauge.Value)

		secondNodeStatus, err := exporter.GaugeMetrics["node_status"].GetMetricWith(prometheus.Labels{
			"node_group": "test",
			"name":       "test-1",
			"status":     converge.ChangedStatus,
		})
		require.NoError(t, err)
		collected = io_prometheus_client.Metric{}
		secondNodeStatus.Write(&collected)
		require.Equal(t, float64(1), *collected.Gauge.Value)

		require.Equal(t, exporter.existedEntities.Nodes, map[string]string{"test-0": "test", "test-1": "test"})

		statisticWithoutOneNode := converge.Statistics{
			Node: []converge.NodeCheckResult{
				{Group: "test", Name: "test-0", Status: converge.OKStatus},
			},
		}

		// if node disappears from statistic, we should mark its status as 0
		exporter.recordStatistic(&statisticWithoutOneNode)

		secondNodeStatus, err = exporter.GaugeMetrics["node_status"].GetMetricWith(prometheus.Labels{
			"node_group": "test",
			"name":       "test-1",
			"status":     converge.ChangedStatus,
		})
		require.NoError(t, err)

		collected = io_prometheus_client.Metric{}
		secondNodeStatus.Write(&collected)
		require.Equal(t, float64(0), *collected.Gauge.Value)
		require.Equal(t, exporter.existedEntities.Nodes, map[string]string{"test-0": "test"})
	})
}
