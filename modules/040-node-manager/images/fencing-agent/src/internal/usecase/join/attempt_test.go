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
	"net"
	"os"
	"slices"
	"sync"
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

func threePeerGroup() (*fakeNodes, *fakeExpected) {
	return mirroredGroup(
		selfPeer(),
		domain.Peer{Name: "worker-2", IP: "10.0.0.2", UID: "uid-2"},
		domain.Peer{Name: "worker-3", IP: "10.0.0.3", UID: "uid-3"},
		domain.Peer{Name: "worker-4", IP: "10.0.0.4", UID: "uid-4"},
	)
}

func TestAttemptReadsOnlyTheOwnNode(t *testing.T) {
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

		if joins := cluster.joins(); len(joins) != 0 {
			t.Errorf("join was called with %v while the own Node read is pending, want none", joins)
		}

		close(release)

		if err := <-result; err != nil {
			t.Fatalf("attempt returned %v, want success once the own Node read answers", err)
		}

		if got := nodes.gets(); !slices.Equal(got, []string{testNodeName}) {
			t.Errorf("reads of the whole attempt are %v, want only the own Node: the candidates come from the cache", got)
		}

		assertJoinedOnce(t, cluster, "10.0.0.2:8500", "10.0.0.3:8500", "10.0.0.4:8500")
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

func TestAttemptCancelledDuringTheOwnReadIsAQuietShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		nodes, expected := threePeerGroup()
		nodes.setAnswer(testNodeName, nodeAnswer{blockUntilCtx: true})
		cluster := &fakeCluster{}

		joiner := newJoiner(t, nodes, expected, cluster)

		result := make(chan error, 1)

		go func() {
			result <- joiner.Attempt(ctx)
		}()

		synctest.Wait()

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
	})
}

func withGroup(record domain.NodeRecord, nodeGroup string) domain.NodeRecord {
	record.NodeGroup = nodeGroup

	return record
}

func withUID(record domain.NodeRecord, uid string) domain.NodeRecord {
	record.UID = uid

	return record
}
