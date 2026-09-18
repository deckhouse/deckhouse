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
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"fencing-agent/internal/domain"
)

func groupWithAPeer() (*fakeNodes, *fakeExpected) {
	return mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-2", IP: "10.0.0.2", UID: "uid-2"},
	)
}

func assertNothingAfterTheOwnRead(t *testing.T, nodes *fakeNodes, cluster *fakeCluster) {
	t.Helper()

	if got := nodes.getsOf(testNodeName); got != 1 {
		t.Errorf("the own Node was read %d times, want 1", got)
	}

	if got := nodes.candidateGets(); len(got) != 0 {
		t.Errorf("candidates %v were read, want none after the own Node check stopped the attempt", got)
	}

	if joins := cluster.joins(); len(joins) != 0 {
		t.Errorf("join was called with %v, want no join after the own Node check stopped the attempt", joins)
	}
}

func TestOwnNodeReadFailureIsAFailedAttemptWithoutCandidatesOrJoin(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "i/o timeout", err: &net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded}},
		{name: "deadline exceeded", err: context.DeadlineExceeded},
		{name: "service unavailable", err: apierrors.NewServiceUnavailable("etcd is unavailable")},
		{
			name: "forbidden",
			err:  apierrors.NewForbidden(schema.GroupResource{Resource: "nodes"}, testNodeName, errors.New("rbac denied")),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes, expected := groupWithAPeer()
			nodes.setAnswer(testNodeName, nodeAnswer{err: tc.err})
			cluster := &fakeCluster{}

			err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context())

			if err == nil {
				t.Fatal("attempt succeeded, want a failure while the own Node cannot be read")
			}

			if errors.Is(err, ErrNotMember) {
				t.Errorf("attempt returned %v, want a failed read, not the ErrNotMember verdict", err)
			}

			if !errors.Is(err, tc.err) {
				t.Errorf("attempt returned %v, want the read error in its chain", err)
			}

			assertNothingAfterTheOwnRead(t, nodes, cluster)
		})
	}
}

func TestOwnNodeReadIsBoundedByTheAPITimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nodes, expected := groupWithAPeer()
		nodes.setAnswer(testNodeName, nodeAnswer{blockUntilCtx: true})
		cluster := &fakeCluster{}
		joiner := newJoiner(t, nodes, expected, cluster)

		start := time.Now()
		err := joiner.Attempt(t.Context())
		elapsed := time.Since(start)

		if want := joinerParams().APITimeout; elapsed != want {
			t.Errorf("attempt returned after %s, want exactly the API timeout %s", elapsed, want)
		}

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("attempt returned %v, want context.DeadlineExceeded in its chain", err)
		}

		if errors.Is(err, ErrNotMember) {
			t.Errorf("attempt returned %v, want a timed-out read, not the ErrNotMember verdict", err)
		}

		assertNothingAfterTheOwnRead(t, nodes, cluster)
	})
}

func TestOwnNodeVerdictsRefuseTheJoin(t *testing.T) {
	cases := []struct {
		name   string
		answer nodeAnswer
	}{
		{name: "not found", answer: nodeAnswer{err: notFound(testNodeName)}},
		{name: "label with an empty value", answer: nodeAnswer{record: withGroup(selfRecord(), "")}},
		{name: "no label key", answer: nodeAnswer{record: domain.NodeRecord{Name: testNodeName, UID: testNodeUID, IP: testNodeIP}}},
		{name: "relabeled into another group", answer: nodeAnswer{record: withGroup(selfRecord(), "worker-2")}},
		{name: "recreated node", answer: nodeAnswer{record: withUID(selfRecord(), "uid-1-recreated")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes, expected := groupWithAPeer()
			nodes.setAnswer(testNodeName, tc.answer)
			cluster := &fakeCluster{}

			err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context())

			if !errors.Is(err, ErrNotMember) {
				t.Fatalf("attempt returned %v, want ErrNotMember", err)
			}

			assertNothingAfterTheOwnRead(t, nodes, cluster)
		})
	}

	t.Run("valid answer", func(t *testing.T) {
		nodes, expected := groupWithAPeer()
		cluster := &fakeCluster{}

		if err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context()); err != nil {
			t.Fatalf("attempt returned %v, want a node confirmed by the API to proceed and join", err)
		}

		if joins := cluster.joins(); len(joins) != 1 {
			t.Errorf("join was called %d times, want once after the own Node check passed", len(joins))
		}
	})
}

