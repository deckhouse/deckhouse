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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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
	lists int

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
	s.lists++

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
	synced   bool
	inGroup  bool
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

	return newLoggedHarness(t, store, log.NewNop())
}

func newLoggedHarness(t *testing.T, store *stubStore, logger *log.Logger) *harness {
	t.Helper()

	h := &harness{
		alive:    &stubAlive{members: slices.Clone(group)},
		expected: &stubExpected{peers: peers(group...)},
		store:    store,
		events:   &stubEvents{},
		clock:    &clock{now: time.Date(2026, 6, 2, 15, 0, 0, 0, time.UTC)},
		synced:   true,
		inGroup:  true,
	}
	store.now = h.clock.Now

	h.monitor = New(
		Params{
			Node:       domain.NodeIdentity{Name: nodeName, UID: nodeUID, IP: "10.0.0.1"},
			Heartbeat:  heartbeat,
			APITimeout: time.Second,
		},
		Deps{
			Alive:       h.alive,
			Expected:    h.expected,
			States:      h.store,
			Events:      h.events,
			Now:         h.clock.Now,
			CacheSynced: func() bool { return h.synced },
			InNodeGroup: func() bool { return h.inGroup },
		},
		logger,
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

func (h *harness) leaveNodeGroup(ctx context.Context) {
	h.expected.peers = peers("worker-2", "worker-3")
	h.inGroup = false
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

type logLine struct {
	Level    string `json:"level"`
	Msg      string `json:"msg"`
	Attempt  int    `json:"attempt"`
	Failures int    `json:"failures"`
}

func drainLogs(t *testing.T, logs *bytes.Buffer) []logLine {
	t.Helper()

	var lines []logLine

	dec := json.NewDecoder(logs)

	for dec.More() {
		var line logLine
		if err := dec.Decode(&line); err != nil {
			t.Fatalf("log output is not JSON lines: %v", err)
		}

		lines = append(lines, line)
	}

	return lines
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

func TestFailuresAfterTheErrorAreLoggedAtDebugUntilTheStreakEnds(t *testing.T) {
	const streak = maxFailures + 3

	unreachable := errors.New("dial tcp: i/o timeout")
	logs := &bytes.Buffer{}
	store := newStore()
	store.failHeartbeat = unreachable
	h := newLoggedHarness(t, store, log.NewLogger(
		log.WithOutput(logs),
		log.WithHandlerType(log.JSONHandlerType),
		log.WithLevel(slog.LevelDebug),
	))

	h.loseQuorum(t.Context())

	for range streak - 1 {
		h.clock.advance(heartbeat)
		h.monitor.reconcile(t.Context())
	}

	var levels []string

	for _, line := range drainLogs(t, logs) {
		if strings.HasPrefix(line.Msg, "fallback heartbeat") {
			levels = append(levels, line.Level)
		}
	}

	want := slices.Concat(
		slices.Repeat([]string{"warn"}, maxFailures-1),
		[]string{"error"},
		slices.Repeat([]string{"debug"}, streak-maxFailures),
	)
	if !slices.Equal(levels, want) {
		t.Errorf("%d failures in a row were logged at %v, want %v", streak, levels, want)
	}

	if !slices.Equal(h.events.warnings, []string{reasonHeartbeatFailed}) {
		t.Errorf("warnings = %v, want exactly one %s", h.events.warnings, reasonHeartbeatFailed)
	}

	store.failHeartbeat = nil
	h.clock.advance(heartbeat)
	h.monitor.reconcile(t.Context())

	recovered := []logLine{{Level: "info", Msg: "fallback heartbeat reaches the Kubernetes API again", Failures: streak}}
	if got := drainLogs(t, logs); !slices.Equal(got, recovered) {
		t.Errorf("logs = %+v when the streak ended, want %+v", got, recovered)
	}

	store.failHeartbeat = unreachable
	h.clock.advance(heartbeat)
	h.monitor.reconcile(t.Context())

	restarted := []logLine{{Level: "warn", Msg: "fallback heartbeat failed", Attempt: 1}}
	if got := drainLogs(t, logs); !slices.Equal(got, restarted) {
		t.Errorf("logs = %+v on the first failure of a new streak, want %+v", got, restarted)
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

// The three tests below pin the monitor's independence from the
// FencingFailedNodeState informer. That cache never syncs while the Kubernetes
// API is unreachable, and the watchdog's feed gate is this monitor's verdict: a
// monitor that waits for the cache leaves the gate open on a node the cluster has
// already given up on.

func TestFirstPassObservesWithoutTheInformerCache(t *testing.T) {
	h := newHarness(t, newStore())
	h.synced = false

	h.monitor.reconcile(t.Context())

	if s := h.monitor.Snapshot(); !s.Observed || !s.HasQuorum {
		t.Errorf("snapshot = %+v after one pass with no cache, want observed with quorum", s)
	}

	if feed, reason := h.monitor.ShouldFeed(); !feed || reason != "" {
		t.Errorf("ShouldFeed = %v %q with quorum, want open on the gossip verdict", feed, reason)
	}
}

func TestGateClosesWithoutTheInformerCache(t *testing.T) {
	store := newStore()
	store.failHeartbeat = errors.New("dial tcp: i/o timeout")
	store.failList = errors.New("dial tcp: i/o timeout")

	h := newHarness(t, store)
	h.synced = false

	h.loseQuorum(t.Context())

	feed, reason := h.monitor.ShouldFeed()
	if feed || !strings.Contains(reason, "unreachable") {
		t.Fatalf("ShouldFeed = %v %q with no quorum, no cache and no API, want closed so the watchdog starves", feed, reason)
	}
}

// The record only protects this Node from evacuation, so leaving it a few passes
// longer is free. Reading an unsynced cache is not: the read blocks until its own
// timeout, once per pass.
func TestOwnRecordIsRemovedOnceTheCacheSyncs(t *testing.T) {
	h := newHarness(t, newStore())
	h.synced = false

	h.loseQuorum(t.Context())
	h.clock.advance(heartbeat)
	h.regainQuorum(t.Context())

	if h.store.lists != 0 {
		t.Fatalf("the store was read %d times before the cache synced, want none", h.store.lists)
	}

	if len(h.store.states) != 1 {
		t.Fatalf("states = %+v, want the record still there while the cache is unsynced", h.store.states)
	}

	h.synced = true
	h.clock.advance(heartbeat)
	h.monitor.reconcile(t.Context())

	if !slices.Contains(h.store.calls, "delete:"+nodeName+":cr-"+nodeName) {
		t.Errorf("calls = %v, want the own record deleted once the cache synced", h.store.calls)
	}
}


func TestStaleVerdictClosesTheGate(t *testing.T) {
	h := newHarness(t, newStore())

	h.monitor.reconcile(t.Context())

	if feed, _ := h.monitor.ShouldFeed(); !feed {
		t.Fatal("a fresh verdict of quorum must keep the gate open")
	}

	h.clock.advance(h.monitor.verdictMaxAge() + time.Second)

	feed, reason := h.monitor.ShouldFeed()
	if feed {
		t.Error("a verdict past its age must close the gate, whatever it said")
	}

	if !strings.Contains(reason, "no longer knows whether it has quorum") {
		t.Errorf("reason = %q, want it to name the lost verdict", reason)
	}
}

// The dangerous stale verdict is "quorum held": it is the one that keeps feeding.
func TestStaleQuorumVerdictDoesNotOutrankItsAge(t *testing.T) {
	h := newHarness(t, newStore())

	h.monitor.reconcile(t.Context())

	if s := h.monitor.Snapshot(); !s.HasQuorum {
		t.Fatalf("snapshot = %+v, want the quorum this test goes stale on", s)
	}

	h.clock.advance(h.monitor.verdictMaxAge() + time.Second)

	if feed, _ := h.monitor.ShouldFeed(); feed {
		t.Error("a stale quorum verdict must not keep the gate open")
	}
}

func TestEveryPassRefreshesTheVerdict(t *testing.T) {
	h := newHarness(t, newStore())

	for range 5 {
		h.clock.advance(h.monitor.verdictMaxAge())
		h.monitor.reconcile(t.Context())

		if feed, reason := h.monitor.ShouldFeed(); !feed {
			t.Fatalf("ShouldFeed = %v %q right after a pass, want open", feed, reason)
		}
	}
}

// Tripping this resets the node, so the budget has to clear the slowest pass the
// loop can have by a wide margin.
func TestVerdictMaxAgeClearsTheSlowestPass(t *testing.T) {
	profiles := map[string]struct {
		heartbeat, apiTimeout, watchdogTimeout time.Duration
	}{
		"critical": {250 * time.Millisecond, 500 * time.Millisecond, 3 * time.Second},
		"medium":   {time.Second, 2 * time.Second, 10 * time.Second},
		"moderate": {2 * time.Second, 5 * time.Second, 30 * time.Second},
		"slow":     {5 * time.Second, 10 * time.Second, 60 * time.Second},
	}

	for name, p := range profiles {
		t.Run(name, func(t *testing.T) {
			m := New(
				Params{
					Node:            domain.NodeIdentity{Name: nodeName, UID: nodeUID},
					Heartbeat:       p.heartbeat,
					APITimeout:      p.apiTimeout,
					WatchdogTimeout: p.watchdogTimeout,
				},
				Deps{},
				log.NewNop(),
			)

			if got, want := m.verdictMaxAge(), maxPasses*m.pass(); got < want {
				t.Errorf("verdictMaxAge = %s, want at least %d passes of %s", got, maxPasses, m.pass())
			}

			if got := m.verdictMaxAge(); got < p.watchdogTimeout {
				t.Errorf("verdictMaxAge = %s, want at least the kernel deadline %s", got, p.watchdogTimeout)
			}
		})
	}
}

func TestNoFallbackRecordIsWrittenAfterLeavingTheNodeGroup(t *testing.T) {
	h := newHarness(t, newStore())

	h.alive.members = []string{nodeName}
	h.leaveNodeGroup(t.Context())

	if len(h.store.calls) != 0 {
		t.Errorf("calls = %v, want no writes into the group this node left", h.store.calls)
	}

	if snapshot := h.monitor.Snapshot(); snapshot.Active {
		t.Errorf("snapshot = %+v, want no fallback mode outside the NodeGroup", snapshot)
	}

	if len(h.events.normal) != 0 {
		t.Errorf("events = %v, want no fallback events for a group this node is not in", h.events.normal)
	}

	// Fallback paces the loop by the heartbeat, and nothing advances it here.
	if wait := h.monitor.wait(); wait != idleTick {
		t.Errorf("wait = %s, want the idle tick %s instead of a spin", wait, idleTick)
	}
}

func TestTheOwnFallbackRecordIsRemovedAfterLeavingTheNodeGroup(t *testing.T) {
	h := newHarness(t, newStore())

	h.loseQuorum(t.Context())

	if len(h.store.states) != 1 {
		t.Fatalf("states = %v, want the fallback record this node wrote while it was a member", h.store.states)
	}

	h.clock.advance(heartbeat)
	h.leaveNodeGroup(t.Context())

	if deletes(h.store.calls) != 1 {
		t.Errorf("calls = %v, want the own record taken back exactly once", h.store.calls)
	}

	if len(h.store.states) != 0 {
		t.Errorf("states = %v, want the record of a node that left the group removed", h.store.states)
	}

	// A second pass must not write it back or delete it twice.
	before := len(h.store.calls)

	h.clock.advance(heartbeat)
	h.monitor.reconcile(t.Context())

	if len(h.store.calls) != before {
		t.Errorf("calls = %v, want nothing after the record is gone", h.store.calls[before:])
	}
}

func TestTheFallbackMonitorResumesAfterTheNodeIsRelabeledBack(t *testing.T) {
	h := newHarness(t, newStore())

	h.alive.members = []string{nodeName}
	h.leaveNodeGroup(t.Context())

	h.expected.peers = peers(group...)
	h.inGroup = true

	h.clock.advance(heartbeat)
	h.monitor.reconcile(t.Context())

	want := []string{"heartbeat:" + nodeName, "create:" + nodeName, "heartbeat:" + nodeName}
	if !slices.Equal(h.store.calls, want) {
		t.Fatalf("calls = %v, want %v once the Node is back in its group", h.store.calls, want)
	}

	if snapshot := h.monitor.Snapshot(); !snapshot.Active {
		t.Errorf("snapshot = %+v, want fallback mode back with the membership", snapshot)
	}
}
