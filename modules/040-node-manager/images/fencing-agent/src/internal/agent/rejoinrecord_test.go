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

package agent

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"math/bits"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/usecase/failedstate"
	"fencing-agent/internal/usecase/fallback"
	"fencing-agent/internal/usecase/rejoin"
)

const (
	roleWriter  = "writer"
	roleMonitor = "monitor"

	opCreate     = "create"
	opMarkFailed = "mark_failed"
	opDelete     = "delete"
)

type recordOp struct {
	at         time.Duration
	agent      string
	role       string
	op         string
	name       string
	uid        types.UID
	fallback   bool
	failed     bool
	detectedAt time.Time
}

func (op recordOp) String() string {
	detectedAt := "none"
	if op.failed {
		detectedAt = op.detectedAt.UTC().Format(time.StampMicro)
	}

	return fmt.Sprintf("%s %s/%s %s %s uid=%s fallback=%t failed=%t detected_at=%s",
		op.at, op.agent, op.role, op.op, op.name, op.uid, op.fallback, op.failed, detectedAt)
}

type recordOps struct {
	mu    sync.Mutex
	start time.Time
	ops   []recordOp
}

func (l *recordOps) add(agent, role, op, name string, record v1alpha1.FencingFailedNodeState) {
	entry := recordOp{
		at:       time.Since(l.start),
		agent:    agent,
		role:     role,
		op:       op,
		name:     name,
		uid:      record.UID,
		fallback: record.Status.Fallback != nil,
		failed:   record.Status.Failed != nil,
	}

	if record.Status.Failed != nil {
		entry.detectedAt = record.Status.Failed.DetectedAt.Time
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.ops = append(l.ops, entry)
}

func (l *recordOps) where(match func(op recordOp) bool) []recordOp {
	l.mu.Lock()
	defer l.mu.Unlock()

	return slices.DeleteFunc(slices.Clone(l.ops), func(op recordOp) bool { return !match(op) })
}

func (a *recordAPI) record(name string) (v1alpha1.FencingFailedNodeState, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	record, ok := a.records[name]

	return *record.DeepCopy(), ok
}

func (a *recordAPI) seed(record v1alpha1.FencingFailedNodeState) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if record.UID == "" {
		a.uids++
		record.UID = types.UID(fmt.Sprintf("uid-%d", a.uids))
	}

	a.records[record.Name] = *record.DeepCopy()
}

func (a *recordAPI) remove(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.records, name)
}

func (a *recordAPI) deletesBy(label string) int {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.deletes[label]
}

type loggedClient struct {
	recordClient

	node string
	role string
	ops  *recordOps
}

func newLoggedClient(api *recordAPI, ops *recordOps, node, role string) loggedClient {
	label := node
	if role != roleWriter {
		label = node + "/" + role
	}

	return loggedClient{recordClient: recordClient{api: api, agent: label}, node: node, role: role, ops: ops}
}

func (c loggedClient) Create(ctx context.Context, peer domain.Peer) (bool, error) {
	created, err := c.recordClient.Create(ctx, peer)
	if created {
		record, _ := c.api.record(peer.Name)
		c.ops.add(c.node, c.role, opCreate, peer.Name, record)
	}

	return created, err
}

func (c loggedClient) MarkFailed(ctx context.Context, name string, failed v1alpha1.FencingFailedNodeStateFailed) (bool, error) {
	recorded, err := c.recordClient.MarkFailed(ctx, name, failed)
	if recorded {
		record, _ := c.api.record(name)
		c.ops.add(c.node, c.role, opMarkFailed, name, record)
	}

	return recorded, err
}

func (c loggedClient) Delete(ctx context.Context, name string, uid types.UID) error {
	record, _ := c.api.record(name)
	deletes := c.api.deletesBy(c.agent)

	err := c.recordClient.Delete(ctx, name, uid)
	if c.api.deletesBy(c.agent) > deletes {
		c.ops.add(c.node, c.role, opDelete, name, record)
	}

	return err
}

type rejoinAttempt struct {
	at     time.Duration
	quorum bool
}

func (a rejoinAttempt) String() string {
	return fmt.Sprintf("%s quorum=%t", a.at, a.quorum)
}

type rejoinAttempts struct {
	mu       sync.Mutex
	attempts []rejoinAttempt
}

func (r *rejoinAttempts) add(attempt rejoinAttempt) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.attempts = append(r.attempts, attempt)
}

func (r *rejoinAttempts) where(match func(attempt rejoinAttempt) bool) []rejoinAttempt {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.DeleteFunc(slices.Clone(r.attempts), func(attempt rejoinAttempt) bool { return !match(attempt) })
}

type recordStep struct {
	at    time.Duration
	apply func(h *recordHarness)
}

type recordScenario struct {
	group      []string
	writers    map[string]time.Duration
	x          string
	monitorAt  time.Duration
	rejoinAt   time.Duration
	writerLogs map[string]io.Writer
	steps      []recordStep
	until      time.Duration
}

type recordLoop struct {
	at  time.Duration
	run func(ctx context.Context)
}

func checkRecordTimeline(t *testing.T, s recordScenario, loops []recordLoop) {
	t.Helper()

	fraction := func(at time.Duration) time.Duration { return at % time.Second }

	for i, loop := range loops {
		for _, other := range loops[i+1:] {
			if fraction(loop.at) == fraction(other.at) {
				t.Fatalf("the loops started at %s and %s pass at the same instants, want every loop on its own fraction of a second", loop.at, other.at)
			}
		}

		for _, step := range s.steps {
			if fraction(step.at) == fraction(loop.at) {
				t.Fatalf("the step at %s falls on a pass of the loop started at %s, want steps between the passes", step.at, loop.at)
			}
		}
	}

	for i, step := range s.steps {
		if i > 0 && step.at <= s.steps[i-1].at {
			t.Fatalf("the step at %s follows the step at %s, want steps in the order of their instants", step.at, s.steps[i-1].at)
		}

		if step.at >= s.until {
			t.Fatalf("the step at %s is not before the end of the scenario at %s", step.at, s.until)
		}
	}
}

