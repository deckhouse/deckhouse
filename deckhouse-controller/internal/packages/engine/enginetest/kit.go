// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package enginetest is the scenario kit of the package engine: a real core, scheduler, package
// Reconcilers, Currents and Global over fakes of the disk, the loader, nelm, the hook bindings,
// the orphan purge and Global's CRD and webhook services. Tests only: no production code may
// import it. Every fake call is recorded in one ordered log, and every wait is bounded and woken
// by the engine's own signals (the Report notify and the fakes' calls), never by sleeping.
package enginetest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/core"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/current"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/global"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/hooklane"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// GlobalTimers owns Global's timers on the kit's Clock (AdvanceToTimerOf).
const GlobalTimers = "global"

// Timeout bounds every wait of the kit; retries and windows wait on the kit's Clock, not on it.
const Timeout = 20 * time.Second

// ModulesNamespace is the release namespace of modules in the kit.
const ModulesNamespace = "d8-system"

// GlobalVersion is the version of the fake global module.
const GlobalVersion = "v1.0.0"

// Kit is one engine under test with its fakes.
type Kit struct {
	t testing.TB

	Engine    *core.Engine
	Scheduler *schedule.Scheduler

	// Clock is the engine's clock: retries and windows wait until a test advances it.
	Clock    *Clock
	Faults   *Faults
	Disk     *Disk
	Loader   *Loader
	Nelm     *Nelm
	Hooks    *Hooks
	Observed *Observed
	Orphans  *Orphans
	CRDs     *CRDs
	Webhooks *Webhooks
	// Global is the fake global module; nil without WithGlobal.
	Global *GlobalModule
	// GlobalSettings is Global's boot checkpoint, empty values by default; set it before Boot (nil:
	// Global waits for its first Apply and holds the barrier until then).
	GlobalSettings *model.Settings
	// EmbeddedDir is the image directory of embedded modules.
	EmbeddedDir string

	logMu sync.Mutex
	log   []string

	sigMu sync.Mutex
	sig   chan struct{} // sig is closed and replaced on every change the kit sees.

	histMu  sync.Mutex
	pending []string                  // pending are notified names the collector has not read.
	history map[string][]model.Report // history are the distinct Reports per name, in order.
	notify  chan struct{}
	stopped chan struct{}
	done    chan struct{}

	shutdown sync.Once
}

type options struct {
	global    bool
	scheduler []schedule.Option
}

// Option configures a Kit.
type Option func(*options)

// WithGlobal runs the engine with Global (EnableModuleV2): a global node, the barrier, the boot gate.
func WithGlobal() Option {
	return func(o *options) { o.global = true }
}

// WithScheduler passes options to the real scheduler (version getters, bootstrap condition, ...).
func WithScheduler(opts ...schedule.Option) Option {
	return func(o *options) { o.scheduler = append(o.scheduler, opts...) }
}

