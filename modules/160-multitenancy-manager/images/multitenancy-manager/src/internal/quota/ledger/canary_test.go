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
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProber answers each API server with its error; a server without an error writes, and the
// webhook sees the nonce unless the server is in skipped.
type fakeProber struct {
	nonces  *Nonces
	errs    map[string]error
	skipped map[string]bool
}

func (p *fakeProber) Probe(_ context.Context, server, nonce string) error {
	if err := p.errs[server]; err != nil {
		return err
	}
	if !p.skipped[server] {
		p.nonces.Record(nonce, time.Now())
	}
	return nil
}

var ours = errors.New(`Internal error occurred: failed calling webhook "quota-ledger.multitenancy-manager.deckhouse.io": dial tcp 10.1.0.5:9444: connect: connection refused`)

func newCanary(servers []string, prober *fakeProber, peerReady bool) (*Canary, *[]string) {
	var stepDowns []string
	c := &Canary{
		Nonces:    prober.nonces,
		Prober:    prober,
		Servers:   func(context.Context) ([]string, error) { return servers, nil },
		PeerReady: func(context.Context) bool { return peerReady },
		StepDown:  func(reason string) { stepDowns = append(stepDowns, reason) },
		Metrics:   NewMetrics(prometheus.NewRegistry()),
		Log:       logr.Discard(),
	}
	return c, &stepDowns
}

func rounds(c *Canary, n int) {
	for range n {
		c.Round(context.Background())
	}
}

func TestCanaryAllReach(t *testing.T) {
	nonces := &Nonces{}
	c, stepDowns := newCanary([]string{"a:6443", "b:6443", "c:6443"}, &fakeProber{nonces: nonces}, true)
	rounds(c, 5)
	assert.Empty(t, *stepDowns)
	assert.InDelta(t, 5, testutil.ToFloat64(c.Metrics.CanaryResults.WithLabelValues("a:6443", CanaryOK)), 0)
}

func TestCanaryStepsDownWhenAMajorityFails(t *testing.T) {
	nonces := &Nonces{}
	prober := &fakeProber{nonces: nonces, errs: map[string]error{"a:6443": ours, "b:6443": ours}}
	c, stepDowns := newCanary([]string{"a:6443", "b:6443", "c:6443"}, prober, true)
	rounds(c, 2)
	assert.Empty(t, *stepDowns, "two rounds are not enough")
	rounds(c, 1)
	assert.Equal(t, []string{"stepdown"}, *stepDowns)
}

func TestCanaryStaysWithoutAReadyPeer(t *testing.T) {
	nonces := &Nonces{}
	prober := &fakeProber{nonces: nonces, errs: map[string]error{"a:6443": ours, "b:6443": ours}}
	c, stepDowns := newCanary([]string{"a:6443", "b:6443"}, prober, false)
	rounds(c, 5)
	assert.Empty(t, *stepDowns)
}

func TestCanaryIgnoresFailuresElsewhere(t *testing.T) {
	nonces := &Nonces{}
	prober := &fakeProber{nonces: nonces, errs: map[string]error{"a:6443": errors.New("the server is currently unable to handle the request"), "b:6443": context.DeadlineExceeded}}
	c, stepDowns := newCanary([]string{"a:6443", "b:6443"}, prober, true)
	rounds(c, 5)
	assert.Empty(t, *stepDowns)
	assert.InDelta(t, 5, testutil.ToFloat64(c.Metrics.CanaryResults.WithLabelValues("a:6443", CanaryUnknown)), 0)
}

// One API server that cannot reach the leader gets one step down to learn whether another pod helps,
// and no more for a while.
func TestCanaryProbesASingleServerOnce(t *testing.T) {
	nonces := &Nonces{}
	prober := &fakeProber{nonces: nonces, errs: map[string]error{"a:6443": ours}}
	c, stepDowns := newCanary([]string{"a:6443", "b:6443", "c:6443"}, prober, true)
	clock := time.Now()
	c.Now = func() time.Time { return clock }
	rounds(c, 3)
	assert.Len(t, *stepDowns, 1)
	rounds(c, 6)
	assert.Len(t, *stepDowns, 1, "not again within five minutes")
	clock = clock.Add(singleProbeEvery)
	rounds(c, 3)
	assert.Len(t, *stepDowns, 2)
}

