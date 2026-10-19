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

// Package current runs the working revision of one package: its configuration, hooks and release.
// Start with next.go (every rule of Current), then loop.go.
package current

import (
	"context"
	"log/slog"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/inbox"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Reconciler is Current: it runs one loaded revision, publishes only its own facts and writes only
// the Current section. The loop owns all state and decides with next; one operation at a time runs
// in a worker the loop starts and cancels, because an upgrade can take minutes and a new
// configuration, a disable, Stop or Uninstall must cancel it.
//
// A disabled package keeps its Current: a module's decision is computed on its node, which must
// stay in the graph. A disabled package still accepts configurations: the enabled script reads
// its settings from the object.
type Reconciler struct {
	// Immutable after Start.
	name      string
	rev       model.Revision // rev is the running revision; the package Reconciler owns its image.
	pkg       packageObject  // pkg is rev.Package; only Current configures it and runs its hooks.
	namespace string         // namespace is the release namespace.
	kind      model.Kind     // kind adds an application's purge to its uninstall (uninstallSteps).
	output    string         // output is the Output Current publishes: its image digests' checksum (OutputOf).
	deps      Deps
	clock     retry.Clock
	reporter  reportWriter
	logger    *log.Logger
	// ctx is Current's own context, the parent of the hooks' lifetime (startHooks).
	ctx context.Context

	// in carries the package Reconciler's pushes and the hook lanes' changed(); only the loop
	// loads it.
	in     *inbox.Inbox[input]
	cancel context.CancelFunc
	done   chan struct{} // done is closed when the loop has exited.
	// uninstallErr is written by the loop before done is closed, read by Uninstall after it.
	uninstallErr error

	// Owned by the loop.

	state state // state is what next decides from, besides the inputs.
	// hooksStop stops the started hooks; nil while none are. It round-trips through the worker,
	// whose steps start and stop the hooks.
	hooksStop func()
	// operation is nil while the worker is idle; operationState is read back when it is done.
	operation      *operation.Operation
	operationState *operationState
}

// Start starts the loop without a configuration; the package Reconciler pushes one when it has one.
// The first run waits for an accepted configuration and an event that echoes it. Non-blocking.
func Start(ctx context.Context, cfg Config, deps Deps, reporter reportWriter) *Reconciler {
	ctx, cancel := context.WithCancel(ctx)

	logger := deps.Logger
	if logger == nil {
		logger = log.NewNop()
	}

	r := &Reconciler{
		ctx:       ctx,
		cancel:    cancel,
		name:      cfg.Name,
		rev:       cfg.Revision,
		pkg:       cfg.Revision.Package,
		namespace: model.ReleaseNamespace(cfg.Kind, cfg.Name, deps.ModulesNamespace),
		kind:      cfg.Kind,
		output:    OutputOf(cfg.Revision.Package),
		deps:      deps,
		clock:     retry.OrWall(deps.Clock),
		reporter:  reporter,
		logger:    logger.With(slog.String("package", cfg.Name), slog.String("version", cfg.Revision.Image.Version)),
		in:        inbox.New[input](),
		done:      make(chan struct{}),
		state:     state{version: cfg.Revision.Image.Version, ran: cfg.Restored},
	}

	go r.loop(ctx)

	return r
}

// Revision returns the running revision.
func (r *Reconciler) Revision() model.Revision {
	return r.rev
}

// Push records the latest configuration with its configstate.Key, and the latest event; nil leaves
// that input as it is. The caller computes the checksum once per configuration: Push runs on every
// wake of the package Reconciler. Non-blocking.
func (r *Reconciler) Push(config *model.Configuration, checksum string, ev *schedule.Event) {
	r.in.Put(func(in *input) {
		if config != nil {
			in.config, in.configChecksum = config, checksum
		}

		if ev != nil {
			in.event = ev
		}
	})
}

// Stop cancels the operation in progress, stops the hooks and waits for the loop to exit, so a
// stopped Current never writes again. It keeps the release and the image. Nil-safe.
func (r *Reconciler) Stop() {
	if r == nil {
		return
	}

	r.cancel()
	<-r.done
}

// Uninstall retries until it succeeds; an error only when ctx is cancelled. It reads no event: the
// uninstall wins over everything (next, rule 1). The loop has exited when it returns; the image
// stays attached. Nil-safe.
func (r *Reconciler) Uninstall(ctx context.Context) error {
	if r == nil {
		return nil
	}

	r.in.Put(func(in *input) { in.uninstall = true })

	select {
	case <-r.done:
		return r.uninstallErr
	case <-ctx.Done():
		r.cancel()
		<-r.done

		return ctx.Err()
	}
}

// changed is the hook lanes' callback after a hook changed the values. Non-blocking.
func (r *Reconciler) changed() {
	r.in.Put(func(in *input) { in.valuesGen++ })
}

// drifted is the drift monitor's callback (resources of the release are absent); every Upgrade
// hands it to nelm, so it reaches the Current that upgraded last. Non-blocking.
func (r *Reconciler) drifted() {
	r.in.Put(func(in *input) { in.driftGen++ })
}