// New builds an engine with fakes; Shutdown runs at the test's cleanup.
func New(t testing.TB, opts ...Option) *Kit {
	t.Helper()

	var o options
	for _, opt := range opts {
		opt(&o)
	}

	k := &Kit{
		t:           t,
		EmbeddedDir: filepath.Join(t.TempDir(), "modules"),
		sig:         make(chan struct{}),
		history:     make(map[string][]model.Report),
		notify:      make(chan struct{}, 1),
		stopped:     make(chan struct{}),
		done:        make(chan struct{}),
	}

	k.Clock = newClock(k.bump)
	k.Faults = &Faults{kit: k, errs: make(map[string]*fault)}
	k.Disk = &Disk{kit: k, attached: make(map[string]int)}
	k.Loader = &Loader{kit: k, specs: make(map[string]Spec), objects: make(map[string][]*Package)}
	k.Nelm = &Nelm{kit: k, released: make(map[string]string), drifted: make(map[string]func()), monitors: make(map[string]bool)}
	k.Hooks = &Hooks{
		kit: k, changed: make(map[string]func()), active: make(map[string]hooklane.Object),
		settingsErr: make(map[string]error), writes: make(map[string]map[string]any),
	}
	k.Observed = &Observed{kit: k, failures: make(map[string]string)}
	k.Orphans = &Orphans{kit: k}
	k.CRDs = &CRDs{kit: k}
	k.Webhooks = &Webhooks{kit: k}

	if o.global {
		k.Global = &GlobalModule{Package: newPackage(k, model.KindModule,
			model.Image{Name: schedule.GlobalNode, Version: GlobalVersion, Path: "global"}, Spec{})}
		k.GlobalSettings = &model.Settings{Values: addonutils.Values{}}
	}

	logger := log.NewNop()
	k.Scheduler = schedule.NewScheduler(logger, o.scheduler...)

	dir := t.TempDir()
	deps := core.Deps{
		Reconciler: lifecycle.Deps{
			Disks: lifecycle.Disks{
				Apps:        lifecycle.Disk{Backend: k.Disk, Dir: filepath.Join(dir, "apps")},
				Modules:     lifecycle.Disk{Backend: k.Disk, Dir: filepath.Join(dir, "modules")},
				EmbeddedDir: k.EmbeddedDir,
			},
			Load: k.Loader.Load,
			Current: current.Deps{
				Nelm:             k.Nelm,
				Hooks:            k.Hooks,
				Orphans:          k.Orphans,
				ModulesNamespace: ModulesNamespace,
				Clock:            k.Clock,
				Logger:           logger,
			},
			Releases: k.Nelm,
			Clock:    k.Clock,
			Logger:   logger,
		},
		Global: global.Deps{
			Hooks:    k.Hooks,
			CRDs:     k.CRDs,
			Webhooks: k.Webhooks,
			Clock:    k.Clock.Owned(GlobalTimers),
			Logger:   logger,
		},
	}

	k.Engine = core.New(core.Config{Notify: k.notified}, recordingScheduler{Scheduler: k.Scheduler, kit: k}, deps)

	go k.collect()

	t.Cleanup(k.Shutdown)

	return k
}

// Boot boots the engine with packages (and Global with WithGlobal). Without Global it resumes the
// scheduler as the default tree's wiring does after addon-operator's converge.
func (k *Kit) Boot(packages ...model.BootPackage) error {
	var bg *core.BootGlobal
	if k.Global != nil {
		bg = &core.BootGlobal{Module: k.Global, Settings: k.GlobalSettings}
	}

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	if err := k.Engine.Boot(ctx, bg, packages); err != nil {
		return err
	}

	if bg == nil {
		k.Engine.Resume()
	}

	return nil
}

// MustBoot is Boot that fails the test on an error.
func (k *Kit) MustBoot(packages ...model.BootPackage) {
	k.t.Helper()

	if err := k.Boot(packages...); err != nil {
		k.t.Fatalf("Boot() = %v", err)
	}
}

// ApplyModule applies desired carrying enabled as its intent, as the facade does.
func (k *Kit) ApplyModule(desired model.Desired, enabled bool) {
	desired.Intent = &enabled
	k.Engine.Apply(desired)
}

// Shutdown stops the engine and the kit; idempotent.
func (k *Kit) Shutdown() {
	k.shutdown.Do(func() {
		k.Engine.Shutdown()
		close(k.stopped)
		<-k.done
	})
}

// Module is the desired state of a downloaded module at version with empty settings.
func Module(name, version string) model.Desired {
	return model.Desired{
		Name:    name,
		Kind:    model.KindModule,
		Version: version,
		Configuration: model.Configuration{
			Settings: model.Settings{Values: addonutils.Values{}},
		},
		Policy: model.Policy{Mode: model.UpdateModeAuto},
		Source: model.Source{Package: name},
	}
}

// Embedded is the desired state of an embedded module at version.
func Embedded(name, version string) model.Desired {
	d := Module(name, version)
	d.Embedded = true

	return d
}

