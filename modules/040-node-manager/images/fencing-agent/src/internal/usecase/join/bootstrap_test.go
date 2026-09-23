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
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"fencing-agent/internal/domain"
	"fencing-agent/internal/logtest"
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

				var logs bytes.Buffer

				joiner := New(nodes, expected, cluster, joinerParams(), logtest.NewJSONLogger(&logs))

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

				records := logtest.Drain(t, &logs)
				logtest.AssertSnakeCaseKeys(t, records)

				for _, record := range records {
					if level := record.Level(); level != "debug" && level != "info" {
						t.Errorf("bootstrap logged %v, want no line above info on a shutdown", record)
					}
				}

				aborted := logtest.WithMsg(records, abortedMsg)
				if len(aborted) != 1 {
					t.Fatalf("abort records are %v, want exactly one", aborted)
				}

				record := aborted[0]
				if record.Level() != "info" || record.Int("attempts") != 1 || record.Str("last_error_class") != classNone ||
					record.Str("last_delay") != "0s" || !strings.Contains(record.Str("error"), context.Canceled.Error()) {
					t.Errorf("abort record is %v, want info with the attempt's shutdown error, attempts 1, last_delay 0s and last_error_class none", record)
				}
			})
		})
	}
}

const (
	failedMsg      = "memberlist bootstrap join failed, retrying"
	notMemberMsg   = "this node is not a member of its NodeGroup, the join is retried until that changes"
	streakEndedMsg = "memberlist bootstrap join failure streak ended"
	finishedMsg    = "memberlist bootstrap join finished"
	abortedMsg     = "memberlist bootstrap join aborted"
)

var bootstrapMsgs = []string{failedMsg, notMemberMsg, streakEndedMsg, finishedMsg, abortedMsg}

func TestBootstrapCancelledDuringBackoffLogsTheSummary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		nodes, expected := mirroredGroup(selfPeer(), domain.Peer{Name: "worker-2", IP: "10.0.0.2"})
		nodes.queueAnswers(testNodeName,
			nodeAnswer{err: errors.New("api server is down")},
			nodeAnswer{err: errors.New("api server is still down")},
		)
		cluster := &fakeCluster{}

		var logs bytes.Buffer

		joiner := New(nodes, expected, cluster, joinerParams(), logtest.NewJSONLogger(&logs))

		done := make(chan struct{})

		go func() {
			defer close(done)

			joiner.Bootstrap(ctx)
		}()

		cancelAt := joinerParams().RetryInterval + time.Second
		time.Sleep(cancelAt)

		if got := nodes.getsOf(testNodeName); got != 2 {
			t.Errorf("the own Node was read %d times before the cancel, want 2 attempts", got)
		}

		cancel()
		<-done

		if joiner.Joined() {
			t.Error("a cancelled bootstrap must not be reported as joined")
		}

		records := logtest.Drain(t, &logs)
		logtest.AssertSnakeCaseKeys(t, records)

		failed := logtest.WithMsg(records, failedMsg)
		if len(failed) != 2 {
			t.Fatalf("failure records are %v, want one per failed attempt", failed)
		}

		if got := logtest.WithMsg(records, finishedMsg); len(got) != 0 {
			t.Errorf("finish records are %v, want none after the cancel", got)
		}

		aborted := logtest.WithMsg(records, abortedMsg)
		if len(aborted) != 1 {
			t.Fatalf("abort records are %v, want exactly one", aborted)
		}

		record := aborted[0]
		if record.Level() != "info" {
			t.Errorf("abort record is %v, want level info", record)
		}

		if got := record.Str("error"); !strings.Contains(got, "api server is still down") {
			t.Errorf("abort record error is %q, want the second attempt's error", got)
		}

		if got := record.Int("attempts"); got != 2 {
			t.Errorf("abort record attempts is %d, want 2", got)
		}

		interrupted := failed[1].Str("next_in")
		if got := record.Str("last_delay"); got != interrupted {
			t.Errorf("abort record last_delay is %q, want the interrupted sleep %q", got, interrupted)
		}

		if got := record.Str("last_error_class"); got != classTransport {
			t.Errorf("abort record last_error_class is %q, want %q", got, classTransport)
		}

		if got, want := record.Str("elapsed"), cancelAt.String(); got != want {
			t.Errorf("abort record elapsed is %q, want %q", got, want)
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

func TestBootstrapFailureStreakWarnsOnceThenDebugs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nodes, expected := mirroredGroup(selfPeer(), domain.Peer{Name: "worker-2", IP: "10.0.0.2"})
		down := nodeAnswer{err: errors.New("api server is down")}
		nodes.queueAnswers(testNodeName, down, down, down, down, down)
		cluster := &fakeCluster{}

		var logs bytes.Buffer

		joiner := New(nodes, expected, cluster, joinerParams(), logtest.NewJSONLogger(&logs))
		joiner.Bootstrap(t.Context())

		if !joiner.Joined() {
			t.Fatal("joined state is not set after the own Node became readable")
		}

		records := logtest.Drain(t, &logs)
		logtest.AssertSnakeCaseKeys(t, records)

		failed := logtest.WithMsg(records, failedMsg)
		if len(failed) != 5 {
			t.Fatalf("failure records are %v, want one per failed attempt", failed)
		}

		for i, record := range failed {
			wantLevel := "debug"
			if i == 0 {
				wantLevel = "warn"
			}

			if record.Level() != wantLevel || record.Int("attempt") != i+1 {
				t.Errorf("failure record %d is %v, want attempt %d at level %s", i, record, i+1, wantLevel)
			}

			for _, key := range []string{"error", "attempt_elapsed", "next_in"} {
				if record.Str(key) == "" {
					t.Errorf("failure record %d is %v, want a %s key", i, record, key)
				}
			}

			if _, ok := record["backoff"]; ok {
				t.Errorf("failure record %d is %v, want next_in instead of backoff", i, record)
			}
		}

		if got := logtest.WithMsg(records, streakEndedMsg); len(got) != 0 {
			t.Errorf("streak end records are %v, want none: success closes the streak with the finish summary", got)
		}

		finished := logtest.WithMsg(records, finishedMsg)
		if len(finished) != 1 {
			t.Fatalf("finish records are %v, want exactly one", finished)
		}

		record := finished[0]
		if record.Level() != "info" || record.Int("attempts") != 6 || record.Str("last_error_class") != classTransport {
			t.Errorf("finish record is %v, want info with attempts 6 and last_error_class transport", record)
		}

		if got := record.Str("last_delay"); got == "" || got == "0s" {
			t.Errorf("finish record last_delay is %q, want the last backoff sleep", got)
		}

		if got := record.Str("elapsed"); got == "" {
			t.Errorf("finish record is %v, want an elapsed key", record)
		}
	})
}

