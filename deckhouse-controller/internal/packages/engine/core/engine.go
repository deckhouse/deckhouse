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

// Package core is the engine core: the registry of package Reconcilers, Global, the boot, the
// dispatcher of scheduler events and readiness. It makes no lifecycle decisions and never reads
// Kubernetes: it passes every input whole to the package Reconciler that owns the name, to Global
// or to the scheduler.
package core

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/global"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const defaultRestoreParallelism = 12

// scheduler is what Engine consumes from the scheduler (*schedule.Scheduler) and hands on: New
// gives the same value to every package Reconciler, Current and Global, so one graph serves all.
type scheduler interface {
	// CheckConstraints, CheckDependents, AddNode and RemoveNode are passed on to the package
	// Reconcilers.
	CheckConstraints(name string, constraints schedule.Constraints) error
	CheckDependents(name string, version *semver.Version) error
	AddNode(node schedule.Node) error
	RemoveNode(name string)

	// Publish is never called by Engine: it is passed on to Current and Global.
	Publish(name string, facts schedule.Facts)

	SetIntent(name string, enabled *bool)
	Resume()
	Recheck()
	Wake() <-chan struct{} // Wake is closed by Stop.
	Take() []schedule.Event
	Converged() bool
	Stop()
}

// Config configures an Engine.
type Config struct {
	// Notify is called with a package name after its Report changed, from the package Reconciler,
	// Current and Global goroutines; it must not block. Nil disables it.
	Notify func(name string)
}

// Deps are the effects of every package Reconciler and of Global; New sets their scheduler fields.
type Deps struct {
	Reconciler lifecycle.Deps
	Global     global.Deps
}

// Engine is the package registry; every method is safe for concurrent use.
//
// Invariant: Boot runs exactly once, before any controller starts; controllers call Apply and
// Remove only after Boot has returned; Shutdown runs exactly once, and Apply and Remove are
// ignored from its start (controllers may still reconcile while it runs).
type Engine struct {
	cfg       Config
	scheduler scheduler
	deps      Deps

	// ctx is the lifetime of every package Reconciler and of Global, cancelled last by Shutdown.
	// Not Boot's ctx: that one covers only restoring and may carry a deadline.
	ctx    context.Context
	cancel context.CancelFunc

	// mu guards the fields below. It is held only for map work and the non-blocking the package
	// Reconciler and Global methods, never across lifecycle.Reconciler.Shutdown, Global.Stop, a
	// restore or a scheduler call.
	mu sync.Mutex
	// packages has one entry per name. A removed package Reconciler stays, reporting Removed,
	// until Apply reuses its name. The name global is never in it.
	packages map[string]*entry
	// global is nil until Boot started it, and for good without EnableModuleV2.
	global *global.Global
	// stopped is set at the start of Shutdown: a package Reconciler started after it would
	// outlive Shutdown, which waits only for those it found.
	stopped      bool
	dispatchDone chan struct{} // dispatchDone is closed when the dispatcher has exited.
	logger       *log.Logger

	ready  atomic.Bool // ready latches the first true Ready.
	resume resumeState

	// expected are the modules Boot restored; Ready waits until each has received its first Apply
	// or Remove, so readiness does not latch on a restored module the controllers have not applied
	// yet. A module passed to Boot but not restored is a first install: readiness never waits for
	// it (a first install that keeps failing must not hold readiness).
	expected map[string]struct{}
}

// entry is one registry slot; it is replaced only after its package Reconciler's loop has exited.
type entry struct {
	reconciler *lifecycle.Reconciler
	// addressed: Apply or Remove was called (bootApplied). An Apply after Remove is ignored by the
	// package Reconciler itself until it reports Removed: in Kubernetes an object being deleted
	// gets only Remove, and an object of the same name appears only after the finalizer is gone.
	addressed bool
	kind      model.Kind // kind is the kind the package Reconciler was started with (Names).
}

