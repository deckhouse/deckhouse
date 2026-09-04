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

package failedstate

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

const (
	nodeGroupSize = 3
	takeoverDelay = 3 * time.Second
)

type stubAlive struct{ members []string }

func (s *stubAlive) Members() []string { return s.members }

// Changed is never read: the tests drive reconcile directly rather than the loop.
func (s *stubAlive) Changed() <-chan struct{} { return nil }

type stubExpected struct{ peers []domain.Peer }

func (s *stubExpected) Snapshot() ([]domain.Peer, int, int) {
	return s.peers, len(s.peers), domain.QuorumSize(len(s.peers))
}

type stubEvents struct {
	normal   []string
	warnings []string
}

func (s *stubEvents) Normal(reason, _ string)  { s.normal = append(s.normal, reason) }
func (s *stubEvents) Warning(reason, _ string) { s.warnings = append(s.warnings, reason) }

type stubStore struct {
	states   []v1alpha1.FencingFailedNodeState
	calls    []string
	recorded map[string]v1alpha1.FencingFailedNodeStateFailed
	// invisible names exist in the API but not in the informer cache yet, the
	// window right after another agent created an object.
	invisible map[string]bool

	failCreate error
	failMark   error
	failList   error
	failDelete error
	// keepAfterDelete models an informer that still serves an object the API has
	// already removed.
	keepAfterDelete bool
}

func newStore(states ...v1alpha1.FencingFailedNodeState) *stubStore {
	return &stubStore{
		states:    states,
		recorded:  make(map[string]v1alpha1.FencingFailedNodeStateFailed),
		invisible: make(map[string]bool),
	}
}

func (s *stubStore) List(_ context.Context) ([]v1alpha1.FencingFailedNodeState, error) {
	if s.failList != nil {
		return nil, s.failList
	}

	visible := make([]v1alpha1.FencingFailedNodeState, 0, len(s.states))

	for _, state := range s.states {
		if !s.invisible[state.Name] {
			visible = append(visible, state)
		}
	}

	return visible, nil
}

func (s *stubStore) Create(_ context.Context, peer domain.Peer) (bool, error) {
	s.calls = append(s.calls, "create:"+peer.Name)

	if s.failCreate != nil {
		return false, s.failCreate
	}

	for i := range s.states {
		if s.states[i].Name == peer.Name {
			return false, nil
		}
	}

	s.states = append(s.states, v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: peer.Name, UID: types.UID("cr-" + peer.Name)},
	})

	return true, nil
}

func (s *stubStore) MarkFailed(_ context.Context, name string, failed v1alpha1.FencingFailedNodeStateFailed) (bool, error) {
	s.calls = append(s.calls, "mark:"+name)

	if s.failMark != nil {
		return false, s.failMark
	}

	for i := range s.states {
		if s.states[i].Name != name {
			continue
		}

		if s.states[i].Status.Failed != nil {
			return false, nil
		}

		s.states[i].Status.Failed = failed.DeepCopy()
		s.recorded[name] = failed

		return true, nil
	}

	s.recorded[name] = failed

	return true, nil
}

func (s *stubStore) Delete(_ context.Context, name string, uid types.UID) error {
	s.calls = append(s.calls, fmt.Sprintf("delete:%s:%s", name, uid))

	if s.failDelete != nil {
		return s.failDelete
	}

	if !s.keepAfterDelete {
		s.states = slices.DeleteFunc(s.states, func(state v1alpha1.FencingFailedNodeState) bool {
			return state.Name == name
		})
	}

	return nil
}

type clock struct{ now time.Time }

func (c *clock) Now() time.Time          { return c.now }
func (c *clock) advance(d time.Duration) { c.now = c.now.Add(d) }

type harness struct {
	writer   *Writer
	alive    *stubAlive
	expected *stubExpected
	store    *stubStore
	events   *stubEvents
	clock    *clock
	size     int
}

func peers(names ...string) []domain.Peer {
	out := make([]domain.Peer, 0, len(names))

	for _, name := range names {
		out = append(out, domain.Peer{Name: name, IP: "10.0.0.1", UID: "uid-" + name})
	}

	return out
}

func groupNames(size int) []string {
	names := make([]string, 0, size)

	for i := 1; i <= size; i++ {
		names = append(names, fmt.Sprintf("worker-%d", i))
	}

	return names
}

