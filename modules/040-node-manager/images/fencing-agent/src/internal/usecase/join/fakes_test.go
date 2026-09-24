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
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

const (
	testNodeName  = "worker-1"
	testNodeUID   = "uid-1"
	testNodeIP    = "10.0.0.1"
	testNodeGroup = "worker"
	testPort      = 8500
)

func selfPeer() domain.Peer {
	return domain.Peer{Name: testNodeName, IP: testNodeIP, UID: testNodeUID}
}

func selfRecord() domain.NodeRecord {
	return domain.NodeRecord{Name: testNodeName, UID: testNodeUID, IP: testNodeIP, NodeGroup: testNodeGroup}
}

func peerRecord(name, ip string) domain.NodeRecord {
	return domain.NodeRecord{Name: name, IP: ip, NodeGroup: testNodeGroup}
}

type nodeAnswer struct {
	record        domain.NodeRecord
	err           error
	delay         time.Duration
	blockUntilCtx bool
	release       <-chan struct{}
}

type fakeNodes struct {
	mu      sync.Mutex
	answers map[string]nodeAnswer
	queued  map[string][]nodeAnswer
	calls   []string
	onCall  func(call int, name string)
}

func newFakeNodes() *fakeNodes {
	return &fakeNodes{answers: map[string]nodeAnswer{}}
}

func (f *fakeNodes) GetNode(ctx context.Context, name string) (domain.NodeRecord, error) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	call := len(f.calls)
	answer, ok := f.answers[name]

	if queue := f.queued[name]; len(queue) > 0 {
		answer, ok = queue[0], true
		f.queued[name] = queue[1:]
	}

	hook := f.onCall
	f.mu.Unlock()

	if hook != nil {
		hook(call, name)
	}

	if !ok {
		return domain.NodeRecord{}, notFound(name)
	}

	switch {
	case answer.blockUntilCtx:
		<-ctx.Done()

		return domain.NodeRecord{}, ctx.Err()
	case answer.delay > 0:
		select {
		case <-ctx.Done():
			return domain.NodeRecord{}, ctx.Err()
		case <-time.After(answer.delay):
		}
	case answer.release != nil:
		select {
		case <-ctx.Done():
			return domain.NodeRecord{}, ctx.Err()
		case <-answer.release:
		}
	}

	return answer.record, answer.err
}

func notFound(name string) error {
	return fmt.Errorf("get node %q: %w", name, domain.ErrNodeNotFound)
}

func (f *fakeNodes) setAnswer(name string, answer nodeAnswer) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.answers[name] = answer
}

func (f *fakeNodes) queueAnswers(name string, answers ...nodeAnswer) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.queued == nil {
		f.queued = map[string][]nodeAnswer{}
	}

	f.queued[name] = append(f.queued[name], answers...)
}

func (f *fakeNodes) setOnCall(hook func(call int, name string)) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.onCall = hook
}

func (f *fakeNodes) gets() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.calls)
}

func (f *fakeNodes) getsOf(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	n := 0

	for _, call := range f.calls {
		if call == name {
			n++
		}
	}

	return n
}

func (f *fakeNodes) candidateGets() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	var names []string

	for _, call := range f.calls {
		if call != testNodeName {
			names = append(names, call)
		}
	}

	return names
}

func (f *fakeNodes) resetJournal() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = nil
}

type fakeExpected struct {
	mu       sync.Mutex
	peers    []domain.Peer
	revision uint64
}

func (f *fakeExpected) Expected() ([]domain.Peer, uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.peers, f.revision
}

func (f *fakeExpected) setPeers(peers []domain.Peer) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.peers = peers
	f.revision++
}

type fakeCluster struct {
	mu       sync.Mutex
	members  []string
	seeds    [][]string
	failures int
	joinFn   func(seeds []string) (int, error)
}

func (f *fakeCluster) Join(seeds []string) (int, error) {
	f.mu.Lock()
	f.seeds = append(f.seeds, slices.Clone(seeds))
	call := len(f.seeds)
	failures := f.failures
	joinFn := f.joinFn
	f.mu.Unlock()

	if joinFn != nil {
		return joinFn(seeds)
	}

	if call <= failures {
		return 0, errors.New("connection refused")
	}

	return len(seeds), nil
}

func (f *fakeCluster) NumMembers() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return max(len(f.members), 1)
}

func (f *fakeCluster) Members() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.members)
}

func (f *fakeCluster) setMembers(members ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.members = members
}

func (f *fakeCluster) resetJournal() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.seeds = nil
}

func (f *fakeCluster) lastSeeds() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.seeds) == 0 {
		return nil
	}

	return slices.Clone(f.seeds[len(f.seeds)-1])
}

func (f *fakeCluster) joins() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()

	joins := make([][]string, 0, len(f.seeds))
	for _, seeds := range f.seeds {
		joins = append(joins, slices.Clone(seeds))
	}

	return joins
}

func assertJoinedOnce(t *testing.T, cluster *fakeCluster, want ...string) {
	t.Helper()

	joins := cluster.joins()
	if len(joins) != 1 {
		t.Errorf("join was called with %v, want a single call with %v", joins, want)

		return
	}

	got := slices.Sorted(slices.Values(joins[0]))
	if !slices.Equal(got, slices.Sorted(slices.Values(want))) {
		t.Errorf("join seeds are %v, want %v in any order", joins[0], want)
	}
}

func mirroredGroup(peers ...domain.Peer) (*fakeNodes, *fakeExpected) {
	nodes := newFakeNodes()

	for _, peer := range peers {
		nodes.answers[peer.Name] = nodeAnswer{record: domain.NodeRecord{
			Name:      peer.Name,
			UID:       peer.UID,
			IP:        peer.IP,
			NodeGroup: testNodeGroup,
		}}
	}

	return nodes, &fakeExpected{peers: slices.Clone(peers), revision: 1}
}

func joinerParams() Params {
	return Params{
		NodeName:         testNodeName,
		NodeUID:          testNodeUID,
		NodeIP:           testNodeIP,
		NodeGroup:        testNodeGroup,
		MemberlistPort:   testPort,
		APITimeout:       2 * time.Second,
		RetryInterval:    5 * time.Second,
		MaxRetryInterval: 30 * time.Second,
	}
}

func newJoiner(t *testing.T, nodes *fakeNodes, expected *fakeExpected, cluster *fakeCluster) *Joiner {
	t.Helper()

	return New(nodes, expected, cluster, joinerParams(), log.NewNop())
}
