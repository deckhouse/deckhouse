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
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"fencing-agent/internal/domain"
	"fencing-agent/internal/logtest"
	"fencing-agent/internal/usecase/join"
	"fencing-agent/internal/usecase/rejoin"
)

const (
	episodeNode      = "worker-1"
	episodeNodeGroup = "worker"
)

const (
	joinPartialMsg    = "some seeds are unreachable, gossip will converge"
	joinCompletedMsg  = "memberlist join completed"
	rejoinStartedMsg  = "gossip quorum lost, rejoin started"
	rejoinFinishedMsg = "rejoin finished, gossip quorum holds and no peer records this node as failed"
	rejoinStoppedMsg  = "rejoin stopped by shutdown"
)

func episodePeers() []domain.Peer {
	return []domain.Peer{
		{Name: episodeNode, UID: "node-uid-worker-1", IP: "10.0.0.1"},
		{Name: "worker-2", UID: "node-uid-worker-2", IP: "10.0.0.2"},
		{Name: "worker-3", UID: "node-uid-worker-3", IP: "10.0.0.3"},
	}
}

type episodeNodes struct {
	mu    sync.Mutex
	nodes map[string]domain.NodeRecord
}

func newEpisodeNodes(peers []domain.Peer) *episodeNodes {
	nodes := make(map[string]domain.NodeRecord, len(peers))
	for _, peer := range peers {
		nodes[peer.Name] = domain.NodeRecord{Name: peer.Name, UID: peer.UID, IP: peer.IP, NodeGroup: episodeNodeGroup}
	}

	return &episodeNodes{nodes: nodes}
}

func (n *episodeNodes) GetNode(_ context.Context, name string) (domain.NodeRecord, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	record, ok := n.nodes[name]
	if !ok {
		return domain.NodeRecord{}, fmt.Errorf("get node %q: %w", name, domain.ErrNodeNotFound)
	}

	return record, nil
}

type episodeExpected struct {
	mu    sync.Mutex
	peers []domain.Peer
}

func (e *episodeExpected) Expected() ([]domain.Peer, uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.peers, 1
}

type episodeCluster struct {
	t      *testing.T
	cancel context.CancelFunc

	mu     sync.Mutex
	joins  int
	joined int
}

func (c *episodeCluster) Join(seeds []string) (int, error) {
	if len(seeds) < 2 {
		c.t.Errorf("join got the seeds %v, want both peers", seeds)
		c.cancel()

		return 0, errors.New("the test expects a join of both peers")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.joins++
	c.joined = len(seeds) - 1

	return c.joined, nil
}

func (c *episodeCluster) NumMembers() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return 1 + c.joined
}

func (c *episodeCluster) Members() []string { return nil }

func (c *episodeCluster) joinCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.joins
}

type episodeTrigger struct {
	mu       sync.Mutex
	on       bool
	offReads int
}

func (tr *episodeTrigger) hasQuorum() bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if !tr.on {
		tr.offReads++
	}

	return !tr.on
}

func (tr *episodeTrigger) set(on bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.on = on
}

func (tr *episodeTrigger) readsOff() int {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	return tr.offReads
}

func stepUntil(t *testing.T, what string, done func() bool) {
	t.Helper()

	const (
		step  = 100 * time.Millisecond
		limit = 5 * time.Minute
	)

	deadline := time.Now().Add(limit)

	for {
		synctest.Wait()

		if done() {
			return
		}

		if !time.Now().Before(deadline) {
			t.Fatalf("%s did not happen within %s of virtual time", what, limit)
		}

		time.Sleep(step)
	}
}