// newHarness wires the writer to stubs. nodeName decides whether this agent is
// the designated writer for the failed peer, so tests pick it deliberately.
func newHarness(t *testing.T, nodeName string, store *stubStore) *harness {
	t.Helper()

	return newHarnessOfSize(t, nodeGroupSize, nodeName, store)
}

func newHarnessOfSize(t *testing.T, size int, nodeName string, store *stubStore) *harness {
	t.Helper()

	h := &harness{
		alive:    &stubAlive{},
		expected: &stubExpected{peers: peers(groupNames(size)...)},
		store:    store,
		events:   &stubEvents{},
		clock:    &clock{now: time.Date(2026, 6, 2, 15, 0, 0, 0, time.UTC)},
		size:     size,
	}

	h.writer = New(
		Params{
			NodeName:         nodeName,
			RetryInterval:    500 * time.Millisecond,
			MaxRetryInterval: 3 * time.Second,
			TakeoverDelay:    takeoverDelay,
		},
		Deps{
			Alive:    h.alive,
			Expected: h.expected,
			States:   h.store,
			Events:   h.events,
			Now:      h.clock.Now,
		},
		log.NewNop(),
	)

	return h
}

// settle runs one pass with every peer alive, so the writer has seen them before
// any of them is taken away: a peer never seen alive is unknown, not failed.
func (h *harness) settle(ctx context.Context) {
	h.alive.members = groupNames(h.size)
	h.writer.reconcile(ctx)
}

func (h *harness) failPeer(ctx context.Context, names ...string) {
	h.alive.members = slices.DeleteFunc(groupNames(h.size), func(member string) bool {
		return slices.Contains(names, member)
	})
	h.writer.reconcile(ctx)
}

// writerFor returns the agent that would be elected once name has dropped out.
func writerFor(name string) string {
	return writerForIn(nodeGroupSize, name)
}

func writerForIn(size int, names ...string) string {
	alive := slices.DeleteFunc(groupNames(size), func(member string) bool { return slices.Contains(names, member) })

	for _, candidate := range alive {
		if domain.WriterRank(alive, names[0], candidate) == 0 {
			return candidate
		}
	}

	return ""
}

func otherThan(name string) string {
	for _, candidate := range groupNames(nodeGroupSize) {
		if candidate != name && candidate != writerFor(name) {
			return candidate
		}
	}

	return ""
}

func TestDesignatedWriterRecordsTheIncident(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())
	h.failPeer(t.Context(), failed)

	if !slices.Equal(store.calls, []string{"create:" + failed, "mark:" + failed}) {
		t.Fatalf("calls = %v, want a create followed by a status write", store.calls)
	}

	got := store.recorded[failed]

	if got.DetectedBy != h.writer.params.NodeName {
		t.Errorf("detectedBy = %q, want the writer %q", got.DetectedBy, h.writer.params.NodeName)
	}

	if got.Reason != v1alpha1.FailedReasonMemberlistDead {
		t.Errorf("reason = %q, want %q", got.Reason, v1alpha1.FailedReasonMemberlistDead)
	}

	if got.AliveCount != nodeGroupSize-1 || got.QuorumSize != int32(domain.QuorumSize(nodeGroupSize)) {
		t.Errorf("aliveCount/quorumSize = %d/%d, want %d/%d",
			got.AliveCount, got.QuorumSize, nodeGroupSize-1, domain.QuorumSize(nodeGroupSize))
	}

	if !got.DetectedAt.Time.Equal(h.clock.now) {
		t.Errorf("detectedAt = %s, want the moment the failure was seen %s", got.DetectedAt, h.clock.now)
	}

	if !slices.Contains(h.events.normal, reasonStateCreated) {
		t.Errorf("events = %v, want a %s event", h.events.normal, reasonStateCreated)
	}
}

func TestNonWriterKeepsQuiet(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, otherThan(failed), store)

	h.settle(t.Context())
	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none: this agent is not the designated writer", store.calls)
	}
}

func TestWithoutQuorumNothingIsWritten(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())

	// One of three left alive: the minority side of a partition sees the healthy
	// majority as gone, and writing from here is how a split brain evicts a
	// working cluster.
	h.alive.members = []string{h.writer.params.NodeName}
	h.writer.reconcile(t.Context())

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none without quorum", store.calls)
	}
}

