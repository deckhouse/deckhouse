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

package health

import (
	"fmt"
	"sync"

	"k8s.io/client-go/kubernetes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/health/monitor"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// LabelKey is the workload label whose value identifies the owning
// package. The same constant is used by the indexer, the event handlers,
// and any caller that wants to add the label to a workload manifest.
const LabelKey = "health.deckhouse.io/package"

// Service watches per-package workload statuses via an embedded Monitor
// and reduces them to a single State per package. Transitions are
// edge-triggered: the configured Callback is invoked only when a
// package's reduced State differs from the last reported value.
//
// The Service owns the Monitor and its lifecycle. Construction is
// I/O-free; goroutines and informers start in Start and stop in Stop.
type Service struct {
	monitor *monitor.Monitor

	// mu guards lastHealth. The reconcile callback holds it only for the
	// read-compare-write of one package's entry; the user's Callback is
	// invoked after the lock drops so it can never deadlock against the
	// Service.
	mu         sync.Mutex
	lastHealth map[string]State
	callback   Callback
}

// Callback is invoked from the reconcile goroutine whenever a package's
// reduced health changes. The callback must not block and must not call
// back into the Service.
type Callback func(name string, event Event)

// NewService constructs a Service. It does not start any goroutines or
// perform any I/O; that happens in Start. The callback is invoked on
// every package health transition and must be non-nil and non-blocking.
func NewService(client kubernetes.Interface, cb Callback, logger *log.Logger) (*Service, error) {
	s := &Service{
		callback:   cb,
		lastHealth: make(map[string]State),
	}

	mon, err := monitor.NewMonitor(client, s.reconcile, LabelKey, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create monitor: %w", err)
	}
	s.monitor = mon

	return s, nil
}

// Start launches the background reconcile goroutine and returns
// immediately. Safe to call more than once; only the first call has any
// effect. Pair every Start with a Stop to release informer and queue
// resources.
func (s *Service) Start() {
	s.monitor.Start()
}

// Stop waits for the reconcile goroutine to drain (caches stopped,
// queue shut down, in-flight reconciles finished). Must be called after
// Start.
func (s *Service) Stop() {
	s.monitor.Stop()
}

// reconcile is the Monitor's Reconcile callback. It reduces the per-workload
// statuses to a single State, dedupes against the last reported value, and
// fires the user callback only on a real transition. The lock is held only
// for the read-compare-write of one map entry; the user callback runs after
// it drops, so a slow callback can't block other reconciles from updating
// state — though it will still serialize with them on the worker goroutine.
func (s *Service) reconcile(name string, status []monitor.WorkloadStatus) {
	current := reducePackage(status)

	s.mu.Lock()
	previous, had := s.lastHealth[name]
	if !had {
		previous = StateUnknown
	}
	if current.State == StateUnknown {
		delete(s.lastHealth, name)
	} else {
		s.lastHealth[name] = current.State
	}
	s.mu.Unlock()

	if current.State == previous {
		return
	}

	s.callback(name, Event{Health: current})
}
