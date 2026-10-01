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

package runtime

import (
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
)

// liveTicksPeriod matches addon-operator's tick, so the Heartbeat panel and the alert keep their thresholds.
const liveTicksPeriod = 10 * time.Second

// ownsLiveTicks reports whether the runtime emits deckhouse_live_ticks; addon-operator emits it itself.
func (r *Runtime) ownsLiveTicks() bool {
	return r.addonModuleManager == nil
}

// startLiveTicks registers the heartbeat counter and starts ticking it until stopLiveTicks.
func (r *Runtime) startLiveTicks() {
	go r.runLiveTicks()
}

// runLiveTicks increments the heartbeat counter on every tick until liveTicksStop closes.
func (r *Runtime) runLiveTicks() {
	ticker := time.NewTicker(liveTicksPeriod)
	defer ticker.Stop()

	for range ticker.C {
		r.metricStorage.CounterAdd(metrics.ShellLiveTicks, 1, map[string]string{})
	}
}
