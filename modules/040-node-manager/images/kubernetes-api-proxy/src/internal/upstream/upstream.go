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

package upstream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"sync/atomic"
	"time"

	"github.com/siderolabs/tcpproxy"
	"k8s.io/client-go/transport"

	"kubernetes-api-proxy/pkg/kubernetes"
)

// maxReadyzBodyBytes bounds how much of a /readyz response is read; the verbose
// output of a failing apiserver is a few KiB.
const maxReadyzBodyBytes = 64 << 10

// readyzPath must stay exactly /readyz: that path is granted by
// system:public-info-viewer and exempted from APF by the "probes" FlowSchema,
// so a probe never queues behind client traffic.
const readyzPath = "/readyz"

var errNoProbeConfig = errors.New("probe config getter is not set")

type Upstream struct {
	Addr string

	probeConfig kubernetes.ClusterConfigGetter
}

func NewUpstream(addr string, probeConfig kubernetes.ClusterConfigGetter) *Upstream {
	return &Upstream{
		Addr:        addr,
		probeConfig: probeConfig,
	}
}

func (u *Upstream) Address() string {
	return u.Addr
}

type probeKind int

const (
	probeReady probeKind = iota + 1
	probeNotReady
	probeSoft
	probeHard
)

func (k probeKind) String() string {
	switch k {
	case probeReady:
		return "ready"
	case probeNotReady:
		return "not-ready"
	case probeSoft:
		return "timeout"
	case probeHard:
		return "unreachable"
	}

	return "unknown"
}

func (k probeKind) answered() bool {
	return k == probeReady || k == probeNotReady
}

type probeResult struct {
	kind    probeKind
	latency time.Duration
}

func (u *Upstream) probe(ctx context.Context, dial *transport.DialHolder) (probeResult, error) {
	rt, err := u.transport(dial)
	if err != nil {
		return probeResult{}, err
	}

	var gotConn atomic.Bool

	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GotConn: func(httptrace.GotConnInfo) { gotConn.Store(true) },
	})

	start := time.Now()
	status, err := get(ctx, rt, u.Addr)

	return classify(status, err, gotConn.Load(), time.Since(start)), nil
}

func (u *Upstream) transport(dial *transport.DialHolder) (http.RoundTripper, error) {
	if u.probeConfig == nil {
		return nil, errNoProbeConfig
	}

	cfg, err := u.probeConfig()
	if err != nil {
		return nil, fmt.Errorf("probe config: %w", err)
	}

	tc, err := cfg.TransportConfig()
	if err != nil {
		return nil, fmt.Errorf("transport config: %w", err)
	}

	tc.DialHolder = dial

	// transport.New keeps transports in a cache. The key includes the TLS
	// settings, the CA bytes and the dial. When the files do not change, the
	// probe gets the cached transport and its warm connection. A new ca.crt
	// gives a new transport without a restart of the proxy.
	rt, err := transport.New(tc)
	if err != nil {
		return nil, fmt.Errorf("build transport: %w", err)
	}

	return rt, nil
}

// probeDial returns the dial for the probe transport. Each List keeps one
// value, because the transport cache compares the dial by its pointer.
//
// The dial sets TCP_USER_TIMEOUT of each probe connection to the probe
// timeout (Linux only). When the network drops packets, the kernel closes the
// connection after this time, and the next probe dials a new connection.
// Without the option, the probes wait on the old connection for the next
// retransmission, which can come some seconds after the network is available
// again.
func probeDial(userTimeout time.Duration) *transport.DialHolder {
	var d net.Dialer

	return &transport.DialHolder{Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := d.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		_ = tcpproxy.SetTCPUserTimeout(c, userTimeout)

		return c, nil
	}}
}

func get(ctx context.Context, rt http.RoundTripper, addr string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+addr+readyzPath, nil)
	if err != nil {
		return 0, err
	}

	resp, err := (&http.Client{Transport: rt}).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxReadyzBodyBytes)); err != nil {
		return 0, err
	}

	return resp.StatusCode, nil
}

func classify(status int, err error, gotConn bool, latency time.Duration) probeResult {
	switch {
	case gotConn && errors.Is(err, context.DeadlineExceeded):
		return probeResult{kind: probeSoft}
	case err != nil:
		return probeResult{kind: probeHard}
	case status == http.StatusOK:
		return probeResult{kind: probeReady, latency: latency}
	default:
		return probeResult{kind: probeNotReady, latency: latency}
	}
}

var tierBucket = []time.Duration{
	0,
	10 * time.Millisecond,
	25 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
}

const tierFailed Tier = 7

func calcTier(latency time.Duration) Tier {
	for i := len(tierBucket) - 1; i > 0; i-- {
		if latency >= tierBucket[i] {
			return Tier(i)
		}
	}

	return 0
}
