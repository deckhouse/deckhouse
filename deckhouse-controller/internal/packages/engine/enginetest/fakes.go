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

package enginetest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	shtypes "github.com/flant/shell-operator/pkg/hook/types"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/hooklane"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
)

// Faults programs the fakes' failures and blocks, keyed by the call's record key (for example
// "attach:app@v1.0.0", "upgrade:app", "hook:app:BeforeHelm"; see each fake). Safe for concurrent use.
type Faults struct {
	kit *Kit

	mu   sync.Mutex
	errs map[string]*fault
}

type fault struct {
	err   error
	times int           // times > 0 fails that many calls, then the fault is gone; 0 fails always.
	block chan struct{} // block, when set, holds the call until closed or the call's ctx ends.
}

// Fail makes every call of key fail with err until Clear.
func (f *Faults) Fail(key string, err error) {
	f.set(key, func(ft *fault) { ft.err, ft.times = err, 0 })
}

// FailTimes makes the next n calls of key fail with err.
func (f *Faults) FailTimes(key string, n int, err error) {
	f.set(key, func(ft *fault) { ft.err, ft.times = err, n })
}

// Block holds every call of key until the returned release is called (or the call is cancelled).
func (f *Faults) Block(key string) func() {
	ch := make(chan struct{})
	f.set(key, func(ft *fault) { ft.block = ch })

	var once sync.Once

	return func() {
		once.Do(func() {
			close(ch)
			f.set(key, func(ft *fault) { ft.block = nil })
		})
	}
}

// Clear removes every fault of key.
func (f *Faults) Clear(key string) {
	f.mu.Lock()
	delete(f.errs, key)
	f.mu.Unlock()
}

func (f *Faults) set(key string, change func(*fault)) {
	f.mu.Lock()
	defer f.mu.Unlock()

	ft, ok := f.errs[key]
	if !ok {
		ft = &fault{}
		f.errs[key] = ft
	}

	change(ft)
}

// hit records key and applies its fault: block first, then the error.
func (f *Faults) hit(ctx context.Context, key string) error {
	f.kit.record(key)

	f.mu.Lock()
	ft := f.errs[key]

	var (
		block chan struct{}
		err   error
	)

	if ft != nil {
		block = ft.block

		if ft.err != nil {
			err = ft.err
			if ft.times > 0 {
				ft.times--
				if ft.times == 0 {
					ft.err = nil
				}
			}
		}
	}
	f.mu.Unlock()

	if block != nil {
		f.kit.record("blocked:" + key)

		select {
		case <-block:
		case <-ctx.Done():
			f.kit.record("cancelled:" + key)
			return ctx.Err()
		}
	}

	return err
}

// Disk is the fake disk backend: attach and detach by instance (name@version), with faults.
type Disk struct {
	kit *Kit

	mu       sync.Mutex
	attached map[string]int // attached counts the mounts per instance.
}

// Attach records "attach:<instance>".
func (d *Disk) Attach(ctx context.Context, _ registry.Remote, instance, _, _ string) error {
	if err := d.kit.Faults.hit(ctx, "attach:"+instance); err != nil {
		return err
	}

	d.mu.Lock()
	d.attached[instance]++
	d.mu.Unlock()

	return nil
}

// Detach records "detach:<instance>".
func (d *Disk) Detach(ctx context.Context, instance string) error {
	key := "detach:" + instance

	if err := d.kit.Faults.hit(ctx, key); err != nil {
		return err
	}

	d.mu.Lock()
	if d.attached[instance] > 0 {
		d.attached[instance]--
	}

	if d.attached[instance] == 0 {
		delete(d.attached, instance)
	}
	d.mu.Unlock()

	return nil
}

// Attached returns the attached instances, sorted.
func (d *Disk) Attached() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	out := make([]string, 0, len(d.attached))
	for instance := range d.attached {
		out = append(out, instance)
	}

	slices.Sort(out)

	return out
}

// Loader serves Specs by name and version; an unknown version loads the zero Spec.
type Loader struct {
	kit *Kit

	mu      sync.Mutex
	specs   map[string]Spec       // specs by name@version, or by name for every version.
	objects map[string][]*Package // objects are every object loaded, by name.
}