// The step down for a single API server is shared by the replicas: the one that takes the lease after it
// does not step down for the same server before singleProbeEvery has passed, so the lease is not handed
// back and forth with a moment without a leader each time.
func TestCanaryProbesASingleServerOnceAcrossReplicas(t *testing.T) {
	var kept map[string]time.Time
	record := func(_ context.Context, probes map[string]time.Time) error { kept = probes; return nil }
	load := func(context.Context) (map[string]time.Time, error) { return kept, nil }
	clock := time.Now()
	replica := func() (*Canary, *[]string) {
		c, stepDowns := newCanary([]string{"a:6443", "b:6443", "c:6443"}, &fakeProber{nonces: &Nonces{}, errs: map[string]error{"a:6443": ours}}, true)
		c.Now = func() time.Time { return clock }
		c.RecordSingle, c.LoadSingle = record, load
		return c, stepDowns
	}
	first, firstStepDowns := replica()
	second, secondStepDowns := replica()

	rounds(first, 3)
	require.Len(t, *firstStepDowns, 1)
	assert.Contains(t, kept, "a:6443")

	clock = clock.Add(stepDownHoldOff)
	second.Reset()
	rounds(second, 6)
	assert.Empty(t, *secondStepDowns, "the next leader knows of the step down of the first one")

	clock = clock.Add(singleProbeEvery)
	rounds(second, 1)
	assert.Len(t, *secondStepDowns, 1)
}

// A step down for a single API server that cannot be kept for the next leader is not made: the next
// leader would make it again at once.
func TestCanaryKeepsLeadingWhenTheSingleProbeIsNotKept(t *testing.T) {
	c, stepDowns := newCanary([]string{"a:6443", "b:6443", "c:6443"}, &fakeProber{nonces: &Nonces{}, errs: map[string]error{"a:6443": ours}}, true)
	c.RecordSingle = func(context.Context, map[string]time.Time) error { return errors.New("unavailable") }
	c.LoadSingle = func(context.Context) (map[string]time.Time, error) { return nil, nil }
	rounds(c, 5)
	assert.Empty(t, *stepDowns)

	c.RecordSingle = func(context.Context, map[string]time.Time) error { return nil }
	rounds(c, 1)
	assert.Len(t, *stepDowns, 1)
}

// A write that succeeds although the webhook never saw it means the webhook is not called.
func TestCanaryNoticesAMissingWebhook(t *testing.T) {
	nonces := &Nonces{}
	prober := &fakeProber{nonces: nonces, skipped: map[string]bool{"a:6443": true}}
	c, stepDowns := newCanary([]string{"a:6443"}, prober, true)
	rounds(c, 1)
	assert.InDelta(t, 1, testutil.ToFloat64(c.Metrics.CanaryResults.WithLabelValues("a:6443", CanaryNoNonce)), 0)
	assert.Empty(t, *stepDowns)
}

// The API servers that write without calling the webhook are reported once they did so canaryRounds
// times in a row, and again whenever that changes or a new leadership starts.
func TestCanaryReportsTheServersThatDoNotCallTheWebhook(t *testing.T) {
	nonces := &Nonces{}
	prober := &fakeProber{nonces: nonces, skipped: map[string]bool{"b:6443": true}}
	c, stepDowns := newCanary([]string{"a:6443", "b:6443"}, prober, true)
	clock := time.Now()
	c.Now = func() time.Time {
		clock = clock.Add(600 * time.Millisecond)
		return clock
	}
	var reports [][]string
	c.Report = func(_ context.Context, notCalled []string) error {
		reports = append(reports, notCalled)
		return nil
	}

	rounds(c, 2)
	assert.Equal(t, [][]string{{}}, reports, "a leadership reports from its first round")
	rounds(c, 1)
	assert.Equal(t, [][]string{{}, {"b:6443"}}, reports)
	rounds(c, 3)
	assert.Len(t, reports, 2, "a report is not repeated")
	assert.Empty(t, *stepDowns, "the leader is not at fault")

	prober.skipped = nil
	rounds(c, 1)
	assert.Equal(t, []string{}, reports[2])

	c.Load = func(context.Context) ([]string, error) { return nil, nil }
	c.Reset()
	rounds(c, 1)
	assert.Len(t, reports, 3, "a new leadership that finds nothing reported has nothing to write")
}

// A new leadership goes on from the report of the former one: an API server stays reported until the
// canary sees it call the webhook, and nothing flaps back to "called" in between.
func TestCanaryGoesOnFromTheFormerReport(t *testing.T) {
	nonces := &Nonces{}
	prober := &fakeProber{nonces: nonces, skipped: map[string]bool{"b:6443": true}}
	c, _ := newCanary([]string{"a:6443", "b:6443"}, prober, true)
	clock := time.Now()
	c.Now = func() time.Time {
		clock = clock.Add(600 * time.Millisecond)
		return clock
	}
	var reports [][]string
	c.Report = func(_ context.Context, notCalled []string) error {
		reports = append(reports, notCalled)
		return nil
	}
	c.Load = func(context.Context) ([]string, error) { return []string{"b:6443"}, nil }

	c.Reset()
	rounds(c, 3)
	assert.Empty(t, reports, "b stays reported")

	prober.skipped = nil
	rounds(c, 1)
	assert.Equal(t, [][]string{{}}, reports, "b called the webhook")

	c.Load = func(context.Context) ([]string, error) { return nil, errors.New("unavailable") }
	c.Reset()
	rounds(c, 2)
	assert.Len(t, reports, 1, "nothing is written while the former report is unknown")
}
