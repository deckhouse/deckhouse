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

package fallback

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

const (
	nodeName  = "worker-1"
	nodeUID   = "uid-worker-1"
	heartbeat = time.Second
)

var group = []string{"worker-1", "worker-2", "worker-3"}

type stubAlive struct{ members []string }

func (s *stubAlive) Members() []string { return s.members }

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
	now      func() time.Time
	states   []v1alpha1.FencingFailedNodeState
	calls    []string
	sections []v1alpha1.FencingFailedNodeStateFallback

	failHeartbeat     error
	failCreate        error
	failList          error
	failDelete        error
	keepAfterDelete   bool
	vanishAfterCreate bool
}

func newStore(states ...v1alpha1.FencingFailedNodeState) *stubStore {
	return &stubStore{states: states}
}

func (s *stubStore) List(_ context.Context) ([]v1alpha1.FencingFailedNodeState, error) {
	if s.failList != nil {
		return nil, s.failList
	}

	return slices.Clone(s.states), nil
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

	if s.vanishAfterCreate {
		s.vanishAfterCreate = false

		return true, nil
	}

	s.states = append(s.states, v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name:              peer.Name,
			UID:               types.UID("cr-" + peer.Name),
			CreationTimestamp: metav1.NewTime(s.now()),
		},
	})

	return true, nil
}

func (s *stubStore) Heartbeat(_ context.Context, name string, section v1alpha1.FencingFailedNodeStateFallback) error {
	s.calls = append(s.calls, "heartbeat:"+name)

	if s.failHeartbeat != nil {
		return s.failHeartbeat
	}

	for i := range s.states {
		if s.states[i].Name != name {
			continue
		}

		s.states[i].Status.Fallback = section.DeepCopy()
		s.sections = append(s.sections, section)

		return nil
	}

	return apierrors.NewNotFound(schema.GroupResource{Group: v1alpha1.GroupVersion.Group, Resource: "fencingfailednodestates"}, name)
}

func (s *stubStore) Delete(_ context.Context, name string, uid types.UID) error {
	s.calls = append(s.calls, "delete:"+name+":"+string(uid))

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
	monitor  *Monitor
	alive    *stubAlive
	expected *stubExpected
	store    *stubStore
	events   *stubEvents
	clock    *clock
}

func peers(names ...string) []domain.Peer {
	out := make([]domain.Peer, 0, len(names))

	for _, name := range names {
		out = append(out, domain.Peer{Name: name, IP: "10.0.0.1", UID: "uid-" + name})
	}

	return out
}