// Set registers the spec of name at version; version "" applies to every version without its own.
func (l *Loader) Set(name, version string, spec Spec) {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := name
	if version != "" {
		key += "@" + version
	}

	l.specs[key] = spec
}

// Load is the lifecycle.LoadFunc; it records "load:<name>@<version>".
func (l *Loader) Load(ctx context.Context, kind model.Kind, image model.Image) (model.Package, error) {
	if err := l.kit.Faults.hit(ctx, "load:"+image.Name+"@"+image.Version); err != nil {
		return nil, err
	}

	l.mu.Lock()
	spec, ok := l.specs[image.Name+"@"+image.Version]
	if !ok {
		spec = l.specs[image.Name]
	}
	l.mu.Unlock()

	obj := newPackage(l.kit, kind, image, spec)

	l.mu.Lock()
	l.objects[image.Name] = append(l.objects[image.Name], obj)
	l.mu.Unlock()

	return obj, nil
}

// Latest returns the object last loaded for name; nil when none was.
func (l *Loader) Latest(name string) *Package {
	l.mu.Lock()
	defer l.mu.Unlock()

	objs := l.objects[name]
	if len(objs) == 0 {
		return nil
	}

	return objs[len(objs)-1]
}

// Nelm is the fake release service: upgrades and deletes with faults, drift monitors, and the
// ApplyingManifests callback when the values changed since the last upgrade of the name.
type Nelm struct {
	kit *Kit

	mu       sync.Mutex
	released map[string]string // released is the values checksum of each installed release.
	drifted  map[string]func()
	monitors map[string]bool // monitors: name → paused.
}

