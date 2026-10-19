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

// The package Reconciler type, its inputs and what it consumes.

package lifecycle

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/current"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/inbox"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/report"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Deps are the effects of every package Reconciler; shared by all of them and safe for concurrent
// use.
type Deps struct {
	Scheduler scheduler
	Disks     Disks
	Load      LoadFunc
	Current   current.Deps
	Releases  releaseDeleter // Releases deletes the release of a package without a Current.
	Clock     retry.Clock    // Clock nil means the wall clock.
	Logger    *log.Logger    // Logger nil means a no-op logger.
}

// scheduler is the part of the scheduler the package Reconciler and Restore use; Current and Global
// declare their own (Publish). No method blocks on a consumer: events go into latest-value slots.
type scheduler interface {
	CheckConstraints(name string, constraints schedule.Constraints) error
	CheckDependents(name string, version *semver.Version) error
	// AddNode is called only at boot and by activate, before the old Current stops: a node's
	// revision changes only there.
	AddNode(node schedule.Node) error
	RemoveNode(name string)
}

// releaseDeleter deletes a release by name, for a remove without a Current (nelm.Service).
type releaseDeleter interface {
	// Delete uninstalls the release; nil, and quick, when it does not exist.
	Delete(ctx context.Context, namespace, name string) error
	RemoveMonitor(name string)
}

// packageObject is the package object as the settings memo and schedulerNode use it; Current
// alone configures it and runs its hooks.
type packageObject interface {
	ValidateSettings(ctx context.Context, settingsVersion int, settings addonutils.Values) (settingscheck.Result, error)
	GetConstraints() schedule.Constraints
	GetVersion() *semver.Version
	GetPath() string // GetPath is the root the object was loaded from.
}

// Config configures one package Reconciler; the desired state arrives only through Apply.
type Config struct {
	Name string
	// Kind selects the disk, Current's steps and the release namespace.
	Kind model.Kind
	// Restored is the checkpoint brought back at boot; nil when nothing is installed.
	Restored *model.Restored
	// Notify is called after the Report changed; it must not block.
	Notify func()
}

// Reconciler reconciles one package from its whole state on every wake; named so because
// "Package" also named the domain package and the loaded object. The exported methods are its
// inputs and never block on the loop's work, except Shutdown.
type Reconciler struct {
	name     string
	kind     model.Kind
	deps     Deps
	reporter *report.Reporter
	clock    retry.Clock
	logger   *log.Logger
	restored *model.Restored // restored is read once, by Start.

	// in carries Engine's inputs; only the loop loads it.
	in *inbox.Inbox[reconcilerInput]

	cancel context.CancelFunc
	done   chan struct{} // done is closed when the loop has exited.

	// Owned by the loop (and by Start before the loop exists).
	// candidate is the preparation of the desired revision; nil when there is none.
	candidate *candidate
	// current runs the node's revision: the package Reconciler has a node exactly while it has a
	// Current (except inside activate, between AddNode and the start, and inside remove, between
	// RemoveNode and the detach).
	current *current.Reconciler
	// config is what the loop pushes to Current: the checkpoint configuration until the first
	// Apply, then that of the latest Desired.
	config *model.Configuration
	// configChecksum is config's configstate.Key, computed once per configuration.
	configChecksum string
	// wake is the one timer: the window gate, a preparation retry or a settings-check retry,
	// whichever is first.
	wake retry.Timer

	// activated is Current's revision for readers outside the loop (Activated); the loop stores it
	// wherever it sets current, nil without one.
	activated atomic.Pointer[model.Revision]
}

type reconcilerInput struct {
	// desired is nil until the first Apply; until then the package Reconciler evaluates no update.
	desired  *model.Desired
	event    *schedule.Event // event is passed to Current uninterpreted.
	removing bool            // removing is never cleared once set.
}

// NewReconciler returns a package Reconciler that is not running yet. Called by Engine.
func NewReconciler(cfg Config, deps Deps) *Reconciler {
	logger := deps.Logger
	if logger == nil {
		logger = log.NewNop()
	}

	clock := retry.OrWall(deps.Clock)

	wake := clock.NewTimer(time.Hour)
	wake.Stop()

	return &Reconciler{
		name:     cfg.Name,
		kind:     cfg.Kind,
		deps:     deps,
		reporter: report.New(cfg.Notify),
		clock:    clock,
		logger:   logger.With(slog.String("package", cfg.Name)),
		restored: cfg.Restored,
		in:       inbox.New[reconcilerInput](),
		done:     make(chan struct{}),
		wake:     wake,
	}
}

// Start starts the loop; a restored package first starts Current from the checkpoint and pushes
// its configuration, when there is one, and a removing one starts removing. Called once by Engine:
// at the end of Boot for a restored package, else on the first Apply or Remove of the name.
func (r *Reconciler) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel

	if cp := r.restored; cp != nil {
		// An embedded module may be restored with a newer version than status.currentVersion.
		r.reporter.SetActivated(cp.Revision.Image.Version)
		r.current = r.startCurrent(ctx, cp.Revision, cp.Configuration != nil)
		r.activated.Store(&cp.Revision)

		if cp.Configuration != nil {
			r.config, r.configChecksum = cp.Configuration, configstate.Key(cp.Configuration)
			r.current.Push(r.config, r.configChecksum, nil)
		}

		if cp.Removing {
			r.Remove()
		}
	}

	go r.loop(ctx)
}

// Apply records the latest desired state. Non-blocking.
func (r *Reconciler) Apply(desired model.Desired) {
	r.in.Put(func(in *reconcilerInput) { in.desired = &desired })
}

// HandleScheduleEvent records the latest scheduler event of this node. Non-blocking.
func (r *Reconciler) HandleScheduleEvent(ev schedule.Event) {
	r.in.Put(func(in *reconcilerInput) { in.event = &ev })
}

// Wake wakes the loop without a new input: for a package Reconciler without a node, whose
// requirements gate still depends on the graph. Non-blocking.
func (r *Reconciler) Wake() {
	r.in.Put(func(*reconcilerInput) {})
}

// Remove starts the removal; an Apply afterwards is ignored. Non-blocking; completion is visible as
// Report.Removed, after which the loop exits.
func (r *Reconciler) Remove() {
	r.in.Put(func(in *reconcilerInput) { in.removing = true })
}

// Shutdown stops the preparation and Current and waits for the loop. It never uninstalls, never
// detaches Current's image (the next boot restores it) and reports nothing: a restart must not
// change statuses. Returns at once if Start never ran.
func (r *Reconciler) Shutdown() {
	if r.cancel == nil {
		return
	}

	r.cancel()
	<-r.done
}

// Done is closed when the loop has exited: after remove or shutdown.
func (r *Reconciler) Done() <-chan struct{} {
	return r.done
}

// Activated returns the revision Current runs; false without one. Safe from any goroutine; it does
// not wait for the loop.
func (r *Reconciler) Activated() (model.Revision, bool) {
	rev := r.activated.Load()
	if rev == nil {
		return model.Revision{}, false
	}

	return *rev, true
}

// Report returns a snapshot of the package's Report. Safe from any goroutine.
func (r *Reconciler) Report() model.Report {
	return r.reporter.Snapshot()
}