const recordTakeoverDelay = 10 * time.Second

type recordHarness struct {
	start    time.Time
	api      *recordAPI
	expected recordExpected
	views    map[string]*recordGossip
	ops      *recordOps
	attempts *rejoinAttempts
}

func runRecordHarness(t *testing.T, s recordScenario) *recordHarness {
	t.Helper()

	start := time.Now()
	startedAt := failedstate.StartOfLife(start)

	h := &recordHarness{
		start:    start,
		api:      newRecordAPI(),
		expected: make(recordExpected, 0, len(s.group)),
		views:    make(map[string]*recordGossip, len(s.group)),
		ops:      &recordOps{start: start},
		attempts: &rejoinAttempts{},
	}

	for _, name := range s.group {
		h.expected = append(h.expected, domain.Peer{Name: name, IP: "10.0.0.1", UID: "node-uid-" + name})
		h.views[name] = &recordGossip{members: slices.Clone(s.group)}
	}

	starts := make([]recordLoop, 0, len(s.group)+2)

	for _, name := range s.group {
		at, ok := s.writers[name]
		if !ok {
			t.Fatalf("the scenario has no writer start offset for %s", name)
		}

		logger := log.NewNop()
		if w, ok := s.writerLogs[name]; ok {
			logger = newJSONLogger(w)
		}

		writer := failedstate.New(
			failedstate.Params{
				NodeName:         name,
				RetryInterval:    time.Second,
				MaxRetryInterval: 10 * time.Second,
				TakeoverDelay:    recordTakeoverDelay,
				FallbackTTL:      recordTTL,
				StartedAt:        startedAt,
			},
			failedstate.Deps{
				Alive:    h.views[name],
				Expected: h.expected,
				States:   newLoggedClient(h.api, h.ops, name, roleWriter),
				Events:   recordEvents{},
			},
			logger,
		)

		starts = append(starts, recordLoop{at: at, run: func(ctx context.Context) { _ = writer.Run(ctx) }})
	}

	if s.x != "" {
		view, ok := h.views[s.x]
		if !ok {
			t.Fatalf("agent x %s is not in the group %v", s.x, s.group)
		}

		monitor := fallback.New(
			fallback.Params{
				Node:            domain.NodeIdentity{Name: s.x, UID: "node-uid-" + s.x, IP: "10.0.0.1"},
				Heartbeat:       recordHeartbeat,
				APITimeout:      2 * time.Second,
				WatchdogTimeout: 10 * time.Second,
			},
			fallback.Deps{
				Alive:    view,
				Expected: h.expected,
				States:   newLoggedClient(h.api, h.ops, s.x, roleMonitor),
				Events:   recordEvents{},
			},
			log.NewNop(),
		)

		hasQuorum := func() bool { return domain.NewView(h.expected, view.Members()).HasQuorum() }

		rejoiner := rejoin.New(
			rejoin.Params{Interval: time.Second, MaxInterval: 10 * time.Second},
			rejoin.Deps{
				Attempt: func(context.Context) error {
					h.attempts.add(rejoinAttempt{at: time.Since(start), quorum: hasQuorum()})

					return nil
				},
				HasQuorum:       hasQuorum,
				OwnFailedRecord: ownFailedRecord(recordClient{api: h.api, agent: s.x}.List, func() bool { return true }, s.x, startedAt),
			},
			log.NewNop(),
		)

		starts = append(starts,
			recordLoop{at: s.monitorAt, run: func(ctx context.Context) { _ = monitor.Run(ctx) }},
			recordLoop{at: s.rejoinAt, run: func(ctx context.Context) { _ = rejoiner.Run(ctx) }},
		)
	}

	checkRecordTimeline(t, s, starts)

	ctx, cancel := context.WithCancel(t.Context())

	var loops sync.WaitGroup

	type event struct {
		at  time.Duration
		run func()
	}

	events := make([]event, 0, len(starts)+len(s.steps))

	for _, loop := range starts {
		events = append(events, event{at: loop.at, run: func() { loops.Go(func() { loop.run(ctx) }) }})
	}

	for _, step := range s.steps {
		events = append(events, event{at: step.at, run: func() { step.apply(h) }})
	}

	slices.SortFunc(events, func(a, b event) int { return cmp.Compare(a.at, b.at) })

	for _, e := range events {
		sleepUntil(start, e.at)
		e.run()
	}

	sleepUntil(start, s.until)
	cancel()
	loops.Wait()

	return h
}

func (h *recordHarness) logOps(t *testing.T) {
	t.Helper()

	for _, op := range h.ops.where(func(recordOp) bool { return true }) {
		t.Logf("%s", op)
	}
}

func firstPassAfter(offset, at time.Duration) time.Duration {
	if at < offset {
		return offset
	}

	return offset + (at - offset).Truncate(time.Second) + time.Second
}

func pauseOf(offset, appearedAt time.Duration) time.Duration {
	return firstPassAfter(offset, appearedAt) + recordTakeoverDelay
}

const (
	rejoinX    = "worker-1"
	rejoinPeer = "worker-2"
	rejoinLost = "worker-3"
)

func rejoinScenario(until time.Duration, steps ...recordStep) recordScenario {
	return recordScenario{
		group: []string{rejoinX, rejoinPeer, rejoinLost},
		writers: map[string]time.Duration{
			rejoinPeer: 600 * time.Millisecond,
			rejoinLost: 700 * time.Millisecond,
			rejoinX:    1200 * time.Millisecond,
		},
		x:         rejoinX,
		monitorAt: 300 * time.Millisecond,
		rejoinAt:  400 * time.Millisecond,
		steps:     steps,
		until:     until,
	}
}

const splitAt = 2500 * time.Millisecond

func splitFromTheMajority(h *recordHarness) {
	h.views[rejoinX].see(rejoinX, rejoinPeer)
	h.views[rejoinPeer].see(rejoinPeer, rejoinLost)
	h.views[rejoinLost].see(rejoinPeer, rejoinLost)
}

