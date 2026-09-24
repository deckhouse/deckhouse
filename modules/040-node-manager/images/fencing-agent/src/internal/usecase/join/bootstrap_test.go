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

package join

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"fencing-agent/internal/domain"
)

func cancelOnOwnRead(nodes *fakeNodes, n int, cancel context.CancelFunc) {
	nodes.setOnCall(func(_ int, name string) {
		if name == testNodeName && nodes.getsOf(testNodeName) >= n {
			cancel()
		}
	})
}

func TestBootstrapRetriesAFailedAttemptWithoutJoining(t *testing.T) {
	cases := []struct {
		name             string
		group            func() (*fakeNodes, *fakeExpected)
		ownReads         int
		noCandidateReads bool
	}{
		{
			name: "peers have no addresses",
			group: func() (*fakeNodes, *fakeExpected) {
				return mirroredGroup(
					selfPeer(),
					domain.Peer{Name: "worker-2", IP: ""},
					domain.Peer{Name: "worker-3", IP: ""},
				)
			},
			ownReads: 3,
		},
		{
			name: "the node left its group",
			group: func() (*fakeNodes, *fakeExpected) {
				nodes, expected := mirroredGroup(
					selfPeer(),
					domain.Peer{Name: "worker-2", IP: "10.0.0.2", UID: "uid-2"},
				)
				nodes.setAnswer(testNodeName, nodeAnswer{err: notFound(testNodeName)})

				return nodes, expected
			},
			ownReads:         3,
			noCandidateReads: true,
		},
		{
			name: "the own Node cannot be read",
			group: func() (*fakeNodes, *fakeExpected) {
				nodes, expected := mirroredGroup(
					selfPeer(),
					domain.Peer{Name: "worker-2", IP: "10.0.0.2", UID: "uid-2"},
				)
				nodes.setAnswer(testNodeName, nodeAnswer{err: errors.New("api server is down")})

				return nodes, expected
			},
			ownReads:         2,
			noCandidateReads: true,
		},
		{
			name:     "the cache does not list this node",
			group:    groupWithoutThisNode,
			ownReads: 3,
		},
		{
			name:     "every candidate is dropped",
			group:    groupWithEveryCandidateDropped,
			ownReads: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				nodes, expected := tc.group()
				cancelOnOwnRead(nodes, tc.ownReads, cancel)
				cluster := &fakeCluster{}

				joiner := newJoiner(t, nodes, expected, cluster)
				joiner.Bootstrap(ctx)

				if joiner.Joined() {
					t.Errorf("joined state is set while %s, want it unset", tc.name)
				}

				if joins := cluster.joins(); len(joins) != 0 {
					t.Errorf("join was called with %v while %s, want none", joins, tc.name)
				}

				if got := nodes.getsOf(testNodeName); got < tc.ownReads {
					t.Errorf("the own Node was read %d times, want at least %d: a failed attempt is retried", got, tc.ownReads)
				}

				if got := nodes.candidateGets(); tc.noCandidateReads && len(got) != 0 {
					t.Errorf("candidates %v were read while %s, want none", got, tc.name)
				}
			})
		})
	}
}

func TestBootstrapRetriesUntilJoinSucceeds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nodes, expected := mirroredGroup(
			selfPeer(),
			domain.Peer{Name: "worker-2", IP: "10.0.0.2"},
		)
		cluster := &fakeCluster{failures: 2}

		joiner := newJoiner(t, nodes, expected, cluster)
		joiner.Bootstrap(t.Context())

		if joins := cluster.joins(); len(joins) != 3 {
			t.Errorf("expected 3 join attempts, got %d", len(joins))
		}

		if got := nodes.getsOf(testNodeName); got != 3 {
			t.Errorf("the own Node was read %d times, want 3", got)
		}

		if got := nodes.gets(); len(got) != 6 || nodes.getsOf("worker-2") != 3 {
			t.Errorf("reads are %v, want 6: the own Node and worker-2 once per attempt", got)
		}

		if !joiner.Joined() {
			t.Error("joined state is not set after a successful join")
		}
	})
}

