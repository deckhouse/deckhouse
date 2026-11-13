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

// The update notification: one send in flight, retried with backoff until it succeeds; the sent
// notification is recorded, and cleared at activation or when the update is cancelled.

package lifecycle

import (
	"context"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// notifier sends one announcement (model.Notifier, which Policy carries).
type notifier interface {
	Notify(ctx context.Context, a model.Announcement) error
}

// notification is the announcement of one desired version; owned by the loop.
type notification struct {
	version string
	// op is the send in flight; nil while waiting for a retry.
	op *operation.Operation
	// failure is the last send's error; nil before the first failure.
	failure error
	backoff retry.Backoff
}

// notify drives the announcement of desired, which the notification gate asked for: another
// version's send is stopped, a new one starts at once, a failed one is retried once its backoff
// allows. applyAt is the activation time the announcement names.
func (r *Reconciler) notify(ctx context.Context, desired model.Desired, applyAt time.Time) {
	if n := r.notification; n == nil || n.version != desired.Version {
		r.stopNotification()
		r.notification = &notification{version: desired.Version}
	}

	n := r.notification
	if n.op != nil || n.backoff.Holds(n.version, r.clock.Now()) {
		return
	}

	a := model.Announcement{Name: r.name, Kind: r.kind, Version: desired.Version, ApplyAt: applyAt}
	if r.current != nil {
		a.From = r.current.Revision().Image.Version
	}

	var sender notifier = desired.Policy.Notification.Notifier

	n.op = operation.Start(ctx, r.name, operation.Notify, n.version, []operation.Step{{
		Name: "notify",
		Run:  func(ctx context.Context) error { return sender.Notify(ctx, a) },
	}})
}

// notificationDone is the Done channel of the send in flight; nil, which blocks, when none is.
func (r *Reconciler) notificationDone() <-chan operation.Result {
	if r.notification == nil {
		return nil
	}

	return r.notification.op.Done()
}

// notificationFinished takes a send's result: a sent notification is recorded and reported, a
// failed one waits for its backoff. A cancelled send is no failure: the one who cancelled it decides
// again.
func (r *Reconciler) notificationFinished(res operation.Result) {
	n := r.notification
	n.op = nil

	if res.Failed == nil {
		r.notification = nil
		r.sent(n.version)

		return
	}

	if res.Canceled {
		return
	}

	n.failure = res.Err

	now := r.clock.Now()
	until := n.backoff.Failed(n.version, now)

	r.logger.Warn("send update notification failed", slog.String("version", n.version),
		slog.Duration("retry_in", until.Sub(now)), log.Err(res.Err))
}

// sent records the notification of version as sent now and reports it.
func (r *Reconciler) sent(version string) {
	r.notified = model.Notified{Version: version, At: r.clock.Now()}
	r.reporter.SetNotified(r.notified)
	r.logger.Info("update notification sent", slog.String("version", version))
}

// clearNotified forgets the sent notification: the update it announced was cancelled, so the version
// is announced again if it is wanted later.
func (r *Reconciler) clearNotified() {
	if r.notified.IsZero() {
		return
	}

	r.notified = model.Notified{}
	r.reporter.SetNotified(model.Notified{})
}

// stopNotification cancels and waits for the send in flight and forgets the announcement. A send
// that succeeded before the cancel is still recorded: the webhook got it, so it is not sent again.
func (r *Reconciler) stopNotification() {
	if version, ok := r.dropNotification(); ok {
		r.sent(version)
	}
}

// dropNotification cancels and waits for the send in flight and forgets the announcement, reporting
// nothing, as shutdown and remove must; it returns the version of a send that succeeded meanwhile.
func (r *Reconciler) dropNotification() (string, bool) {
	n := r.notification
	if n == nil {
		return "", false
	}

	r.notification = nil

	if n.op == nil {
		return "", false
	}

	n.op.Cancel()
	res := <-n.op.Done()

	return n.version, res.Failed == nil
}

// notificationRetry is when a failed send is retried; zero when none waits.
func (r *Reconciler) notificationRetry() time.Time {
	if n := r.notification; n != nil && n.op == nil && n.failure != nil {
		return n.backoff.Until()
	}

	return time.Time{}
}
