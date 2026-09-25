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
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/usecase/failedstate"
	"fencing-agent/internal/usecase/fallback"
)

// The record of a node is written by two loops on two different agents: the
// designated writer on a peer, and the fallback monitor on the node itself. Each
// acts on its own gossip view, and the tests below run both, in virtual time,
// against one API that behaves like the real one where it matters: a new UID on
// every create, NotFound for a status patch of a missing object, a UID
// precondition on delete.

const (
	recordNode = "worker-1"

	recordHeartbeat     = time.Second
	recordTTL           = 4 * time.Second
	recordTakeoverDelay = 10 * time.Second
)

var recordGroup = []string{"worker-1", "worker-2", "worker-3"}

var errAPIUnreachable = errors.New("dial tcp: i/o timeout")

type recordAPI struct {
	mu        sync.Mutex
	records   map[string]v1alpha1.FencingFailedNodeState
	uids      int
	lost      map[string]bool
	creates   map[string]int
	deletes   map[string]int
	beatAt    time.Time
	deletedAt time.Time
}

func newRecordAPI() *recordAPI {
	return &recordAPI{
		records: make(map[string]v1alpha1.FencingFailedNodeState),
		lost:    make(map[string]bool),
		creates: make(map[string]int),
		deletes: make(map[string]int),
	}
}

func (a *recordAPI) loseAPI(agent string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lost[agent] = true
}

// recordClient is one agent's connection to recordAPI.
type recordClient struct {
	api   *recordAPI
	agent string
}

var recordResource = schema.GroupResource{Group: v1alpha1.GroupVersion.Group, Resource: "fencingfailednodestates"}

func (c recordClient) lock() (func(), error) {
	c.api.mu.Lock()

	if c.api.lost[c.agent] {
		c.api.mu.Unlock()

		return nil, errAPIUnreachable
	}

	return c.api.mu.Unlock, nil
}

func (c recordClient) List(context.Context) ([]v1alpha1.FencingFailedNodeState, error) {
	unlock, err := c.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()

	states := make([]v1alpha1.FencingFailedNodeState, 0, len(c.api.records))
	for _, record := range c.api.records {
		states = append(states, *record.DeepCopy())
	}

	return states, nil
}

func (c recordClient) Get(_ context.Context, name string) (*v1alpha1.FencingFailedNodeState, error) {
	unlock, err := c.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()

	record, ok := c.api.records[name]
	if !ok {
		return nil, nil
	}

	return record.DeepCopy(), nil
}

func (c recordClient) Create(_ context.Context, peer domain.Peer) (bool, error) {
	unlock, err := c.lock()
	if err != nil {
		return false, err
	}
	defer unlock()

	if _, ok := c.api.records[peer.Name]; ok {
		return false, nil
	}

	c.api.uids++
	c.api.creates[c.agent]++
	c.api.records[peer.Name] = v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{
		Name:              peer.Name,
		UID:               types.UID(fmt.Sprintf("uid-%d", c.api.uids)),
		CreationTimestamp: metav1.NewTime(time.Now().Truncate(time.Second)),
	}}

	return true, nil
}

func (c recordClient) MarkFailed(_ context.Context, name string, failed v1alpha1.FencingFailedNodeStateFailed) (bool, error) {
	unlock, err := c.lock()
	if err != nil {
		return false, err
	}
	defer unlock()

	record, ok := c.api.records[name]
	if !ok {
		return false, apierrors.NewNotFound(recordResource, name)
	}

	if record.Status.Failed != nil {
		return false, nil
	}

	record.Status.Failed = failed.DeepCopy()
	c.api.records[name] = record

	return true, nil
}

func (c recordClient) Heartbeat(_ context.Context, name string, section v1alpha1.FencingFailedNodeStateFallback) error {
	unlock, err := c.lock()
	if err != nil {
		return err
	}
	defer unlock()

	record, ok := c.api.records[name]
	if !ok {
		return apierrors.NewNotFound(recordResource, name)
	}

	record.Status.Fallback = section.DeepCopy()
	c.api.records[name] = record
	c.api.beatAt = time.Now()

	return nil
}

func (c recordClient) Delete(_ context.Context, name string, uid types.UID) error {
	unlock, err := c.lock()
	if err != nil {
		return err
	}
	defer unlock()

	record, ok := c.api.records[name]
	if !ok {
		return nil
	}

	if record.UID != uid {
		return apierrors.NewConflict(recordResource, name, errors.New("uid precondition failed"))
	}

	delete(c.api.records, name)
	c.api.deletes[c.agent]++
	c.api.deletedAt = time.Now()

	return nil
}

type recordGossip struct {
	mu      sync.Mutex
	members []string
}

func (g *recordGossip) Members() []string {
	g.mu.Lock()
	defer g.mu.Unlock()

	return slices.Clone(g.members)
}

func (g *recordGossip) see(members ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.members = members
}

func (g *recordGossip) Changed() <-chan struct{} { return nil }

type recordExpected []domain.Peer

func (e recordExpected) Expected() ([]domain.Peer, uint64) { return e, 1 }

func (e recordExpected) Snapshot() ([]domain.Peer, int, int) {
	return e, len(e), domain.QuorumSize(len(e))
}

type recordEvents struct{}

func (recordEvents) Normal(string, string)  {}
func (recordEvents) Warning(string, string) {}

