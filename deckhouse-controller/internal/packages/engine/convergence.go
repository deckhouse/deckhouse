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
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// convergence turns the scheduler's convergence changes (schedule.WithConvergenceHook) into
// deckhouse_convergence_seconds, deckhouse_convergence_total and deckhouse_convergence_last_seconds.
// Its changed runs under the scheduler's lock, so the calls never overlap.
type convergence struct {
	metrics metricsstorage.Storage
	since   time.Time
}

// changed opens a period on false and records it on true.
func (c *convergence) changed(converged bool) {
	if !converged {
		c.since = time.Now()
		return
	}

	// No period was opened (the graph started converged): there is nothing to measure.
	if c.since.IsZero() {
		return
	}

	took := time.Since(c.since)
	c.metrics.CounterAdd(metrics.ConvergenceSeconds, took.Seconds(), map[string]string{})
	c.metrics.CounterAdd(metrics.ConvergenceTotal, 1, map[string]string{})
	c.metrics.GaugeSet(metrics.ConvergenceLastSeconds, took.Seconds(), map[string]string{})
}
