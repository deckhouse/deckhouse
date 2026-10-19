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

// Package observed holds what the cluster-facing services observe about each package: workload
// health, the release progress, the endpoints, whether the release has workloads, and the last
// failure of a hook that ran outside the package's operations. It is the observed half of a
// package's status; the other is the engine's lifecycle Report.
//
//   - Observations are never lifecycle inputs: a flapping pod must not start a run or change
//     convergence, so the engine core never imports this package.
//   - Each field has one writer: the release service (tracking, URLs, "no workloads"), the health
//     callback (health) and the hook lanes (the hook failure); the composition root forgets a
//     package once its removal is reported done. Writers declare their own narrow interfaces.
//   - The status projection joins both halves by name; both notify through the same status
//     queues.
package observed

import (
	"encoding/json"
	"maps"
	"slices"
	"sync"
	"time"
)

// HealthState is the reduced health of a package's workloads.
type HealthState string

const (
	// HealthUnknown means no workload of the package reported yet.
	HealthUnknown HealthState = ""
	// HealthReady means every workload is ready.
	HealthReady HealthState = "Ready"
	// HealthProgressing means a workload is rolling out or scaling.
	HealthProgressing HealthState = "Progressing"
	// HealthDegraded means a workload is not ready.
	HealthDegraded HealthState = "Degraded"
)

// Health is the reduced workload health of one package; it becomes the Scaled condition.
type Health struct {
	State   HealthState
	Message string
}

// Tracking is the progress of the latest release apply or uninstall: json.Marshal(Tracking) is
// status.tracking byte for byte, an external contract that carries nelm's report as nelm produced
// it. Raw JSON keeps this package free of nelm types.
type Tracking struct {
	// Completed is the number of completed resource operations of Report.
	Completed int `json:"completed"`
	// Remaining is the number of the other resource operations of Report.
	Remaining int `json:"remaining"`
	// Report is nelm's progress report, marshaled by the release service; empty when nothing was
	// reported since the last reset.
	Report json.RawMessage `json:"report"`
}

// Empty reports whether no progress is recorded; the projection then leaves status.tracking as
// it is.
func (t Tracking) Empty() bool {
	return len(t.Report) == 0
}

// URL is an endpoint a package exposes, extracted by the release service from the rendered
// Ingresses (it becomes an entry of Application status.urls).
type URL struct {
	URL         string
	Description string
}

// Hook failure reasons: the projection reports a failed synchronization as a hook initialization
// failure and a failed event run as a hook failure.
const (
	// HookReasonSync: a hook failed its synchronization run (the binding's initial snapshot).
	HookReasonSync = "SyncHookFailed"
	// HookReasonEvent: a hook failed its run on a Kubernetes event or a schedule.
	HookReasonEvent = "EventHookFailed"
)

// HookFailure is the last failure of a hook run in the hook lanes (a synchronization, or a run on
// an event or a schedule). It lasts until that hook succeeds, because its lane retries it and
// holds the items behind it. Failures of hooks an operation runs are in the engine's Report.
type HookFailure struct {
	Hook    string
	Reason  string // Reason is HookReasonSync or HookReasonEvent.
	Message string
	// Since is when this failure (same hook, reason and message) was first recorded; a retry that
	// fails the same way does not move it.
	Since time.Time
}

// Observation is everything observed about one package. Returned by value; its slices and the
// hook failure are copies the caller may keep.
type Observation struct {
	Health      Health
	Tracking    Tracking
	URLs        []URL
	NoWorkloads bool
	// HookFailure is the last failure of a runtime or synchronization hook; nil when none is
	// outstanding.
	HookFailure *HookFailure
}

// Store is the observation store. Safe for concurrent use. Every write that changes the stored
// observation calls notify with the package name after the lock is released; a write that
// changes nothing does not.
type Store struct {
	mu     sync.Mutex
	items  map[string]*Observation
	notify func(name string)
	now    func() time.Time
}

// New returns an empty store. notify is called with a package name after its observation
// changed; it must not block (enqueue and return) and must not call back into the store
// synchronously. Nil disables notifications.
func New(notify func(name string)) *Store {
	return &Store{items: make(map[string]*Observation), notify: notify, now: time.Now}
}

