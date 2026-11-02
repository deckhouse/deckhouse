/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ledger

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// MinFenceWindow is the least time between an approval of a former holder of the lease and the moment
// a new holder can take the lease and see the writes: L - S - timeout. A write saved later than that
// after its approval is what the inflight limit bounds.
const MinFenceWindow = 9 * time.Second

// Timing are the intervals of the lease, the fence and the webhook. They come from one ConfigMap,
// which also renders the timeout of the webhook, so that they cannot disagree.
type Timing struct {
	// Lease is the duration of the lease: a candidate takes it no earlier than this after it saw the
	// last renewal.
	Lease time.Duration
	// RenewDeadline is how long the holder keeps trying to renew before it gives the lease up.
	RenewDeadline time.Duration
	// Retry is the interval of the renewals and of the attempts to take the lease.
	Retry time.Duration
	// Fence is how long after it sent its last successful renewal the holder still answers.
	Fence time.Duration
	// WebhookTimeout is timeoutSeconds of the webhook.
	WebhookTimeout time.Duration
}

// DefaultTiming are 17 s lease, 10 s renew deadline, 1 s retry, 7 s fence and 1 s webhook timeout.
var DefaultTiming = Timing{Lease: 17 * time.Second, RenewDeadline: 10 * time.Second, Retry: time.Second, Fence: 7 * time.Second, WebhookTimeout: time.Second}

// Validate refuses intervals that leave the fence window below MinFenceWindow or break the rules of
// the leader election.
func (t Timing) Validate() error {
	if window := t.Lease - t.Fence - t.WebhookTimeout; window < MinFenceWindow {
		return fmt.Errorf("lease %s - fence %s - webhook timeout %s = %s is below %s", t.Lease, t.Fence, t.WebhookTimeout, window, MinFenceWindow)
	}
	if t.Fence <= t.Retry || t.Fence >= t.RenewDeadline {
		return fmt.Errorf("the fence %s must be longer than the retry period %s and shorter than the renew deadline %s", t.Fence, t.Retry, t.RenewDeadline)
	}
	if t.RenewDeadline >= t.Lease {
		return fmt.Errorf("the renew deadline %s must be shorter than the lease %s", t.RenewDeadline, t.Lease)
	}
	return nil
}

// TimingFromConfigMap reads the intervals from the ConfigMap Helm renders; a key that is missing keeps
// its default.
func TimingFromConfigMap(cm *corev1.ConfigMap) (Timing, error) {
	t := DefaultTiming
	if cm == nil {
		return t, t.Validate()
	}
	for key, target := range map[string]*time.Duration{
		"leaseSeconds":          &t.Lease,
		"renewDeadlineSeconds":  &t.RenewDeadline,
		"retryPeriodSeconds":    &t.Retry,
		"fenceSeconds":          &t.Fence,
		"webhookTimeoutSeconds": &t.WebhookTimeout,
	} {
		raw, ok := cm.Data[key]
		if !ok {
			continue
		}
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			return Timing{}, fmt.Errorf("%s: %q is not a positive number of seconds", key, raw)
		}
		*target = time.Duration(seconds) * time.Second
	}
	return t, t.Validate()
}

// FencedLock is the lock of the leader election that remembers when the holder sent its last
// successful renewal. client-go keeps the time the answer came, which a slow API server makes later;
// a fence counted from it would let the holder answer after a candidate may have taken the lease.
type FencedLock struct {
	resourcelock.Interface
	// Now is time.Now unless a test sets it.
	Now func() time.Time

	mu sync.Mutex
	// sent is when the last successful renewal was sent, with the monotonic reading.
	sent time.Time
}

func (l *FencedLock) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

// Create takes the lease.
func (l *FencedLock) Create(ctx context.Context, record resourcelock.LeaderElectionRecord) error {
	sent := l.now()
	err := l.Interface.Create(ctx, record)
	l.settle(record, sent, err)
	return err
}

// Update renews or releases the lease.
func (l *FencedLock) Update(ctx context.Context, record resourcelock.LeaderElectionRecord) error {
	sent := l.now()
	err := l.Interface.Update(ctx, record)
	l.settle(record, sent, err)
	return err
}

func (l *FencedLock) settle(record resourcelock.LeaderElectionRecord, sent time.Time, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch {
	case err != nil:
	case record.HolderIdentity == l.Identity():
		l.sent = sent
	default:
		// a release writes an empty holder
		l.sent = time.Time{}
	}
}

// Forget drops the last renewal, so the holder stops answering at once.
func (l *FencedLock) Forget() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sent = time.Time{}
}

// Fresh reports whether the last successful renewal was sent less than the fence ago, by the
// monotonic clock and by the wall clock alike: a paused machine may keep its monotonic clock still.
func (l *FencedLock) Fresh(fence time.Duration) bool {
	l.mu.Lock()
	sent := l.sent
	l.mu.Unlock()
	if sent.IsZero() {
		return false
	}
	now := l.now()
	return now.Sub(sent) < fence && now.Round(0).Sub(sent.Round(0)) < fence
}

// Deadline is when the holder stops answering: the fence after the last successful renewal was sent,
// the zero time without one.
func (l *FencedLock) Deadline(fence time.Duration) time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sent.IsZero() {
		return time.Time{}
	}
	return l.sent.Add(fence)
}

// RenewAge is the time since the last successful renewal was sent, zero without one.
func (l *FencedLock) RenewAge() time.Duration {
	l.mu.Lock()
	sent := l.sent
	l.mu.Unlock()
	if sent.IsZero() {
		return 0
	}
	return l.now().Sub(sent)
}