func TestAgentTheMajorityRecordedFailedRejoinsAndRecordsNobody(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var xLog bytes.Buffer

		s := rejoinScenario(30*time.Second, recordStep{at: splitAt, apply: splitFromTheMajority})
		s.writerLogs = map[string]io.Writer{rejoinX: &xLog}

		h := runRecordHarness(t, s)

		defer func() {
			if t.Failed() {
				h.logOps(t)
			}
		}()

		record, ok := h.api.record(rejoinX)
		if !ok || record.Status.Failed == nil {
			t.Fatalf("the record of %s exists: %t, carries failed: %t, want both", rejoinX, ok, ok && record.Status.Failed != nil)
		}

		creates := h.ops.where(func(op recordOp) bool { return op.op == opCreate && op.uid == record.UID })
		if len(creates) != 1 || creates[0].agent == rejoinX {
			t.Fatalf("the record of %s was created by %v, want once by the majority", rejoinX, creates)
		}

		pausedAt := pauseOf(s.writers[rejoinX], creates[0].at)
		if pausedAt >= s.until {
			t.Fatalf("the writer of %s pauses at %s, want before the end of the scenario at %s", rejoinX, pausedAt, s.until)
		}

		if ops := h.ops.where(func(op recordOp) bool {
			verdict := op.role == roleWriter && op.name == rejoinLost && op.op != opDelete && op.at < pausedAt

			return op.agent == rejoinX && !verdict
		}); len(ops) != 0 {
			t.Errorf("%s wrote %v, want only verdicts of its writer about %s before its pause at %s", rejoinX, ops, rejoinLost, pausedAt)
		}

		verdicts := make(map[types.UID]bool)

		for _, op := range h.ops.where(func(op recordOp) bool { return op.agent == rejoinX && op.op == opCreate }) {
			verdicts[op.uid] = true
		}

		if deletes := h.ops.where(func(op recordOp) bool {
			return op.agent == rejoinPeer && op.op == opDelete && !verdicts[op.uid]
		}); len(deletes) != 0 {
			t.Errorf("%s deleted records %v, want only the verdicts of %s", rejoinPeer, deletes, rejoinX)
		}

		if lost, ok := h.api.record(rejoinLost); ok {
			t.Errorf("a record of %s with uid %s is left, want every verdict of %s removed by the majority", rejoinLost, lost.UID, rejoinX)
		}

		if churn := h.ops.where(func(op recordOp) bool { return op.name == rejoinLost && op.at >= pausedAt }); len(churn) != 0 {
			t.Errorf("the record of %s changed after the writer of %s paused at %s: %v, want no churn", rejoinLost, rejoinX, pausedAt, churn)
		}

		logs := decodeLogs(t, xLog.String())
		if pauses, resumes := countMsg(logs, writerPausedMsg), countMsg(logs, writerResumedMsg); pauses != 1 || resumes != 0 {
			t.Errorf("the writer of %s paused %d times and resumed %d times, want one pause that lasts", rejoinX, pauses, resumes)
		}

		if attempts := h.attempts.where(func(a rejoinAttempt) bool { return a.quorum }); len(attempts) < 2 {
			t.Errorf("%s made %d rejoin attempts while it held quorum, want at least 2", rejoinX, len(attempts))
		}

		majority := make(map[types.UID]bool)

		for _, op := range h.ops.where(func(op recordOp) bool {
			return op.op == opCreate && op.name == rejoinX && op.agent != rejoinX
		}) {
			majority[op.uid] = true
		}

		if deletes := h.ops.where(func(op recordOp) bool {
			return op.agent == rejoinX && op.role == roleMonitor && op.op == opDelete && majority[op.uid]
		}); len(deletes) != 0 {
			t.Errorf("the monitor of %s deleted the record the majority wrote: %v", rejoinX, deletes)
		}
	})
}

