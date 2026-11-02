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

package ledger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
)

const (
	// CanaryName is the ResourceQuota in the namespace of the module that the canary writes the status
	// of. It limits a resource nothing creates, so no create of the module goes through the ledger.
	CanaryName = "quota-ledger-canary"
	// CanaryNonceAnnotation carries the nonce of a canary write; the ledger records the nonces it sees.
	CanaryNonceAnnotation = "quota-ledger.deckhouse.io/canary-nonce"
	// ControllerUser is the service account the canary writes as.
	ControllerUser = "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager"
	// WebhookName is the name of the webhook of the ledger, which the API server puts in its errors.
	WebhookName = "quota-ledger.multitenancy-manager.deckhouse.io"

	canaryInterval = 2 * time.Second
	// canaryWait is how long after a successful write the ledger has to have seen its nonce.
	canaryWait = time.Second
	// canaryRounds is how many rounds in a row a majority of the API servers fails to reach the leader
	// before it steps down.
	canaryRounds = 3
	// stepDownHoldOff is how long a leader that stepped down waits before it takes the lease again.
	stepDownHoldOff = 30 * time.Second
	// singleProbeEvery is how often a failure of a single API server is answered by a step down, to
	// learn whether another pod fixes it.
	singleProbeEvery = 5 * time.Minute
	nonceTTL         = time.Minute
	// maxNonces bounds the nonces kept: a canary round records one per API server, and made-up canary
	// requests must not grow the memory of the ledger.
	maxNonces = 1024
)

// Outcomes of a canary write, the values of the result label of the canary metric.
const (
	CanaryOK      = "ok"
	CanaryOurs    = "ours"
	CanaryUnknown = "unknown"
	CanaryNoNonce = "no_nonce"
)

// Nonces are the canary nonces the webhook has seen.
type Nonces struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

// Record notes a nonce the webhook saw. Only when the nonces fill up are the ones older than the TTL
// dropped, and then the oldest, so that a record costs O(1) but under a flood.
func (n *Nonces) Record(nonce string, at time.Time) {
	if nonce == "" {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.seen == nil {
		n.seen = map[string]time.Time{}
	}
	if _, ok := n.seen[nonce]; !ok && len(n.seen) >= maxNonces {
		n.evictLocked(at)
	}
	n.seen[nonce] = at
}

// evictLocked drops the nonces older than the TTL, or the oldest one when none is.
func (n *Nonces) evictLocked(now time.Time) {
	oldest, oldestAt := "", now
	for key, t := range n.seen {
		if now.Sub(t) > nonceTTL {
			delete(n.seen, key)
			continue
		}
		if !t.After(oldestAt) {
			oldest, oldestAt = key, t
		}
	}
	if len(n.seen) >= maxNonces {
		delete(n.seen, oldest)
	}
}

// Seen reports whether the webhook saw the nonce.
func (n *Nonces) Seen(nonce string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	_, ok := n.seen[nonce]
	return ok
}

// Prober sends a dry-run write of the status of the canary quota through one API server.
type Prober interface {
	Probe(ctx context.Context, server, nonce string) error
}

// Canary checks that every API server reaches the leader. A failure that names the webhook of the
// ledger is the ledger's; a throttled or timed out canary request, or an error without the name, says
// nothing about it. A write that succeeds although the webhook never saw its nonce means the webhook
// is not called at all: canaryRounds such writes in a row are reported.
type Canary struct {
	Nonces  *Nonces
	Prober  Prober
	Servers func(ctx context.Context) ([]string, error)
	// PeerReady reports whether another replica has its informers synced and could take the lease.
	PeerReady func(ctx context.Context) bool
	// StepDown hands the lease over; the reason goes to the handover metric.
	StepDown func(reason string)
	// Report records the API servers that do not call the webhook, sorted, when they change. Optional.
	Report func(ctx context.Context, notCalled []string) error
	// Load reads what Report recorded last, by this leadership or a former one. Optional.
	Load func(ctx context.Context) ([]string, error)
	// RecordSingle keeps, for the replica that leads next, when a step down was last made for each set
	// of API servers that alone failed to reach the leader. Optional.
	RecordSingle func(ctx context.Context, probes map[string]time.Time) error
	// LoadSingle reads what RecordSingle kept, by any leadership. Optional.
	LoadSingle func(ctx context.Context) (map[string]time.Time, error)
	Metrics    *Metrics
	Log        logr.Logger
	// Now is time.Now unless a test sets it.
	Now func() time.Time

	// mu keeps the rounds of a former leadership and of a new one apart.
	mu         sync.Mutex
	failing    map[string]int
	lastSingle map[string]time.Time
	// uncalled counts the writes in a row that the webhook did not see, by API server.
	uncalled map[string]int
	reported []string
	// hasReported is false until the report of a leadership is known: loaded or written.
	hasReported bool
	// singleLoaded is false until the single probes of the former leaderships are known.
	singleLoaded bool
}

// Reset starts the count of a new leadership, which goes on from what the former one reported.
func (c *Canary) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.uncalled = map[string]int{}
	c.reported, c.hasReported = nil, false
	c.singleLoaded = false
}

// loadSingleLocked takes over, once per leadership, when the former leaders stepped down for a single
// failing API server: each replica keeps such a step down to one every singleProbeEvery only if all of
// them know of the others'.
func (c *Canary) loadSingleLocked(ctx context.Context) bool {
	if c.singleLoaded || c.LoadSingle == nil {
		return true
	}
	probes, err := c.LoadSingle(ctx)
	if err != nil {
		c.Log.Error(err, "read when the former leaders stepped down for a single API server")
		return false
	}
	for key, at := range probes {
		if at.After(c.lastSingle[key]) {
			c.lastSingle[key] = at
		}
	}
	c.singleLoaded = true
	return true
}

