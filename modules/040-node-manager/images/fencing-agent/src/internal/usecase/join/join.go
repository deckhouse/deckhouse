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
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

// maxSeeds caps a join. memberlist exchanges full state with each seed, so a few
// reachable ones are enough; gossip does the rest.
const maxSeeds = 3

const (
	notAliveSlots = 2
	aliveSlots    = 1
)

const (
	reasonNotFound        = "not_found"
	reasonReadFailed      = "read_failed"
	reasonLeftNodeGroup   = "left_node_group"
	reasonNoInternalIP    = "no_internal_ip"
	reasonLocalInternalIP = "local_internal_ip"
)

const (
	classNone      = "none"
	classTransport = "transport"
	classNotMember = "not_member"
)

var ErrNotMember = errors.New("this node is not a member of its NodeGroup any more")

type NodeReader interface {
	GetNode(ctx context.Context, name string) (domain.NodeRecord, error)
}

type ExpectedSource interface {
	Expected() ([]domain.Peer, uint64)
}

type Cluster interface {
	Join(seeds []string) (int, error)
	NumMembers() int
	Members() []string
}

type Params struct {
	NodeName string
	NodeUID  string
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
	nodes    NodeReader
	expected ExpectedSource
	cluster  Cluster
	params   Params
	logger   *log.Logger
	joined   atomic.Bool

	mu     sync.Mutex
	logged map[string]struct{}
}

func New(nodes NodeReader, expected ExpectedSource, cluster Cluster, params Params, logger *log.Logger) *Joiner {
	return &Joiner{
		nodes:    nodes,
		expected: expected,
		cluster:  cluster,
		params:   params,
		logger:   logger,
		logged:   make(map[string]struct{}),
	}
}

func (j *Joiner) StartEpisode() {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.logged = make(map[string]struct{})
}

func (j *Joiner) logOnce(level slog.Level, key, msg string, args ...any) {
	j.mu.Lock()

	if _, seen := j.logged[key]; seen {
		level = slog.LevelDebug
	} else {
		j.logged[key] = struct{}{}
	}

	j.mu.Unlock()

	j.logger.Log(context.Background(), level, msg, args...)
}

// Joined is false until the startup join completes; the fencing flow must not start before it.
func (j *Joiner) Joined() bool {
	return j.joined.Load()
}

// Bootstrap retries the join with exponential backoff until it succeeds or ctx
// is cancelled; a permanent failure keeps the pod NotReady instead of crashing.
func (j *Joiner) Bootstrap(ctx context.Context) {
	j.StartEpisode()

	ep := episode{start: time.Now()}
	backoff := j.params.RetryInterval

	for {
		attemptStart := time.Now()
		ep.attempts++

		err := j.Attempt(ctx)
		if err == nil {
			j.joined.Store(true)
			j.logger.Info("memberlist bootstrap join finished", ep.summary()...)

			return
		}

		if ctx.Err() != nil {
			j.logger.Info("memberlist bootstrap join aborted", slices.Concat([]any{"error", err}, ep.summary())...)

			return
		}

		class := classOf(err)
		delay := j.delay(backoff)

		level := slog.LevelDebug

		if ep.streak.class != class {
			if ep.streak.class != "" {
				j.logger.Info("memberlist bootstrap join failure streak ended", ep.streakSummary(attemptStart)...)
			}

			ep.streak = streak{class: class, start: attemptStart}
			level = slog.LevelWarn
		}

		ep.streak.attempts++

		msg := "memberlist bootstrap join failed, retrying"
		if class == classNotMember {
			msg = "this node is not a member of its NodeGroup, the join is retried until that changes"
		}

		j.logger.Log(context.Background(), level, msg,
			"error", err,
			"attempt", ep.attempts,
			"attempt_elapsed", time.Since(attemptStart).String(),
			"next_in", delay.String(),
		)

		ep.lastDelay = delay

		if !sleep(ctx, delay) {
			j.logger.Info("memberlist bootstrap join aborted", slices.Concat([]any{"error", err}, ep.summary())...)

			return
		}

		backoff = min(backoff*2, j.params.MaxRetryInterval)
	}
}

type episode struct {
	start     time.Time
	attempts  int
	lastDelay time.Duration
	streak    streak
}

type streak struct {
	class    string
	attempts int
	start    time.Time
}

func (e *episode) summary() []any {
	return runSummary(e.attempts, time.Since(e.start), e.lastDelay, cmp.Or(e.streak.class, classNone))
}

func (e *episode) streakSummary(end time.Time) []any {
	return runSummary(e.streak.attempts, end.Sub(e.streak.start), e.lastDelay, e.streak.class)
}

func runSummary(attempts int, elapsed, lastDelay time.Duration, class string) []any {
	return []any{
		"attempts", attempts,
		"elapsed", elapsed.Truncate(time.Millisecond).String(),
		"last_delay", lastDelay.String(),
		"last_error_class", class,
	}
}

func classOf(err error) string {
	if errors.Is(err, ErrNotMember) {
		return classNotMember
	}

	return classTransport
}

