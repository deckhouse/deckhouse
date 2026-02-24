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
	"sync"
	"sync/atomic"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/condition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/dependency"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/version"
)

const (
	// packageGlobal is the sentinel node that all other nodes implicitly depend on.
	// It acts as the root of the dependency graph and must complete before any scheduling begins.
	packageGlobal = "global"

	// FunctionalOrder is the Order value assigned to functional (non-critical) packages.
	// It is higher than any critical package order, ensuring functional packages are
	// scheduled only after all critical packages have been processed.
	FunctionalOrder = 999
)

// Scheduler manages a dependency graph of packages and their lifecycle.
// Each scheduling pass recomputes eligibility, cascade-disables nodes
// that lost it, and advances newly-eligible nodes — all in topological order.
// All exported methods are safe for concurrent use.
type Scheduler struct {
	mu    sync.RWMutex
	nodes map[string]*node

	kubeVersionGetter      version.Getter      // Gets current Kubernetes version
	deckhouseVersionGetter version.Getter      // Gets current Deckhouse version
	bootstrapCondition     condition.Condition // Bootstrap readiness check

	pause atomic.Bool // When true, no state changes are processed

	onSchedule   Callback
	onDisable    Callback
	onGlobalDone func(enabled []string)
}

// Callback is a function invoked by the scheduler on state transitions.
type Callback func(name string)

// Option configures a Scheduler during construction.
type Option func(*Scheduler)

// WithOnSchedule sets the callback fired when a node becomes scheduled.
func WithOnSchedule(f Callback) Option {
	return func(s *Scheduler) {
		s.onSchedule = f
	}
}

// WithOnDisable sets the callback fired when a node is cascade-disabled.
func WithOnDisable(f Callback) Option {
	return func(s *Scheduler) {
		s.onDisable = f
	}
}

// WithOnGlobalDone sets the callback fired once the global node completes,
// receiving the list of currently enabled package names.
func WithOnGlobalDone(f func(enabled []string)) Option {
	return func(s *Scheduler) {
		s.onGlobalDone = f
	}
}

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

// NewScheduler creates a Scheduler with an empty dependency graph.
// Use functional options to configure callbacks and version providers.
func NewScheduler(opts ...Option) *Scheduler {
	s := &Scheduler{
		nodes:        make(map[string]*node),
		onSchedule:   func(_ string) {},
		onDisable:    func(_ string) {},
		onGlobalDone: func(_ []string) {},
	}

	for _, opt := range opts {
		opt(s)
	}

	s.pause.Store(true) // Start paused - no state changes until Resume()

	return s
}

// Pause prevents any state changes from being processed.
// Packages can still be added/removed, but no callbacks will be invoked.
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

// CheckByConstraints evaluates the given constraints against the current cluster state
// and returns an error describing the first unsatisfied constraint, or nil if all are met.
func (s *Scheduler) CheckByConstraints(constraints Constraints) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var checkers []checker.Checker

	if constraints.Kubernetes != nil && s.kubeVersionGetter != nil {
		checkers = append(checkers, version.NewChecker(s.kubeVersionGetter, constraints.Kubernetes, string(ConditionReasonRequirementsKubernetes)))
	}

	if constraints.Deckhouse != nil && s.deckhouseVersionGetter != nil {
		checkers = append(checkers, version.NewChecker(s.deckhouseVersionGetter, constraints.Deckhouse, string(ConditionReasonRequirementsDeckhouse)))
	}

	if constraints.Order == FunctionalOrder && s.bootstrapCondition != nil {
		checkers = append(checkers, condition.NewChecker(s.bootstrapCondition, string(ConditionReasonRequirementsBootstrap)))
	}

	if len(constraints.Dependencies) > 0 {
		deps := make(map[string]dependency.Dependency)
		for name, dep := range constraints.Dependencies {
			deps[name] = dependency.Dependency{
				Constraint: dep.Constraint,
				Optional:   dep.Optional,
			}
		}

		checkers = append(checkers, dependency.NewChecker(s.getVersion, deps))
	}

	if res := checker.Check(checkers...); !res.Enabled {
		return newRequirementsErr(res.Reason, res.Message)
	}

	return nil
}

// Initialize bulk-loads packages into the graph and runs a single
// scheduling pass. Use this for the initial population of the graph
// instead of calling AddNode in a loop, which would trigger a
// reconverge after every insertion.
func (s *Scheduler) Initialize(pkgs ...Package) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, pkg := range pkgs {
		s.addNode(pkg)
	}

	s.schedule()
}