func TestHealedAgentLeavesRejoinAndResumesWriting(t *testing.T) {
	alive := []string{rejoinX, rejoinPeer}

	if rank := domain.WriterRank(alive, rejoinLost, rejoinX); rank != 0 {
		t.Fatalf("%s has writer rank %d for %s among %v, want 0", rejoinX, rank, rejoinLost, alive)
	}

	synctest.Test(t, func(t *testing.T) {
		const (
			healAt       = 20 * time.Second
			lostVanishes = 25 * time.Second
		)

		var xLog bytes.Buffer

		s := rejoinScenario(45*time.Second,
			recordStep{at: splitAt, apply: splitFromTheMajority},
			recordStep{at: healAt, apply: func(h *recordHarness) {
				for _, name := range []string{rejoinX, rejoinPeer, rejoinLost} {
					h.views[name].see(rejoinX, rejoinPeer, rejoinLost)
				}
			}},
			recordStep{at: lostVanishes, apply: func(h *recordHarness) {
				h.views[rejoinX].see(rejoinX, rejoinPeer)
				h.views[rejoinPeer].see(rejoinX, rejoinPeer)
				h.views[rejoinLost].see(rejoinLost)
			}},
		)
		s.writerLogs = map[string]io.Writer{rejoinX: &xLog}

		h := runRecordHarness(t, s)

		defer func() {
			if t.Failed() {
				h.logOps(t)
			}
		}()

		recorded := h.ops.where(func(op recordOp) bool {
			return op.op == opCreate && op.name == rejoinX && op.agent != rejoinX
		})
		if len(recorded) == 0 {
			t.Fatalf("the majority never recorded %s after the split at %s, the scenario did not start", rejoinX, splitAt)
		}

		pausedAt := pauseOf(s.writers[rejoinX], recorded[0].at)
		if pausedAt >= healAt {
			t.Fatalf("the writer of %s pauses at %s, want before the views heal at %s", rejoinX, pausedAt, healAt)
		}

		cleared := h.ops.where(func(op recordOp) bool {
			return op.op == opDelete && op.name == rejoinX && op.agent != rejoinX
		})
		if len(cleared) == 0 {
			t.Fatalf("the majority never deleted the record of %s after the views healed", rejoinX)
		}

		deletedAt := cleared[0].at

		if before := h.attempts.where(func(a rejoinAttempt) bool { return a.at < deletedAt }); len(before) == 0 {
			t.Fatalf("%s made no rejoin attempt before its record was deleted at %s, the scenario did not start", rejoinX, deletedAt)
		}

		if after := h.attempts.where(func(a rejoinAttempt) bool { return a.at >= deletedAt }); len(after) != 0 {
			t.Errorf("%s made rejoin attempts at %v after its record was deleted at %s, want none", rejoinX, after, deletedAt)
		}

		if writes := h.ops.where(func(op recordOp) bool {
			return op.agent == rejoinX && op.role == roleWriter && op.at >= pausedAt && op.at < deletedAt
		}); len(writes) != 0 {
			t.Errorf("the writer of %s wrote %v between its pause at %s and the delete of its record at %s, want nothing",
				rejoinX, writes, pausedAt, deletedAt)
		}

		logs := decodeLogs(t, xLog.String())
		if pauses, resumes := countMsg(logs, writerPausedMsg), countMsg(logs, writerResumedMsg); pauses != 1 || resumes != 1 {
			t.Errorf("the writer of %s paused %d times and resumed %d times, want one pause, ended once", rejoinX, pauses, resumes)
		}

		creates := h.ops.where(func(op recordOp) bool {
			return op.op == opCreate && op.name == rejoinLost && op.at >= lostVanishes
		})
		if len(creates) != 1 || creates[0].role != roleWriter || domain.WriterRank(alive, rejoinLost, creates[0].agent) != 0 {
			t.Errorf("the record of %s was created by %v after it vanished at %s, want once by the writer of rank 0 among %v",
				rejoinLost, creates, lostVanishes, alive)
		}

		if deletes := h.ops.where(func(op recordOp) bool { return op.op == opDelete && op.at >= lostVanishes }); len(deletes) != 0 {
			t.Errorf("records were deleted while %s was absent: %v, want none", rejoinLost, deletes)
		}
	})
}

func TestFallbackRecordMarkedFailedIsDroppedByItsNodeOnceItsViewRegainsQuorum(t *testing.T) {
	const regainAt = 10 * time.Second

	synctest.Test(t, func(t *testing.T) {
		var xLog bytes.Buffer

		s := recordScenario{
			group: []string{rejoinX, rejoinPeer, rejoinLost},
			writers: map[string]time.Duration{
				rejoinX:    700 * time.Millisecond,
				rejoinPeer: 800 * time.Millisecond,
				rejoinLost: 900 * time.Millisecond,
			},
			x:          rejoinX,
			monitorAt:  600 * time.Millisecond,
			rejoinAt:   400 * time.Millisecond,
			writerLogs: map[string]io.Writer{rejoinX: &xLog},
			steps: []recordStep{
				{at: splitAt, apply: func(h *recordHarness) {
					h.views[rejoinX].see(rejoinX)
					h.views[rejoinPeer].see(rejoinPeer, rejoinLost)
					h.views[rejoinLost].see(rejoinPeer, rejoinLost)
				}},
				{at: regainAt, apply: func(h *recordHarness) {
					h.views[rejoinX].see(rejoinX, rejoinPeer)
				}},
			},
			until: 30 * time.Second,
		}

		h := runRecordHarness(t, s)

		defer func() {
			if t.Failed() {
				h.logOps(t)
			}
		}()

		creates := h.ops.where(func(op recordOp) bool { return op.op == opCreate && op.name == rejoinX })
		if len(creates) == 0 || creates[0].agent != rejoinX || creates[0].role != roleMonitor || creates[0].at < splitAt {
			t.Fatalf("the record of %s was created by %v, want first by its fallback monitor after the split at %s: the scenario did not start",
				rejoinX, creates, splitAt)
		}

		fallbackRecord := creates[0]

		marked := h.ops.where(func(op recordOp) bool { return op.op == opMarkFailed && op.uid == fallbackRecord.uid })
		if len(marked) != 1 || marked[0].agent == rejoinX || !marked[0].fallback {
			t.Fatalf("the fallback record of %s was marked failed by %v, want once by the majority while it carries fallback: the scenario did not start",
				rejoinX, marked)
		}

		deletes := h.ops.where(func(op recordOp) bool { return op.op == opDelete && op.name == rejoinX })
		if len(deletes) != 1 || deletes[0].agent != rejoinX || deletes[0].role != roleMonitor || deletes[0].uid != fallbackRecord.uid ||
			!deletes[0].fallback || !deletes[0].failed || deletes[0].at < regainAt {
			t.Fatalf("the record of %s was deleted by %v, want once by its fallback monitor, carrying fallback and failed, after its view regained quorum at %s",
				rejoinX, deletes, regainAt)
		}

		dropped := deletes[0]

		if fallbackPause := pauseOf(s.writers[rejoinX], marked[0].at); fallbackPause <= dropped.at {
			t.Fatalf("the verdict on the fallback record of %s pauses its writer at %s, want after its monitor dropped the record at %s",
				rejoinX, fallbackPause, dropped.at)
		}

		if len(creates) != 2 || creates[1].agent == rejoinX || creates[1].role != roleWriter || creates[1].at < dropped.at {
			t.Fatalf("the record of %s was created by %v, want by its fallback monitor and then once again by a majority writer after the delete at %s",
				rejoinX, creates, dropped.at)
		}

		recreated := creates[1]
		recreatedAt := h.start.Add(recreated.at)

		record, ok := h.api.record(rejoinX)

		switch {
		case !ok || record.UID != recreated.uid:
			t.Errorf("the record of %s at the end exists: %t, has uid %q, want the one the majority created with uid %q",
				rejoinX, ok, record.UID, recreated.uid)
		case record.Status.Fallback != nil || record.Status.Failed == nil:
			t.Errorf("the recreated record of %s carries fallback: %t, failed: %t, want failed without fallback",
				rejoinX, record.Status.Fallback != nil, record.Status.Failed != nil)
		default:
			detectedAt := record.Status.Failed.DetectedAt.Time

			if !detectedAt.Equal(recreatedAt) {
				t.Errorf("the recreated record of %s has detectedAt %s, want the moment it was created, %s",
					rejoinX, detectedAt.Sub(h.start), recreated.at)
			}

			if first := marked[0].detectedAt; !detectedAt.After(first) {
				t.Errorf("the recreated record of %s has detectedAt %s, want later than the first verdict's %s",
					rejoinX, detectedAt.Sub(h.start), first.Sub(h.start))
			}
		}

		if attempts := h.attempts.where(func(a rejoinAttempt) bool { return a.at >= recreated.at && a.quorum }); len(attempts) == 0 {
			t.Errorf("%s made no rejoin attempt while it held quorum after its record came back at %s, want the attempts to go on",
				rejoinX, recreated.at)
		}

		pausedAt := pauseOf(s.writers[rejoinX], recreated.at)
		if pausedAt >= s.until {
			t.Fatalf("the writer of %s pauses at %s, want before the end of the scenario at %s", rejoinX, pausedAt, s.until)
		}

		writes := h.ops.where(func(op recordOp) bool { return op.agent == rejoinX && op.role == roleWriter && op.op == opCreate })
		stray := make(map[types.UID]bool, len(writes))

		for _, op := range writes {
			stray[op.uid] = true
		}

		if slices.ContainsFunc(writes, func(op recordOp) bool { return op.name != rejoinLost || op.at >= pausedAt }) {
			t.Errorf("the writer of %s created %v, want records of %s only, before its pause at %s",
				rejoinX, writes, rejoinLost, pausedAt)
		}

		churn := h.ops.where(func(op recordOp) bool {
			switch {
			case op.at < recreated.at, op.uid == recreated.uid && op.op != opDelete:
				return false
			case op.at >= pausedAt:
				return true
			case op.agent == rejoinX:
				return op.role != roleWriter || op.name != rejoinLost || op.op == opDelete
			default:
				return op.op != opDelete || !stray[op.uid]
			}
		})
		if len(churn) != 0 {
			t.Errorf("records changed after the record of %s came back at %s: %v, want only the verdicts of its writer until its pause at %s and their removal",
				rejoinX, recreated.at, churn, pausedAt)
		}

		if lost, ok := h.api.record(rejoinLost); ok {
			t.Errorf("a record of %s with uid %s is left, want every verdict of %s removed by the majority", rejoinLost, lost.UID, rejoinX)
		}

		logs := decodeLogs(t, xLog.String())
		if pauses, resumes := countMsg(logs, writerPausedMsg), countMsg(logs, writerResumedMsg); pauses != 1 || resumes != 0 {
			t.Errorf("the writer of %s paused %d times and resumed %d times, want one pause, over the recreated record, that lasts",
				rejoinX, pauses, resumes)
		}
	})
}