func TestBootstrapStopsOnContextCancel(t *testing.T) {
	t.Run("cancelled before the start", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		nodes, expected := mirroredGroup(selfPeer(), domain.Peer{Name: "worker-2", IP: "10.0.0.2"})
		nodes.setAnswer(testNodeName, nodeAnswer{err: errors.New("api server is down")})
		joiner := newJoiner(t, nodes, expected, &fakeCluster{})

		done := make(chan struct{})

		go func() {
			defer close(done)

			joiner.Bootstrap(ctx)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("bootstrap did not return after the context was cancelled")
		}

		if joiner.Joined() {
			t.Error("a cancelled bootstrap must not be reported as joined")
		}
	})

	cases := []struct {
		name    string
		blocked string
	}{
		{name: "during the own read", blocked: testNodeName},
		{name: "during a candidate read", blocked: "worker-2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				nodes, expected := mirroredGroup(selfPeer(), domain.Peer{Name: "worker-2", IP: "10.0.0.2"})
				nodes.setAnswer(tc.blocked, nodeAnswer{blockUntilCtx: true})
				cluster := &fakeCluster{}

				joiner := newJoiner(t, nodes, expected, cluster)

				start := time.Now()
				done := make(chan struct{})

				go func() {
					defer close(done)

					joiner.Bootstrap(ctx)
				}()

				cancelAfter := joinerParams().APITimeout / 2
				time.Sleep(cancelAfter)
				cancel()
				<-done

				if elapsed := time.Since(start); elapsed != cancelAfter {
					t.Errorf("bootstrap returned %s after the start, want right at the cancel after %s, without a backoff sleep", elapsed, cancelAfter)
				}

				if got := nodes.getsOf(testNodeName); got != 1 {
					t.Errorf("the own Node was read %d times, want a single attempt", got)
				}

				if joins := cluster.joins(); len(joins) != 0 {
					t.Errorf("join was called with %v, want none after the cancel", joins)
				}

				if joiner.Joined() {
					t.Error("a cancelled bootstrap must not be reported as joined")
				}
			})
		})
	}
}

func TestBootstrapCancelledDuringBackoffStopsWithoutJoining(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		nodes, expected := mirroredGroup(selfPeer(), domain.Peer{Name: "worker-2", IP: "10.0.0.2"})
		nodes.queueAnswers(testNodeName,
			nodeAnswer{err: errors.New("api server is down")},
			nodeAnswer{err: errors.New("api server is still down")},
		)
		cluster := &fakeCluster{}

		joiner := newJoiner(t, nodes, expected, cluster)

		done := make(chan struct{})

		go func() {
			defer close(done)

			joiner.Bootstrap(ctx)
		}()

		time.Sleep(joinerParams().RetryInterval + time.Second)

		if got := nodes.getsOf(testNodeName); got != 2 {
			t.Errorf("the own Node was read %d times before the cancel, want 2 attempts", got)
		}

		cancel()
		<-done

		if joiner.Joined() {
			t.Error("a cancelled bootstrap must not be reported as joined")
		}

	})
}

func TestBootstrapDelayIsFullJitter(t *testing.T) {
	const (
		runs   = 30
		pauses = 5
	)

	params := joinerParams()

	synctest.Test(t, func(t *testing.T) {
		belowNarrowJitter := false

		for run := range runs {
			ctx, cancel := context.WithCancel(t.Context())

			nodes, expected := mirroredGroup(selfPeer(), domain.Peer{Name: "worker-2", IP: "10.0.0.2"})
			nodes.setAnswer(testNodeName, nodeAnswer{err: errors.New("api server is down")})

			var readAt []time.Time

			nodes.setOnCall(func(_ int, name string) {
				if name != testNodeName {
					return
				}

				readAt = append(readAt, time.Now())
				if len(readAt) > pauses {
					cancel()
				}
			})

			newJoiner(t, nodes, expected, &fakeCluster{}).Bootstrap(ctx)
			cancel()

			if len(readAt) != pauses+1 {
				t.Fatalf("run %d: the own Node was read %d times, want %d", run, len(readAt), pauses+1)
			}

			backoff := params.RetryInterval

			for k := 1; k < len(readAt); k++ {
				pause := readAt[k].Sub(readAt[k-1])

				if k == 1 && pause != params.RetryInterval {
					t.Errorf("run %d: the first pause is %s, want exactly RetryInterval %s", run, pause, params.RetryInterval)
				}

				if pause < params.RetryInterval || pause > backoff {
					t.Errorf("run %d: pause %d is %s, want it in [%s, %s]", run, k, pause, params.RetryInterval, backoff)
				}

				if pause < backoff*8/10 {
					belowNarrowJitter = true
				}

				backoff = min(backoff*2, params.MaxRetryInterval)
			}
		}

		if !belowNarrowJitter {
			t.Errorf("no pause of %d runs fell below 80%% of its backoff, want full jitter down to RetryInterval", runs)
		}
	})
}
