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

// Package hooklane runs a package's runtime and synchronization hooks in lanes: one lane per hook
// queue of the package, a FIFO with one goroutine, so the hooks of one queue run one at a time in
// event order and the queues run in parallel, beside the package's operations. The lanes of a
// package live from Start to its stop, which Current (or Global) calls; the Router (Serve) delivers
// the events of shell-operator's shared managers to them and never creates one. Waiting runs of one
// hook coalesce into one run with all their contexts, and a hook's executionMinInterval holds its
// runs meanwhile; a failed head blocks its lane until its retry.
package hooklane

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/hooks"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// stopWarnPeriod is how often a stop that waits for a hook in flight says which.
const stopWarnPeriod = 10 * time.Second

// Object is a package's hooks as the lanes run them: an application, a module or the global module.
type Object interface {
	GetName() string
	GetValuesChecksum() string
	// InitializeHooks creates the hook controllers and binds them to events.
	InitializeHooks()
	// DisableHooks stops the bindings and drops the controllers; called once every lane exited.
	DisableHooks()
	GetHooksByBinding(binding shtypes.BindingType) []hooks.ControllableHook
	// GetHooksQueues are the queues of the schedule and Kubernetes bindings: one lane each.
	GetHooksQueues() []string
	RunHookByName(ctx context.Context, hook string, bctx []bctx.BindingContext) error
	// UnlockKubernetesMonitors lets the events of monitors flow after their synchronization.
	UnlockKubernetesMonitors(hook string, monitors ...string)
}

// pauser pauses the package's release drift monitor while a hook runs (nelm.Service): a hook may
// change release resources, which is not drift. A package without a monitor is a no-op.
type pauser interface {
	PauseMonitor(name string)
	ResumeMonitor(name string)
}

// failureRecorder is the part of the observation store the lanes write: one slot per package,
// recorded by a failed run, cleared by a success and reset by the stop.
type failureRecorder interface {
	RecordHookFailure(name, hook, reason, message string)
	ClearHookFailure(name, hook string)
	ResetHookFailure(name string)
}

// Lanes are the started lanes of one package, by queue.
type Lanes struct {
	name    string
	object  Object
	changed func()
	router  *Router
	logger  *log.Logger

	cancel context.CancelFunc
	lanes  map[string]*lane // lanes is fixed by Start.
	// limiters hold the hooks that set an execution rate, by hook name; fixed by Start.
	limiters map[string]*limiter
	once     sync.Once
}

// Start starts the hooks of pkg and returns the stop that ends them:
//  1. initialize the hook controllers;
//  2. start one lane per hook queue and register them with the router;
//  3. enable the Kubernetes bindings, one hook after another as addon-operator does, and put one
//     synchronization item per binding into its hook's lane in hook order, the bindings that wait
//     for it first;
//  4. enable the schedule bindings;
//  5. wait for the synchronization of the bindings that wait for it, bounded by ctx.
//
// The lanes live under a child of parent, not of ctx: a binding that does not wait is synchronized
// after Start returned. changed is called whenever a hook changed the values checksum; it must not
// block. On an error Start stops what it started; the caller gets no stop.
func (r *Router) Start(ctx, parent context.Context, pkg Object, changed func()) (func(), error) {
	lifetime, cancel := context.WithCancel(parent)

	l := &Lanes{
		name:     pkg.GetName(),
		object:   pkg,
		changed:  changed,
		router:   r,
		logger:   r.lanesLogger.With(slog.String("name", pkg.GetName())),
		cancel:   cancel,
		lanes:    make(map[string]*lane),
		limiters: make(map[string]*limiter),
	}

	if err := l.start(ctx, lifetime); err != nil {
		l.Stop()
		return nil, err
	}

	return l.Stop, nil
}