// AddNode registers a single package, wires its dependency edges into the
// existing graph, and triggers a full scheduling pass. Newly-eligible
// dependents are advanced automatically.
func (s *Scheduler) AddNode(pkg Package) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.addNode(pkg)

	s.reconverge()
}

// RemoveNode removes a package from the graph, cleans up all dependency
// edges that reference it, and triggers a full reschedule.
func (s *Scheduler) RemoveNode(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.nodes[name]
	if !ok {
		return
	}

	// Remove this node from the followers set of every node it depends on.
	for dep := range n.followees {
		if parent, ok := s.nodes[dep]; ok {
			delete(parent.followers, name)
		}
	}

	delete(s.nodes, name)

	s.reconverge()
}

// Complete marks the named package as active (processing finished) and
// runs a scheduling pass to advance any newly-eligible dependents.
func (s *Scheduler) Complete(completed string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n, ok := s.nodes[completed]; ok && n.state == nodeStateScheduled {
		n.state = nodeStateActive
	}

	if completed == packageGlobal {
		var enabled []string
		for _, n := range s.compute() {
			if n.name == packageGlobal || !n.status.Enabled {
				continue
			}
			enabled = append(enabled, n.name)
		}

		s.onGlobalDone(enabled)
	}

	s.schedule()
}

// Trigger resets a node's direct followers to idle, then runs a full
// scheduling pass so they are re-evaluated and potentially rescheduled.
func (s *Scheduler) Trigger(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.trigger(name)
	s.schedule()
}

// trigger resets a node's direct followers to idle.
func (s *Scheduler) trigger(name string) {
	n, ok := s.nodes[name]
	if !ok {
		return
	}

	for follower := range n.followers {
		if fn, ok := s.nodes[follower]; ok {
			fn.state = nodeStateIdle
		}
	}
}

// reconverge marks the global node as scheduled, resets all its direct
// followers to idle, and runs a full scheduling pass — effectively
// forcing the entire graph to re-converge from scratch.
func (s *Scheduler) reconverge() {
	n, ok := s.nodes[packageGlobal]
	if !ok {
		return
	}

	n.state = nodeStateIdle

	for follower := range n.followers {
		if fn, ok := s.nodes[follower]; ok {
			fn.state = nodeStateIdle
		}
	}

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
// eligible to the scheduled state, firing onSchedule for each.
func (s *Scheduler) schedule() {
	if s.pause.Load() {
		return
	}

	for _, n := range s.compute() {
		if n.state != nodeStateIdle {
			continue
		}

		if s.canSchedule(n) {
			n.state = nodeStateScheduled
			s.onSchedule(n.name)
		}
	}
}

// compute recomputes the enabled status for all nodes in topological order,
// guaranteeing that dependencies are resolved before dependents. Nodes that
// lose eligibility fire onDisable immediately. If any status changed,
// reconverge is called to reset the graph from the global node.
func (s *Scheduler) compute() []*node {
	var changed bool
	sorted := topoSort(s.nodes)
	for _, n := range sorted {
		current := n.status.Enabled
		n.status = checker.Check(n.checkers...)
		if current != n.status.Enabled {
			changed = true

			if !n.status.Enabled {
				s.onDisable(n.name)
			}
		}
	}

	if changed {
		s.reconverge()
	}

	return sorted
}

// canSchedule returns true if a node is eligible to transition from idle to scheduled.
// Three conditions must hold:
//  1. The node must be enabled (all dependency checks passed).
//  2. All direct dependencies (followees) must be active.
//  3. All nodes with a strictly lower Order must be active.
func (s *Scheduler) canSchedule(n *node) bool {
	if !n.status.Enabled {
		return false
	}

	for dep := range n.followees {
		if existing, ok := s.nodes[dep]; ok {
			if existing.state != nodeStateActive {
				return false
			}
		}
	}

	for _, other := range s.nodes {
		if other.order < n.order && other.state != nodeStateActive {
			return false
		}
	}

	return true
}

// getVersion returns the semver version of the named node, or nil if not found.
func (s *Scheduler) getVersion(name string) *semver.Version {
	n, ok := s.nodes[name]
	if !ok || !n.status.Enabled {
		return nil
	}

	return n.version
}