func TestPeerNeverSeenAliveIsNotFailed(t *testing.T) {
	const missing = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(missing), store)

	// Straight from start: the peer is in the NodeGroup but has never been in
	// this agent's gossip view, which is a bootstrap state, not a failure.
	h.alive.members = slices.DeleteFunc(groupNames(nodeGroupSize), func(member string) bool { return member == missing })
	h.writer.reconcile(t.Context())

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none for a peer that was never seen alive", store.calls)
	}

	// Once it has been seen, the same absence is a failure.
	h.settle(t.Context())
	h.failPeer(t.Context(), missing)

	if len(store.calls) == 0 {
		t.Error("no calls after the peer was seen alive and then disappeared")
	}
}

func TestPeerWithoutNodeUIDIsSkipped(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(failed), store)

	for i := range h.expected.peers {
		if h.expected.peers[i].Name == failed {
			h.expected.peers[i].UID = ""
		}
	}

	h.settle(t.Context())
	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none: without a UID the owner reference is unusable", store.calls)
	}
}

func TestExistingRecordIsLeftAlone(t *testing.T) {
	const failed = "worker-3"

	earlier := v1alpha1.FencingFailedNodeStateFailed{
		DetectedAt: metav1.NewTime(time.Date(2026, 6, 2, 14, 0, 0, 0, time.UTC)),
		DetectedBy: "worker-9",
		Reason:     v1alpha1.FailedReasonMemberlistDead,
	}
	store := newStore()
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())

	// Recorded by the agent that held the role before this one.
	store.states = append(store.states, v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: failed, UID: "cr-" + failed},
		Status:     v1alpha1.FencingFailedNodeStateStatus{Failed: &earlier},
	})

	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none: the incident is already on record", store.calls)
	}
}

func TestRecoveredPeerRecordIsRemoved(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())
	h.failPeer(t.Context(), failed)

	store.calls = nil
	h.settle(t.Context())

	if !slices.Equal(store.calls, []string{"delete:" + failed + ":cr-" + failed}) {
		t.Errorf("calls = %v, want the record of the recovered peer deleted with its UID", store.calls)
	}

	if !slices.Contains(h.events.normal, reasonStateCleared) {
		t.Errorf("events = %v, want a %s event", h.events.normal, reasonStateCleared)
	}
}

func TestFallbackRecordOfAnAlivePeerIsRemoved(t *testing.T) {
	store := newStore(v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-3", UID: "cr-worker-3"},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Fallback: &v1alpha1.FencingFailedNodeStateFallback{
				Active:            true,
				APIReachable:      true,
				HeartbeatInterval: metav1.Duration{Duration: time.Second},
			},
		},
	})
	h := newHarness(t, writerFor("worker-3"), store)

	h.settle(t.Context())

	want := []string{"delete:worker-3:cr-worker-3"}
	if !slices.Equal(store.calls, want) {
		t.Errorf("calls = %v, want %v", store.calls, want)
	}

	if !slices.Contains(h.events.normal, reasonStateCleared) {
		t.Errorf("events = %v, want a %s event", h.events.normal, reasonStateCleared)
	}
}

func TestWriteFailuresEndInACooldown(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	store.failCreate = errors.New("api server is unavailable")
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())

	for attempt := range maxAttempts {
		// Past the backoff of the previous attempt, but not past the cooldown
		// the last one sets.
		if attempt > 0 {
			h.clock.advance(time.Minute)
		}

		h.failPeer(t.Context(), failed)
	}

	if len(store.calls) != maxAttempts {
		t.Fatalf("got %d attempts %v, want %d before the burst ends", len(store.calls), store.calls, maxAttempts)
	}

	if !slices.Contains(h.events.warnings, reasonStateWriteFailed) {
		t.Errorf("warnings = %v, want a %s event when the burst ends", h.events.warnings, reasonStateWriteFailed)
	}

	// Inside the cooldown the writer stays quiet.
	h.failPeer(t.Context(), failed)

	if len(store.calls) != maxAttempts {
		t.Errorf("got %d attempts, want no new one during the cooldown", len(store.calls))
	}

	// And picks the peer up again once it is over: an unreported dead node must
	// not stay unreported because the API had a bad minute.
	h.clock.advance(cooldown + time.Second)
	h.failPeer(t.Context(), failed)

	if len(store.calls) != maxAttempts+1 {
		t.Errorf("got %d attempts, want the burst to start over after the cooldown", len(store.calls))
	}
}

func TestDetectedAtIsTheFirstSighting(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	store.failCreate = errors.New("api server is unavailable")
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())
	h.failPeer(t.Context(), failed)

	firstSighting := h.clock.now

	h.clock.advance(time.Minute)
	store.failCreate = nil
	h.failPeer(t.Context(), failed)

	if got := store.recorded[failed].DetectedAt; !got.Time.Equal(firstSighting) {
		t.Errorf("detectedAt = %s, want the first sighting %s, not the moment the write went through", got, firstSighting)
	}
}