func (l *Lanes) start(ctx, lifetime context.Context) error {
	l.logger.Debug("initialize package hooks")
	l.object.InitializeHooks()

	for _, binding := range []shtypes.BindingType{shtypes.OnKubernetesEvent, shtypes.Schedule} {
		for _, hook := range l.object.GetHooksByBinding(binding) {
			if s := hooks.ExecutionSettings(hook); s != nil && s.ExecutionMinInterval > 0 {
				l.limiters[hook.GetName()] = newLimiter(s.ExecutionMinInterval, s.ExecutionBurst)
			}
		}
	}

	for _, queue := range l.object.GetHooksQueues() {
		ln := newLane(l, queue)
		l.lanes[queue] = ln

		go ln.loop(lifetime)
	}

	// Routing starts before the bindings: an event of a binding is delivered only after its
	// synchronization unlocked the monitor, so nothing is routed before a lane can run it.
	l.router.register(l)

	syncs, err := l.enableKubernetesHooks(ctx)
	if err != nil {
		return err
	}

	waits := l.deliverSync(syncs)

	for _, hook := range l.object.GetHooksByBinding(shtypes.Schedule) {
		l.logger.Debug("enable schedule hook", slog.String("hook", hook.GetName()))
		hook.GetHookController().EnableScheduleBindings()
	}

	l.logger.Debug("wait for the synchronization", slog.Int("bindings", len(waits)))

	for _, done := range waits {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// enableKubernetes enables the Kubernetes bindings of the hooks one after another and returns one
// synchronization item per binding, in hook order. Not in parallel: enables of many hooks contend in
// shell-operator's factory store (a handler added to a running informer gets its whole store under
// the store's lock), and in parallel every enable took seconds.
func (l *Lanes) enableKubernetesHooks(ctx context.Context) ([]*item, error) {
	parent := trace.SpanContextFromContext(ctx)

	var syncs []*item

	for _, hook := range l.object.GetHooksByBinding(shtypes.OnKubernetesEvent) {
		infos, err := l.enableKubernetesHook(ctx, hook)
		if err != nil {
			return nil, err
		}

		for i := range infos {
			syncs = append(syncs, syncItem(hook.GetName(), infos[i], parent))
		}
	}

	return syncs, nil
}

// enableKubernetesHook enables the Kubernetes bindings of one hook and returns one synchronization per binding
// (its initial snapshot). The span covers what shell-operator does per binding: the kind's
// discovery, the informer's initial list and the wait for its sync.
func (l *Lanes) enableKubernetesHook(ctx context.Context, hook hooks.ControllableHook) ([]hookcontroller.BindingExecutionInfo, error) {
	l.logger.Debug("enable kube hook", slog.String("hook", hook.GetName()))

	ctx, span := otel.Tracer(tracer).Start(ctx, "enableBindings", trace.WithAttributes(
		attribute.String("package", l.name), attribute.String("hook", hook.GetName())))
	defer span.End()

	var infos []hookcontroller.BindingExecutionInfo

	err := hook.GetHookController().HandleEnableKubernetesBindings(ctx, func(info hookcontroller.BindingExecutionInfo) {
		infos = append(infos, info)
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("enable kubernetes bindings of hook '%s': %w", hook.GetName(), err)
	}

	span.SetAttributes(attribute.Int("bindings", len(infos)))

	return infos, nil
}

// deliverSync puts the synchronization items into their lanes, in every lane those that wait
// first, and returns the done channels of those that wait.
func (l *Lanes) deliverSync(syncs []*item) []<-chan struct{} {
	var (
		waiting, rest []*item
		dones         []<-chan struct{}
	)

	for _, it := range syncs {
		if it.done != nil {
			waiting = append(waiting, it)
			dones = append(dones, it.done)
		} else {
			rest = append(rest, it)
		}
	}

	for _, it := range append(waiting, rest...) {
		if !l.deliver(it.queue, it) {
			// A lane missing for a binding's queue would lose its synchronization: its monitor
			// would stay locked.
			l.logger.Error("no lane for the queue of a synchronization", slog.String("hook", it.hook), slog.String("queue", it.queue))
			it.leave()
		}
	}

	return dones
}

// deliver appends it to the lane of queue; false when there is no such lane or it is closed.
func (l *Lanes) deliver(queue string, it *item) bool {
	ln, ok := l.lanes[queue]
	if !ok {
		return false
	}

	return ln.deliver(it)
}

// Stop ends the hooks; idempotent, so also a Start's own failure can call it:
//  1. unregister from the router, so no event is routed any more;
//  2. close every lane, dropping what waits in it;
//  3. cancel the lifetime;
//  4. wait for every lane's goroutine, with no bound: a hook in flight returns before its controller
//     is cleared (a warning every stopWarnPeriod names it);
//  5. disable the bindings and drop the controllers;
//  6. reset the observed hook failure, which nothing records any more.
func (l *Lanes) Stop() {
	l.once.Do(func() {
		l.router.unregister(l)

		for _, ln := range l.lanes {
			ln.close()
		}

		l.cancel()

		for _, ln := range l.lanes {
			l.wait(ln)
		}

		l.object.DisableHooks()
		l.router.failures.ResetHookFailure(l.name)
	})
}

// wait waits for ln's goroutine and logs while a hook keeps it busy.
func (l *Lanes) wait(ln *lane) {
	timer := l.router.clock.NewTimer(stopWarnPeriod)
	defer timer.Stop()

	for {
		select {
		case <-ln.exited:
			return
		case <-timer.C():
			l.logger.Warn("stop waits for a hook in flight", slog.String("queue", ln.queue), slog.String("hook", ln.inFlightHook()))
			timer.Reset(stopWarnPeriod)
		}
	}
}