type ownFailedRecordRow struct {
	name   string
	states []v1alpha1.FencingFailedNodeState
	want   int
}

func failedRecordAt(name string, uid types.UID, created time.Time) v1alpha1.FencingFailedNodeState {
	return v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid, CreationTimestamp: metav1.NewTime(created)},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: rejoinPeer},
		},
	}
}

func fallbackOnlyRecordAt(name string, uid types.UID, created time.Time) v1alpha1.FencingFailedNodeState {
	return v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid, CreationTimestamp: metav1.NewTime(created)},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Fallback: &v1alpha1.FencingFailedNodeStateFallback{Active: true, APIReachable: true},
		},
	}
}

func ownFailedRecordRows(startedAt time.Time) []ownFailedRecordRow {
	return []ownFailedRecordRow{
		{
			name:   "own failed record created in the start second",
			states: []v1alpha1.FencingFailedNodeState{failedRecordAt(rejoinX, "own-000000", startedAt)},
			want:   0,
		},
		{
			name:   "own failed record created after the start",
			states: []v1alpha1.FencingFailedNodeState{failedRecordAt(rejoinX, "own-000001", startedAt.Add(time.Second))},
			want:   0,
		},
		{
			name:   "own failed record from an earlier life",
			states: []v1alpha1.FencingFailedNodeState{failedRecordAt(rejoinX, "own-235959", startedAt.Add(-time.Second))},
			want:   -1,
		},
		{
			name:   "own fresh record without a failed verdict",
			states: []v1alpha1.FencingFailedNodeState{fallbackOnlyRecordAt(rejoinX, "own-fallback", startedAt.Add(time.Second))},
			want:   -1,
		},
		{
			name:   "fresh failed record of another node",
			states: []v1alpha1.FencingFailedNodeState{failedRecordAt(rejoinPeer, "peer-000001", startedAt.Add(time.Second))},
			want:   -1,
		},
		{
			name:   "no records",
			states: nil,
			want:   -1,
		},
		{
			name: "own failed record behind a fresh failed record of another node",
			states: []v1alpha1.FencingFailedNodeState{
				failedRecordAt(rejoinPeer, "peer-000001", startedAt.Add(time.Second)),
				failedRecordAt(rejoinX, "own-000002", startedAt.Add(2*time.Second)),
			},
			want: 1,
		},
	}
}