func TestOwnNodeLabelRuleIsTheSharedGroupRule(t *testing.T) {
	for _, label := range []string{"worker", "", "worker-2", "Worker", " worker", "worker "} {
		t.Run(strconv.Quote(label), func(t *testing.T) {
			nodes, expected := groupWithAPeer()
			nodes.setAnswer(testNodeName, nodeAnswer{record: withGroup(selfRecord(), label)})
			cluster := &fakeCluster{}

			err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context())

			inGroup := domain.InNodeGroup(label, testNodeGroup)
			if errors.Is(err, ErrNotMember) != !inGroup {
				t.Errorf("label %q: attempt returned %v, want ErrNotMember exactly when InNodeGroup is false (%t)", label, err, inGroup)
			}

			if inGroup && err != nil {
				t.Errorf("label %q: attempt returned %v, want success for a node in its group", label, err)
			}
		})
	}
}

func threePeerGroup() (*fakeNodes, *fakeExpected) {
	return mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-2", IP: "10.0.0.2", UID: "uid-2"},
		domain.Peer{Name: "worker-3", IP: "10.0.0.3", UID: "uid-3"},
		domain.Peer{Name: "worker-4", IP: "10.0.0.4", UID: "uid-4"},
	)
}

func TestAttemptReadsTheOwnNodeBeforeAnyCandidate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nodes, expected := threePeerGroup()
		release := make(chan struct{})
		nodes.setAnswer(testNodeName, nodeAnswer{record: selfRecord(), release: release})
		cluster := &fakeCluster{}
		joiner := newJoiner(t, nodes, expected, cluster)

		result := make(chan error, 1)

		go func() {
			result <- joiner.Attempt(t.Context())
		}()

		synctest.Wait()

		if got := nodes.gets(); !slices.Equal(got, []string{testNodeName}) {
			t.Errorf("reads while the own Node read is pending are %v, want only the own Node", got)
		}

		close(release)

		if err := <-result; err != nil {
			t.Fatalf("attempt returned %v, want success once the own Node read answers", err)
		}

		if got := nodes.candidateGets(); len(got) != 3 {
			t.Errorf("candidates read after the own Node answered are %v, want all 3 peers", got)
		}
	})
}

func TestCandidateReadsRunInParallel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nodes, expected := threePeerGroup()
		for i, name := range []string{"worker-2", "worker-3", "worker-4"} {
			nodes.setAnswer(name, nodeAnswer{record: peerRecord(name, "10.0.0."+strconv.Itoa(i+2)), delay: time.Second})
		}

		cluster := &fakeCluster{}
		joiner := newJoiner(t, nodes, expected, cluster)

		start := time.Now()
		err := joiner.Attempt(t.Context())
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("attempt returned %v, want success", err)
		}

		if elapsed != time.Second {
			t.Errorf("three candidate reads of 1s each took %s, want 1s: they must run in parallel", elapsed)
		}

		if got := nodes.peakInFlight(); got != 3 {
			t.Errorf("at most %d reads were in flight, want all 3 candidates at once", got)
		}

		assertJoinedOnce(t, cluster, "10.0.0.2:8500", "10.0.0.3:8500", "10.0.0.4:8500")
	})
}

func TestCandidateErrorDoesNotCancelItsSiblings(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nodes, expected := threePeerGroup()
		nodes.setAnswer("worker-2", nodeAnswer{err: &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}})
		nodes.setAnswer("worker-3", nodeAnswer{record: peerRecord("worker-3", "10.0.0.3"), delay: 100 * time.Millisecond})
		nodes.setAnswer("worker-4", nodeAnswer{record: peerRecord("worker-4", "10.0.0.4"), delay: 100 * time.Millisecond})
		cluster := &fakeCluster{}

		if err := newJoiner(t, nodes, expected, cluster).Attempt(t.Context()); err != nil {
			t.Fatalf("attempt returned %v, want success from the two candidates that answered", err)
		}

		assertJoinedOnce(t, cluster, "10.0.0.3:8500", "10.0.0.4:8500")

		for _, read := range nodes.answeredReads() {
			if (read.name == "worker-3" || read.name == "worker-4") && read.ctxErr != nil {
				t.Errorf("the read of %s answered with its ctx already ended (%v), want the failed read of worker-2 to leave it alone", read.name, read.ctxErr)
			}
		}
	})
}

