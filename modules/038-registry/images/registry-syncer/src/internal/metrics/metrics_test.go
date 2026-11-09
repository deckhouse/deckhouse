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

package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exposed is what a scrape of the registry would show, by metric name.
func exposed(t *testing.T, registry *prometheus.Registry) map[string]float64 {
	t.Helper()

	families, err := registry.Gather()
	require.NoError(t, err)

	values := map[string]float64{}
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			values[family.GetName()] = metric.GetGauge().GetValue()
		}
	}
	return values
}

// A zero before anything was estimated would say "a collection frees nothing" about a store nobody
// has looked at.
func TestReclaimableIsAbsentUntilEstimated(t *testing.T) {
	registry := prometheus.NewRegistry()
	m := New(registry)

	before := exposed(t, registry)
	assert.NotContains(t, before, "d8_registry_storage_reclaimable_bytes")
	assert.NotContains(t, before, "d8_registry_storage_reclaimable_estimated_timestamp_seconds")

	at := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	m.SetReclaimable(0, at)

	after := exposed(t, registry)
	assert.Contains(t, after, "d8_registry_storage_reclaimable_bytes", "a zero that was estimated is exposed")
	assert.Zero(t, after["d8_registry_storage_reclaimable_bytes"])
	assert.InDelta(t, float64(at.Unix()), after["d8_registry_storage_reclaimable_estimated_timestamp_seconds"], 0)
}