// App is the desired state of the application namespace/name at version.
func App(namespace, name, version string) model.Desired {
	return model.Desired{
		Name:    namespace + "." + name,
		Kind:    model.KindApplication,
		Version: version,
		Configuration: model.Configuration{
			Settings: model.Settings{Values: addonutils.Values{}},
		},
		Policy: model.Policy{Mode: model.UpdateModeAuto},
		Source: model.Source{Package: name},
	}
}

// WithSettings returns desired with settings values.
func WithSettings(desired model.Desired, values addonutils.Values) model.Desired {
	desired.Configuration.Settings = model.Settings{Values: values}

	return desired
}

// Restored is the boot checkpoint of a package installed at version with settings values (nil:
// absent checkpoint).
func Restored(desired model.Desired, settings addonutils.Values) model.BootPackage {
	bp := model.BootPackage{
		Name:     desired.Name,
		Kind:     desired.Kind,
		Version:  desired.Version,
		Source:   desired.Source,
		Embedded: desired.Embedded,
	}

	if settings != nil {
		bp.Settings = &model.Settings{Values: settings}
	}

	return bp
}

// record appends an entry to the log and wakes the waiters.
func (k *Kit) record(entry string) {
	k.logMu.Lock()
	k.log = append(k.log, entry)
	k.logMu.Unlock()

	k.bump()
}

// Log returns the log from mark (Mark) on.
func (k *Kit) Log(mark int) []string {
	k.logMu.Lock()
	defer k.logMu.Unlock()

	return slices.Clone(k.log[mark:])
}

// Mark returns the position of the next log entry.
func (k *Kit) Mark() int {
	k.logMu.Lock()
	defer k.logMu.Unlock()

	return len(k.log)
}

// Count returns how many log entries since mark equal entry.
func (k *Kit) Count(mark int, entry string) int {
	n := 0

	for _, e := range k.Log(mark) {
		if e == entry {
			n++
		}
	}

	return n
}

// Index returns the position (relative to mark) of the first entry equal to entry; -1 if none.
func (k *Kit) Index(mark int, entry string) int {
	return slices.Index(k.Log(mark), entry)
}

func (k *Kit) bump() {
	k.sigMu.Lock()
	close(k.sig)
	k.sig = make(chan struct{})
	k.sigMu.Unlock()
}

func (k *Kit) signal() <-chan struct{} {
	k.sigMu.Lock()
	defer k.sigMu.Unlock()

	return k.sig
}

// Wait waits until cond holds, woken by every change the kit sees; it fails the test after
// Timeout with desc.
func (k *Kit) Wait(desc string, cond func() bool) {
	k.t.Helper()

	if !k.WaitFor(Timeout, cond) {
		k.t.Fatalf("timed out waiting for %s\nlog:\n  %s", desc, strings.Join(k.Log(0), "\n  "))
	}
}

// WaitFor waits up to d for cond; false on timeout. Use it to assert that something does not happen
// within d after the engine had every chance to do it.
func (k *Kit) WaitFor(d time.Duration, cond func() bool) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	for {
		ch := k.signal()
		if cond() {
			return true
		}

		select {
		case <-ch:
		case <-timer.C:
			return cond()
		}
	}
}

// WaitReport waits until name's Report satisfies pred and returns it.
func (k *Kit) WaitReport(name, desc string, pred func(model.Report) bool) model.Report {
	k.t.Helper()

	var last model.Report

	ok := k.WaitFor(Timeout, func() bool {
		r, _ := k.Engine.Report(name)
		last = r

		return pred(r)
	})
	if !ok {
		k.t.Fatalf("timed out waiting for %s: %s\nreport: %s\nlog:\n  %s", name, desc, Dump(last),
			strings.Join(k.Log(0), "\n  "))
	}

	return last
}

