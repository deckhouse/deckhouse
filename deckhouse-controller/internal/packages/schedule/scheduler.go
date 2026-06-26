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

package schedule

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/condition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/dependency"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/version"
)

const (
	// FunctionalOrder is the Order value assigned to functional (non-critical) packages.
	// It is higher than any critical package order, ensuring functional packages are
	// scheduled only after all critical packages have been processed.
	FunctionalOrder = 999

	// defaultBufferSize is the capacity of the scheduler's notification channel
	// used to signal enable/disable events to consumers without blocking callers.
	defaultBufferSize = 1000

	reasonRequirementsKubernetes = "KubernetesRequirementsUnmet"
	reasonRequirementsDeckhouse  = "DeckhouseRequirementsUnmet"
	reasonRequirementsBootstrap  = "BootstrapRequirementsUnmet"

	// reasonDisabled is the status reason recorded when a node is explicitly
	// disabled by an operator via [Scheduler.Disable], as opposed to losing
	// eligibility through a failed checker.
	reasonDisabled = "PackageDisabled"
)

// Scheduler manages a dependency graph of packages and their lifecycle.
// Each scheduling pass recomputes eligibility, cascade-disables nodes
// that lost it, and advances newly-eligible nodes — all in topological order.
// All exported methods are safe for concurrent use.
type Scheduler struct {
	mu    sync.RWMutex
	nodes map[string]*node

	eventCh chan Event

	dependencyGetter       dependency.Getter
	kubeVersionGetter      version.Getter      // Gets current Kubernetes version
	deckhouseVersionGetter version.Getter      // Gets current Deckhouse version
	bootstrapCondition     condition.Condition // Bootstrap readiness check

	pause atomic.Bool // When true, no state changes are processed
}

// Option configures a Scheduler during construction.
type Option func(*Scheduler)

// WithKubeVersionGetter sets the provider for the current Kubernetes version.
func WithKubeVersionGetter(kubeVersionGetter version.Getter) Option {
	return func(s *Scheduler) {
		s.kubeVersionGetter = kubeVersionGetter
	}
}

// WithDeckhouseVersionGetter sets the provider for the current Deckhouse version.
func WithDeckhouseVersionGetter(deckhouseVersionGetter version.Getter) Option {
	return func(s *Scheduler) {
		s.deckhouseVersionGetter = deckhouseVersionGetter
	}
}

// WithBootstrapCondition sets the predicate that gates scheduling until bootstrap is ready.
func WithBootstrapCondition(cond condition.Condition) Option {
	return func(s *Scheduler) {
		s.bootstrapCondition = cond
	}
}

// WithDependencyGetter sets the provider for the current dependency version.
func WithDependencyGetter(getter dependency.Getter) Option {
	return func(s *Scheduler) {
		s.dependencyGetter = getter
	}
}

// NewScheduler creates a Scheduler with an empty dependency graph and a
// buffered event channel. Use functional options to configure version
// providers and conditions. Call [Scheduler.Ch] to consume lifecycle events.
func NewScheduler(opts ...Option) *Scheduler {
	s := &Scheduler{
		nodes:   make(map[string]*node),
		eventCh: make(chan Event, defaultBufferSize),
	}

	for _, opt := range opts {
		opt(s)
	}

	s.pause.Store(true) // Start paused - no state changes until Resume()

	return s
}

// Pause prevents any state changes from being processed.
func (s *Scheduler) Pause() {
	s.pause.Store(true)
}

// Resume enables state change processing and re-evaluates all packages.
// For each package whose state changed, the appropriate callback is invoked.
func (s *Scheduler) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Only process if transitioning from paused to running
	if !s.pause.CompareAndSwap(true, false) {
		return // Already running, no-op
	}

	s.schedule()
}