func TestJoinPathDedupeResetsForANewRejoinEpisode(t *testing.T) {
	const attemptsPerEpisode = 3

	const (
		interval    = 5 * time.Second
		maxInterval = 30 * time.Second
	)

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var logs bytes.Buffer

		logger := logtest.NewJSONLogger(&logs)
		peers := episodePeers()
		cluster := &episodeCluster{t: t, cancel: cancel}

		joiner := join.New(newEpisodeNodes(peers), &episodeExpected{peers: peers}, cluster, join.Params{
			NodeName:         peers[0].Name,
			NodeUID:          peers[0].UID,
			NodeIP:           peers[0].IP,
			NodeGroup:        episodeNodeGroup,
			MemberlistPort:   8500,
			APITimeout:       2 * time.Second,
			RetryInterval:    interval,
			MaxRetryInterval: maxInterval,
		}, logger)

		joiner.Bootstrap(ctx)

		if bootstrapJoins := cluster.joinCount(); !joiner.Joined() || bootstrapJoins != 1 {
			t.Fatalf("Bootstrap joined: %t after %d joins, want a join at the first attempt", joiner.Joined(), bootstrapJoins)
		}

		trigger := &episodeTrigger{on: true}

		loop := rejoin.New(rejoin.Params{Interval: interval, MaxInterval: maxInterval}, rejoin.Deps{
			Attempt:        joiner.Attempt,
			EpisodeStarted: joiner.StartEpisode,
			NotMember:      func(err error) bool { return errors.Is(err, join.ErrNotMember) },
			HasQuorum:      trigger.hasQuorum,
		}, logger)

		var loops sync.WaitGroup

		defer func() {
			cancel()
			loops.Wait()
		}()

		loops.Go(func() { _ = loop.Run(ctx) })

		stepUntil(t, "the last attempt of the first episode", func() bool {
			return cluster.joinCount() >= 1+attemptsPerEpisode
		})
		trigger.set(false)

		stepUntil(t, "the end of the first episode", func() bool { return trigger.readsOff() > 0 })
		trigger.set(true)

		stepUntil(t, "the last attempt of the second episode", func() bool {
			return cluster.joinCount() >= 1+2*attemptsPerEpisode
		})

		cancel()
		loops.Wait()

		if joins, want := cluster.joinCount(), 1+2*attemptsPerEpisode; joins != want {
			t.Fatalf("the join path joined %d times, want %d: one Bootstrap and two episodes of %d attempts", joins, want, attemptsPerEpisode)
		}

		records := logtest.Decode(t, logs.String())
		logtest.AssertSnakeCaseKeys(t, records)

		bounds := []int{0}

		for i, record := range records {
			if record.Msg() == rejoinStartedMsg {
				bounds = append(bounds, i)
			}
		}

		if len(bounds) != 3 {
			t.Fatalf("the rejoin loop started %d episodes, want 2", len(bounds)-1)
		}

		bounds = append(bounds, len(records))

		rows := []struct {
			name      string
			partial   []string
			completed []string
			summary   string
		}{
			{
				name:      "the bootstrap",
				partial:   []string{"warn"},
				completed: []string{"info"},
			},
			{
				name:      "the first rejoin episode",
				partial:   []string{"warn", "debug", "debug"},
				completed: []string{"info", "debug", "debug"},
				summary:   rejoinFinishedMsg,
			},
			{
				name:      "the second rejoin episode",
				partial:   []string{"warn", "debug", "debug"},
				completed: []string{"info", "debug", "debug"},
				summary:   rejoinStoppedMsg,
			},
		}

		for i, row := range rows {
			segment := records[bounds[i]:bounds[i+1]]

			if got := logtest.Levels(logtest.WithMsg(segment, joinPartialMsg)); !slices.Equal(got, row.partial) {
				t.Errorf("in %s %q was logged at %v, want %v: the first of an episode at warn, its repeats at debug",
					row.name, joinPartialMsg, got, row.partial)
			}

			if got := logtest.Levels(logtest.WithMsg(segment, joinCompletedMsg)); !slices.Equal(got, row.completed) {
				t.Errorf("in %s %q was logged at %v, want %v: the first of an episode at info, its repeats at debug",
					row.name, joinCompletedMsg, got, row.completed)
			}

			if row.summary == "" {
				continue
			}

			summaries := logtest.WithMsg(segment, row.summary)
			if len(summaries) != 1 || summaries[0].Int("attempts") != attemptsPerEpisode {
				t.Errorf("%s ended with %v, want one %q with %d attempts", row.name, summaries, row.summary, attemptsPerEpisode)
			}
		}
	})
}
