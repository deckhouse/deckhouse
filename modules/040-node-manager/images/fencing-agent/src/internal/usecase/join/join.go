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

// Package join builds a NodeGroup's seed list from the Kubernetes API and does
// the startup join into its gossip network.
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

// maxSeeds caps a join. memberlist dials seeds one by one and exchanges full
// state with each, so a few reachable ones are enough; gossip does the rest.
const maxSeeds = 3

type NodeLister interface {
	ListNodeGroup(ctx context.Context, nodeGroup string) ([]domain.Peer, error)
}

type Cluster interface {
	Join(seeds []string) (int, error)
	NumMembers() int
}

type Params struct {
	NodeName string
	// NodeIP also drops a stale Node object holding the local IP under another
	// name, and blocks a hairpin self-join.
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

// Joined is false until the startup join completes; the fencing flow must not start before it.
func (j *Joiner) Joined() bool {
	return j.joined.Load()
}

// Bootstrap retries the join with exponential backoff until it succeeds or ctx
// is cancelled; a permanent failure keeps the pod NotReady instead of crashing.
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

	// First agent of the group: listeners are up, later peers seed from us.
	if peers == 0 {
		j.logger.Info("no peers in node group, starting alone", "node_group", j.params.NodeGroup)

		return nil
	}

	// Peers exist but have no address yet; declaring "alone" would split the group into islands.
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

// join wraps the uncancellable Cluster.Join so a SIGTERM does not sit through its
// per-seed dial timeouts. The abandoned goroutine ends with the transport.
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

// seedList is rebuilt every attempt so a retry uses current membership, not a
// pre-outage snapshot. It also returns the peer count (self excluded).
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

		// Stale Node object of this machine under an old name, not a peer.
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

// delay is full jitter in [RetryInterval, backoff]. Narrow jitter would keep the
// group's agents retrying in lockstep after a shared outage.
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
