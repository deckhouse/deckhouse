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
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// Two replicas on one Lease: one leads at a time, and the other takes over
// as soon as the first stops and releases the Lease.
func TestRunLeaderElection(t *testing.T) {
	oldLease, oldRenew, oldRetry := leaseDuration, renewDeadline, retryPeriod
	leaseDuration, renewDeadline, retryPeriod = time.Second, 500*time.Millisecond, 100*time.Millisecond
	t.Cleanup(func() { leaseDuration, renewDeadline, retryPeriod = oldLease, oldRenew, oldRetry })

	client := fake.NewSimpleClientset()
	var active, overlaps atomic.Int32
	var mu sync.Mutex
	var leaders []string

	run := func(ctx context.Context, id string, done chan<- struct{}) {
		RunLeaderElection(ctx, client, "d8-system", id, func(lctx context.Context) {
			if active.Add(1) > 1 {
				overlaps.Add(1)
			}
			mu.Lock()
			leaders = append(leaders, id)
			mu.Unlock()
			<-lctx.Done()
			active.Add(-1)
		}, log.NewNop())
		close(done)
	}

	ctxA, stopA := context.WithCancel(context.Background())
	defer stopA()
	doneA := make(chan struct{})
	go run(ctxA, "a", doneA)
	waitLeaders := func(n int) []string {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			mu.Lock()
			got := append([]string(nil), leaders...)
			mu.Unlock()
			if len(got) >= n {
				return got
			}
			if time.Now().After(deadline) {
				t.Fatalf("leaders %v, want %d", got, n)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitLeaders(1)

	ctxB, stopB := context.WithCancel(context.Background())
	defer stopB()
	doneB := make(chan struct{})
	go run(ctxB, "b", doneB)

	// b waits while a holds the Lease.
	time.Sleep(3 * leaseDuration / 2)
	if got := waitLeaders(1); len(got) != 1 {
		t.Fatalf("leaders %v while a holds the Lease", got)
	}

	// a stops and releases it: b takes over well before the Lease expires.
	stopA()
	<-doneA
	start := time.Now()
	if got := waitLeaders(2); got[1] != "b" {
		t.Fatalf("leaders %v, want b after a", got)
	}
	if d := time.Since(start); d > leaseDuration {
		t.Errorf("b took over after %v: the Lease was not released", d)
	}
	stopB()
	<-doneB

	if overlaps.Load() != 0 {
		t.Fatal("two replicas led at once")
	}
}

// The Lease is lost in the middle of a term (the API server refuses its
// renewal): the term ends, and the replica leads again once the Lease can be
// taken, only after the previous term has finished.
func TestRunLeaderElectionLosesLease(t *testing.T) {
	oldLease, oldRenew, oldRetry := leaseDuration, renewDeadline, retryPeriod
	leaseDuration, renewDeadline, retryPeriod = time.Second, 500*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { leaseDuration, renewDeadline, retryPeriod = oldLease, oldRenew, oldRetry })

	client := fake.NewSimpleClientset()
	var refuse atomic.Bool
	client.PrependReactor("update", "leases", func(k8stesting.Action) (bool, runtime.Object, error) {
		if refuse.Load() {
			return true, nil, errors.New("the API server is away")
		}
		return false, nil, nil
	})

	var active, overlaps, finished atomic.Int32
	started := make(chan int, 4)
	lost := make(chan struct{}, 4)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		term := 0
		RunLeaderElection(ctx, client, "d8-system", "a", func(lctx context.Context) {
			if active.Add(1) > 1 {
				overlaps.Add(1)
			}
			term++
			started <- term
			<-lctx.Done()
			lost <- struct{}{}
			// The work of a term takes a while to stop: the Lease is won
			// back meanwhile.
			time.Sleep(4 * retryPeriod)
			active.Add(-1)
			finished.Add(1)
		}, log.NewNop())
	}()

	wait := func(what string, ch <-chan struct{}) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for %s", what)
		}
	}
	waitTerm := func(want int) {
		t.Helper()
		select {
		case got := <-started:
			if got != want {
				t.Fatalf("term %d started, want %d", got, want)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for term %d", want)
		}
	}

	waitTerm(1)
	refuse.Store(true)
	wait("the context of the term to end with the Lease", lost)
	if ctx.Err() != nil {
		t.Fatal("the election stopped")
	}

	refuse.Store(false)
	waitTerm(2)
	if got := finished.Load(); got != 1 {
		t.Fatalf("the second term started with %d terms finished, want 1", got)
	}

	stop()
	wait("the election to stop", done)
	// A slow runner may lose the Lease once more before the stop.
	if got := finished.Load(); got < 2 {
		t.Errorf("the election returned with %d terms finished, want 2 or more", got)
	}
	if overlaps.Load() != 0 {
		t.Error("two terms ran at once")
	}
}