func TestClearOwnStateRemovesTheRecordOfThisNode(t *testing.T) {
	// After a fencing reboot the node comes back with its own record still in the
	// cluster. It is running, which is the one thing it can assert without quorum.
	const self = "worker-1"

	store := newStore(v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: self, UID: "cr-" + self},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: "worker-2"},
		},
	})
	h := newHarness(t, self, store)

	h.settle(t.Context())

	if !slices.Equal(store.calls, []string{"delete:" + self + ":cr-" + self}) {
		t.Errorf("calls = %v, want the own record deleted", store.calls)
	}
}

func TestClearOwnStateNeedsNoQuorum(t *testing.T) {
	// A NodeGroup too small to reach quorum after a failure would otherwise keep
	// the record of a node that is demonstrably running.
	const self = "worker-1"

	store := newStore(v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: self, UID: "cr-" + self},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: "worker-2"},
		},
	})
	h := newHarness(t, self, store)

	h.alive.members = []string{self}
	h.writer.reconcile(t.Context())

	if !slices.Equal(store.calls, []string{"delete:" + self + ":cr-" + self}) {
		t.Errorf("calls = %v, want the own record deleted even without quorum", store.calls)
	}
}

func TestClearOwnStateRetriesUntilItSucceeds(t *testing.T) {
	const self = "worker-1"

	store := newStore(v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: self, UID: "cr-" + self},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: "worker-2"},
		},
	})
	store.failDelete = errors.New("api server is unavailable")
	h := newHarness(t, self, store)

	h.settle(t.Context())
	store.failDelete = nil
	h.settle(t.Context())

	if len(store.calls) != 2 {
		t.Errorf("calls = %v, want the delete retried on the next pass", store.calls)
	}
}

func TestSimultaneousFailuresAreSplitBetweenWriters(t *testing.T) {
	// Two peers gone at once is where the alive set stops being "everyone but
	// the failed one": an election run over the expected membership could hand a
	// peer to a writer that is itself dead, and nobody would report it.
	const size = 5

	failed := []string{"worker-4", "worker-5"}
	recordedBy := make(map[string][]string)

	for _, node := range groupNames(size) {
		if slices.Contains(failed, node) {
			continue
		}

		store := newStore()
		h := newHarnessOfSize(t, size, node, store)

		h.settle(t.Context())
		h.failPeer(t.Context(), failed...)

		for _, call := range store.calls {
			if name, ok := strings.CutPrefix(call, "create:"); ok {
				recordedBy[name] = append(recordedBy[name], node)
			}
		}
	}

	for _, peer := range failed {
		writers := recordedBy[peer]

		if len(writers) != 1 {
			t.Errorf("peer %s was recorded by %v, want exactly one live agent", peer, writers)
		}
	}
}

func TestHalfWrittenRecordIsFinished(t *testing.T) {
	// The object exists but its /status request never landed, because the agent
	// that created it died in between. Whoever holds the role now has to finish
	// the write instead of treating the object as proof of a record.
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())

	store.states = append(store.states, v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: failed, UID: "cr-" + failed},
	})

	h.failPeer(t.Context(), failed)

	if !slices.Equal(store.calls, []string{"mark:" + failed}) {
		t.Errorf("calls = %v, want the status write finished without a second create", store.calls)
	}
}

func TestIncidentIsRememberedWhileAnotherAgentHoldsTheRole(t *testing.T) {
	// The role can move to this agent later, and the incident has to keep the
	// moment this agent saw the failure, not the moment it inherited the job.
	const size = 5

	const failed = "worker-5"

	firstWriter := writerForIn(size, failed)
	self := writerForIn(size, failed, firstWriter)

	if self == firstWriter {
		t.Fatalf("the test needs two different writers, both resolved to %q", self)
	}

	store := newStore()
	h := newHarnessOfSize(t, size, self, store)

	h.settle(t.Context())
	h.failPeer(t.Context(), failed)

	firstSighting := h.clock.now

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none while another agent holds the role", store.calls)
	}

	// The agent that held the role is gone too, so the role lands here.
	h.clock.advance(time.Minute)
	h.failPeer(t.Context(), failed, firstWriter)

	got, ok := store.recorded[failed]
	if !ok {
		t.Fatalf("calls = %v, want the incident recorded once the role moved here", store.calls)
	}

	if !got.DetectedAt.Time.Equal(firstSighting) {
		t.Errorf("detectedAt = %s, want the first sighting %s", got.DetectedAt, firstSighting)
	}
}