func TestWriterPauseAndRejoinTriggerAgreeOnEveryRow(t *testing.T) {
	alive := []string{rejoinX, rejoinPeer}

	if rank := domain.WriterRank(alive, rejoinLost, rejoinX); rank != 0 {
		t.Fatalf("%s has writer rank %d for %s among %v, want 0", rejoinX, rank, rejoinLost, alive)
	}

	const (
		firstRead = time.Second
		pauseAt   = firstRead + recordTakeoverDelay
	)

	probes := []struct {
		pass   time.Duration
		pauses bool
	}{
		{pass: firstRead, pauses: false},
		{pass: pauseAt - time.Second, pauses: false},
		{pass: pauseAt, pauses: true},
	}

	for i, outside := range ownFailedRecordRows(time.Time{}) {
		t.Run(outside.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				startedAt := failedstate.StartOfLife(start)
				row := ownFailedRecordRows(startedAt)[i]

				api := newRecordAPI()
				view := &recordGossip{members: []string{rejoinX, rejoinPeer, rejoinLost}}

				expected := make(recordExpected, 0, len(view.members))
				for _, name := range view.members {
					expected = append(expected, domain.Peer{Name: name, IP: "10.0.0.1", UID: "node-uid-" + name})
				}

				writer := failedstate.New(
					failedstate.Params{
						NodeName:         rejoinX,
						RetryInterval:    time.Second,
						MaxRetryInterval: 10 * time.Second,
						TakeoverDelay:    recordTakeoverDelay,
						FallbackTTL:      recordTTL,
						StartedAt:        startedAt,
					},
					failedstate.Deps{
						Alive:    view,
						Expected: expected,
						States:   recordClient{api: api, agent: rejoinX},
						Events:   recordEvents{},
					},
					log.NewNop(),
				)

				ctx, cancel := context.WithCancel(t.Context())

				var loop sync.WaitGroup

				loop.Go(func() { _ = writer.Run(ctx) })

				sleepUntil(start, firstRead-500*time.Millisecond)

				for _, record := range row.states {
					api.seed(record)
				}

				view.see(rejoinX, rejoinPeer)

				want := row.want >= 0

				for _, probe := range probes {
					sleepUntil(start, probe.pass-500*time.Millisecond)
					api.remove(rejoinLost)

					trigger := ownFailedRecord(recordClient{api: api, agent: rejoinX}.List, func() bool { return true }, rejoinX, startedAt)(t.Context())
					if trigger != want {
						t.Errorf("before the pass at %s the rejoin trigger is %t, want %t: the copy of the row lost its meaning",
							probe.pass, trigger, want)
					}

					sleepUntil(start, probe.pass+500*time.Millisecond)

					_, created := api.record(rejoinLost)
					if paused, wantPaused := !created, probe.pauses && trigger; paused != wantPaused {
						t.Errorf("in the pass at %s the writer is paused: %t (created the record of %s: %t), the rejoin trigger fires: %t, want paused: %t",
							probe.pass, paused, rejoinLost, created, trigger, wantPaused)
					}
				}

				cancel()
				loop.Wait()
			})
		})
	}
}

const (
	writerPausedMsg  = "a peer recorded this node as failed, the fencing state writer is paused"
	writerResumedMsg = "no peer records this node as failed any more, the fencing state writer resumes"
)

func mutuallyRankedGroup(t *testing.T) []string {
	t.Helper()

	const candidates = 9

	names := make([]string, 0, candidates)
	for i := 1; i <= candidates; i++ {
		names = append(names, fmt.Sprintf("worker-%d", i))
	}

	for _, first := range names {
		for _, second := range names {
			for _, third := range names {
				if first == second || first == third || second == third {
					continue
				}

				group := []string{first, second, third}
				if domain.WriterRank(group, first, second) == 0 && domain.WriterRank(group, second, first) == 0 {
					return group
				}
			}
		}
	}

	t.Fatalf("no two of %v are each other's writer of rank 0 in a group of three", names)

	return nil
}

func countMsg(records []logRecord, msg string) int {
	count := 0

	for _, record := range records {
		if record.msg() == msg {
			count++
		}
	}

	return count
}