func newHarness(t *testing.T, store *stubStore) *harness {
	t.Helper()

	h := &harness{
		alive:    &stubAlive{members: slices.Clone(group)},
		expected: &stubExpected{peers: peers(group...)},
		store:    store,
		events:   &stubEvents{},
		clock:    &clock{now: time.Date(2026, 6, 2, 15, 0, 0, 0, time.UTC)},
	}
	store.now = h.clock.Now

	h.monitor = New(
		Params{
			Node:       domain.NodeIdentity{Name: nodeName, UID: nodeUID, IP: "10.0.0.1"},
			Heartbeat:  heartbeat,
			APITimeout: time.Second,
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

func (h *harness) loseQuorum(ctx context.Context) {
	h.alive.members = []string{nodeName}
	h.monitor.reconcile(ctx)
}

func (h *harness) regainQuorum(ctx context.Context) {
	h.alive.members = slices.Clone(group)
	h.monitor.reconcile(ctx)
}

func deletes(calls []string) int {
	n := 0

	for _, call := range calls {
		if strings.HasPrefix(call, "delete:") {
			n++
		}
	}

	return n
}

func TestGateIsOpenBeforeTheFirstPass(t *testing.T) {
	h := newHarness(t, newStore())

	if feed, reason := h.monitor.ShouldFeed(); !feed || reason == "" {
		t.Errorf("ShouldFeed = %v %q before the first pass, want open with a reason", feed, reason)
	}

	if s := h.monitor.Snapshot(); s.Observed || !s.APIReachable {
		t.Errorf("snapshot = %+v before the first pass, want unobserved with the API presumed reachable", s)
	}
}

func TestQuorumLossEntersFallbackAndWritesAHeartbeat(t *testing.T) {
	h := newHarness(t, newStore())

	h.loseQuorum(t.Context())

	want := []string{"heartbeat:" + nodeName, "create:" + nodeName, "heartbeat:" + nodeName}
	if !slices.Equal(h.store.calls, want) {
		t.Fatalf("calls = %v, want %v", h.store.calls, want)
	}

	section := h.store.sections[0]
	if !section.Active || !section.APIReachable {
		t.Errorf("section = %+v, want active and apiReachable", section)
	}

	if section.HeartbeatInterval.Duration != heartbeat {
		t.Errorf("heartbeatInterval = %s, want %s", section.HeartbeatInterval.Duration, heartbeat)
	}

	if section.LastHeartbeatAt == nil || !section.LastHeartbeatAt.Time.Equal(h.clock.now) {
		t.Errorf("lastHeartbeatAt = %v, want the clock %v", section.LastHeartbeatAt, h.clock.now)
	}

	if section.QuorumLostAt == nil || !section.QuorumLostAt.Time.Equal(h.clock.now) {
		t.Errorf("quorumLostAt = %v, want the clock %v", section.QuorumLostAt, h.clock.now)
	}

	if !slices.Equal(h.events.normal, []string{reasonEntered}) {
		t.Errorf("events = %v, want %s", h.events.normal, reasonEntered)
	}

	snapshot := h.monitor.Snapshot()
	if !snapshot.Active || snapshot.HasQuorum || !snapshot.APIReachable || snapshot.Alive != 1 || snapshot.Quorum != 2 {
		t.Errorf("snapshot = %+v", snapshot)
	}

	if feed, _ := h.monitor.ShouldFeed(); !feed {
		t.Error("the gate must stay open while the heartbeat reaches the API")
	}
}

func TestHeartbeatIsPacedByTheProfile(t *testing.T) {
	h := newHarness(t, newStore())

	h.loseQuorum(t.Context())
	h.monitor.reconcile(t.Context())

	if len(h.store.sections) != 1 {
		t.Fatalf("a second pass within the interval wrote %d heartbeats, want the first one only", len(h.store.sections))
	}

	h.clock.advance(heartbeat)
	h.monitor.reconcile(t.Context())

	if len(h.store.sections) != 2 {
		t.Errorf("after one interval there are %d heartbeats, want 2", len(h.store.sections))
	}
}

func TestQuorumLostAtIsFixedForTheEpisode(t *testing.T) {
	h := newHarness(t, newStore())

	h.loseQuorum(t.Context())
	lostAt := h.clock.now

	h.clock.advance(heartbeat)
	h.monitor.reconcile(t.Context())

	if got := h.store.sections[1].QuorumLostAt; !got.Time.Equal(lostAt) {
		t.Errorf("quorumLostAt moved to %v, want the first loss %v", got, lostAt)
	}

	h.clock.advance(heartbeat)
	h.regainQuorum(t.Context())
	h.clock.advance(heartbeat)
	h.loseQuorum(t.Context())

	if got := h.store.sections[2].QuorumLostAt; !got.Time.Equal(h.clock.now) {
		t.Errorf("a new episode kept quorumLostAt %v, want %v", got, h.clock.now)
	}
}

func TestHeartbeatFailureClosesTheGateUntilTheNextSuccess(t *testing.T) {
	store := newStore()
	store.failHeartbeat = errors.New("dial tcp: i/o timeout")
	h := newHarness(t, store)

	h.loseQuorum(t.Context())

	feed, reason := h.monitor.ShouldFeed()
	if feed || !strings.Contains(reason, "unreachable") {
		t.Fatalf("ShouldFeed = %v %q after a failed heartbeat, want closed with the reason", feed, reason)
	}

	store.failHeartbeat = nil
	h.clock.advance(heartbeat)
	h.monitor.reconcile(t.Context())

	if feed, _ := h.monitor.ShouldFeed(); !feed {
		t.Error("the gate must reopen on the first heartbeat that lands")
	}
}

func TestRepeatedFailuresRaiseOneWarning(t *testing.T) {
	store := newStore()
	store.failHeartbeat = errors.New("dial tcp: i/o timeout")
	h := newHarness(t, store)

	for range maxFailures + 2 {
		h.loseQuorum(t.Context())
		h.clock.advance(heartbeat)
	}

	if !slices.Equal(h.events.warnings, []string{reasonHeartbeatFailed}) {
		t.Errorf("warnings = %v, want exactly one %s", h.events.warnings, reasonHeartbeatFailed)
	}
}

func TestQuorumBackDeletesTheRecordAndLeavesFallback(t *testing.T) {
	h := newHarness(t, newStore())

	h.loseQuorum(t.Context())
	h.clock.advance(heartbeat)
	h.regainQuorum(t.Context())

	if !slices.Contains(h.store.calls, "delete:"+nodeName+":cr-"+nodeName) {
		t.Errorf("calls = %v, want the own record deleted by uid", h.store.calls)
	}

	if len(h.store.states) != 0 {
		t.Errorf("record still present: %+v", h.store.states)
	}

	if !slices.Equal(h.events.normal, []string{reasonEntered, reasonLeft}) {
		t.Errorf("events = %v, want entered then left", h.events.normal)
	}

	if s := h.monitor.Snapshot(); s.Active || !s.HasQuorum || !s.QuorumLostAt.IsZero() {
		t.Errorf("snapshot = %+v, want inactive with quorum", s)
	}
}

func TestRecordOfAnEarlierLifeIsNotTouched(t *testing.T) {
	store := newStore(v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name:              nodeName,
			UID:               "cr-old",
			CreationTimestamp: metav1.NewTime(time.Date(2026, 6, 2, 14, 59, 0, 0, time.UTC)),
		},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Fallback: &v1alpha1.FencingFailedNodeStateFallback{Active: true, APIReachable: true},
		},
	})
	h := newHarness(t, store)

	h.monitor.reconcile(t.Context())

	if len(h.store.calls) != 0 {
		t.Errorf("calls = %v, want none for a record of an earlier life", h.store.calls)
	}
}

