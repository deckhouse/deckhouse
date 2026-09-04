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

package run

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingElection stands in for the real election: it reports when it is entered and
// when it is left, which is all the supervisor above it can observe.
type recordingElection struct {
	mutex    sync.Mutex
	attempts int
	leading  bool
	left     chan struct{}
}

func newRecordingElection() *recordingElection {
	return &recordingElection{left: make(chan struct{}, 8)}
}

func (e *recordingElection) elect(ctx context.Context) {
	e.mutex.Lock()
	e.attempts++
	e.leading = true
	e.mutex.Unlock()

	<-ctx.Done()

	e.mutex.Lock()
	e.leading = false
	e.mutex.Unlock()
	e.left <- struct{}{}
}

func (e *recordingElection) state() (int, bool) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.attempts, e.leading
}

func newCandidate(eligible func(context.Context) (bool, error), elect func(context.Context)) *Candidate {
	return &Candidate{
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Eligible: eligible,
		Elect:    elect,
		Review:   5 * time.Millisecond,
	}
}

func eventually(t *testing.T, condition func() bool, message string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal(message)
}

// TestCandidateStandsAsideWhileIneligible is the entry half of the rule: an empty replica
// must not take the lease from a full one.
func TestCandidateStandsAsideWhileIneligible(t *testing.T) {
	election := newRecordingElection()
	candidate := newCandidate(
		func(context.Context) (bool, error) { return false, nil },
		election.elect)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() { defer close(done); candidate.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	attempts, _ := election.state()
	assert.Zero(t, attempts, "an ineligible replica entered the election")

	cancel()
	<-done
}

// TestCandidateEntersWhenItBecomesEligible covers the ordinary start: nobody is full yet,
// so somebody has to lead in order to begin filling at all.
func TestCandidateEntersWhenItBecomesEligible(t *testing.T) {
	var allowed atomic.Bool
	election := newRecordingElection()
	candidate := newCandidate(
		func(context.Context) (bool, error) { return allowed.Load(), nil },
		election.elect)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go candidate.Run(ctx)

	time.Sleep(30 * time.Millisecond)
	attempts, _ := election.state()
	require.Zero(t, attempts)

	allowed.Store(true)
	eventually(t, func() bool {
		_, leading := election.state()
		return leading
	}, "the replica never entered the election after becoming eligible")
}

// TestCandidateStepsDownWhenAnotherBecomesFull is the half that plain election cannot do,
// and the reason it is needed: `d8 mirror push` lands on whichever replica the ingress
// chose. If that is not the leader, an air-gapped cluster is left with an empty leader and
// a full follower, and recovers from neither on its own.
func TestCandidateStepsDownWhenAnotherBecomesFull(t *testing.T) {
	allowed := atomic.Bool{}
	allowed.Store(true)

	election := newRecordingElection()
	candidate := newCandidate(
		func(context.Context) (bool, error) { return allowed.Load(), nil },
		election.elect)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go candidate.Run(ctx)

	eventually(t, func() bool {
		_, leading := election.state()
		return leading
	}, "the replica never started leading")

	// Another replica reports the whole set.
	allowed.Store(false)

	select {
	case <-election.left:
	case <-time.After(2 * time.Second):
		t.Fatal("the replica kept the lease while another replica held the images")
	}

	_, leading := election.state()
	assert.False(t, leading)
}

// TestCandidateKeepsLeadingWhenEligibilityCannotBeRead: the answer comes from the API
// server, and an unreachable one is no reason to hand over a lease that is working. This
// is the same principle the whole design rests on — losing the control plane must not
// change what the data plane is doing.
func TestCandidateKeepsLeadingWhenEligibilityCannotBeRead(t *testing.T) {
	var failing atomic.Bool
	election := newRecordingElection()
	candidate := newCandidate(
		func(context.Context) (bool, error) {
			if failing.Load() {
				return false, errors.New("dial tcp 10.0.0.1:6443: connect: connection refused")
			}
			return true, nil
		},
		election.elect)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go candidate.Run(ctx)

	eventually(t, func() bool {
		_, leading := election.state()
		return leading
	}, "the replica never started leading")

	failing.Store(true)
	time.Sleep(50 * time.Millisecond)

	_, leading := election.state()
	assert.True(t, leading, "an unreachable API server cost the cluster its storage leader")
}

// TestCandidateReentersAfterLosingTheLease: an election that ends on its own — a lease
// that could not be renewed — must be stood for again rather than left.
func TestCandidateReentersAfterLosingTheLease(t *testing.T) {
	var attempts atomic.Int32
	candidate := newCandidate(
		func(context.Context) (bool, error) { return true, nil },
		func(context.Context) {
			attempts.Add(1)
			// Returns immediately, as an elector does when it loses the lease.
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go candidate.Run(ctx)

	eventually(t, func() bool { return attempts.Load() >= 3 },
		"the replica stopped standing in the election after losing it once")
}

// TestCandidateStopsWithTheContext keeps a shutdown from hanging on the supervisor.
func TestCandidateStopsWithTheContext(t *testing.T) {
	election := newRecordingElection()
	candidate := newCandidate(
		func(context.Context) (bool, error) { return true, nil },
		election.elect)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); candidate.Run(ctx) }()

	eventually(t, func() bool {
		_, leading := election.state()
		return leading
	}, "the replica never started leading")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the candidate did not stop with its context")
	}
}