// AdvanceToNextTimer waits until an engine timer is armed (a retry's backoff, a window), then moves
// the clock to its deadline, which fires it. It returns the time moved.
func (k *Kit) AdvanceToNextTimer(desc string) time.Duration {
	k.t.Helper()

	var at time.Time

	k.Wait("an armed timer: "+desc, func() bool {
		next, ok := k.Clock.Next()
		at = next

		return ok
	})

	before := k.Clock.Now()
	k.Clock.AdvanceTo(at)

	return at.Sub(before)
}

// AdvanceToTimerOf is AdvanceToNextTimer over the timers of owner (Clock.Owned): another owner's
// timer armed first, at the same deadline, would otherwise be fired before owner's is armed.
func (k *Kit) AdvanceToTimerOf(owner, desc string) time.Duration {
	k.t.Helper()

	var at time.Time

	k.Wait("an armed timer of "+owner+": "+desc, func() bool {
		next, ok := k.Clock.NextOf(owner)
		at = next

		return ok
	})

	before := k.Clock.Now()
	k.Clock.AdvanceTo(at)

	return at.Sub(before)
}

// WaitArmed waits until an engine timer is armed: a failed attempt has been handled and backs off.
// Clear a fault only after it: a fault cleared while the attempt is still in its call may let that
// attempt pass.
func (k *Kit) WaitArmed(desc string) {
	k.t.Helper()
	k.Wait("an armed timer: "+desc, func() bool {
		_, ok := k.Clock.Next()
		return ok
	})
}

// WaitLog waits until entry appears in the log after mark.
func (k *Kit) WaitLog(mark int, entry string) {
	k.t.Helper()
	k.Wait("log entry "+entry, func() bool { return k.Index(mark, entry) >= 0 })
}

// Report returns name's Report.
func (k *Kit) Report(name string) model.Report {
	r, _ := k.Engine.Report(name)
	return r
}

// History returns the distinct Reports of name in the order they were notified (collapsed when
// the collector read several changes at once).
func (k *Kit) History(name string) []model.Report {
	k.histMu.Lock()
	defer k.histMu.Unlock()

	return slices.Clone(k.history[name])
}

// notified is core.Config.Notify: it never calls into the engine (the Reconciler may hold the
// engine's lock); the collector reads the Report.
func (k *Kit) notified(name string) {
	k.histMu.Lock()
	k.pending = append(k.pending, name)
	k.histMu.Unlock()

	select {
	case k.notify <- struct{}{}:
	default:
	}

	k.bump()
}

func (k *Kit) collect() {
	defer close(k.done)

	for {
		select {
		case <-k.stopped:
			return
		case <-k.notify:
		}

		k.histMu.Lock()
		names := k.pending
		k.pending = nil
		k.histMu.Unlock()

		for _, name := range names {
			r, ok := k.Engine.Report(name)
			if !ok {
				continue
			}

			k.histMu.Lock()
			h := k.history[name]
			if len(h) == 0 || Dump(h[len(h)-1]) != Dump(r) {
				k.history[name] = append(h, r)
			}
			k.histMu.Unlock()
		}

		k.bump()
	}
}

// Dump renders a Report for failure messages and comparisons.
func Dump(r model.Report) string {
	data, _ := json.Marshal(r) //nolint:musttag // a test dump of the engine's Report, which carries no JSON contract
	return string(data)
}

// True reports whether c is True.
func True(c model.Condition) bool { return c.Status == model.ConditionTrue }

// False reports whether c is False with reason ("" matches any reason).
func False(c model.Condition, reason string) bool {
	return c.Status == model.ConditionFalse && (reason == "" || c.Reason == reason)
}

// Unknown reports whether c was not reported.
func Unknown(c model.Condition) bool { return c.Status == model.ConditionUnknown }

// Running reports whether r shows version installed and applied: CurrentVersion, the Current
// section of that version with HooksProcessed, ManifestsApplied and Installed True, and no
// candidate.
func Running(r model.Report, version string) bool {
	c := r.Current

	return r.CurrentVersion == version && c.Version == version && True(c.HooksProcessed) &&
		True(c.ManifestsApplied) && True(c.Installed) && r.Candidate == (model.CandidateReport{})
}