func TestPeersRecordWithoutFallbackIsNotTouched(t *testing.T) {
	store := newStore(v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name:              nodeName,
			UID:               "cr-peers",
			CreationTimestamp: metav1.NewTime(time.Date(2026, 6, 2, 15, 0, 0, 0, time.UTC)),
		},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: "worker-2", AliveCount: 2, QuorumSize: 2},
		},
	})
	h := newHarness(t, store)

	h.monitor.reconcile(t.Context())

	if len(h.store.calls) != 0 {
		t.Errorf("calls = %v, want none for a record written by peers", h.store.calls)
	}
}

func TestRecordIsDeletedOnceWhileTheInformerLags(t *testing.T) {
	store := newStore()
	store.keepAfterDelete = true
	h := newHarness(t, store)

	h.loseQuorum(t.Context())
	h.clock.advance(heartbeat)
	h.regainQuorum(t.Context())
	h.clock.advance(idleTick)
	h.regainQuorum(t.Context())

	if got := deletes(h.store.calls); got != 1 {
		t.Errorf("record deleted %d times, want once", got)
	}
}

func TestMissingRecordIsRecreatedOnTheNextBeat(t *testing.T) {
	h := newHarness(t, newStore())

	h.loseQuorum(t.Context())

	h.store.states = nil
	h.store.calls = nil

	h.clock.advance(heartbeat)
	h.monitor.reconcile(t.Context())

	want := []string{"heartbeat:" + nodeName, "create:" + nodeName, "heartbeat:" + nodeName}
	if !slices.Equal(h.store.calls, want) {
		t.Errorf("calls = %v, want %v", h.store.calls, want)
	}
}

func TestListFailureDoesNotStopTheHeartbeat(t *testing.T) {
	store := newStore()
	store.failList = errors.New("cache not synced")
	h := newHarness(t, store)

	h.loseQuorum(t.Context())

	if len(h.store.sections) != 1 {
		t.Fatalf("heartbeats = %d, want one even though the list failed", len(h.store.sections))
	}

	h.clock.advance(heartbeat)
	h.regainQuorum(t.Context())

	if deletes(h.store.calls) != 0 {
		t.Errorf("calls = %v, want no delete while the list fails", h.store.calls)
	}

	if s := h.monitor.Snapshot(); s.Active {
		t.Errorf("snapshot = %+v, want fallback left even though the record stayed", s)
	}

	h.clock.advance(heartbeat)
	h.loseQuorum(t.Context())

	if len(h.store.sections) != 2 {
		t.Errorf("heartbeats = %d after a new quorum loss, want 2", len(h.store.sections))
	}
}

