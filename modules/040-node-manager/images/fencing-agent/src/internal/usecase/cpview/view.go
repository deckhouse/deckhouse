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

// Package cpview tells whether the control plane is visible from the node.
// The data comes from /upstreams of the local kubernetes-api-proxy
// (cpSource=proxy) or from the own probes of the agent (cpSource=self).
package cpview

import (
	"context"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/proxyclient"
	"fencing-agent/internal/domain"
)

const (
	readInterval     = 200 * time.Millisecond
	fastReadInterval = 100 * time.Millisecond
	// readTimeout is less than fastReadInterval, so a read that does not
	// get an answer does not move the next read.
	readTimeout = 80 * time.Millisecond
	// proxyLostAfter is the time without an answer of the proxy after which
	// the own probes start. One failed read does not change the source.
	proxyLostAfter = time.Second
)

// Client reads the local kubernetes-api-proxy and probes the apiserver.
type Client interface {
	// Upstreams reads /upstreams. An error means that the proxy does not answer.
	Upstreams(ctx context.Context) (proxyclient.Upstreams, error)
	// Ready probes /readyz at addr (host:port) and tells whether the answer is 200.
	Ready(ctx context.Context, addr string) bool
	// APIServers returns the apiserver addresses (host:port) from the
	// EndpointSlice default/kubernetes.
	APIServers() []string
}

// View implements decision.ControlPlane. It reads the control plane state
// every 200 ms, and every 100 ms while lastReadyAgo > fastReadAfter, from the
// start of one read to the start of the next.
type View struct {
	client Client
	status domain.Status
	ownIPs func() []string
	logger *log.Logger

	updates chan domain.CPState
	start   time.Time
	// proxyAnswers is true when the last read of /upstreams got an answer.
	proxyAnswers atomic.Bool
	// answeredAt is the time of the last answer of the proxy, at first the
	// start. Only the Run goroutine uses it.
	answeredAt time.Time

	mu                    sync.Mutex
	controlPlaneLostAfter time.Duration
	fastReadAfter         time.Duration
	source                domain.CPSource
	// seq and ok are the number and the result of the last probe of any source.
	seq uint64
	ok  bool
	// lastReady is the time of the last ready answer of any upstream.
	lastReady time.Time
	visible   bool
	// misses counts the reads in a row where the control plane visibility rule fails.
	misses int

	prober     *prober
	stopProber func()
}

// New makes a View. controlPlaneLostAfter comes from the applied profile,
// fastReadAfter from the common timings of the configuration. ownIPs returns the IP addresses of the own Node: on a
// master the own probes do not use the own apiserver while other apiservers
// exist.
func New(client Client, status domain.Status, ownIPs func() []string, controlPlaneLostAfter, fastReadAfter time.Duration, logger *log.Logger) *View {
	now := time.Now()

	return &View{
		client:                client,
		status:                status,
		ownIPs:                ownIPs,
		logger:                logger,
		updates:               make(chan domain.CPState, 1),
		start:                 now,
		answeredAt:            now,
		controlPlaneLostAfter: controlPlaneLostAfter,
		fastReadAfter:         fastReadAfter,
	}
}

// SetTimings applies new controlPlaneLostAfter and fastReadAfter.
func (v *View) SetTimings(controlPlaneLostAfter, fastReadAfter time.Duration) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.controlPlaneLostAfter, v.fastReadAfter = controlPlaneLostAfter, fastReadAfter
}

// Updates delivers each new read. When the reader is slow, it gets only the
// last read.
func (v *View) Updates() <-chan domain.CPState {
	return v.updates
}

// Run blocks until ctx ends.
func (v *View) Run(ctx context.Context) error {
	defer v.stopSelf()

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}

		begin := time.Now()
		st := v.read(ctx)

		select {
		case <-v.updates:
		default:
		}
		v.updates <- st

		// The next read starts an interval after the start of this read: the
		// period does not depend on the time of the read.
		timer.Reset(time.Until(begin.Add(v.interval(st.LastReadyAgo))))
	}
}

func (v *View) interval(lastReadyAgo time.Duration) time.Duration {
	v.mu.Lock()
	defer v.mu.Unlock()

	if lastReadyAgo > v.fastReadAfter {
		return fastReadInterval
	}

	return readInterval
}

