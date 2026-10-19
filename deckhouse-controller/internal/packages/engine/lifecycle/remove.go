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

// Removal and shutdown: the two ways a package Reconciler's loop ends.

package lifecycle

import (
	"context"
	"log/slog"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// remove removes the package from the graph and uninstalls it, blocking until done or cancelled:
// stop the preparation; RemoveNode; uninstall (Current.Uninstall: Current owns the hooks the
// deletion hooks need and retries itself; without a Current, delete the release by name, without
// deletion hooks); detach Current's image; report Removed. The node goes first: a deletion is never
// undone, and a node left to an uninstall that keeps failing would hold readiness and every higher
// tier. A cancelled remove reports nothing, so the finalizer stays and the next boot restores the
// package to remove it again; RemoveNode of an absent node is a no-op.
func (r *Reconciler) remove(ctx context.Context) {
	r.stopCandidate(ctx)

	r.deps.Scheduler.RemoveNode(r.name)

	if r.current != nil {
		if err := r.current.Uninstall(ctx); err != nil {
			return
		}
	} else if !r.deleteRelease(ctx) {
		return
	}

	if ctx.Err() != nil {
		return
	}

	if r.current != nil {
		r.detach(ctx, r.current.Revision().Image)
		r.current = nil
		r.activated.Store(nil)
	}

	if ctx.Err() != nil {
		return
	}

	r.reporter.SetRemoved()
	r.logger.Info("package removed")
}

// deleteRelease retries with backoff until it succeeds (true) or ctx is cancelled; a new failure
// is reported in RemoveError, cleared on success.
func (r *Reconciler) deleteRelease(ctx context.Context) bool {
	var (
		backoff  retry.Backoff
		reported string
	)

	namespace := model.ReleaseNamespace(r.kind, r.name, r.deps.Current.ModulesNamespace)

	r.deps.Releases.RemoveMonitor(r.name)

	for {
		err := r.deps.Releases.Delete(ctx, namespace, r.name)
		if err == nil {
			if reported != "" {
				r.reporter.SetRemoveError("")
			}

			return true
		}

		if ctx.Err() != nil {
			return false
		}

		if msg := err.Error(); msg != reported {
			reported = msg
			r.reporter.SetRemoveError(msg)
		}

		now := r.clock.Now()
		delay := backoff.Failed("", now).Sub(now)
		timer := r.clock.NewTimer(delay)

		r.logger.Warn("delete release failed", slog.Duration("retry_in", delay), log.Err(err))

		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C():
		}
	}
}

func (r *Reconciler) shutdown(ctx context.Context) {
	r.stopCandidate(ctx)
	r.current.Stop()
}