// Upgrade records "upgrade:<name>" (fault key) and "upgraded:<name>@<version>" on success.
func (n *Nelm) Upgrade(ctx context.Context, _ string, pkg nelm.Package, applying, drifted func()) error {
	name := pkg.GetName()
	sum := pkg.(interface{ GetValuesChecksum() string }).GetValuesChecksum()

	n.mu.Lock()
	changed := n.released[name] != sum
	n.mu.Unlock()

	if changed && applying != nil {
		applying()
	}

	if err := n.kit.Faults.hit(ctx, "upgrade:"+name); err != nil {
		return err
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	n.mu.Lock()
	n.released[name] = sum
	n.drifted[name] = drifted
	if _, ok := n.monitors[name]; !ok {
		n.monitors[name] = false
	}
	n.mu.Unlock()

	version := ""
	if p, ok := pkg.(*Package); ok {
		version = p.version.Original()
	}

	n.kit.record("upgraded:" + name + "@" + version)

	return nil
}

// Delete records "delete:<name>"; a missing release is no error.
func (n *Nelm) Delete(ctx context.Context, _ string, name string) error {
	if err := n.kit.Faults.hit(ctx, "delete:"+name); err != nil {
		return err
	}

	n.mu.Lock()
	delete(n.released, name)
	n.mu.Unlock()

	return nil
}

// HasMonitor reports whether name has a drift monitor.
func (n *Nelm) HasMonitor(name string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	_, ok := n.monitors[name]

	return ok
}

// PauseMonitor pauses name's monitor.
func (n *Nelm) PauseMonitor(name string) { n.setMonitor(name, true) }

// ResumeMonitor resumes name's monitor.
func (n *Nelm) ResumeMonitor(name string) { n.setMonitor(name, false) }

func (n *Nelm) setMonitor(name string, paused bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, ok := n.monitors[name]; ok {
		n.monitors[name] = paused
	}
}

// RemoveMonitor records "remove-monitor:<name>".
func (n *Nelm) RemoveMonitor(name string) {
	n.mu.Lock()
	delete(n.monitors, name)
	delete(n.drifted, name)
	n.mu.Unlock()

	n.kit.record("remove-monitor:" + name)
}

// Installed reports whether name has a release.
func (n *Nelm) Installed(name string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	_, ok := n.released[name]

	return ok
}

// Drift fires name's drift callback, as the monitor does when release resources are gone. False
// when name has no monitor or it is paused.
func (n *Nelm) Drift(name string) bool {
	n.mu.Lock()
	paused, ok := n.monitors[name]
	cb := n.drifted[name]
	n.mu.Unlock()

	if !ok || paused || cb == nil {
		return false
	}

	cb()

	return true
}

// Hooks fakes hooklane.Router.Start: start (with a fault), its stop, and the runtime hooks'
// changed callback, plus the per-binding programs of the package objects.
type Hooks struct {
	kit *Kit

	mu      sync.Mutex
	changed map[string]func()
	active  map[string]hooklane.Object
	// settingsErr makes ValidateSettings of a name return an error (a transient failure).
	settingsErr map[string]error
	// writes are values a binding writes into the object when it runs, by name and binding.
	writes map[string]map[string]any
	// syncWrites are values the synchronizations of name write when its hooks start (SyncWrite).
	syncWrites map[string]map[string]any
}

// Start records "enable-hooks:<name>" (fault key) and keeps changed for RuntimeChange. A failed
// enable stops what it started, as the real one does; the stop it returns is idempotent. The
// synchronizations' writes (SyncWrite) land and report the change, as a hook lane does.
func (h *Hooks) Start(ctx, _ context.Context, pkg hooklane.Object, changed func()) (func(), error) {
	pkg.InitializeHooks()

	if err := h.kit.Faults.hit(ctx, "enable-hooks:"+pkg.GetName()); err != nil {
		h.disable(pkg)
		return nil, err
	}

	h.mu.Lock()
	h.changed[pkg.GetName()] = changed
	h.active[pkg.GetName()] = pkg
	writes := h.syncWrites[pkg.GetName()]
	h.mu.Unlock()

	if len(writes) > 0 {
		if p, ok := pkg.(interface{ SetValue(string, any) }); ok {
			for key, value := range writes {
				p.SetValue(key, value)
			}
		}

		h.kit.record("sync-change:" + pkg.GetName())
		changed()
	}

	var once sync.Once

	return func() { once.Do(func() { h.disable(pkg) }) }, nil
}

// disable records "disable:<name>" and disables the object's hooks; changed is dropped.
func (h *Hooks) disable(pkg hooklane.Object) {
	h.mu.Lock()
	delete(h.changed, pkg.GetName())
	delete(h.active, pkg.GetName())
	h.mu.Unlock()

	h.kit.record("disable:" + pkg.GetName())
	pkg.DisableHooks()
	h.kit.Observed.reset(pkg.GetName())
}

// Running reports whether name's hooks are enabled.
func (h *Hooks) Running(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	_, ok := h.active[name]

	return ok
}

// RuntimeChange runs a runtime hook of name that writes key=value and reports the change, as a
// hook lane does. False when name's hooks are not running.
func (h *Hooks) RuntimeChange(name, key string, value any) bool {
	h.mu.Lock()
	pkg := h.active[name]
	cb := h.changed[name]
	h.mu.Unlock()

	if pkg == nil || cb == nil {
		return false
	}

	if p, ok := pkg.(interface{ SetValue(string, any) }); ok {
		p.SetValue(key, value)
	}

	h.kit.record("runtime-hook:" + name)
	cb()

	return true
}

// RuntimeFailure records a failed runtime hook in the observations, as a hook lane does.
func (h *Hooks) RuntimeFailure(name, hook, message string) {
	h.kit.Observed.record(name, hook, message)
}

// SettingsError makes ValidateSettings of name fail with err (nil clears it).
func (h *Hooks) SettingsError(name string, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err == nil {
		delete(h.settingsErr, name)
		return
	}

	h.settingsErr[name] = err
}

// SyncWrite makes the synchronizations of name write key=value whenever its hooks start.
func (h *Hooks) SyncWrite(name, key string, value any) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.syncWrites[name] == nil {
		h.syncWrites[name] = map[string]any{}
	}

	h.syncWrites[name][key] = value
}

// WriteOn makes binding of name write key=value when it runs (AfterHelm changing values).
func (h *Hooks) WriteOn(name string, binding shtypes.BindingType, key string, value any) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.writes[name+"/"+string(binding)] == nil {
		h.writes[name+"/"+string(binding)] = map[string]any{}
	}

	h.writes[name+"/"+string(binding)][key] = value
}

