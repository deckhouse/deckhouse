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

package hooklane_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	addonhooks "github.com/flant/addon-operator/pkg/module_manager/models/hooks"
	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	shconfig "github.com/flant/shell-operator/pkg/hook/config"
	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"
	smtypes "github.com/flant/shell-operator/pkg/schedule_manager/types"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const waitTimeout = 5 * time.Second

// calls is an ordered call log shared by the fakes; wait returns once an entry appears.
type calls struct {
	mu      sync.Mutex
	entries []string
	changed chan struct{}
}

func newCalls() *calls { return &calls{changed: make(chan struct{})} }

func (c *calls) add(entry string) {
	c.mu.Lock()
	c.entries = append(c.entries, entry)
	close(c.changed)
	c.changed = make(chan struct{})
	c.mu.Unlock()
}

func (c *calls) list() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.entries)
}

func (c *calls) count(entry string) int {
	n := 0
	for _, e := range c.list() {
		if e == entry {
			n++
		}
	}

	return n
}

// wait waits until cond holds over the log; false after waitTimeout.
func (c *calls) wait(cond func([]string) bool) bool {
	timer := time.NewTimer(waitTimeout)
	defer timer.Stop()

	for {
		c.mu.Lock()
		ok := cond(c.entries)
		ch := c.changed
		c.mu.Unlock()

		if ok {
			return true
		}

		select {
		case <-ch:
		case <-timer.C:
			return false
		}
	}
}

// waitFor waits until entry was recorded.
func (c *calls) waitFor(entry string) bool {
	return c.wait(func(e []string) bool { return slices.Contains(e, entry) })
}

// clock is a fake retry.Clock: time moves only by Advance; armed timers are recorded in calls
// ("armed:<delay>") so a test waits for them instead of sleeping.
type clock struct {
	calls *calls

	mu     sync.Mutex
	now    time.Time
	timers []*timer
}

