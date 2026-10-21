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
	"sync/atomic"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// Activations of the convergence metrics, as addon-operator names the startup one.
const (
	activationOperatorStartup = "OperatorStartup" // activationOperatorStartup: from Resume to the first convergence.
	activationEvent           = "Event"           // activationEvent: any later period the graph was unconverged.
)

// convergence turns the scheduler's convergence changes (schedule.WithConvergenceHook) into
// deckhouse_convergence_seconds and deckhouse_convergence_total. Its changed runs under the
// scheduler's lock, so the calls never overlap.
type convergence struct {
	metrics metricsstorage.Storage
	since   time.Time
	// started: the startup convergence is over, so the next periods are activationEvent.
	started atomic.Bool
}

// changed opens a period on false and records it on true.
func (c *convergence) changed(converged bool) {
	if !converged {
		c.since = time.Now()
		return
	}

	activation := activationEvent
	if !c.started.Swap(true) {
		activation = activationOperatorStartup
	}

	labels := map[string]string{metrics.LabelActivation: activation}
	c.metrics.CounterAdd(metrics.ConvergenceSeconds, time.Since(c.since).Seconds(), labels)
	c.metrics.CounterAdd(metrics.ConvergenceTotal, 1, labels)
}