// CheckConstraints evaluates the given constraints against the current cluster
// state and the current dependency graph. Returns an error describing the
// first unsatisfied constraint (version, dependency) or a *CycleError if
// adding a node named `name` with these dependencies would create a
// topological cycle. Returns nil only when every check passes and the
// proposed addition would leave the dep graph acyclic.
//
// `name` is the scheduler-side identifier of the package that would be added.
// It is used by the cycle-simulation step to identify the proposed graph vertex.
func (s *Scheduler) CheckConstraints(name string, constraints Constraints) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var checkers []checker.Checker

	if constraints.Kubernetes != nil && s.kubeVersionGetter != nil {
		checkers = append(checkers, version.NewChecker(s.kubeVersionGetter, constraints.Kubernetes, reasonRequirementsKubernetes))
	}

	if constraints.Deckhouse != nil && s.deckhouseVersionGetter != nil {
		checkers = append(checkers, version.NewChecker(s.deckhouseVersionGetter, constraints.Deckhouse, reasonRequirementsDeckhouse))
	}

	if constraints.Order == FunctionalOrder && s.bootstrapCondition != nil {
		checkers = append(checkers, condition.NewChecker(s.bootstrapCondition, reasonRequirementsBootstrap))
	}

	if len(constraints.Dependencies) > 0 && s.dependencyGetter != nil {
		deps := make(map[string]dependency.Dependency, len(constraints.Dependencies))
		for depName, dep := range constraints.Dependencies {
			deps[depName] = dependency.Dependency{
				Constraint: dep.Constraint,
				Optional:   dep.Optional,
			}
		}

		checkers = append(checkers, dependency.NewChecker(s.dependencyGetter, deps))
	}

	if len(constraints.AnyOf) > 0 && s.dependencyGetter != nil {
		checkers = append(checkers, dependency.NewAnyOfChecker(s.dependencyGetter, toAnyOfGroups(constraints.AnyOf)))
	}

	if len(constraints.NoneOf) > 0 && s.dependencyGetter != nil {
		checkers = append(checkers, dependency.NewNoneOfChecker(s.dependencyGetter, toNoneOfGroups(constraints.NoneOf)))
	}

	if res := checker.Check(checkers...); !res.Enabled {
		return errors.New(res.Message)
	}

	return s.simulateCycle(name, constraints)
}

// simulateCycle returns a *CycleError if adding (or replacing) a node named
// `name` with the given constraints would create a topological cycle in the
// current graph. Used by both CheckConstraints (admission-time pre-check) and
// AddNode (the authoritative gate before any mutation).
//
// Must be called with s.mu held in some mode.
func (s *Scheduler) simulateCycle(name string, constraints Constraints) error {
	snapshot := make(map[string]*node, len(s.nodes)+1)
	for nodeName, n := range s.nodes {
		if nodeName == name {
			continue
		}

		snapshot[nodeName] = n
	}

	snapshot[name] = &node{
		name:         name,
		order:        constraints.Order,
		dependencies: constraints.Dependencies,
	}

	if _, err := topoSort(snapshot); err != nil {
		return err
	}

	return nil
}

// AddNode registers a single package, wires it into the existing graph, and
// triggers a full scheduling pass. Newly-eligible dependents are advanced
// automatically.
//
// Returns a *CycleError (without mutating any state) if adding the package
// would close a dependency cycle. Callers are expected to handle the error —
// typically by surfacing a status condition on the corresponding CR — and to
// retry once the manifest is fixed.
func (s *Scheduler) AddNode(pkg Package) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.simulateCycle(pkg.GetName(), pkg.GetConstraints()); err != nil {
		return err
	}

	s.addNode(pkg)

	s.schedule()

	return nil
}

// RemoveNode removes a package from the graph and triggers a full reschedule.
func (s *Scheduler) RemoveNode(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.nodes[name]; !ok {
		return
	}

	delete(s.nodes, name)

	s.schedule()
}

// Complete marks the named package as active (processing finished) and
// runs a scheduling pass to advance any newly-eligible dependents.
func (s *Scheduler) Complete(completed string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n, ok := s.nodes[completed]; ok && n.state == nodeStateScheduled {
		n.state = nodeStateActive
	}

	s.schedule()
}

// Disable explicitly turns the named package off, forcing it disabled on every
// subsequent scheduling pass regardless of its checker chain. A scheduling pass
// runs immediately, cascade-disabling the node (and emitting an [EventDisable]
// if it was previously enabled). It is a no-op if the package does not exist or
// is already disabled.
func (s *Scheduler) Disable(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.nodes[name]
	if !ok || n.disabled {
		return
	}

	n.disabled = true

	s.schedule()
}

// Enable clears an explicit disable set by [Scheduler.Disable], allowing the
// node's checker chain to govern eligibility again. A scheduling pass runs
// immediately, re-scheduling the node if its checkers now pass. It is a no-op
// if the package does not exist or is not explicitly disabled.
func (s *Scheduler) Enable(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.nodes[name]
	if !ok || !n.disabled {
		return
	}

	n.disabled = false

	s.schedule()
}

