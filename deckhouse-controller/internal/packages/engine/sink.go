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
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/util/workqueue"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/core"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

const (
	// The status queue names label the workqueue metrics; they are part of the metrics contract.
	appStatusQueueName    = "application-status"
	moduleStatusQueueName = "module-status"

	// deckhouseModule is the module whose effective settings the deckhouse settings channel
	// carries.
	deckhouseModule = "deckhouse"

	resyncPeriod = 10 * time.Minute
)

// statusSink receives the core's and the observation store's notifications: the names whose
// status may have changed. The status controllers drain its queues and write the CRD status.
// Workqueues, because Notify must neither block nor drop a name, and a workqueue coalesces. The
// sink also wakes the settings and maintenance publishers and periodically re-enqueues every name
// it has seen; one goroutine does all three (Engine.runSink).
type statusSink struct {
	apps    workqueue.TypedRateLimitingInterface[string]
	modules workqueue.TypedRateLimitingInterface[string]

	// seen are the names resync re-enqueues; maintenance the applications the maintenance
	// publisher has not taken yet.
	mu          sync.Mutex
	seen        map[string]struct{}
	maintenance map[string]struct{}

	settingsWake chan struct{} // settingsWake: deckhouse or global may have new settings.
	// deckhouseSettings and globalSettings hold the latest settings; never closed.
	deckhouseSettings chan addonutils.Values
	globalSettings    chan addonutils.Values

	// maintenanceWake: maintenance has names to check.
	maintenanceWake chan struct{}
}

func newStatusSink() *statusSink {
	return &statusSink{
		apps: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: appStatusQueueName},
		),
		modules: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: moduleStatusQueueName},
		),
		seen:              make(map[string]struct{}),
		maintenance:       make(map[string]struct{}),
		maintenanceWake:   make(chan struct{}, 1),
		settingsWake:      make(chan struct{}, 1),
		deckhouseSettings: make(chan addonutils.Values, 1),
		globalSettings:    make(chan addonutils.Values, 1),
	}
}

// add enqueues name; never blocks and never calls back into the engine or the store.
func (s *statusSink) add(name string) {
	app := isApplication(name)

	s.mu.Lock()
	s.seen[name] = struct{}{}
	if app {
		s.maintenance[name] = struct{}{}
	}
	s.mu.Unlock()

	s.enqueue(name)

	switch {
	case app:
		wake(s.maintenanceWake)
	case name == deckhouseModule || name == schedule.GlobalNode:
		wake(s.settingsWake)
	}
}

func (s *statusSink) takeMaintenance() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	names := make([]string, 0, len(s.maintenance))
	for name := range s.maintenance {
		names = append(names, name)
	}

	clear(s.maintenance)

	return names
}

func wake(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

// isApplication reports whether name is an application's: "namespace.name" (apps.BuildName);
// module names have no dot.
func isApplication(name string) bool {
	return strings.Contains(name, ".")
}

func (s *statusSink) enqueue(name string) {
	if isApplication(name) {
		s.apps.Add(name)
	} else {
		s.modules.Add(name)
	}
}

func (s *statusSink) forget(name string) {
	s.mu.Lock()
	delete(s.seen, name)
	s.mu.Unlock()
}

// resync re-enqueues every name the sink has seen: the level-triggered backstop for a status
// whose reconcile failed, run on every resyncPeriod tick. The sink's own names suffice, because
// every notification passes through add, and need no call into the engine.
func (s *statusSink) resync() {
	s.mu.Lock()
	names := make([]string, 0, len(s.seen))
	for name := range s.seen {
		names = append(names, name)
	}
	s.mu.Unlock()

	for _, name := range names {
		s.enqueue(name)
	}
}

// shutdown shuts the status queues down; a later add is ignored.
func (s *statusSink) shutdown() {
	s.apps.ShutDown()
	s.modules.ShutDown()
}

// publishSettings publishes the accepted effective settings of deckhouse and Global
// (CurrentReport.AcceptedValues) on the settings channels when their checksum changed since the
// last publish (published, by name). Accepted, not the checkpoint of a successful run: the
// deckhouse settings container blocks its readers (every Application reconcile, the ModuleConfig
// webhook) until the first value, and a failing first run would block them all.
func (s *statusSink) publishSettings(engine *core.Engine, published map[string]string) {
	targets := map[string]chan addonutils.Values{
		deckhouseModule:     s.deckhouseSettings,
		schedule.GlobalNode: s.globalSettings,
	}

	for name, ch := range targets {
		report, ok := engine.Report(name)
		if !ok {
			continue
		}

		accepted := report.Current
		if accepted.AcceptedChecksum == "" || accepted.AcceptedChecksum == published[name] {
			continue
		}

		published[name] = accepted.AcceptedChecksum
		publishLatest(ch, accepted.AcceptedValues)
	}
}

// publishLatest replaces an unread value with values, without blocking.
func publishLatest(ch chan addonutils.Values, values addonutils.Values) {
	for {
		select {
		case ch <- values:
			return
		default:
		}

		select {
		case <-ch:
		default:
		}
	}
}