func TestSlowCandidateIsDroppedAfterTheAPITimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		nodes, expected := threePeerGroup()
		nodes.setAnswer("worker-4", nodeAnswer{blockUntilCtx: true})
		cluster := &fakeCluster{}

		var logs bytes.Buffer

		joiner := New(nodes, expected, cluster, joinerParams(), newJSONLogger(&logs))

		start := time.Now()
		err := joiner.Attempt(t.Context())
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("attempt returned %v, want success from the candidates that answered", err)
		}

		if want := joinerParams().APITimeout; elapsed != want {
			t.Errorf("attempt took %s, want the candidate reads to end at the API timeout %s", elapsed, want)
		}

		assertJoinedOnce(t, cluster, "10.0.0.2:8500", "10.0.0.3:8500")

		if got := nodes.getsOf("worker-4"); got != 1 {
			t.Errorf("the slow candidate was read %d times, want once: a timed-out read is dropped, not retried", got)
		}

		records := drainLogs(t, &logs)
		assertSnakeCaseKeys(t, records)

		dropped := withMsg(records, droppedMsg)
		if len(dropped) != 1 {
			t.Fatalf("drop records are %v, want exactly one for the slow candidate", dropped)
		}

		record := dropped[0]
		if got, want := lineOf(record), droppedMsg+"|warn|worker-4|read_failed"; got != want {
			t.Errorf("drop record is %q, want %q", got, want)
		}

		if got := record.str("error"); !strings.Contains(got, context.DeadlineExceeded.Error()) {
			t.Errorf("drop record error is %q, want it to name %q", got, context.DeadlineExceeded)
		}
	})
}

func TestAttemptReturnsOnCancelWhileTheJoinHangs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		release := make(chan struct{})
		releaseJoin := sync.OnceFunc(func() { close(release) })
		t.Cleanup(releaseJoin)

		joinEnded := make(chan struct{})
		nodes, expected := groupWithAPeer()
		cluster := &fakeCluster{joinFn: func(seeds []string) (int, error) {
			defer close(joinEnded)

			<-release

			return len(seeds), nil
		}}
		joiner := newJoiner(t, nodes, expected, cluster)

		result := make(chan error, 1)

		go func() {
			result <- joiner.Attempt(ctx)
		}()

		synctest.Wait()

		if joins := cluster.joins(); len(joins) != 1 {
			t.Fatalf("join was called with %v, want one call in flight before the cancel", joins)
		}

		select {
		case err := <-result:
			t.Fatalf("attempt returned %v while the join hangs, want it to wait for the join or the cancel", err)
		default:
		}

		start := time.Now()

		cancel()

		var err error

		select {
		case err = <-result:
		case <-time.After(time.Minute):
			t.Fatal("attempt did not return within a minute of the cancel while the join hangs")
		}

		if elapsed := time.Since(start); elapsed != 0 {
			t.Errorf("attempt returned %s after the cancel, want at once", elapsed)
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("attempt returned %v, want ctx.Err(), context.Canceled", err)
		}

		select {
		case <-joinEnded:
			t.Error("the join ended before its release, want the attempt to return without waiting for it")
		default:
		}

		releaseJoin()
		synctest.Wait()

		select {
		case <-joinEnded:
		default:
			t.Error("the abandoned join is still running after its release, want its goroutine to end")
		}
	})
}

func TestAttemptCancelledDuringCandidateReadsIsAQuietShutdown(t *testing.T) {
	cases := []struct {
		name     string
		answered string
	}{
		{name: "every read in flight"},
		{name: "one candidate already answered", answered: "worker-2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				nodes, expected := threePeerGroup()
				for _, name := range []string{"worker-2", "worker-3", "worker-4"} {
					if name != tc.answered {
						nodes.setAnswer(name, nodeAnswer{blockUntilCtx: true})
					}
				}

				cluster := &fakeCluster{}

				var logs bytes.Buffer

				joiner := New(nodes, expected, cluster, joinerParams(), newJSONLogger(&logs))

				result := make(chan error, 1)

				go func() {
					result <- joiner.Attempt(ctx)
				}()

				synctest.Wait()

				if got := nodes.candidateGets(); len(got) != 3 {
					t.Fatalf("candidates read before the cancel are %v, want all 3 peers", got)
				}

				start := time.Now()

				cancel()

				err := <-result

				if elapsed := time.Since(start); elapsed != 0 {
					t.Errorf("attempt returned %s after the cancel, want at once", elapsed)
				}

				if !errors.Is(err, context.Canceled) {
					t.Errorf("attempt returned %v, want ctx.Err(), context.Canceled", err)
				}

				synctest.Wait()

				if joins := cluster.joins(); len(joins) != 0 {
					t.Errorf("join was called with %v, want none after the cancel", joins)
				}

				records := drainLogs(t, &logs)
				assertSnakeCaseKeys(t, records)

				for _, record := range records {
					if level := record.level(); level != "debug" && level != "info" {
						t.Errorf("attempt logged %v, want no line above info on a shutdown", record)
					}
				}

				if dropped := withMsg(records, droppedMsg); len(dropped) != 0 {
					t.Errorf("drop records are %v, want none: the reads failed only because of the shutdown", dropped)
				}
			})
		})
	}
}

func withGroup(record domain.NodeRecord, nodeGroup string) domain.NodeRecord {
	record.NodeGroup = nodeGroup

	return record
}

func withUID(record domain.NodeRecord, uid string) domain.NodeRecord {
	record.UID = uid

	return record
}