// Reschedule reverts the named package to idle and runs a full scheduling
// pass, causing it (and potentially its dependents) to be rescheduled.
// It is a no-op if the package does not exist.
func (s *Scheduler) Reschedule(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n, ok := s.nodes[name]; ok {
		n.state = nodeStateIdle
	}

	s.schedule()
}

// Schedule forces a full scheduling pass without changing any node state.
// Use when external conditions (e.g. Kubernetes version) have changed
// and the graph needs re-evaluation.
func (s *Scheduler) Schedule() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.schedule()
}

// schedule recomputes enabled status and advances idle nodes that are
// eligible to the scheduled state, emitting an [EventSchedule] for each.
func (s *Scheduler) schedule() {
	if s.pause.Load() {
		return
	}

	enabled, sorted := s.compute()
	for _, n := range sorted {
		if n.state != nodeStateIdle {
			continue
		}

		if s.canSchedule(n) {
			n.state = nodeStateScheduled
			s.send(Event{Name: n.name, Kind: EventSchedule, Enabled: enabled})
		}
	}
}

// compute recomputes the enabled status for all nodes in topological order,
// guaranteeing that dependencies are resolved before dependents. Nodes whose
// Enabled status flipped are individually reset to idle so they re-enter the
// scheduling path on the next pass; nodes that lose eligibility emit an
// [EventDisable]. No global reconverge happens — canSchedule no longer gates
// on per-dep state, so one node's status change cannot invalidate another
// node's schedulability beyond the live order-tier check.
func (s *Scheduler) compute() ([]string, []*node) {
	// AddNode is the authoritative cycle gate, so topoSort should never
	// return an error here. The disabled-mark-active loop below walks `sorted`
	// and relies on that invariant; a cycle slipping through (gate bug) would
	// leave its members frozen at nodeStateIdle, surfaced quickly by stalled
	// higher-tier nodes via canSchedule's order-tier gate.
	sorted, _ := topoSort(s.nodes)
	for _, n := range sorted {
		current := n.status.Enabled
		n.status = checker.Check(n.checkers...)
		// An explicit operator disable is the final gate: it forces the node
		// off even when every checker passes, but defers to the checker chain's
		// more specific reason when that already disabled the node.
		if n.disabled && n.status.Enabled {
			n.status = checker.Result{Reason: reasonDisabled, Message: "package is explicitly disabled"}
		}
		if current == n.status.Enabled {
			continue
		}

		// Status flipped — reset this node so the next schedule pass can
		// either re-schedule it (now enabled) or mark it active via the
		// disabled-mark-active loop below (now disabled).
		n.state = nodeStateIdle

		if !n.status.Enabled {
			s.send(Event{Name: n.name, Kind: EventDisable, Reason: n.status.Reason, Message: n.status.Message})
		}
	}

	var enabled []string

	// Disabled nodes have nothing to wait for — mark them active so they do
	// not block higher-order nodes via canSchedule's order-tier gate. This
	// sweep is unconditional (not gated on a status flip), so nodes that are
	// born disabled — never enabled to begin with — are parked active too.
	// Nodes that later flip back to enabled are reset to idle by the loop
	// above and go through normal scheduling from there.
	for _, n := range sorted {
		if n.state == nodeStateIdle && !n.status.Enabled {
			n.state = nodeStateActive
		}

		// global is the barrier node, not a module — never advertise it in the
		// enabled set the runtime publishes to global values.
		if n.status.Enabled && n.name != "global" {
			enabled = append(enabled, n.name)
		}
	}

	return enabled, sorted
}

// canSchedule returns true if a node is eligible to transition from idle to
// scheduled. Two conditions must hold:
//  1. The node must be enabled (all checkers passed).
//  2. All nodes with a strictly lower Order must be active.
//
// Dependency-level ordering between same-tier nodes is encoded in the checker
// chain (the dependency.Getter contract returns versions only for nodes that
// have reached nodeStateActive).
func (s *Scheduler) canSchedule(n *node) bool {
	if !n.status.Enabled {
		return false
	}

	for _, other := range s.nodes {
		if other.order < n.order && other.state != nodeStateActive {
			return false
		}
	}

	return true
}
