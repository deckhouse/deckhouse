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
// go read-only" — and it is deliberately a SECOND lease, separate from the one that decides
// which replica fills from the upstream. Those two answers must be free to differ: the replica
// filling the cache is the last one that should stop accepting writes.
type ClusterLease struct {
	Log *slog.Logger

	// Client reaches the lease objects.
	Client coordinationv1client.CoordinationV1Interface

	// Namespace and Name identify the lease.
	Namespace, Name string

	// Identity is this replica, as recorded in the lease.
	Identity string
}

// Hold waits for this replica's turn, does the work, and gives the turn up.
//
// The lease is released as soon as the work returns rather than at its natural expiry, because
// the replicas behind this one are waiting to collect and every second of the window matters
// to them.
func (l *ClusterLease) Hold(ctx context.Context, work func(context.Context) error) error {
	held, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		once   sync.Once
		result error
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
		LeaseDuration:   60 * time.Second,
		RenewDeadline:   40 * time.Second,
		RetryPeriod:     5 * time.Second,
		ReleaseOnCancel: true,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				defer close(done)
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

	select {
	case <-done:
		return result
	default:
		// Never became the holder. The caller reads a cancelled context as "another replica
		// had the turn", which is not a failure.
		if err := ctx.Err(); err != nil {
			return err
		}
		return errors.New("the collection turn was never obtained")
	}
}