func TestPeerThatFailsAgainIsTimedFromTheNewFailure(t *testing.T) {
	// detectedAt is what the controller counts the evacuation delay from, so the
	// memory of an incident must not outlive the incident.
	const size = 5

	const failed = "worker-5"

	firstWriter := writerForIn(size, failed)
	self := writerForIn(size, failed, firstWriter)

	store := newStore()
	h := newHarnessOfSize(t, size, self, store)

	h.settle(t.Context())
	h.failPeer(t.Context(), failed)
	h.settle(t.Context())

	h.clock.advance(time.Hour)
	secondFailure := h.clock.now
	h.failPeer(t.Context(), failed, firstWriter)

	if got := store.recorded[failed].DetectedAt; !got.Time.Equal(secondFailure) {
		t.Errorf("detectedAt = %s, want the second failure %s: the peer recovered in between", got, secondFailure)
	}
}

func TestSeenAliveIsForgottenWhenTheNodeLeavesTheGroup(t *testing.T) {
	// A Node recreated under a known name must earn the seen-alive gate again,
	// otherwise it can be reported failed before it has ever joined gossip.
	const rejoining = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(rejoining), store)

	h.settle(t.Context())

	// Out of the NodeGroup for a pass.
	full := h.expected.peers
	h.expected.peers = slices.DeleteFunc(slices.Clone(full), func(peer domain.Peer) bool { return peer.Name == rejoining })
	h.alive.members = groupNames(nodeGroupSize - 1)
	h.writer.reconcile(t.Context())

	// Back in the NodeGroup, but not in gossip yet.
	h.expected.peers = full
	h.writer.reconcile(t.Context())

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none: the node has not been seen alive since it came back", store.calls)
	}
}

func TestRecordOfANodeThatLeftTheGroupIsRemoved(t *testing.T) {
	// Nothing else can clear it: agents of the new NodeGroup never list an object
	// labelled with the old one, and the affected node is healthy.
	const gone = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(gone), store)

	h.settle(t.Context())
	h.failPeer(t.Context(), gone)
	store.calls = nil

	h.expected.peers = slices.DeleteFunc(h.expected.peers, func(peer domain.Peer) bool { return peer.Name == gone })
	h.alive.members = groupNames(nodeGroupSize - 1)

	// Inside the grace period the record is left alone: the informer may just
	// be lagging.
	h.writer.reconcile(t.Context())

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none inside the grace period", store.calls)
	}

	h.clock.advance(unexpectedGrace + time.Second)
	h.writer.reconcile(t.Context())

	if !slices.Equal(store.calls, []string{"delete:" + gone + ":cr-" + gone}) {
		t.Errorf("calls = %v, want the record of the departed node removed", store.calls)
	}
}

func TestAlreadyRecordedIncidentIsNotAnnouncedAgain(t *testing.T) {
	// Another agent recorded the incident and this agent's informer has not seen
	// the object yet, so it runs create and mark anyway. Both report that nothing
	// changed, and the event carrying this agent's own alive and quorum numbers
	// must not be attached to a record that does not describe them.
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())

	store.states = append(store.states, v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: failed, UID: "cr-" + failed},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: "worker-9"},
		},
	})
	store.invisible[failed] = true

	h.failPeer(t.Context(), failed)

	if !slices.Equal(store.calls, []string{"create:" + failed, "mark:" + failed}) {
		t.Fatalf("calls = %v, want both attempts to run against an object the cache cannot see", store.calls)
	}

	if slices.Contains(h.events.normal, reasonStateCreated) {
		t.Errorf("events = %v, want no %s: the incident was already on record", h.events.normal, reasonStateCreated)
	}
}

// rankOneFor returns the agent that takes over if the designated writer for name
// never produces a record.
func rankOneFor(t *testing.T, size int, name string) string {
	t.Helper()

	alive := slices.DeleteFunc(groupNames(size), func(member string) bool { return member == name })

	for _, candidate := range alive {
		if domain.WriterRank(alive, name, candidate) == 1 {
			return candidate
		}
	}

	t.Fatalf("no rank one candidate for %q", name)

	return ""
}

