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

package gc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationv1client "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// ClusterLease serialises collection across replicas using a Kubernetes lease.
//
// Built on the same leader election the rest of the cluster uses rather than a lock of our
// own. What it guards is unusual for a lease — not "who is in charge" but "whose turn is it to
// collect" — and it is deliberately a SECOND lease, separate from the one that decides which
// replica fills from the upstream. Those two answers must be free to differ: the replica filling
// the cache is the last one that should pause its writes.
type ClusterLease struct {
	Log *slog.Logger

	// Client reaches the lease objects.
	Client coordinationv1client.CoordinationV1Interface

	// Namespace and Name identify the lease.
	Namespace, Name string

	// Identity is this replica, as recorded in the lease.
	Identity string

	// LeaseDuration, RenewDeadline and RetryPeriod are leader election's own. Zero means the defaults
	// below; set apart so a test can lose a lease in a second rather than a minute.
	LeaseDuration, RenewDeadline, RetryPeriod time.Duration
}

func orDefault(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

// Hold waits for this replica's turn, does the work, and gives the turn up.
//
// The lease is released as soon as the work returns rather than at its natural expiry, because
// the replicas behind this one are waiting to collect and every second of the window matters
// to them.
func (l *ClusterLease) Hold(ctx context.Context, work func(context.Context) error) error {
	held, cancel := context.WithCancel(ctx)
	defer cancel()

	// Whether the work started, decided once: client-go starts it on a goroutine of its own, which may
	// not have run yet when Run returns. Whichever claims the turn first wins it — the work, which then
	// is waited out, or Hold on its way out, after which the work never starts.
	const (
		turnOpen int32 = iota
		turnStarted
		turnClosed
	)
	var (
		once   sync.Once
		result error
		turn   atomic.Int32
		done   = make(chan struct{})
	)

	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta:  metav1.ObjectMeta{Namespace: l.Namespace, Name: l.Name},
			Client:     l.Client,
			LockConfig: resourcelock.ResourceLockConfig{Identity: l.Identity},
		},
		// Short, because the work is bounded and a holder that dies must not keep the others
		// out for long: the next replica's whole turn may be inside this window.
		LeaseDuration:   orDefault(l.LeaseDuration, 60*time.Second),
		RenewDeadline:   orDefault(l.RenewDeadline, 40*time.Second),
		RetryPeriod:     orDefault(l.RetryPeriod, 5*time.Second),
		ReleaseOnCancel: true,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				defer close(done)
				if !turn.CompareAndSwap(turnOpen, turnStarted) {
					return
				}
				result = work(ctx)
				// Giving the turn up immediately rather than holding it to expiry.
				cancel()
			},
			OnStoppedLeading: func() {
				// Reached both after finishing and after losing the lease mid-work. The
				// latter is worth saying: the collection may have been interrupted, and the
				// next replica is about to start one.
				once.Do(func() {
					l.Log.Info("the collection turn ended", "identity", l.Identity)
				})
			},
		},
	})
	if err != nil {
		return fmt.Errorf("building the collection lease: %w", err)
	}

	elector.Run(held)

	// Run returns as soon as the lease is lost, and the work it started goes on: it cancels the work's
	// context and does not wait for it. Returned there, the caller would read the outcome while the
	// work still writes it — and the next replica, holding the lease now, would be collecting beside a
	// deletion this one had not stopped. So a started turn is waited out.
	if !turn.CompareAndSwap(turnOpen, turnClosed) {
		<-done
		// Canceled with the caller's context alive: only the elector cancels then, on losing the lease.
		// Every other stop — the caller's own — ends the caller's context too.
		if result != nil && ctx.Err() == nil && errors.Is(result, context.Canceled) {
			return fmt.Errorf("the collection lease was lost while collecting: %w", result)
		}
		return result
	}

	// Never became the holder, or became it only as the turn was closing. The caller reads a cancelled
	// context as "another replica had the turn", which is not a failure.
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("the collection turn was never obtained")
}