// Get returns the observation of name; the zero Observation when nothing was observed.
func (s *Store) Get(name string) Observation {
	s.mu.Lock()
	defer s.mu.Unlock()

	o, ok := s.items[name]
	if !ok {
		return Observation{}
	}

	res := Observation{
		Health:      o.Health,
		Tracking:    o.Tracking,
		URLs:        slices.Clone(o.URLs),
		NoWorkloads: o.NoWorkloads,
	}

	res.Tracking.Report = slices.Clone(o.Tracking.Report)

	if o.HookFailure != nil {
		failure := *o.HookFailure
		res.HookFailure = &failure
	}

	return res
}

// UpdateHealth records a health transition. Writer: the health service (its callback).
func (s *Store) UpdateHealth(name string, h Health) {
	s.update(name, func(o *Observation) bool {
		if o.Health == h {
			return false
		}

		o.Health = h

		return true
	})
}

// ResetTracking clears the progress before a new apply, so the progress of the previous apply is
// not shown as this one's. Writer: the release service.
func (s *Store) ResetTracking(name string) {
	s.update(name, func(o *Observation) bool {
		if o.Tracking.Empty() {
			return false
		}

		o.Tracking = Tracking{}

		return true
	})
}

// UpdateTracking records the progress nelm reported for an apply or a release delete. Writer: the
// release service.
func (s *Store) UpdateTracking(name string, t Tracking) {
	s.update(name, func(o *Observation) bool {
		o.Tracking = Tracking{
			Completed: t.Completed,
			Remaining: t.Remaining,
			Report:    slices.Clone(t.Report),
		}

		return true
	})
}

// UpdateURLs records the endpoints of the last render. Writer: the release service.
func (s *Store) UpdateURLs(name string, urls []URL) {
	s.update(name, func(o *Observation) bool {
		if slices.Equal(o.URLs, urls) {
			return false
		}

		o.URLs = slices.Clone(urls)

		return true
	})
}

// SetNoWorkloads records whether the last render has no workloads (health then has nothing to
// watch, and the projection must not wait for Scaled). Writer: the release service.
func (s *Store) SetNoWorkloads(name string, none bool) {
	s.update(name, func(o *Observation) bool {
		if o.NoWorkloads == none {
			return false
		}

		o.NoWorkloads = none

		return true
	})
}

// RecordHookFailure records a failure of hook with reason and message, replacing the recorded one.
// A repeated identical failure (a retry that fails the same way) changes nothing and does not
// notify. Writer: the hook lanes (synchronization and runtime hook runs).
func (s *Store) RecordHookFailure(name, hook, reason, message string) {
	s.update(name, func(o *Observation) bool {
		if f := o.HookFailure; f != nil && f.Hook == hook && f.Reason == reason && f.Message == message {
			return false
		}

		o.HookFailure = &HookFailure{Hook: hook, Reason: reason, Message: message, Since: s.now()}

		return true
	})
}

// ClearHookFailure clears the recorded failure when it is hook's: hook just ran successfully, so
// its lane moved on. A failure of another hook is kept. Writer: the hook lanes.
func (s *Store) ClearHookFailure(name, hook string) {
	s.update(name, func(o *Observation) bool {
		if o.HookFailure == nil || o.HookFailure.Hook != hook {
			return false
		}

		o.HookFailure = nil

		return true
	})
}

// ResetHookFailure clears the recorded failure whatever hook it is of: the package's hook lanes
// stopped, so no failed item is retried any more. Writer: the hook
// lanes (their stop, hooklane).
func (s *Store) ResetHookFailure(name string) {
	s.update(name, func(o *Observation) bool {
		if o.HookFailure == nil {
			return false
		}

		o.HookFailure = nil

		return true
	})
}

// Forget drops everything observed about name, after the package was removed. Writer: the
// composition root (the engine facade), when it reports the removal done to the controller.
func (s *Store) Forget(name string) {
	s.mu.Lock()
	_, ok := s.items[name]
	delete(s.items, name)
	s.mu.Unlock()

	if ok && s.notify != nil {
		s.notify(name)
	}
}

// Names returns the names with an observation, sorted.
func (s *Store) Names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Sorted(maps.Keys(s.items))
}

// update applies f to name's observation under the lock and notifies after the lock is
// released when f reports a change. An entry is created only by a change: a write that changes
// nothing (a hook success clearing no failure, the most frequent write) leaves no empty entry
// behind, also for a package that was forgotten.
func (s *Store) update(name string, f func(o *Observation) bool) {
	s.mu.Lock()
	o, ok := s.items[name]
	if !ok {
		o = &Observation{}
	}

	changed := f(o)
	if changed && !ok {
		s.items[name] = o
	}
	s.mu.Unlock()

	if changed && s.notify != nil {
		s.notify(name)
	}
}