func TestNextInLineTakesOverWhenTheRecordNeverAppears(t *testing.T) {
	// The designated writer can be alive in gossip and still unable to reach
	// Kubernetes, or it can have restarted and never seen the peer alive. Either
	// way it keeps the role, so the agents behind it have to stop deferring.
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, rankOneFor(t, nodeGroupSize, failed), store)

	h.settle(t.Context())
	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none: the designated writer gets its turn first", store.calls)
	}

	h.clock.advance(takeoverDelay - time.Millisecond)
	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none before the handover step has passed", store.calls)
	}

	h.clock.advance(2 * time.Millisecond)
	h.failPeer(t.Context(), failed)

	if !slices.Equal(store.calls, []string{"create:" + failed, "mark:" + failed}) {
		t.Errorf("calls = %v, want the incident recorded after the handover", store.calls)
	}
}

func TestNextInLineStandsDownWhenTheRecordExists(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, rankOneFor(t, nodeGroupSize, failed), store)

	h.settle(t.Context())

	store.states = append(store.states, v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: failed, UID: "cr-" + failed},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: "worker-9"},
		},
	})

	h.clock.advance(10 * takeoverDelay)
	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none: the designated writer already recorded the incident", store.calls)
	}
}

func TestNextInLineAlsoTakesOverTheRemoval(t *testing.T) {
	// A record left behind for a node that is back is the dangerous direction:
	// it is what makes the controller evict a healthy node.
	const recovered = "worker-3"

	store := newStore(v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: recovered, UID: "cr-" + recovered},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: "worker-9"},
		},
	})
	h := newHarness(t, rankOneFor(t, nodeGroupSize, recovered), store)

	h.settle(t.Context())

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none: the designated writer gets its turn first", store.calls)
	}

	h.clock.advance(takeoverDelay + time.Millisecond)
	h.settle(t.Context())

	if !slices.Equal(store.calls, []string{"delete:" + recovered + ":cr-" + recovered}) {
		t.Errorf("calls = %v, want the stale record removed after the handover", store.calls)
	}
}

func TestOwnRecordWrittenAfterThisProcessStartedIsLeftAlone(t *testing.T) {
	// A record younger than the agent means a peer considers this node dead right
	// now. That is a live disagreement for the fallback path to settle: deleting
	// it would only start a delete-and-create war with the quorate side.
	const self = "worker-1"

	store := newStore()
	h := newHarness(t, self, store)

	store.states = append(store.states, v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name:              self,
			UID:               "cr-" + self,
			CreationTimestamp: metav1.NewTime(h.clock.now.Add(time.Second)),
		},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: "worker-2"},
		},
	})

	h.clock.advance(2 * time.Second)
	h.settle(t.Context())

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none: the record is younger than this process", store.calls)
	}
}

func TestRecordIsRemovedOnceEvenWhileTheInformerLags(t *testing.T) {
	// Delete succeeds but the label-scoped informer keeps serving the object for
	// a while. Without a memory of what it removed, the writer would delete and
	// announce the same record on every pass.
	const failed = "worker-3"

	store := newStore()
	store.keepAfterDelete = true
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())
	h.failPeer(t.Context(), failed)
	store.calls = nil

	h.settle(t.Context())
	h.settle(t.Context())
	h.settle(t.Context())

	deletes := 0

	for _, call := range store.calls {
		if strings.HasPrefix(call, "delete:") {
			deletes++
		}
	}

	if deletes != 1 {
		t.Errorf("issued %d deletes across three passes (%v), want exactly one", deletes, store.calls)
	}

	cleared := 0

	for _, reason := range h.events.normal {
		if reason == reasonStateCleared {
			cleared++
		}
	}

	if cleared != 1 {
		t.Errorf("emitted %d %s events, want exactly one", cleared, reasonStateCleared)
	}
}

func TestARecreatedRecordIsNotMistakenForTheDeletedOne(t *testing.T) {
	// The memory of a removal is keyed by UID, so a fresh object under the same
	// name is still acted on.
	const failed = "worker-3"

	store := newStore()
	store.keepAfterDelete = true
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())
	h.failPeer(t.Context(), failed)
	h.settle(t.Context())
	store.calls = nil

	for i := range store.states {
		if store.states[i].Name == failed {
			store.states[i].UID = "cr-" + failed + "-recreated"
		}
	}

	h.settle(t.Context())

	if !slices.Equal(store.calls, []string{"delete:" + failed + ":cr-" + failed + "-recreated"}) {
		t.Errorf("calls = %v, want the recreated object removed on its own merits", store.calls)
	}
}