func (h *Hooks) settingsError(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.settingsErr[name]
}

func (h *Hooks) run(ctx context.Context, p *Package, binding shtypes.BindingType) error {
	if err := h.kit.Faults.hit(ctx, "run:"+p.name+":"+string(binding)); err != nil {
		return err
	}

	h.mu.Lock()
	writes := h.writes[p.name+"/"+string(binding)]
	h.mu.Unlock()

	for k, v := range writes {
		p.SetValue(k, v)
	}

	return nil
}

// Observed fakes the hook-failure part of the observation store.
type Observed struct {
	kit *Kit

	mu       sync.Mutex
	failures map[string]string // failures: name → "hook: message".
}

func (o *Observed) record(name, hook, message string) {
	o.mu.Lock()
	o.failures[name] = hook + ": " + message
	o.mu.Unlock()

	o.kit.record("hook-failure:" + name)
}

func (o *Observed) reset(name string) {
	o.mu.Lock()
	delete(o.failures, name)
	o.mu.Unlock()
}

// HookFailure returns name's recorded hook failure; "" when none.
func (o *Observed) HookFailure(name string) string {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.failures[name]
}

// Orphans fakes the orphan purge of a removed application: records "purge:<name>".
type Orphans struct{ kit *Kit }

// Purge records "purge:<name>" (fault key).
func (o *Orphans) Purge(ctx context.Context, pkg model.Package) error {
	return o.kit.Faults.hit(ctx, "purge:"+pkg.GetName())
}

// CRDs fakes the CRD installer of Global: records "crds:<module>".
type CRDs struct {
	kit *Kit
}

// Install records "crds:<name>" (fault key).
func (c *CRDs) Install(ctx context.Context, name, _ string) error {
	return c.kit.Faults.hit(ctx, "crds:"+name)
}

// HasCRDs reports that no fake module ships CRDs.
func (c *CRDs) HasCRDs(string) error {
	return errors.New("no crds")
}

// GetManagedGVKs returns nothing.
func (c *CRDs) GetManagedGVKs([]string) []string { return nil }

// Webhooks fakes the conversion webhooks of Global: records "webhooks:<module>".
type Webhooks struct{ kit *Kit }

// EnsureWebhooks records "webhooks:<name>" (fault key).
func (w *Webhooks) EnsureWebhooks(ctx context.Context, view model.ModuleView) error {
	return w.kit.Faults.hit(ctx, "webhooks:"+view.Name)
}

// Window is an update window that opens at a set time; the zero value is closed forever.
type Window struct {
	mu    sync.Mutex
	opens time.Time
}

// OpenAt sets the time the window opens; a time in the past opens it now.
func (w *Window) OpenAt(t time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.opens = t
}

// IsAllowed reports whether t is inside the window.
func (w *Window) IsAllowed(t time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	return !w.opens.IsZero() && !t.Before(w.opens)
}

// NextAllowedTime is the opening time; a day ahead while it is unset.
func (w *Window) NextAllowedTime(t time.Time) time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.opens.IsZero() {
		return t.Add(24 * time.Hour)
	}

	return w.opens
}

// recordingScheduler records the graph changes of the real scheduler in the kit's log.
type recordingScheduler struct {
	*schedule.Scheduler

	kit *Kit
}

func (s recordingScheduler) AddNode(node schedule.Node) error {
	err := s.Scheduler.AddNode(node)
	if err != nil {
		s.kit.record(fmt.Sprintf("add-node-rejected:%s@%s", node.Name, node.Revision))
		return err
	}

	s.kit.record(fmt.Sprintf("add-node:%s@%s", node.Name, node.Revision))

	return nil
}

func (s recordingScheduler) RemoveNode(name string) {
	s.Scheduler.RemoveNode(name)
	s.kit.record("remove-node:" + name)
}

// Publish wakes the kit's waiters after the scheduler took the facts: convergence changes only here.
func (s recordingScheduler) Publish(name string, facts schedule.Facts) {
	s.Scheduler.Publish(name, facts)
	s.kit.bump()
}
