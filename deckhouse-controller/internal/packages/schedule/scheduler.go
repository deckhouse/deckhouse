// Copyright 2025 Flant JSC
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
	"context"
	"sync"
	"sync/atomic"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/condition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/dependency"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker/version"
)

// Package represents a package that can be scheduled for enable/disable based on conditions.
type Package interface {
	GetName() string
	GetChecks() Checks
}

// Scheduler manages package enable/disable state based on version constraints and conditions.
// It evaluates checkers for each package and invokes callbacks when state changes.
//
// Thread-safety: Uses mutex for nodes map and atomic.Bool for pause state.
type Scheduler struct {
	onEnable  Callback // Called when package transitions to enabled state
	onDisable Callback // Called when package transitions to disabled state

	ctx context.Context

	kubeVersionGetter      version.Getter      // Gets current Kubernetes version
	deckhouseVersionGetter version.Getter      // Gets current Deckhouse version
	dependencyGetter       dependency.Getter   // Get dependencies
	bootstrapCondition     condition.Condition // Bootstrap readiness check

	pause atomic.Bool // When true, no state changes are processed

	mu    sync.Mutex       // Protects nodes map
	nodes map[string]*node // Package name -> node mapping
}

// Callback is invoked when package state changes.
type Callback func(ctx context.Context, name string)

// Checks defines version constraints that must be satisfied for a package to be enabled.
type Checks struct {
	Kubernetes *semver.Constraints              // Kubernetes version constraint (e.g., ">=1.21")
	Deckhouse  *semver.Constraints              // Deckhouse version constraint
	Modules    map[string]dependency.Dependency // Module dependency constraints
}

type Option func(*Scheduler)

func WithKubeVersionGetter(kubeVersionGetter version.Getter) Option {
	return func(s *Scheduler) {
		s.kubeVersionGetter = kubeVersionGetter
	}
}

func WithDeckhouseVersionGetter(deckhouseVersionGetter version.Getter) Option {
	return func(s *Scheduler) {
		s.deckhouseVersionGetter = deckhouseVersionGetter
	}
}

func WithBootstrapCondition(cond condition.Condition) Option {
	return func(s *Scheduler) {
		s.bootstrapCondition = cond
	}
}

func WithDependencyGetter(dependencyGetter dependency.Getter) Option {
	return func(s *Scheduler) {
		s.dependencyGetter = dependencyGetter
	}
}

func WithOnEnable(callback Callback) Option {
	return func(s *Scheduler) {
		s.onEnable = callback
	}
}

func WithOnDisable(callback Callback) Option {
	return func(s *Scheduler) {
		s.onDisable = callback
	}
}

// NewScheduler creates a new Scheduler instance.
// The scheduler starts in paused state and must be explicitly resumed.
func NewScheduler(opts ...Option) *Scheduler {
	sch := new(Scheduler)

	sch.ctx = context.Background()
	sch.nodes = make(map[string]*node)
	sch.pause.Store(true) // Start paused - no state changes until Resume()

	for _, opt := range opts {
		opt(sch)
	}

	return sch
}

// State represents the current enable/disable state of a package.
type State struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`                   // Whether package is enabled
	Reason  string `json:"reason,omitempty" yaml:"reason,omitempty"` // Reason for current state (typically set when disabled)
}

// State returns the current enable/disable state for a package.
// Returns State{Enabled: false} if package is not registered.
func (s *Scheduler) State(name string) State {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.nodes[name]
	if !ok {
		return State{Enabled: false}
	}

	return State{
		Enabled: n.enabled,
		Reason:  n.reason,
	}
}

// Add registers a package with the scheduler and creates checkers based on its constraints.
// If scheduler is not paused and checks pass, onEnable callback is invoked immediately.
//
// Checker evaluation order:
//  1. Kubernetes version
//  2. Deckhouse version
//  3. Bootstrap condition
//
// Thread-safety: Acquires mutex to add node, releases before invoking callbacks to avoid deadlock.
func (s *Scheduler) Add(pkg Package) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pkg == nil {
		return
	}

	var checkers []checker.Checker
	checks := pkg.GetChecks()

	// Add version constraint checkers (all are blockers)
	if checks.Kubernetes != nil && s.kubeVersionGetter != nil {
		checkers = append(checkers, version.NewChecker(s.kubeVersionGetter, checks.Kubernetes))
	}

	if checks.Deckhouse != nil && s.deckhouseVersionGetter != nil {
		checkers = append(checkers, version.NewChecker(s.deckhouseVersionGetter, checks.Deckhouse))
	}

	if len(checks.Modules) > 0 && s.dependencyGetter != nil {
		checkers = append(checkers, dependency.NewChecker(s.dependencyGetter, checks.Modules))
	}

	// Add bootstrap condition as blocker (prevents enabling during startup)
	if s.bootstrapCondition != nil {
		checkers = append(checkers, condition.NewChecker(s.bootstrapCondition))
	}

	ctx, cancel := context.WithCancel(s.ctx)
	s.nodes[pkg.GetName()] = &node{
		ctx:      ctx,
		cancel:   cancel,
		name:     pkg.GetName(),
		checkers: checkers,
	}

	if !s.pause.Load() {
		s.schedule(s.nodes[pkg.GetName()])
	}
}

// Remove unregisters a package from the scheduler.
// No callback is invoked - the package is simply removed from tracking.
func (s *Scheduler) Remove(pkg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.nodes, pkg)
}

// Pause prevents any state changes from being processed.
// Packages can still be added/removed, but no callbacks will be invoked.
func (s *Scheduler) Pause() {
	s.pause.Store(true)
}

// Resume enables state change processing and re-evaluates all packages.
// For each package whose state changed, the appropriate callback is invoked.
func (s *Scheduler) Resume() {
	// Only process if transitioning from paused to running
	if !s.pause.CompareAndSwap(true, false) {
		return // Already running, no-op
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Re-evaluate all packages and invoke callbacks for state changes
	for _, n := range s.nodes {
		s.schedule(n)
	}
}

// schedule evaluates a node's checkers and invokes callbacks if state changed.
//
// Logic:
//  1. Check current state against all checkers
//  2. If no state change, return early
//  3. If state changed to enabled, call onEnable
//  4. If state changed to disabled, call onDisable
//
// WARNING: Called while holding mutex from Resume(), callbacks must not deadlock.
func (s *Scheduler) schedule(n *node) {
	stateChanged := n.check()
	if !stateChanged {
		return // No state change, nothing to do
	}

	// to cancel the current task
	n.cancel()

	// renew context
	n.ctx, n.cancel = context.WithCancel(s.ctx)

	// State changed - invoke appropriate callback
	switch n.enabled {
	case true:
		if s.onEnable != nil {
			s.onEnable(n.ctx, n.name)
		}
	case false:
		if s.onDisable != nil {
			s.onDisable(n.ctx, n.name)
		}
	}
}

// node represents a package with its enable/disable state and checkers.
type node struct {
	ctx    context.Context
	cancel context.CancelFunc

	name     string            // Package name
	enabled  bool              // Current enable/disable state
	reason   string            // Reason for current state (set by failing checker)
	checkers []checker.Checker // Ordered list of checkers to evaluate
}

// check evaluates all checkers and updates the node's state.
// Returns true if state changed (enabled ↔ disabled).
func (n *node) check() bool {
	current := n.enabled

	n.enabled = true
	for _, ch := range n.checkers {
		if res := ch.Check(); !res.Enabled {
			n.enabled = false
			n.reason = res.Reason
			break
		}
	}

	return current != n.enabled // Return true if state changed
}