func TestMutuallyRecordedPairClearsEachOtherWhilePaused(t *testing.T) {
	group := mutuallyRankedGroup(t)
	first, second, third := group[0], group[1], group[2]

	for _, want := range []struct {
		failed string
		writer string
		rank   int
	}{
		{failed: first, writer: second, rank: 0},
		{failed: second, writer: first, rank: 0},
		{failed: first, writer: third, rank: 1},
		{failed: second, writer: third, rank: 1},
	} {
		if rank := domain.WriterRank(group, want.failed, want.writer); rank != want.rank {
			t.Fatalf("%s has writer rank %d for %s among %v, want %d", want.writer, rank, want.failed, group, want.rank)
		}
	}

	const (
		seedAt = 1500 * time.Millisecond
		healAt = seedAt + recordTakeoverDelay + time.Second
	)

	writers := map[string]time.Duration{
		first:  200 * time.Millisecond,
		second: 300 * time.Millisecond,
		third:  400 * time.Millisecond,
	}

	for _, member := range []string{first, second} {
		if pausedAt := pauseOf(writers[member], seedAt); pausedAt >= healAt {
			t.Fatalf("the writer of %s pauses at %s, want before the views heal at %s", member, pausedAt, healAt)
		}
	}

	rows := []struct {
		name       string
		detectedBy map[string]string
		clearedBy  map[string]string
		from       time.Duration
		to         time.Duration
	}{
		{
			name:       "each recorded by its partner is cleared by the partner at once",
			detectedBy: map[string]string{first: second, second: first},
			clearedBy:  map[string]string{first: second, second: first},
			from:       0,
			to:         2 * time.Second,
		},
		{
			name:       "both recorded by the third node are cleared by the third node after the takeover delay",
			detectedBy: map[string]string{first: third, second: third},
			clearedBy:  map[string]string{first: third, second: third},
			from:       recordTakeoverDelay,
			to:         recordTakeoverDelay + 2*time.Second,
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				logs := map[string]*bytes.Buffer{first: {}, second: {}}

				var seeded []v1alpha1.FencingFailedNodeState

				h := runRecordHarness(t, recordScenario{
					group:      group,
					writers:    writers,
					writerLogs: map[string]io.Writer{first: logs[first], second: logs[second]},
					steps: []recordStep{
						{at: seedAt, apply: func(h *recordHarness) {
							now := time.Now()

							for _, member := range []string{first, second} {
								record := v1alpha1.FencingFailedNodeState{
									ObjectMeta: metav1.ObjectMeta{Name: member, CreationTimestamp: metav1.NewTime(now.Truncate(time.Second))},
									Status: v1alpha1.FencingFailedNodeStateStatus{
										Failed: &v1alpha1.FencingFailedNodeStateFailed{
											DetectedAt: metav1.NewMicroTime(now),
											DetectedBy: row.detectedBy[member],
											Reason:     v1alpha1.FailedReasonMemberlistDead,
											AliveCount: 2,
											QuorumSize: 2,
										},
									},
								}

								h.api.seed(record)
								seeded = append(seeded, record)
							}

							h.views[first].see(first, third)
							h.views[second].see(second, third)
							h.views[third].see(third)
						}},
						{at: healAt, apply: func(h *recordHarness) {
							for _, member := range group {
								h.views[member].see(slices.Clone(group)...)
							}
						}},
					},
					until: healAt + recordTakeoverDelay + 3*time.Second,
				})

				defer func() {
					if t.Failed() {
						h.logOps(t)
					}
				}()

				startedAt := failedstate.StartOfLife(h.start)
				for _, record := range seeded {
					if record.CreationTimestamp.Time.Before(startedAt) {
						t.Fatalf("the record of %s was created at %s, before the writers started at %s, want a record of this life",
							record.Name, record.CreationTimestamp.UTC(), startedAt.UTC())
					}
				}

				if writes := h.ops.where(func(op recordOp) bool { return op.op != opDelete }); len(writes) != 0 {
					t.Errorf("records were created or marked: %v, want none: every peer an agent misses already carries a verdict", writes)
				}

				for _, member := range []string{first, second} {
					deletes := h.ops.where(func(op recordOp) bool { return op.op == opDelete && op.name == member })
					if len(deletes) != 1 || deletes[0].agent != row.clearedBy[member] || deletes[0].role != roleWriter {
						t.Errorf("the record of %s was deleted by %v, want once by the writer of %s", member, deletes, row.clearedBy[member])

						continue
					}

					if at, from, to := deletes[0].at, healAt+row.from, healAt+row.to; at < from || at >= to {
						t.Errorf("the record of %s was deleted at %s, want within [%s, %s)", member, at, from, to)
					}
				}

				for _, member := range []string{first, second} {
					records := decodeLogs(t, logs[member].String())

					if pauses, resumes := countMsg(records, writerPausedMsg), countMsg(records, writerResumedMsg); pauses != 1 || resumes != 1 {
						t.Errorf("the writer of %s paused %d times and resumed %d times, want one pause, ended once", member, pauses, resumes)
					}
				}
			})
		})
	}
}

type asymmetricView struct {
	group   []string
	x       string
	victims []string
}

func asymmetricViewGroup(t *testing.T) asymmetricView {
	t.Helper()

	const (
		candidates = 9
		size       = 5
	)

	names := make([]string, 0, candidates)
	for i := 1; i <= candidates; i++ {
		names = append(names, fmt.Sprintf("worker-%d", i))
	}

	for mask := range 1 << candidates {
		if bits.OnesCount(uint(mask)) != size {
			continue
		}

		group := make([]string, 0, size)

		for i, name := range names {
			if mask&(1<<i) != 0 {
				group = append(group, name)
			}
		}

		for _, x := range group {
			for i, one := range group {
				for _, other := range group[i+1:] {
					if x == one || x == other {
						continue
					}

					xAlive := slices.DeleteFunc(slices.Clone(group), func(name string) bool { return name == one || name == other })

					if domain.WriterRank(xAlive, one, x) == 0 && domain.WriterRank(xAlive, other, x) == 0 &&
						domain.WriterRank(group, one, other) == 0 && domain.WriterRank(group, other, one) == 0 {
						return asymmetricView{group: group, x: x, victims: []string{one, other}}
					}
				}
			}
		}
	}

	t.Fatalf("no NodeGroup of %d among %v has an agent that is the writer of rank 0 for two peers each other's writer of rank 0", size, names)

	return asymmetricView{}
}