func (j *Joiner) Attempt(ctx context.Context) error {
	if err := j.checkSelf(ctx); err != nil {
		return err
	}

	notAlive, alive, clones, err := j.candidates()
	if err != nil {
		return err
	}

	// First agent of the group: listeners are up, later peers seed from us.
	if len(notAlive)+len(alive) == len(clones) {
		for _, name := range clones {
			j.logOnce(slog.LevelWarn, "clone/"+name, "node shares the local InternalIP, not counted as a peer", "member", name)
		}

		j.logOnce(slog.LevelInfo, "alone", "no peers in node group, starting alone", "node_group", j.params.NodeGroup)

		return nil
	}

	picked := pick(notAlive, alive)
	seeds := j.readCandidates(ctx, picked)

	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Peers exist but none of the picked ones gave a usable address; declaring
	// "alone" would split the group into islands.
	if len(seeds) == 0 {
		return fmt.Errorf("none of the %d join candidates has a usable address: %s", len(picked), strings.Join(picked, ", "))
	}

	joined, err := j.join(ctx, seeds)
	if err != nil {
		return err
	}

	if joined < len(seeds) {
		j.logOnce(slog.LevelWarn, "partial", "some seeds are unreachable, gossip will converge",
			"seeds", len(seeds),
			"joined", joined,
		)
	}

	j.logOnce(slog.LevelInfo, "completed", "memberlist join completed",
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

func (j *Joiner) checkSelf(ctx context.Context) error {
	readCtx, cancel := context.WithTimeout(ctx, j.params.APITimeout)
	defer cancel()

	rec, err := j.nodes.GetNode(readCtx, j.params.NodeName)

	switch {
	case errors.Is(err, domain.ErrNodeNotFound):
		return fmt.Errorf("%w: %w", ErrNotMember, err)
	case err != nil:
		return fmt.Errorf("read own node: %w", err)
	case !domain.InNodeGroup(rec.NodeGroup, j.params.NodeGroup):
		return fmt.Errorf("%w: node %q is labeled into NodeGroup %q, this agent runs for %q",
			ErrNotMember, j.params.NodeName, rec.NodeGroup, j.params.NodeGroup)
	case rec.UID != j.params.NodeUID:
		return fmt.Errorf("%w: node %q has uid %q, this agent started with %q",
			ErrNotMember, j.params.NodeName, rec.UID, j.params.NodeUID)
	}

	return nil
}

func (j *Joiner) candidates() ([]string, []string, []string, error) {
	expected, _ := j.expected.Expected()
	view := domain.NewView(expected, j.cluster.Members())

	notAlive := make([]string, 0, len(expected))
	alive := make([]string, 0, len(expected))
	var clones []string
	self := false

	for _, peer := range expected {
		if peer.Name == j.params.NodeName {
			self = true

			continue
		}

		if view.IsAlive(peer.Name) {
			alive = append(alive, peer.Name)
		} else {
			notAlive = append(notAlive, peer.Name)
		}

		// Stale Node object of this machine under an old name, not a peer.
		if peer.IP != "" && peer.IP == j.params.NodeIP {
			clones = append(clones, peer.Name)
		}
	}

	if !self {
		return nil, nil, nil, errors.New("the node cache does not list this node yet")
	}

	return notAlive, alive, clones, nil
}

func pick(notAlive, alive []string) []string {
	notAlive, alive = shuffled(notAlive), shuffled(alive)

	notAliveN, aliveN := min(maxSeeds, len(notAlive)), min(maxSeeds, len(alive))
	if len(notAlive) > 0 && len(alive) > 0 {
		notAliveN, aliveN = min(notAliveSlots, len(notAlive)), min(aliveSlots, len(alive))
	}

	picked := make([]string, 0, notAliveN+aliveN)
	picked = append(picked, notAlive[:notAliveN]...)

	return append(picked, alive[:aliveN]...)
}

func shuffled(names []string) []string {
	names = slices.Clone(names)
	rand.Shuffle(len(names), func(a, b int) { names[a], names[b] = names[b], names[a] })

	return names
}

type candidate struct {
	seed   string
	reason string
	err    error
}

func (j *Joiner) readCandidates(ctx context.Context, names []string) []string {
	readCtx, cancel := context.WithTimeout(ctx, j.params.APITimeout)
	defer cancel()

	results := make([]candidate, len(names))

	var g errgroup.Group

	for i, name := range names {
		g.Go(func() error {
			results[i] = j.readCandidate(readCtx, name)

			return results[i].err
		})
	}

	_ = g.Wait()

	if ctx.Err() != nil {
		return nil
	}

	seeds := make([]string, 0, len(names))

	for i, res := range results {
		if res.reason != "" {
			j.logDropped(names[i], res)

			continue
		}

		seeds = append(seeds, res.seed)
	}

	return seeds
}

func (j *Joiner) readCandidate(ctx context.Context, name string) candidate {
	rec, err := j.nodes.GetNode(ctx, name)

	switch {
	case errors.Is(err, domain.ErrNodeNotFound):
		return candidate{reason: reasonNotFound}
	case err != nil:
		return candidate{reason: reasonReadFailed, err: err}
	case !domain.InNodeGroup(rec.NodeGroup, j.params.NodeGroup):
		return candidate{reason: reasonLeftNodeGroup}
	case rec.IP == "":
		return candidate{reason: reasonNoInternalIP}
	case rec.IP == j.params.NodeIP:
		return candidate{reason: reasonLocalInternalIP}
	}

	return candidate{seed: net.JoinHostPort(rec.IP, strconv.Itoa(j.params.MemberlistPort))}
}

func (j *Joiner) logDropped(name string, res candidate) {
	key := "dropped/" + name + "/" + res.reason

	switch res.reason {
	case reasonNotFound, reasonLeftNodeGroup:
		j.logOnce(slog.LevelInfo, key, "join candidate dropped", "member", name, "reason", res.reason)
	case reasonReadFailed:
		j.logOnce(slog.LevelWarn, key, "join candidate dropped", "member", name, "reason", res.reason, "error", res.err)
	default:
		j.logOnce(slog.LevelWarn, key, "join candidate dropped", "member", name, "reason", res.reason)
	}
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