func recordPeers(names []string) recordExpected {
	peers := make(recordExpected, 0, len(names))
	for _, name := range names {
		peers = append(peers, domain.Peer{Name: name, IP: "10.0.0.1", UID: "node-uid-" + name})
	}

	return peers
}

func newRecordMonitor(node string, alive *recordGossip, expected recordExpected, states fallback.StateStore) *fallback.Monitor {
	return fallback.New(
		fallback.Params{
			Node:            domain.NodeIdentity{Name: node, UID: "node-uid-" + node, IP: "10.0.0.1"},
			Heartbeat:       recordHeartbeat,
			APITimeout:      2 * time.Second,
			WatchdogTimeout: 10 * time.Second,
		},
		fallback.Deps{
			Alive:    alive,
			Expected: expected,
			States:   states,
			Events:   recordEvents{},
		},
		log.NewNop(),
	)
}

func newRecordWriter(
	node string,
	startedAt time.Time,
	alive *recordGossip,
	expected recordExpected,
	states failedstate.StateStore,
	logger *log.Logger,
) *failedstate.Writer {
	return failedstate.New(
		failedstate.Params{
			NodeName:         node,
			RetryInterval:    time.Second,
			MaxRetryInterval: 10 * time.Second,
			TakeoverDelay:    recordTakeoverDelay,
			FallbackTTL:      recordTTL,
			StartedAt:        startedAt,
		},
		failedstate.Deps{
			Alive:    alive,
			Expected: expected,
			States:   states,
			Events:   recordEvents{},
		},
		logger,
	)
}

// recordLoops is the node running its fallback monitor and its designated writer
// running the fencing state writer, each with a gossip view the test controls.
type recordLoops struct {
	api        *recordAPI
	writer     string
	nodeView   *recordGossip
	writerView *recordGossip
}

// runRecordLoops starts both loops, the writer half a beat after the monitor so
// that no two passes share a virtual instant, lets script move the views, and
// stops both at until.
func runRecordLoops(t *testing.T, until time.Duration, script func(l *recordLoops)) *recordLoops {
	t.Helper()

	expected := recordPeers(recordGroup)

	l := &recordLoops{
		api:        newRecordAPI(),
		nodeView:   &recordGossip{members: slices.Clone(recordGroup)},
		writerView: &recordGossip{members: slices.Clone(recordGroup)},
	}

	for _, candidate := range recordGroup {
		if domain.WriterRank(recordGroup, recordNode, candidate) == 0 {
			l.writer = candidate
		}
	}

	start := time.Now()

	monitor := newRecordMonitor(recordNode, l.nodeView, expected, recordClient{api: l.api, agent: recordNode})
	writer := newRecordWriter(l.writer, failedstate.StartOfLife(start), l.writerView, expected, recordClient{api: l.api, agent: l.writer}, log.NewNop())

	ctx, cancel := context.WithCancel(t.Context())

	var loops sync.WaitGroup

	loops.Go(func() { _ = monitor.Run(ctx) })
	time.Sleep(recordHeartbeat / 2)
	loops.Go(func() { _ = writer.Run(ctx) })

	script(l)

	time.Sleep(until - time.Since(start))
	cancel()
	loops.Wait()

	return l
}

func sleepUntil(start time.Time, at time.Duration) {
	time.Sleep(at - time.Since(start))
}

func TestWriterDoesNotFightTheFallbackMonitorOverItsRecord(t *testing.T) {
	// The node lost quorum and heartbeats its record; its writer already sees it
	// in gossip. Every delete the writer issues is answered by a create on the
	// node's next heartbeat and costs an event, for as long as the views differ.
	synctest.Test(t, func(t *testing.T) {
		l := runRecordLoops(t, 10*time.Second, func(l *recordLoops) {
			l.nodeView.see(recordNode)
		})

		if deletes, creates := l.api.deletes[l.writer], l.api.creates[recordNode]; deletes != 0 || creates != 1 {
			t.Errorf("the writer deleted the record %d times and the node created it %d times, want 0 and 1", deletes, creates)
		}
	})
}

func TestFailedVerdictOfANodeBackInGossipGoesBeforeItsHeartbeatExpires(t *testing.T) {
	// The writer records the partitioned node as failed and the node adopts the
	// record with its heartbeat. Gossip heals just as the node loses the API, so
	// the node can neither heartbeat nor take the record back. From the moment
	// that last heartbeat is a TTL old, the controller evicts a node that is back.
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()

		l := runRecordLoops(t, 16*time.Second, func(l *recordLoops) {
			sleepUntil(start, 2250*time.Millisecond)
			l.nodeView.see(recordNode)
			l.writerView.see(slices.DeleteFunc(slices.Clone(recordGroup), func(name string) bool { return name == recordNode })...)

			sleepUntil(start, 10250*time.Millisecond)
			l.api.loseAPI(recordNode)
			l.nodeView.see(recordGroup...)
			l.writerView.see(recordGroup...)
		})

		if l.api.deletes[l.writer] != 1 {
			t.Fatalf("the writer deleted the record %d times, want once", l.api.deletes[l.writer])
		}

		if deadline := l.api.beatAt.Add(recordTTL); !l.api.deletedAt.Before(deadline) {
			t.Errorf("the failed verdict went at %s, want before the last heartbeat expired at %s",
				l.api.deletedAt.Sub(start), deadline.Sub(start))
		}
	})
}
