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

// The items of a lane and how one runs.

package hooklane

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	bctx "github.com/flant/shell-operator/pkg/hook/binding_context"
	hookcontroller "github.com/flant/shell-operator/pkg/hook/controller"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const tracer = "hook-lane"

// Kind is what brought an item into its lane.
type Kind string

const (
	KindEvent    Kind = "Event"    // KindEvent is a Kubernetes event of a binding.
	KindSchedule Kind = "Schedule" // KindSchedule is a crontab tick.
	KindSync     Kind = "Sync"     // KindSync is a binding's synchronization at Start.
)

// item is one run of a hook waiting in a lane.
type item struct {
	hook  string
	queue string
	kind  Kind
	// contexts are compacted by group as they are added (appendCompact): the run gets them as is.
	contexts []bctx.BindingContext
	// allowFailure: a failure is logged and counted in the lane's stats, and the item leaves the
	// lane as if it had succeeded.
	allowFailure bool
	delivered    time.Time
	seq          uint64 // seq is the lane's number of the item.

	// Of a synchronization: the binding (its monitor, executeHookOnSynchronization) and, when Start
	// waits for it, done, closed once the item left the lane.
	binding *hookcontroller.BindingExecutionInfo
	done    chan struct{}
	// parent is the span of the Start that enabled the binding: its run is that span's child, so
	// one trace shows a package's startup with its synchronizations.
	parent trace.SpanContext
}

// syncItem is the synchronization of one binding of hook; parent is the span of the Start.
func syncItem(hook string, info hookcontroller.BindingExecutionInfo, parent trace.SpanContext) *item {
	it := &item{
		hook:     hook,
		queue:    info.QueueName,
		kind:     KindSync,
		contexts: appendCompact(nil, info.BindingContext...),
		binding:  &info,
		parent:   parent,

		allowFailure: info.AllowFailure,
	}

	if info.KubernetesBinding.WaitForSynchronization {
		it.done = make(chan struct{})
	}

	return it
}

// eventItem is the run of hook for an event or a schedule tick.
func eventItem(hook string, kind Kind, info hookcontroller.BindingExecutionInfo) *item {
	return &item{
		hook:         hook,
		queue:        info.QueueName,
		kind:         kind,
		contexts:     appendCompact(nil, info.BindingContext...),
		allowFailure: info.AllowFailure,
	}
}

// key is the item's identity in its lane: what the retry of a failed head is keyed on.
func (it *item) key() string {
	return strconv.FormatUint(it.seq, 10)
}

// runsHook reports whether running it runs the hook: not a synchronization that only unlocks.
func (it *item) runsHook() bool {
	return it.kind != KindSync || it.binding.KubernetesBinding.ExecuteHookOnSynchronization
}

// monitor is the monitor a synchronization unlocks.
func (it *item) monitor() string {
	return it.binding.KubernetesBinding.Monitor.Metadata.MonitorId
}

// bindings are the names of the item's binding contexts, in order and without repeats.
func (it *item) bindings() []string {
	var names []string

	for _, c := range it.contexts {
		if len(names) == 0 || names[len(names)-1] != c.Binding {
			names = append(names, c.Binding)
		}
	}

	return names
}

// leave closes done, once: the item left its lane (ran, or the lane closed).
func (it *item) leave() {
	if it.done != nil {
		close(it.done)
		it.done = nil
	}
}

// exec runs it: the hook (unless a synchronization does not execute it), then, for a
// synchronization, the unlock of its monitor. A failure is recorded in the observations unless ctx
// ended (the lane stops, which is no failure of the hook); one the item allows (allowFailure) is
// returned as an allowedFailure, and the item counts as done.
func (l *Lanes) exec(ctx context.Context, it *item) error {
	var allowed error

	logger := l.logger.With(slog.String("hook", it.hook), slog.String("kind", string(it.kind)))

	if !it.runsHook() {
		l.object.UnlockKubernetesMonitors(it.hook, it.monitor())
		return nil
	}

	reason := observed.HookReasonEvent
	if it.kind == KindSync {
		reason = observed.HookReasonSync
	}

	if err := l.run(ctx, it); err != nil {
		if !it.allowFailure {
			if ctx.Err() == nil {
				l.router.failures.RecordHookFailure(l.name, it.hook, reason, err.Error())
			}

			return fmt.Errorf("run hook '%s': %w", it.hook, err)
		}

		logger.Warn("hook failed, allowed", log.Err(err))

		allowed = allowedFailure{err: err}
	} else {
		l.router.failures.ClearHookFailure(l.name, it.hook)
	}

	if it.kind == KindSync {
		l.object.UnlockKubernetesMonitors(it.hook, it.monitor())
	}

	return allowed
}

// allowedFailure is a failure of an item that allows it: the item leaves its lane as if it ran.
type allowedFailure struct{ err error }

func (f allowedFailure) Error() string { return f.err.Error() }

func (f allowedFailure) Unwrap() error { return f.err }

// run runs the hook with the item's contexts, the drift monitor paused, and calls changed when the
// hook changed the values checksum: a synchronization too, which may run beside the run that read
// the values.
func (l *Lanes) run(ctx context.Context, it *item) error {
	if it.parent.IsValid() {
		ctx = trace.ContextWithSpanContext(ctx, it.parent)
	}

	ctx, span := otel.Tracer(tracer).Start(ctx, "RunHook")
	defer span.End()

	span.SetAttributes(attribute.String("package", l.name), attribute.String("hook", it.hook),
		attribute.String("kind", string(it.kind)))

	l.router.pauser.PauseMonitor(l.name)
	defer l.router.pauser.ResumeMonitor(l.name)

	before := l.object.GetValuesChecksum()

	if err := l.object.RunHookByName(ctx, it.hook, it.contexts); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if before != l.object.GetValuesChecksum() {
		l.changed()
	}

	return nil
}