func TestCreateFailureClosesTheGate(t *testing.T) {
	store := newStore()
	store.failCreate = errors.New("dial tcp: i/o timeout")
	h := newHarness(t, store)

	h.loseQuorum(t.Context())

	if feed, _ := h.monitor.ShouldFeed(); feed {
		t.Error("the gate must close when the record cannot be created")
	}

	if s := h.monitor.Snapshot(); s.APIReachable {
		t.Errorf("snapshot = %+v, want the API unreachable after a failed create", s)
	}

	if len(h.store.sections) != 0 {
		t.Errorf("heartbeats = %d, want none without a record", len(h.store.sections))
	}
}

func TestDeleteFailureIsRetriedNextPass(t *testing.T) {
	store := newStore()
	h := newHarness(t, store)

	h.loseQuorum(t.Context())
	h.clock.advance(heartbeat)

	store.failDelete = errors.New("dial tcp: i/o timeout")
	h.regainQuorum(t.Context())

	if deletes(h.store.calls) != 1 || len(h.store.states) != 1 {
		t.Fatalf("calls = %v, records = %d, want one failed delete with the record kept", h.store.calls, len(h.store.states))
	}

	store.failDelete = nil
	h.clock.advance(idleTick)
	h.monitor.reconcile(t.Context())

	if deletes(h.store.calls) != 2 {
		t.Errorf("calls = %v, want the delete retried on the next pass", h.store.calls)
	}

	if len(h.store.states) != 0 {
		t.Errorf("record still present: %+v", h.store.states)
	}
}

func TestRecordRemovedBetweenCreateAndHeartbeatKeepsTheGateOpen(t *testing.T) {
	store := newStore()
	store.vanishAfterCreate = true
	h := newHarness(t, store)

	h.loseQuorum(t.Context())

	if feed, reason := h.monitor.ShouldFeed(); !feed {
		t.Errorf("ShouldFeed = %v %q, want open: a NotFound is an answer from the API", feed, reason)
	}

	if s := h.monitor.Snapshot(); !s.APIReachable {
		t.Errorf("snapshot = %+v, want the API reachable", s)
	}

	if h.monitor.failures != 0 {
		t.Errorf("failures = %d, want none counted for a record a peer removed", h.monitor.failures)
	}

	if len(h.store.sections) != 0 {
		t.Fatalf("heartbeats = %d, want none landed on the vanished record", len(h.store.sections))
	}

	h.clock.advance(heartbeat)
	h.monitor.reconcile(t.Context())

	if len(h.store.sections) != 1 {
		t.Errorf("heartbeats = %d after the next beat, want the record recreated and written", len(h.store.sections))
	}
}

func TestAdoptedPeersRecordIsRemovedOnQuorum(t *testing.T) {
	h := newHarness(t, newStore())
	h.store.states = append(h.store.states, v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name:              nodeName,
			UID:               "cr-peers",
			CreationTimestamp: metav1.NewTime(h.clock.now),
		},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: "worker-2", AliveCount: 2, QuorumSize: 2},
		},
	})

	h.loseQuorum(t.Context())

	if want := []string{"heartbeat:" + nodeName}; !slices.Equal(h.store.calls, want) {
		t.Fatalf("calls = %v, want %v: the heartbeat adopts the record", h.store.calls, want)
	}

	if h.store.states[0].Status.Fallback == nil {
		t.Fatal("the adopted record carries no fallback section")
	}

	h.clock.advance(heartbeat)
	h.regainQuorum(t.Context())

	if !slices.Contains(h.store.calls, "delete:"+nodeName+":cr-peers") {
		t.Errorf("calls = %v, want the adopted record deleted by uid", h.store.calls)
	}

	if len(h.store.states) != 0 {
		t.Errorf("record still present: %+v", h.store.states)
	}
}