func TestBootstrapClassChangeEndsTheStreak(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nodes, expected := mirroredGroup(selfPeer(), domain.Peer{Name: "worker-2", IP: "10.0.0.2"})
		down := nodeAnswer{err: errors.New("api server is down")}
		gone := nodeAnswer{err: notFound(testNodeName)}
		nodes.queueAnswers(testNodeName, down, down, gone, gone)
		cluster := &fakeCluster{}

		var logs bytes.Buffer

		joiner := New(nodes, expected, cluster, joinerParams(), logtest.NewJSONLogger(&logs))
		joiner.Bootstrap(t.Context())

		if !joiner.Joined() {
			t.Fatal("joined state is not set after the own Node became a member again")
		}

		records := logtest.Drain(t, &logs)
		logtest.AssertSnakeCaseKeys(t, records)

		var own []logtest.Record

		for _, record := range records {
			if slices.Contains(bootstrapMsgs, record.Msg()) {
				own = append(own, record)
			}
		}

		want := []struct{ msg, level string }{
			{failedMsg, "warn"},
			{failedMsg, "debug"},
			{streakEndedMsg, "info"},
			{notMemberMsg, "warn"},
			{notMemberMsg, "debug"},
			{finishedMsg, "info"},
		}

		if len(own) != len(want) {
			t.Fatalf("bootstrap records are %v, want %v", own, want)
		}

		for i, w := range want {
			if own[i].Msg() != w.msg || own[i].Level() != w.level {
				t.Errorf("bootstrap record %d is %v, want %q at level %s", i, own[i], w.msg, w.level)
			}
		}

		ended := own[2]
		if ended.Int("attempts") != 2 || ended.Str("last_error_class") != classTransport {
			t.Errorf("streak end record is %v, want attempts 2 and last_error_class transport", ended)
		}

		secondSleep, err := time.ParseDuration(own[1].Str("next_in"))
		if err != nil {
			t.Fatalf("second failure next_in: %v", err)
		}

		if got, want := ended.Str("last_delay"), secondSleep.String(); got != want {
			t.Errorf("streak end record last_delay is %q, want %q", got, want)
		}

		if got, want := ended.Str("elapsed"), (joinerParams().RetryInterval + secondSleep).Truncate(time.Millisecond).String(); got != want {
			t.Errorf("streak end record elapsed is %q, want %q", got, want)
		}

		if own[3].Int("attempt") != 3 || own[4].Int("attempt") != 4 {
			t.Errorf("NotMember records are %v and %v, want attempts 3 and 4", own[3], own[4])
		}

		finished := own[5]
		if finished.Int("attempts") != 5 || finished.Str("last_error_class") != classNotMember {
			t.Errorf("finish record is %v, want attempts 5 and last_error_class not_member", finished)
		}
	})
}
