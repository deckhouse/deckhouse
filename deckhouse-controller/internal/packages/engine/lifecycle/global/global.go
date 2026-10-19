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

// Package global runs Global: the global hooks, the global settings and the global run, which
// ensures the CRDs and conversion webhooks of the enabled modules and publishes the enabled set
// into the global values. Start reading with next.go (next and supersedes: every rule of Global),
// then loop.go.
//
// Global is not a package Reconciler: it is never installed, updated or removed, its object is the
// global module the deckhouse image ships, and its hooks start once. It shares the convergence
// protocol with Current (facts for the node schedule.GlobalNode) and is the barrier: no other
// node's gate opens while Global is not converged, so a failing global run holds every new run.
// Packages hash Global's Output, not its Applied, so a failed or cancelled run cancels no run in
// flight. Global exists only with EnableModuleV2.
package global

import (
	"context"
	"log/slog"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/inbox"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/report"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Global is the global component; Engine creates, starts and stops it. The loop goroutine owns all
// state and decides with next. One operation at a time runs in a worker goroutine that only the
// loop starts and cancels: first the sync (next, rule 2), then the global runs. Why a worker: new
// settings and Stop must be able to cancel a run in flight.
type Global struct {
	// Immutable after New.
	module     Module
	version    string
	deps       Deps
	clock      retry.Clock
	reporter   *report.Reporter
	logger     *log.Logger
	checkpoint *model.Settings // checkpoint is pushed once by Start.

	// ctx is Global's own context, set by Start. The global hooks live under it, not under the
	// sync operation's: they outlive the sync.
	ctx context.Context
	// in carries Engine's inputs and the hook lanes' changed(); only the loop loads it.
	in     *inbox.Inbox[input]
	cancel context.CancelFunc
	done   chan struct{} // done is closed when the loop has exited.
	synced chan struct{} // synced is closed once the sync succeeded.

	// Owned by the loop.

	state state // state is what next decides from, besides the inputs.
	// output is Facts.Output: the values checksum after the last publish step, kept when the run
	// fails later.
	output string
	// stopHooks stops the started global hooks; nil until the sync succeeded.
	stopHooks func()
	// operation is the operation in progress, nil when the worker is idle; operationState is the
	// state its steps share, read back when it is done.
	operation      *operation.Operation
	operationState *operationState
}

// New returns a Global that is not running yet.
func New(cfg Config, deps Deps) *Global {
	logger := deps.Logger
	if logger == nil {
		logger = log.NewNop()
	}

	return &Global{
		module:     cfg.Module,
		version:    cfg.Version,
		deps:       deps,
		clock:      retry.OrWall(deps.Clock),
		reporter:   report.New(cfg.Notify),
		logger:     logger.With(slog.String("package", schedule.GlobalNode)),
		checkpoint: cfg.Settings,
		in:         inbox.New[input](),
		done:       make(chan struct{}),
		synced:     make(chan struct{}),
	}
}

// Start reports its version, pushes the checkpoint settings, when there are any, and starts the
// loop, whose first operation is the sync. parent is the engine's context, not Boot's. Non-blocking.
func (g *Global) Start(parent context.Context) {
	g.ctx, g.cancel = context.WithCancel(parent)

	// Global has no package Reconciler to activate it: it runs the image's version from the start.
	g.reporter.SetActivated(g.version)

	if cp := g.checkpoint; cp != nil {
		g.Apply(*cp)
	}

	go g.loop(g.ctx)
}

// Apply records the latest global settings. Non-blocking.
func (g *Global) Apply(settings model.Settings) {
	// The checksum is computed here, outside the inbox lock, once per push.
	checksum := configstate.Key(&settings)

	g.in.Put(func(in *input) { in.settings, in.settingsChecksum = &settings, checksum })
}

// HandleScheduleEvent records the latest event of the global node. Non-blocking.
func (g *Global) HandleScheduleEvent(ev schedule.Event) {
	g.in.Put(func(in *input) { in.event = &ev })
}

// Synced is closed once the global hooks are started and synchronized: the boot gate.
func (g *Global) Synced() <-chan struct{} {
	return g.synced
}

// Report returns a snapshot of Global's Report (only its Current section is written). Safe from
// any goroutine.
func (g *Global) Report() model.Report {
	return g.reporter.Snapshot()
}

// Stop cancels the operation in progress, stops the global hook lanes and waits for every
// Global goroutine to exit, so a stopped Global never writes again. Nil-safe; returns at once if
// Start never ran.
func (g *Global) Stop() {
	if g == nil || g.cancel == nil {
		return
	}

	g.cancel()
	<-g.done
}

// changed is the global hooks' callback after a runtime hook changed the values. Non-blocking.
func (g *Global) changed() {
	g.in.Put(func(in *input) { in.valuesGen++ })
}
