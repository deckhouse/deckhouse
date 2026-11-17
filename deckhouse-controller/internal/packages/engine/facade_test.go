// Copyright 2026 Flant JSC
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

package engine

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func TestMaintenanceMetricPreservesOtherNamespaces(t *testing.T) {
	storage := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	require.NoError(t, metrics.RegisterDeckhouseControllerMetrics(storage))
	engine := &Engine{metrics: storage}

	assertNamespaces := func(namespaces ...string) {
		t.Helper()
		families, err := storage.Gather()
		require.NoError(t, err)
		var actual []map[string]string
		for _, family := range families {
			if family.GetName() != metrics.ApplicationMaintenance {
				continue
			}
			require.Equal(t, "Gauge indicating an application in maintenance mode (1.0 = in maintenance)", family.GetHelp())
			for _, metric := range family.GetMetric() {
				require.InDelta(t, 1, metric.GetGauge().GetValue(), 0)
				labels := make(map[string]string)
				for _, label := range metric.GetLabel() {
					labels[label.GetName()] = label.GetValue()
				}
				actual = append(actual, labels)
			}
		}

		var expected []map[string]string
		for _, namespace := range namespaces {
			expected = append(expected, map[string]string{
				"application_namespace": namespace,
				"name":                  "shop",
				"state":                 "NoResourceReconciliation",
			})
		}
		require.ElementsMatch(t, expected, actual)
	}

	engine.setMaintenanceMetric("shop-a.shop", nelm.NoResourceReconciliation)
	engine.setMaintenanceMetric("shop-b.shop", nelm.NoResourceReconciliation)
	assertNamespaces("shop-a", "shop-b")

	engine.setMaintenanceMetric("shop-a.shop", nelm.NoResourceReconciliation)
	assertNamespaces("shop-a", "shop-b")

	engine.setMaintenanceMetric("shop-a.shop", nelm.Managed)
	assertNamespaces("shop-b")

	engine.setMaintenanceMetric("shop-b.shop", nelm.Managed)
	assertNamespaces()
}