func (v *View) read(ctx context.Context) domain.CPState {
	rctx, cancel := context.WithTimeout(ctx, readTimeout)
	u, err := v.client.Upstreams(rctx)
	cancel()

	now := time.Now()
	v.proxyAnswers.Store(err == nil)

	if err == nil {
		v.answeredAt = now
	}

	switch {
	case err == nil && u.HasProbe:
		v.stopSelf()
		v.setSource(domain.CPSourceProxy)
		v.observe(u.ProbeSeq, u.ProbeOK, now, u.LastReadyAgo...)
	case err == nil || v.prober != nil || now.Sub(v.answeredAt) >= proxyLostAfter:
		// An old proxy, or no answer of the proxy for proxyLostAfter.
		if v.prober == nil && err != nil {
			v.logger.Warn("local kubernetes-api-proxy does not answer, start the own probes", "error", err)
		}

		p := v.startSelf(ctx)
		v.setSource(domain.CPSourceSelf)

		seq, ok, lastReady := p.state()
		var ago []time.Duration
		if !lastReady.IsZero() {
			ago = append(ago, now.Sub(lastReady))
		}
		v.observe(seq, ok, now, ago...)
	}

	return v.state(now)
}

// observe takes a probe and the ages of the ready answers. A probe number
// that is not larger than the last one is not a new probe.
func (v *View) observe(seq uint64, ok bool, now time.Time, ago ...time.Duration) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if seq > v.seq {
		v.seq, v.ok = seq, ok
	}

	for _, a := range ago {
		if t := now.Add(-a); t.After(v.lastReady) {
			v.lastReady = t
		}
	}
}

// state applies the control plane visibility rule: the control plane is
// visible when an upstream was ready no more than controlPlaneLostAfter ago,
// and lost after two reads in a row without it.
func (v *View) state(now time.Time) domain.CPState {
	v.mu.Lock()
	defer v.mu.Unlock()

	seen := !v.lastReady.IsZero()

	// Before the first ready answer lastReadyAgo is the time since the start.
	lastReadyAgo := now.Sub(v.start)
	if seen {
		lastReadyAgo = now.Sub(v.lastReady)
	}

	wasVisible := v.visible

	if seen && lastReadyAgo <= v.controlPlaneLostAfter {
		v.visible, v.misses = true, 0
	} else {
		v.misses++
		if v.misses >= 2 {
			v.visible = false
		}
	}

	if v.visible != wasVisible {
		v.logger.Info("control plane visibility changed", "visible", v.visible, "cp_source", v.source, "last_ready_ago", lastReadyAgo.String())
	}

	return domain.CPState{
		Source:       v.source,
		Visible:      v.visible,
		Seen:         seen,
		LastReadyAgo: lastReadyAgo,
		ProbeSeq:     v.seq,
		ProbeOK:      v.ok,
		ReadAt:       now,
	}
}

func (v *View) setSource(s domain.CPSource) {
	v.mu.Lock()
	changed := v.source != s
	v.source = s
	v.mu.Unlock()

	if changed {
		v.logger.Info("control plane source changed", "cp_source", s)
		v.status.SetCPSource(s)
	}
}

// startSelf starts the own probes if they do not run. The probe numbers are
// larger than the last number of the proxy.
func (v *View) startSelf(ctx context.Context) *prober {
	if v.prober != nil {
		return v.prober
	}

	v.mu.Lock()
	p := &prober{ready: v.client.Ready, targets: v.targets, started: v.seq, seq: v.seq, ok: v.ok}
	v.mu.Unlock()

	pctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		p.run(pctx)
	}()

	v.prober = p
	v.stopProber = func() {
		cancel()
		<-done
	}

	return p
}

// stopSelf stops the own probes: they never run together with the data of the proxy.
func (v *View) stopSelf() {
	if v.prober == nil {
		return
	}

	v.stopProber()
	v.prober, v.stopProber = nil, nil
}

// targets returns the local proxy when it answers, else the apiservers from
// the EndpointSlice. On a master the proxy is not a target because it can send
// the probe to the own apiserver. The own apiserver is a target only when the
// EndpointSlice has no other apiservers (a single master).
func (v *View) targets() []string {
	all := v.client.APIServers()
	own := v.ownIPs()

	direct := slices.DeleteFunc(slices.Clone(all), func(addr string) bool {
		host, _, err := net.SplitHostPort(addr)

		return err == nil && slices.Contains(own, host)
	})

	master := len(direct) < len(all)
	if v.proxyAnswers.Load() && !master {
		return []string{proxyclient.ProxyAddr}
	}

	if len(direct) == 0 {
		return all
	}

	return direct
}
