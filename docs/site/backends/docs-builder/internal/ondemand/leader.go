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

package ondemand

import (
	"context"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// LeaderLease is the Lease the replicas of docs-builder sharing a cold cache
// elect the one that reads the registry with.
const LeaderLease = "docs-builder-ondemand-leader"

// The timing of the Lease; variables for the tests.
var (
	leaseDuration = 15 * time.Second
	renewDeadline = 10 * time.Second
	retryPeriod   = 2 * time.Second
)

// RunLeaderElection runs lead while this pod holds the Lease in namespace,
// until ctx is done; identity names the pod in the Lease. lead gets a
// context that ends when the Lease is lost. The Lease is released when ctx
// ends, so a rolling update hands it over at once. It returns once the last
// term has finished.
func RunLeaderElection(ctx context.Context, client kubernetes.Interface, namespace, identity string, lead func(context.Context), logger *log.Logger) {
	lock := &resourcelock.LeaseLock{
		LeaseMeta:  metav1.ObjectMeta{Name: LeaderLease, Namespace: namespace},
		Client:     client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{Identity: identity},
	}

	// RunOrDie starts OnStartedLeading in a goroutine and returns on a lost
	// Lease without waiting for it: a term won back right away waits here for
	// the previous one to finish, so two never run at once.
	var term sync.Mutex
	for ctx.Err() == nil {
		// Run returns when the Lease is lost; the pod stands for it again.
		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			LeaseDuration:   leaseDuration,
			RenewDeadline:   renewDeadline,
			RetryPeriod:     retryPeriod,
			ReleaseOnCancel: true,
			Name:            LeaderLease,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(lctx context.Context) {
					term.Lock()
					defer term.Unlock()
					if lctx.Err() != nil {
						return
					}
					logger.Info("leading: reading the registry for the replicas")
					lead(lctx)
				},
				OnStoppedLeading: func() {
					logger.Info("following: the versions come from the leader")
				},
			},
		})
	}

	// Only waits for the term that is still stopping, so the caller knows
	// when its work is over.
	term.Lock()
	term.Unlock() //nolint:staticcheck // SA2001: the wait is the point
}
