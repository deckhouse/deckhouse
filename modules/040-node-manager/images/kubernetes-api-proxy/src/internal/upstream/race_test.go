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

package upstream

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"sync"
	"testing"
	"time"
)

func TestList_ConcurrentAccess(t *testing.T) {
	probes := newFakeProbes()
	for _, addr := range []string{"m0", "m1"} {
		probes.set(addr, probeReady, time.Millisecond)
	}

	list, err := NewList(nil, WithHealthcheckInterval(time.Millisecond), withProbes(probes))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(list.Shutdown)

	fb, err := NewFallbackList()
	if err != nil {
		t.Fatal(err)
	}

	both := ups("m0", "m1")
	one := both[:1]
	deadline := time.Now().Add(200 * time.Millisecond)

	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			for time.Now().Before(deadline) {
				if u, err := list.Pick(); err == nil && i == 0 {
					list.Down(u)
				}

				list.Reconcile(both)
				list.Reconcile(one)
				_, _ = list.ExportNodes()
				_ = list.ListFullAddresses()
				fb.UpdateFromList(list)
				_, _ = fb.Pick()
			}
		})
	}
	wg.Wait()

	list.Reconcile(both)
	list.round(context.Background(), list.beginRound())

	if _, err := list.Pick(); err != nil {
		t.Fatalf("Pick() after churn error = %v", err)
	}
}

// No race on score: rounds in fail-step update score while /upstreams data
// is read.
func TestList_NoRaceOnScore(t *testing.T) {
	probes := newFakeProbes()
	slow := func(ctx context.Context, u *Upstream) (probeResult, error) {
		time.Sleep(time.Duration(rand.IntN(3)) * time.Millisecond)

		return probes.probe(ctx, u)
	}

	list, err := NewList(ups("m0", "m1"),
		WithHealthcheckInterval(time.Millisecond),
		WithFailStep(time.Millisecond, time.Hour),
		WithStallTimeout(time.Millisecond),
		withProbe(slow),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(list.Shutdown)

	deadline := time.Now().Add(300 * time.Millisecond)
	kinds := []probeKind{probeReady, probeNotReady, probeSoft, probeHard}

	var wg sync.WaitGroup

	wg.Go(func() {
		for time.Now().Before(deadline) {
			probes.set("m0", kinds[rand.IntN(len(kinds))], time.Millisecond)
			probes.set("m1", kinds[rand.IntN(len(kinds))], time.Millisecond)
			time.Sleep(time.Millisecond)
		}
	})

	for range 4 {
		wg.Go(func() {
			for time.Now().Before(deadline) {
				nodes, _ := list.ExportNodes()
				if _, err := json.Marshal(nodes); err != nil {
					t.Error(err)
				}

				_, _ = list.Probe()
				_ = list.cycleDelay(1)
				time.Sleep(100 * time.Microsecond)
			}
		})
	}
	wg.Wait()

	if seq, _ := list.Probe(); seq == 0 {
		t.Error("no round applied")
	}
}

func TestList_NoDeadlockUnderReconcile(t *testing.T) {
	probes := newFakeProbes()
	for _, addr := range []string{"m0", "m1", "m2"} {
		probes.set(addr, probeReady, time.Millisecond)
	}

	list, err := NewList(nil, WithHealthcheckInterval(time.Millisecond), withProbes(probes))
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		all := ups("m0", "m1", "m2")
		some := ups("m0")

		for range 2000 {
			list.Reconcile(all)

			if u, err := list.Pick(); err == nil {
				list.Down(u)
			}

			list.Reconcile(some)
			list.Reconcile(nil)
		}
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("reconcile loop did not finish in 10s")
	}

	stopped := make(chan struct{})
	go func() {
		list.Shutdown()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown() did not return in 5s")
	}
}