// New returns an Engine and starts its dispatcher. The scheduler must be paused.
func New(cfg Config, sched scheduler, deps Deps) *Engine {
	deps.Reconciler.Scheduler = sched
	deps.Reconciler.Current.Scheduler = sched
	deps.Global.Scheduler = sched

	ctx, cancel := context.WithCancel(context.Background())

	logger := deps.Reconciler.Logger
	if logger == nil {
		logger = log.NewNop()
	}

	e := &Engine{
		cfg:          cfg,
		scheduler:    sched,
		deps:         deps,
		ctx:          ctx,
		cancel:       cancel,
		packages:     make(map[string]*entry),
		dispatchDone: make(chan struct{}),
		logger:       logger,
	}

	// Started here, not in Boot, so that Shutdown can always wait for it, also after a failed Boot;
	// a paused sched sends no events.
	go e.dispatch()

	return e
}

// Recheck makes the scheduler compute once more, so the rules re-read inputs that are not facts
// of any node: without a Global, addon-operator's state (the wiring must call it at the end of
// every addon-operator converge); with one, a global hook's enable signal, which changes no values
// checksum (Build wires it). A no-op before the boot gate ended and after Shutdown.
func (e *Engine) Recheck() {
	e.scheduler.Recheck()
}

// Apply passes desired to the package Reconciler of its name, creating it for an unknown name or
// one that has finished (see finished); the package Reconciler ignores it while it removes (see
// entry).
// KindGlobal and the reserved name global go to Global (dropped without one), so no package
// Reconciler can take Global's node. A module's Intent reaches the scheduler first, also while the
// package Reconciler removes. Non-blocking.
func (e *Engine) Apply(desired model.Desired) {
	if desired.Kind == model.KindModule && !isGlobal(desired.Name, desired.Kind) {
		e.scheduler.SetIntent(desired.Name, desired.Intent)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped {
		e.logger.Debug("apply after shutdown ignored", slog.String("package", desired.Name))
		return
	}

	if isGlobal(desired.Name, desired.Kind) {
		if e.global != nil {
			e.global.Apply(desired.Configuration.Settings)
		}

		return
	}

	ent, ok := e.packages[desired.Name]
	if ok && !finished(ent) {
		ent.addressed = true
		ent.reconciler.Apply(desired)

		return
	}

	if ok {
		// Immediate: Removed is the last thing the loop does.
		<-ent.reconciler.Done()
	}

	ent = e.startLocked(desired.Name, desired.Kind, nil)
	ent.addressed = true
	ent.reconciler.Apply(desired)
}

// Remove asks the package Reconciler of name to uninstall; completion is visible as Report.Removed.
// For an unknown name it creates a package Reconciler that deletes the release by name (kind
// selects its namespace): this is how a remove interrupted by a restart finishes. A repeated Remove
// keeps reporting Removed. global is never removed. Non-blocking.
func (e *Engine) Remove(name string, kind model.Kind) {
	if name == schedule.GlobalNode {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped {
		e.logger.Debug("remove after shutdown ignored", slog.String("package", name))
		return
	}

	ent, ok := e.packages[name]

	switch {
	case ok && ent.reconciler.Report().Removed:
		return
	case !ok || done(ent.reconciler):
		ent = e.startLocked(name, kind, nil)
	}

	ent.addressed = true
	ent.reconciler.Remove()
}

// Shutdown stops everything without uninstalling or detaching anything, and blocks until every
// goroutine has exited; also after a failed Boot. Apply and Remove are ignored from its start. Package Reconcilers stop concurrently, then
// Global, because packages run against what Global provides; then the scheduler, whose Stop ends
// the dispatcher.
func (e *Engine) Shutdown() {
	e.mu.Lock()
	e.stopped = true
	reconcilers := make([]*lifecycle.Reconciler, 0, len(e.packages))
	for _, ent := range e.packages {
		reconcilers = append(reconcilers, ent.reconciler)
	}
	g := e.global
	e.mu.Unlock()

	var wg sync.WaitGroup
	for _, rec := range reconcilers {
		wg.Go(rec.Shutdown)
	}

	wg.Wait()

	g.Stop()

	e.scheduler.Stop()
	<-e.dispatchDone
	e.cancel()
}

// Report returns the lifecycle half of the named package's status and whether the package is
// known; global is Global's. Never blocks on the package Reconciler or Global work.
func (e *Engine) Report(name string) (model.Report, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if name == schedule.GlobalNode {
		if e.global == nil {
			return model.Report{}, false
		}

		return e.global.Report(), true
	}

	ent, ok := e.packages[name]
	if !ok {
		return model.Report{}, false
	}

	return ent.reconciler.Report(), true
}

// Activated returns the revision name's Current runs, for the admission webhooks and the debug
// server; false without one, also once removed. Never blocks on the package Reconciler's work.
func (e *Engine) Activated(name string) (model.Revision, bool) {
	e.mu.Lock()
	ent, ok := e.packages[name]
	e.mu.Unlock()

	if !ok {
		return model.Revision{}, false
	}

	return ent.reconciler.Activated()
}

// Names returns the names of the package Reconcilers of kind, removed ones included, sorted.
// Never blocks on the package Reconciler work.
func (e *Engine) Names(kind model.Kind) []string {
	e.mu.Lock()
	defer e.mu.Unlock()

	names := make([]string, 0, len(e.packages))
	for name, ent := range e.packages {
		if ent.kind == kind {
			names = append(names, name)
		}
	}

	slices.Sort(names)

	return names
}

// Ready reports whether the startup converge has finished: every module Boot restored was applied
// or removed, and Converged was true, at least once. It latches, because it backs the
// readiness probe: every settings change or update makes some node unconverged for a while, and a
// live answer would take deckhouse's own webhooks off their Service endpoints each time. A restored
// module that keeps failing keeps it false, as it held addon-operator's first converge (decided).
func (e *Engine) Ready() bool {
	if e.ready.Load() {
		return true
	}

	if !e.bootApplied() || !e.Converged() {
		return false
	}

	e.ready.Store(true)

	return true
}

func (e *Engine) bootApplied() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	for name := range e.expected {
		if ent, ok := e.packages[name]; !ok || !ent.addressed {
			return false
		}
	}

	return true
}

// Converged reports the live convergence: Global synchronized, when there is one, and the
// scheduler's global and module nodes converged (applications never hold it).
func (e *Engine) Converged() bool {
	return e.GlobalSynced() && e.scheduler.Converged()
}

// GlobalSynced reports whether Global's hooks are synchronized; true without Global.
func (e *Engine) GlobalSynced() bool {
	e.mu.Lock()
	g := e.global
	e.mu.Unlock()

	return g == nil || closed(g.Synced())
}

// dispatch is the dispatcher goroutine, the only reader of scheduler events: it hands every event
// of a batch to its package Reconciler or to Global, never blocking on them, and wakes every
// package Reconciler the batch has no event for. Every computation emits
// an event for every node, so those are the package Reconcilers without a node, whose requirements
// gate still depends on the graph. It exits when Stop closes Wake.
func (e *Engine) dispatch() {
	defer close(e.dispatchDone)

	for range e.scheduler.Wake() {
		batch := e.scheduler.Take()

		e.mu.Lock()
		delivered := make(map[string]bool, len(batch))

		for _, ev := range batch {
			if ev.Name == schedule.GlobalNode {
				if e.global != nil {
					e.global.HandleScheduleEvent(ev)
				}

				continue
			}

			if ent, ok := e.packages[ev.Name]; ok {
				ent.reconciler.HandleScheduleEvent(ev)
				delivered[ev.Name] = true
			}
		}

		for name, ent := range e.packages {
			if !delivered[name] {
				ent.reconciler.Wake()
			}
		}
		e.mu.Unlock()
	}
}

// startLocked creates, registers and starts a package Reconciler, replacing any previous entry;
// e.mu held.
func (e *Engine) startLocked(name string, kind model.Kind, restored *model.Restored) *entry {
	cfg := lifecycle.Config{
		Name:     name,
		Kind:     kind,
		Restored: restored,
		Notify:   e.notifier(name),
	}

	ent := &entry{reconciler: lifecycle.NewReconciler(cfg, e.deps.Reconciler), kind: kind}
	e.packages[name] = ent
	ent.reconciler.Start(e.ctx)

	return ent
}

func (e *Engine) notifier(name string) func() {
	if e.cfg.Notify == nil {
		return nil
	}

	return func() { e.cfg.Notify(name) }
}

func isGlobal(name string, kind model.Kind) bool {
	return kind == model.KindGlobal || name == schedule.GlobalNode
}

// finished reports whether ent's loop has exited or reported Removed, its last act before it exits:
// the controller sees only the Report, so an Apply it sends after Removed starts a new package.
func finished(ent *entry) bool {
	return done(ent.reconciler) || ent.reconciler.Report().Removed
}

func done(rec *lifecycle.Reconciler) bool {
	return closed(rec.Done())
}

func closed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