func newClock(c *calls) *clock {
	return &clock{calls: c, now: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

func (c *clock) NewTimer(d time.Duration) retry.Timer {
	t := &timer{clock: c, ch: make(chan time.Time, 1)}
	t.Reset(d)

	return t
}

// Advance moves the time by d and fires every timer due.
func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now

	var due []*timer

	for _, t := range c.timers {
		if t.armed && !t.at.After(now) {
			t.armed = false
			due = append(due, t)
		}
	}
	c.mu.Unlock()

	for _, t := range due {
		select {
		case t.ch <- now:
		default:
		}
	}
}

type timer struct {
	clock *clock
	ch    chan time.Time
	at    time.Time
	armed bool
}

func (t *timer) C() <-chan time.Time { return t.ch }

func (t *timer) Reset(d time.Duration) bool {
	c := t.clock

	c.mu.Lock()
	was := t.armed
	t.at = c.now.Add(d)
	t.armed = true

	if !slices.Contains(c.timers, t) {
		c.timers = append(c.timers, t)
	}

	fire := d <= 0
	if fire {
		t.armed = false
	}
	c.mu.Unlock()

	if fire {
		select {
		case t.ch <- c.Now():
		default:
		}
	}

	c.calls.add(fmt.Sprintf("armed:%s", d))

	return was
}

func (t *timer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()

	was := t.armed
	t.armed = false

	return was
}

// kubeEvents is a kube events manager without a cluster: monitors are accepted (rejected when
// fail) and never exist.
type kubeEvents struct {
	calls *calls
	fail  bool
}

func (k *kubeEvents) AddMonitor(cfg *kubeeventsmanager.MonitorConfig) error {
	if k.fail {
		return errors.New("monitor rejected")
	}

	k.calls.add("add-monitor:" + cfg.Metadata.MonitorId)

	return nil
}
func (k *kubeEvents) HasMonitor(string) bool                      { return false }
func (k *kubeEvents) GetMonitor(string) kubeeventsmanager.Monitor { return nil }
func (k *kubeEvents) StartMonitor(string)                         {}
func (k *kubeEvents) StopMonitor(string) error                    { return nil }
func (k *kubeEvents) Ch() chan kemtypes.KubeEvent                 { return nil }

// schedules accepts schedule entries; nothing fires.
type schedules struct{}

func (schedules) Add(smtypes.ScheduleEntry)    {}
func (schedules) Remove(smtypes.ScheduleEntry) {}

// hook is a hook with a real controller; settings is its execution rate.
type hook struct {
	name     string
	ctrl     *hookcontroller.HookController
	settings *shtypes.Settings
}

func (h hook) GetName() string                                   { return h.name }
func (h hook) GetHookController() *hookcontroller.HookController { return h.ctrl }

func (h hook) GetHookConfig() *addonhooks.ModuleHookConfig {
	return &addonhooks.ModuleHookConfig{HookConfig: shconfig.HookConfig{Settings: h.settings}}
}

// binding describes one Kubernetes binding of a fake hook.
type binding struct {
	queue    string
	wait     bool
	noSync   bool // noSync: executeHookOnSynchronization false
	group    string
	allowErr bool
}

// kubeHook is a hook with Kubernetes bindings over a cluster-less events manager; its monitor ids
// are "<name>-<i>".
func kubeHook(c *calls, name string, bindings ...binding) hook {
	return kubeHookOver(&kubeEvents{calls: c}, name, bindings...)
}

// kubeHookOver is kubeHook over events.
func kubeHookOver(events *kubeEvents, name string, bindings ...binding) hook {
	configs := make([]shtypes.OnKubernetesEventConfig, 0, len(bindings))

	for i, b := range bindings {
		cfg := &kubeeventsmanager.MonitorConfig{}
		cfg.Metadata.MonitorId = fmt.Sprintf("%s-%d", name, i)

		configs = append(configs, shtypes.OnKubernetesEventConfig{
			CommonBindingConfig:          shtypes.CommonBindingConfig{BindingName: fmt.Sprintf("%s-%d", name, i), AllowFailure: b.allowErr},
			Monitor:                      cfg,
			Queue:                        b.queue,
			Group:                        b.group,
			ExecuteHookOnSynchronization: !b.noSync,
			WaitForSynchronization:       b.wait,
		})
	}

	ctrl := hookcontroller.NewHookController()
	ctrl.InitKubernetesBindings(configs, events, log.NewNop())

	return hook{name: name, ctrl: ctrl}
}

// scheduleHook is a hook with one schedule binding on crontab in queue.
func scheduleHook(name, queue, crontab string) hook {
	ctrl := hookcontroller.NewHookController()
	ctrl.InitScheduleBindings([]shtypes.ScheduleConfig{{
		CommonBindingConfig: shtypes.CommonBindingConfig{BindingName: name},
		ScheduleEntry:       smtypes.ScheduleEntry{Crontab: crontab, Id: name},
		Queue:               queue,
	}}, schedules{})

	return hook{name: name, ctrl: ctrl}
}

// object is a package object whose hooks run run: "run:<hook>:<ids>" is recorded before run is
// called, with ids the events' object ids ("sync" for a synchronization).
type object struct {
	name     string
	calls    *calls
	kube     []hooks.ControllableHook
	schedule []hooks.ControllableHook
	queues   []string

	mu       sync.Mutex
	checksum string
	run      func(ctx context.Context, hook string, contexts []bctx.BindingContext) error
}

func (o *object) GetName() string { return o.name }

func (o *object) GetValuesChecksum() string {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.checksum
}

// setChecksum is what a hook that patches values does.
func (o *object) setChecksum(sum string) {
	o.mu.Lock()
	o.checksum = sum
	o.mu.Unlock()
}

func (o *object) InitializeHooks() {
	o.calls.add("initialize")
}

func (o *object) DisableHooks() {
	o.calls.add("disable-hooks")
}

func (o *object) GetHooksByBinding(binding shtypes.BindingType) []hooks.ControllableHook {
	switch binding {
	case shtypes.OnKubernetesEvent:
		return o.kube
	case shtypes.Schedule:
		return o.schedule
	default:
		return nil
	}
}

func (o *object) GetHooksQueues() []string { return o.queues }

func (o *object) RunHookByName(ctx context.Context, hook string, contexts []bctx.BindingContext) error {
	o.calls.add("run:" + hook + ":" + ids(contexts))

	if o.run == nil {
		return nil
	}

	return o.run(ctx, hook, contexts)
}

func (o *object) UnlockKubernetesMonitors(hook string, monitors ...string) {
	o.calls.add("unlock:" + hook + ":" + strings.Join(monitors, ","))
}

// ids names a run's contexts: the object id of an event, "sync" or "schedule".
func ids(contexts []bctx.BindingContext) string {
	out := make([]string, 0, len(contexts))

	for _, c := range contexts {
		switch {
		case c.Type == kemtypes.TypeSynchronization:
			out = append(out, "sync")
		case len(c.Objects) > 0:
			out = append(out, fmt.Sprint(c.Objects[0].FilterResult))
		default:
			out = append(out, "schedule")
		}
	}

	return strings.Join(out, ",")
}

// event is a Kubernetes Added event of monitor carrying object id.
func event(monitor, id string) kemtypes.KubeEvent {
	return kemtypes.KubeEvent{
		MonitorId:   monitor,
		Type:        kemtypes.TypeEvent,
		WatchEvents: []kemtypes.WatchEventType{kemtypes.WatchEventAdded},
		Objects:     []kemtypes.ObjectAndFilterResult{{FilterResult: id}},
	}
}

// failures records the observation store's hook-failure calls.
type failures struct{ calls *calls }

func (f failures) RecordHookFailure(_, hook, reason, _ string) {
	f.calls.add("record-failure:" + hook + ":" + reason)
}
func (f failures) ClearHookFailure(_, hook string) { f.calls.add("clear-failure:" + hook) }
func (f failures) ResetHookFailure(name string)    { f.calls.add("reset-failures:" + name) }

// pauser records the drift monitor's pauses.
type pauser struct{ calls *calls }

func (p pauser) PauseMonitor(name string)  { p.calls.add("pause:" + name) }
func (p pauser) ResumeMonitor(name string) { p.calls.add("resume:" + name) }

// logs is a logger's output, safe to read while the logger writes; every write is recorded in calls
// ("logged") so a test can wait for it.
type logs struct {
	calls *calls

	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logs) Write(p []byte) (int, error) {
	l.mu.Lock()
	n, err := l.buf.Write(p)
	l.mu.Unlock()

	l.calls.add("logged")

	return n, err
}

func (l *logs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.buf.String()
}
