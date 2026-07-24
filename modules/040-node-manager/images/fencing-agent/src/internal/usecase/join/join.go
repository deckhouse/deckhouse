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

// Package join builds the seed list of a NodeGroup from the Kubernetes API and
// performs the startup join into the gossip network of that NodeGroup.
package join

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

// maxSeeds bounds a single join. memberlist contacts every seed sequentially
// and exchanges the full member state with each of them, even after the first
// success, and every unreachable seed costs a full TCP dial timeout. One
// reachable seed is enough, gossip delivers the rest, and every retry samples
// a fresh subset.
const maxSeeds = 3

// NodeLister reads the current NodeGroup membership from the Kubernetes API.
type NodeLister interface {
	ListNodeGroup(ctx context.Context, nodeGroup string) ([]domain.Peer, error)
}

// Cluster is the gossip network of the NodeGroup.
type Cluster interface {
	Join(seeds []string) (int, error)
	NumMembers() int
}

type Params struct {
	NodeName string
	// NodeIP also filters the local node out of the seed list: a stale Node
	// object may carry the same IP under another name, and joining our own
	// advertised address is a hairpin that can succeed and fake a join.
	NodeIP           string
	NodeGroup        string
	MemberlistPort   int
	APITimeout       time.Duration
	RetryInterval    time.Duration
	MaxRetryInterval time.Duration
}

type Joiner struct {
	nodes   NodeLister
	cluster Cluster
	params  Params
	logger  *log.Logger
	joined  atomic.Bool
}

func New(nodes NodeLister, cluster Cluster, params Params, logger *log.Logger) *Joiner {
	return &Joiner{
		nodes:   nodes,
		cluster: cluster,
		params:  params,
		logger:  logger,
	}
}

// Joined reports whether the startup join has completed. The fencing flow must
// not start before it has.
func (j *Joiner) Joined() bool {
	return j.joined.Load()
}

// Bootstrap joins the gossip network, retrying with exponential backoff until
// it succeeds or ctx is cancelled. A permanent failure keeps the pod NotReady
// instead of crashing it, and cancellation is a shutdown, not a failure.
func (j *Joiner) Bootstrap(ctx context.Context) {
	backoff := j.params.RetryInterval

	for attempt := 1; ; attempt++ {
		err := j.attempt(ctx)
		if err == nil {
			j.joined.Store(true)

			return
		}

		if ctx.Err() != nil {
			j.logger.Info("memberlist bootstrap join aborted", "error", err)

			return
		}

		j.logger.Warn("memberlist bootstrap join failed, retrying",
			"error", err,
			"attempt", attempt,
			"backoff", backoff.String(),
		)

		if !sleep(ctx, j.delay(backoff)) {
			return
		}

		backoff = min(backoff*2, j.params.MaxRetryInterval)
	}
}

func (j *Joiner) attempt(ctx context.Context) error {
	seeds, peers, err := j.seedList(ctx)
	if err != nil {
		return err
	}

	// Being the first agent of the NodeGroup is not a failure: the listeners are
	// already up and the peers that start later seed themselves from this node.
	if peers == 0 {
		j.logger.Info("no peers in node group, starting alone", "node_group", j.params.NodeGroup)

		return nil
	}

	// Peers exist but none is usable yet (addresses not populated): declaring
	// "alone" here would split the group into islands, so keep retrying.
	if len(seeds) == 0 {
		return fmt.Errorf("none of the %d peers has a usable address yet", peers)
	}

	joined, err := j.join(ctx, seeds)
	if err != nil {
		return err
	}

	if joined < len(seeds) {
		j.logger.Warn("some seeds are unreachable, gossip will converge",
			"seeds", len(seeds),
			"joined", joined,
		)
	}

	j.logger.Info("memberlist bootstrap join completed",
		"seeds", len(seeds),
		"joined", joined,
		"members", j.cluster.NumMembers(),
	)

	return nil
}

// join runs Cluster.Join on its own goroutine: the underlying call dials seeds
// sequentially without honoring a context, and a SIGTERM must not wait out its
// dial timeouts. An abandoned call unblocks once the transport is closed.
func (j *Joiner) join(ctx context.Context, seeds []string) (int, error) {
	type result struct {
		joined int
		err    error
	}

	resCh := make(chan result, 1)

	go func() {
		joined, err := j.cluster.Join(seeds)
		resCh <- result{joined: joined, err: err}
	}()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case res := <-resCh:
		if res.joined == 0 {
			err := res.err
			if err == nil {
				err = errors.New("no seed accepted the connection")
			}

			return 0, fmt.Errorf("join gossip network of %d seeds: %w", len(seeds), err)
		}

		return res.joined, nil
	}
}

// seedList is rebuilt on every attempt so that a retry always uses the current
// NodeGroup membership instead of a snapshot taken before the outage. It also
// returns how many peer Nodes (self excluded) the group currently has.
func (j *Joiner) seedList(ctx context.Context) ([]string, int, error) {
	listCtx, cancel := context.WithTimeout(ctx, j.params.APITimeout)
	defer cancel()

	nodes, err := j.nodes.ListNodeGroup(listCtx, j.params.NodeGroup)
	if err != nil {
		return nil, 0, err
	}

	port := strconv.Itoa(j.params.MemberlistPort)

	peers := 0
	seeds := make([]string, 0, len(nodes))

	for _, peer := range nodes {
		if peer.Name == j.params.NodeName {
			continue
		}

		// A stale Node object can carry the local IP under another name; it is
		// leftover metadata of this machine, not a peer.
		if peer.IP != "" && peer.IP == j.params.NodeIP {
			j.logger.Warn("node shares the local InternalIP, excluded from seed list", "member", peer.Name)

			continue
		}

		peers++

		if peer.IP == "" {
			j.logger.Warn("node has no InternalIP, excluded from seed list", "member", peer.Name)

			continue
		}

		seeds = append(seeds, net.JoinHostPort(peer.IP, port))
	}

	if len(seeds) > maxSeeds {
		j.logger.Info("seed list sampled", "eligible", len(seeds), "sampled", maxSeeds)

		rand.Shuffle(len(seeds), func(a, b int) { seeds[a], seeds[b] = seeds[b], seeds[a] })
		seeds = seeds[:maxSeeds]
	}

	return seeds, peers, nil
}

// delay is a full-jitter sample in [RetryInterval, backoff]: agents of one
// NodeGroup share outages, and a narrow jitter would keep their retries in
// lockstep.
func (j *Joiner) delay(backoff time.Duration) time.Duration {
	spread := backoff - j.params.RetryInterval
	if spread <= 0 {
		return backoff
	}

	return j.params.RetryInterval + rand.N(spread+1)
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