// recordSingleLocked keeps the single probes for the next leader, the ones within singleProbeEvery.
func (c *Canary) recordSingleLocked(ctx context.Context) error {
	if c.RecordSingle == nil {
		return nil
	}
	now := c.now()
	probes := make(map[string]time.Time, len(c.lastSingle))
	for key, at := range c.lastSingle {
		if now.Sub(at) < singleProbeEvery {
			probes[key] = at
		}
	}
	return c.RecordSingle(ctx, probes)
}

// loadLocked takes over the report of a former leadership, once per leadership, so that an API server
// found not calling the webhook stays reported until this leader sees it call.
func (c *Canary) loadLocked(ctx context.Context) bool {
	if c.hasReported || c.Load == nil {
		return true
	}
	notCalled, err := c.Load(ctx)
	if err != nil {
		c.Log.Error(err, "read what the canary reported")
		return false
	}
	for _, server := range notCalled {
		c.uncalled[server] = canaryRounds
	}
	slices.Sort(notCalled)
	c.reported, c.hasReported = append([]string{}, notCalled...), true
	return true
}

func (c *Canary) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Round writes through every API server once and steps down when a majority of them has failed to
// reach this leader canaryRounds times in a row and another replica is ready.
func (c *Canary) Round(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	servers, err := c.Servers(ctx)
	if err != nil || len(servers) == 0 {
		c.Log.Error(err, "find the API servers for the canary")
		return
	}
	if c.failing == nil {
		c.failing = map[string]int{}
		c.lastSingle = map[string]time.Time{}
	}
	if c.uncalled == nil {
		c.uncalled = map[string]int{}
	}
	loaded := c.loadLocked(ctx)
	for server := range c.failing {
		if !slices.Contains(servers, server) {
			delete(c.failing, server)
		}
	}
	for server := range c.uncalled {
		if !slices.Contains(servers, server) {
			delete(c.uncalled, server)
		}
	}

	var failingServers []string
	for _, server := range servers {
		outcome := c.probe(ctx, server)
		if c.Metrics != nil {
			c.Metrics.CanaryResults.WithLabelValues(server, outcome).Inc()
		}
		switch outcome {
		case CanaryOurs:
			c.failing[server]++
		case CanaryOK:
			c.failing[server] = 0
			c.uncalled[server] = 0
		case CanaryNoNonce:
			c.failing[server] = 0
			c.uncalled[server]++
		}
		if c.failing[server] >= canaryRounds {
			failingServers = append(failingServers, server)
		}
	}
	if loaded {
		c.report(ctx)
	}
	if len(failingServers) == 0 {
		return
	}

	majority := len(failingServers) > len(servers)/2
	key := strings.Join(failingServers, ",")
	// the step downs of the former leaders for a single API server count too: without them the
	// replicas would hand the lease to each other every few seconds
	if !majority && !c.loadSingleLocked(ctx) {
		return
	}
	single := !majority && c.now().Sub(c.lastSingle[key]) >= singleProbeEvery
	if !majority && !single {
		return
	}
	if !c.PeerReady(ctx) {
		c.Log.Info("the API servers do not reach the ledger leader, and no other replica is ready to take over", "servers", failingServers)
		return
	}
	if single {
		previous, had := c.lastSingle[key]
		c.lastSingle[key] = c.now()
		if err := c.recordSingleLocked(ctx); err != nil {
			c.Log.Error(err, "keep the step down for a single API server for the next leader; not stepping down")
			if had {
				c.lastSingle[key] = previous
			} else {
				delete(c.lastSingle, key)
			}
			return
		}
	}
	c.Log.Info("stepping down: the API servers do not reach the ledger leader", "servers", failingServers)
	for _, server := range failingServers {
		c.failing[server] = 0
	}
	c.StepDown("stepdown")
}

// report hands the API servers that do not call the webhook to Report when they changed.
func (c *Canary) report(ctx context.Context) {
	if c.Report == nil {
		return
	}
	notCalled := []string{}
	for server, rounds := range c.uncalled {
		if rounds >= canaryRounds {
			notCalled = append(notCalled, server)
		}
	}
	slices.Sort(notCalled)
	if c.hasReported && slices.Equal(notCalled, c.reported) {
		return
	}
	if err := c.Report(ctx, notCalled); err != nil {
		c.Log.Error(err, "report the API servers that do not call the webhook")
		return
	}
	c.reported, c.hasReported = notCalled, true
}

func (c *Canary) probe(ctx context.Context, server string) string {
	nonce, err := newNonce()
	if err != nil {
		return CanaryUnknown
	}
	if err := c.Prober.Probe(ctx, server, nonce); err != nil {
		return attribute(err)
	}
	deadline := c.now().Add(canaryWait)
	for !c.Nonces.Seen(nonce) {
		if c.now().After(deadline) {
			return CanaryNoNonce
		}
		select {
		case <-ctx.Done():
			return CanaryUnknown
		case <-time.After(10 * time.Millisecond):
		}
	}
	return CanaryOK
}

// attribute tells whether a failed canary write failed on the ledger.
func attribute(err error) string {
	if strings.Contains(err.Error(), WebhookName) {
		return CanaryOurs
	}
	return CanaryUnknown
}

func newNonce() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