func TestHealthyPeersRecordedByOneAsymmetricViewClearEachOtherWithoutPausing(t *testing.T) {
	view := asymmetricViewGroup(t)
	x, victims := view.x, view.victims

	const (
		mediumEvacuationDelay = 6 * time.Second
		clearWithin           = 2 * time.Second
		brokenAt              = 2050 * time.Millisecond
		until                 = 15 * time.Second
	)

	if clearWithin >= mediumEvacuationDelay || mediumEvacuationDelay >= recordTakeoverDelay {
		t.Fatalf("clearWithin %s, evacuation delay %s, TakeoverDelay %s, want them in that order", clearWithin, mediumEvacuationDelay, recordTakeoverDelay)
	}

	others := slices.DeleteFunc(slices.Clone(view.group), func(name string) bool { return name == x || slices.Contains(victims, name) })
	writers := map[string]time.Duration{x: 100 * time.Millisecond}

	for i, name := range append(slices.Clone(victims), others...) {
		writers[name] = time.Duration(200+100*i) * time.Millisecond
	}

	rank0 := make(map[string]string, len(victims))

	for _, victim := range victims {
		for _, name := range view.group {
			if domain.WriterRank(view.group, victim, name) == 0 {
				rank0[victim] = name
			}
		}
	}

	synctest.Test(t, func(t *testing.T) {
		logs := map[string]*bytes.Buffer{victims[0]: {}, victims[1]: {}}

		h := runRecordHarness(t, recordScenario{
			group:      view.group,
			writers:    writers,
			writerLogs: map[string]io.Writer{victims[0]: logs[victims[0]], victims[1]: logs[victims[1]]},
			steps: []recordStep{{at: brokenAt, apply: func(h *recordHarness) {
				h.views[x].see(slices.DeleteFunc(slices.Clone(view.group), func(name string) bool { return slices.Contains(victims, name) })...)
			}}},
			until: until,
		})

		defer func() {
			if t.Failed() {
				h.logOps(t)
			}
		}()

		verdicts := h.ops.where(func(op recordOp) bool { return op.agent == x && op.op == opCreate })
		if len(verdicts) == 0 {
			t.Fatalf("%s recorded nobody after it lost sight of %v at %s: the scenario did not start", x, victims, brokenAt)
		}

		removals := make(map[types.UID]recordOp)

		for _, op := range h.ops.where(func(recordOp) bool { return true }) {
			switch {
			case !slices.Contains(victims, op.name):
				t.Errorf("unexpected write %s, want writes on the records of %v only", op, victims)
			case op.op == opDelete && op.agent == rank0[op.name] && op.role == roleWriter:
				removals[op.uid] = op
			case op.op == opDelete:
				t.Errorf("unexpected delete %s, want only the writer of rank 0, %s, to remove the record of %s", op, rank0[op.name], op.name)
			case op.agent != x || op.role != roleWriter:
				t.Errorf("unexpected write %s, want only the writer of %s to record %v", op, x, victims)
			}
		}

		for _, verdict := range verdicts {
			removed, ok := removals[verdict.uid]
			if !ok {
				t.Errorf("the record of %s that %s created at %s with uid %s was never removed by its writer of rank 0, %s",
					verdict.name, x, verdict.at, verdict.uid, rank0[verdict.name])

				continue
			}

			if age := h.start.Add(removed.at).Sub(removed.detectedAt); removed.at-verdict.at >= clearWithin || age >= clearWithin {
				t.Errorf("the record of %s created at %s was removed at %s, %s after its detectedAt, want within %s of both",
					verdict.name, verdict.at, removed.at, age, clearWithin)
			}
		}

		for _, victim := range victims {
			if pauses := countMsg(decodeLogs(t, logs[victim].String()), writerPausedMsg); pauses != 0 {
				t.Errorf("the writer of %s paused %d times, want never: no record about it stood for a TakeoverDelay", victim, pauses)
			}

			if !slices.ContainsFunc(verdicts, func(op recordOp) bool {
				return op.name == victim && op.at >= verdicts[0].at+recordTakeoverDelay
			}) {
				t.Errorf("%s last recorded %s before %s, want again one TakeoverDelay after its first verdict at %s",
					x, victim, verdicts[0].at+recordTakeoverDelay, verdicts[0].at)
			}
		}
	})
}

func TestPartitionedPairThatSeesEveryoneClearsEachOtherAndNeverPauses(t *testing.T) {
	view := asymmetricViewGroup(t)
	pair := view.victims
	majority := slices.DeleteFunc(slices.Clone(view.group), func(name string) bool { return slices.Contains(pair, name) })
	partner := map[string]string{pair[0]: pair[1], pair[1]: pair[0]}

	for _, member := range pair {
		if rank := domain.WriterRank(view.group, partner[member], member); rank != 0 {
			t.Fatalf("%s has writer rank %d for %s among %v, want 0", member, rank, partner[member], view.group)
		}
	}

	const (
		mediumEvacuationDelay = 6 * time.Second
		brokenAt              = 2050 * time.Millisecond
		until                 = 60 * time.Second
		minRecords            = int((until - brokenAt) / recordTakeoverDelay)
	)

	writers := make(map[string]time.Duration, len(view.group))

	for i, name := range append(slices.Clone(majority), pair...) {
		writers[name] = time.Duration(100+100*i) * time.Millisecond
	}

	synctest.Test(t, func(t *testing.T) {
		logs := map[string]*bytes.Buffer{pair[0]: {}, pair[1]: {}}

		h := runRecordHarness(t, recordScenario{
			group:      view.group,
			writers:    writers,
			writerLogs: map[string]io.Writer{pair[0]: logs[pair[0]], pair[1]: logs[pair[1]]},
			steps: []recordStep{{at: brokenAt, apply: func(h *recordHarness) {
				for _, name := range majority {
					h.views[name].see(slices.Clone(majority)...)
				}
			}}},
			until: until,
		})

		defer func() {
			if t.Failed() {
				h.logOps(t)
			}
		}()

		standing := make(map[types.UID]recordOp)
		records := make(map[string][]recordOp, len(pair))

		lived := func(created recordOp, gone time.Duration, how string) {
			if lifetime := gone - created.at; lifetime >= mediumEvacuationDelay {
				t.Errorf("the record of %s created at %s with uid %s %s at %s, %s later, want within the evacuation delay %s",
					created.name, created.at, created.uid, how, gone, lifetime, mediumEvacuationDelay)
			}
		}

		for _, op := range h.ops.where(func(recordOp) bool { return true }) {
			switch {
			case !slices.Contains(pair, op.name):
				t.Errorf("unexpected write %s, want writes on the records of %v only", op, pair)
			case op.op == opDelete && op.agent == partner[op.name] && op.role == roleWriter:
				if created, ok := standing[op.uid]; ok {
					lived(created, op.at, "was removed")
					delete(standing, op.uid)
				}
			case op.op == opDelete:
				t.Errorf("unexpected delete %s, want only the writer of %s to remove the record of %s", op, partner[op.name], op.name)
			case !slices.Contains(majority, op.agent) || op.role != roleWriter:
				t.Errorf("unexpected write %s, want only the writers of the majority %v to record %v", op, majority, pair)
			case op.op == opCreate:
				standing[op.uid] = op
				records[op.name] = append(records[op.name], op)
			}
		}

		for _, created := range standing {
			lived(created, until, "still stands")
		}

		for _, member := range pair {
			created := records[member]
			if len(created) < minRecords || created[len(created)-1].at < until-recordTakeoverDelay {
				t.Errorf("the majority recorded %s %d times, want at least %d times, and again within the last TakeoverDelay before %s: %v",
					member, len(created), minRecords, until, created)
			}

			if pauses := countMsg(decodeLogs(t, logs[member].String()), writerPausedMsg); pauses != 0 {
				t.Errorf("the writer of %s paused %d times, want never: every record about it goes before it stands a TakeoverDelay",
					member, pauses)
			}
		}
	})
}
