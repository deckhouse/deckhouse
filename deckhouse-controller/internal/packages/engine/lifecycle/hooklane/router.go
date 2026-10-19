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

package hooklane

import (
	"context"
	"log/slog"
	"sync"

	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	kemtypes "github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Router starts the lanes of packages and of Global and delivers the events of shell-operator's
// shared kube events and schedule managers to them (Serve reads them); one per engine. Start
// registers a package's lanes and their stop unregisters them first, so an entry never outlives its
// lanes, and a delivery that races the stop finds the lane closed. Safe for concurrent use.
type Router struct {
	mu       sync.RWMutex
	lanes    map[string]*Lanes
	pauser   pauser
	failures failureRecorder
	clock    retry.Clock
	logger   *log.Logger
	// lanesLogger is the parent of every Lanes' logger.
	lanesLogger *log.Logger
}

// NewRouter returns an empty Router; clock nil means the wall clock.
func NewRouter(pauser pauser, failures failureRecorder, clock retry.Clock, logger *log.Logger) *Router {
	return &Router{
		lanes:       make(map[string]*Lanes),
		pauser:      pauser,
		failures:    failures,
		clock:       retry.OrWall(clock),
		logger:      logger.Named("hook-router"),
		lanesLogger: logger.Named("hook-lanes"),
	}
}

// Serve reads the kube events and schedule managers' channels and delivers every event to the
// lanes until done is closed. One goroutine serves every package and Global, so event intake is
// serial; it stays short because a delivery only appends to a lane and never waits for a hook.
// The managers send without a timeout and a run in flight may be one of them (a synchronization's
// unlock flushes its buffered events, a binding action recreates a monitor): Serve must run until
// every lane has stopped, so the engine closes done only after the core's shutdown and the
// managers' stop. The schedule manager's channel is drained only here, so Serve starts before it.
//
// Routing needs no cancellation: matching an event builds its binding contexts and reads no cache,
// so the routes get a background context.
func (r *Router) Serve(done <-chan struct{}, kube <-chan kemtypes.KubeEvent, schedule <-chan string) {
	ctx := context.Background()

	for {
		select {
		case <-done:
			r.logger.Info("stop the hook event loop")
			return

		case crontab := <-schedule:
			// Debug: events are the hot path of every hook; a failed run is logged by its lane.
			r.logger.Debug("route schedule event", slog.String("crontab", crontab))
			r.RouteSchedule(ctx, crontab)

		case kubeEvent := <-kube:
			r.logger.Debug("route kube event", slog.String("kube_event", kubeEvent.String()))
			r.RouteKubeEvent(ctx, kubeEvent)
		}
	}
}

// register routes the events of l's package to l, replacing a previous registration of the name.
func (r *Router) register(l *Lanes) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lanes[l.name] = l
}

// unregister stops routing to l; a later registration of the same name is kept.
func (r *Router) unregister(l *Lanes) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lanes[l.name] == l {
		delete(r.lanes, l.name)
	}
}

// RouteKubeEvent delivers kubeEvent to the lane of every Kubernetes binding that handles it, as
// one item per binding execution. Never blocks on a hook.
func (r *Router) RouteKubeEvent(ctx context.Context, kubeEvent kemtypes.KubeEvent) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, l := range r.lanes {
		l.route(shtypes.OnKubernetesEvent, KindEvent, func(ctrl *hookcontroller.HookController, deliver func(hookcontroller.BindingExecutionInfo)) {
			if ctrl.CanHandleKubeEvent(kubeEvent) {
				ctrl.HandleKubeEvent(ctx, kubeEvent, deliver)
			}
		})
	}
}

// RouteSchedule delivers a crontab tick to the lane of every schedule binding on that crontab.
func (r *Router) RouteSchedule(ctx context.Context, crontab string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, l := range r.lanes {
		l.route(shtypes.Schedule, KindSchedule, func(ctrl *hookcontroller.HookController, deliver func(hookcontroller.BindingExecutionInfo)) {
			if ctrl.CanHandleScheduleEvent(crontab) {
				ctrl.HandleScheduleEvent(ctx, crontab, deliver)
			}
		})
	}
}

// route asks every hook of binding type whether it handles the event (handle) and delivers what it
// builds. Registered lanes always have controllers: Start initializes before it registers and the
// stop unregisters before it disables.
func (l *Lanes) route(binding shtypes.BindingType, kind Kind, handle func(*hookcontroller.HookController, func(hookcontroller.BindingExecutionInfo))) {
	for _, hook := range l.object.GetHooksByBinding(binding) {
		ctrl := hook.GetHookController()
		if ctrl == nil {
			// One Current per name has its hooks started at a time, and a module's Go hooks are
			// SDK-registry singletons shared by every load: another load disabled them under these
			// lanes. Skipped, not dereferenced, because this goroutine serves every package.
			l.logger.Error("routed hook has no controller: hooks of one package started twice",
				slog.String("hook", hook.GetName()))

			continue
		}

		handle(ctrl, func(info hookcontroller.BindingExecutionInfo) {
			it := eventItem(hook.GetName(), kind, info)
			if !l.deliver(it.queue, it) {
				l.logger.Debug("event dropped: no open lane", slog.String("hook", it.hook), slog.String("queue", it.queue))
			}
		})
	}
}

// Dump returns the state of every package's lanes, or of name's when it is set.
func (r *Router) Dump(name string) Dump {
	r.mu.RLock()
	defer r.mu.RUnlock()

	d := Dump{Lanes: make(map[string]map[string]LaneDump)}

	for owner, l := range r.lanes {
		if name == "" || owner == name {
			d.Lanes[owner] = l.dump()
		}
	}

	return d
}
