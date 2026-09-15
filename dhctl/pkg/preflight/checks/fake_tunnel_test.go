// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cenkalti/backoff/v4"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/utils"
)

// The two checks that reach the outside world from a node — registry-access-through-proxy and
// cloud-api-accessibility — were the last ones with no test at all, because everything they do
// happens on the far side of an SSH port-forward.
//
// The forward is the only part that needs faking. A tunnel listens on a local port and copies
// bytes to somewhere the local process cannot otherwise reach; fakeTunnel does exactly that, on
// the same local port and to a target the test chooses. Everything downstream then runs
// unmodified: the HTTP client really is built for localhost:22323, the proxy handshake really
// happens, the registry's CA really is verified against the certificate presented. What the test
// controls is only what sits at the far end.

// fakeSSHClient is an SSH client whose Tunnel is local. The embedded nil interface is deliberate:
// a check that reaches for a method these tests do not stand up panics at that line instead of
// silently getting a zero value, and lib-connection can add methods without breaking the build.
type fakeSSHClient struct {
	libcon.SSHClient

	sess *session.Session
	// target is the host:port every tunnel forwards to, whatever address it was asked for.
	target string
	// tunnelErr, when set, is what Up() returns — the "sshd refuses to forward" case.
	tunnelErr error
	// reachErr, when set, is what the availability probe returns — the "master never booted"
	// case.
	reachErr error
	// onAwait, when set, receives the retry parameters the check asked the probe for.
	onAwait func(retry.Params)

	mu      sync.Mutex
	tunnels []string
}

func newFakeSSHClient(target string, hosts ...string) *fakeSSHClient {
	if len(hosts) == 0 {
		hosts = []string{"10.0.0.5"}
	}
	available := make([]session.Host, 0, len(hosts))
	for _, host := range hosts {
		available = append(available, session.Host{Host: host})
	}

	return &fakeSSHClient{
		sess:   session.NewSession(session.Input{User: "ubuntu", AvailableHosts: available}),
		target: target,
	}
}

func (c *fakeSSHClient) Tunnel(address string) libcon.Tunnel {
	c.mu.Lock()
	c.tunnels = append(c.tunnels, address)
	c.mu.Unlock()

	return &fakeTunnel{spec: address, target: c.target, upErr: c.tunnelErr}
}

func (c *fakeSSHClient) Session() *session.Session { return c.sess }

// Check is the availability probe cloud-api-accessibility waits on before it opens the tunnel.
// reachErr stands in for a master that never answers SSH.
func (c *fakeSSHClient) Check() libcon.Check {
	return fakeCheck{err: c.reachErr, onAwait: c.onAwait}
}

type fakeCheck struct {
	libcon.Check

	err error
	// onAwait records the retry parameters the check asked for. AwaitAvailability is the whole
	// wait — it loops inside lib-connection and returns only the final outcome — so what a test
	// can check here is the budget the check requested, not the succession of failures within it.
	onAwait func(retry.Params)
}

func (c fakeCheck) WithDelaySeconds(int) libcon.Check { return c }

func (c fakeCheck) AwaitAvailability(_ context.Context, params retry.Params) error {
	if c.onAwait != nil {
		c.onAwait(params)
	}
	return c.err
}

func (c fakeCheck) CheckAvailability(context.Context) error { return c.err }

func (c fakeCheck) String() string { return "fake availability check" }

// fakeSSHProvider hands out the one client the test set up.
type fakeSSHProvider struct {
	libcon.SSHProvider

	client libcon.SSHClient
	err    error
}

func (p fakeSSHProvider) Client(context.Context) (libcon.SSHClient, error) {
	return p.client, p.err
}

// fakeSSHProviderInitializer is the sshClientSource cloud-api-accessibility resolves its client
// through.
type fakeSSHProviderInitializer struct {
	provider libcon.SSHProvider
	err      error
	legacy   bool
}

func (i fakeSSHProviderInitializer) GetSSHProvider(context.Context) (libcon.SSHProvider, error) {
	return i.provider, i.err
}

func (i fakeSSHProviderInitializer) IsLegacyMode() bool { return i.legacy }

// sourceOf wires a client up as the initializer the check takes.
func sourceOf(client libcon.SSHClient) fakeSSHProviderInitializer {
	return fakeSSHProviderInitializer{provider: fakeSSHProvider{client: client}}
}

// tunnelSpecs returns every forward the check asked for, in order. The spelling differs between
// the two SSH backends and nothing else asserts it.
func (c *fakeSSHClient) tunnelSpecs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]string(nil), c.tunnels...)
}

// fakeTunnel forwards utils.ProxyTunnelPort on this host to target, which is what the real tunnel
// does across SSH.
type fakeTunnel struct {
	spec   string
	target string
	upErr  error

	listener net.Listener
	wg       sync.WaitGroup
}

func (t *fakeTunnel) Up(context.Context) error {
	if t.upErr != nil {
		return t.upErr
	}

	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", utils.ProxyTunnelPort))
	if err != nil {
		return err
	}
	t.listener = listener

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // the listener was closed by Stop
			}
			t.wg.Add(1)
			go func() {
				defer t.wg.Done()
				t.forward(conn)
			}()
		}
	}()

	return nil
}

func (t *fakeTunnel) forward(local net.Conn) {
	remote, err := net.Dial("tcp", t.target)
	if err != nil {
		_ = local.Close()
		return
	}

	splice(local, remote)
}

func (t *fakeTunnel) Stop() {
	if t.listener != nil {
		_ = t.listener.Close()
	}
	t.wg.Wait()
}

func (t *fakeTunnel) HealthMonitor(chan<- error) {}

func (t *fakeTunnel) String() string { return t.spec }

// fakeProxy is an HTTP proxy: it answers CONNECT by opening a tunnel to upstream, and plain
// requests by forwarding them there. Whatever host the client names, the connection lands on
// upstream — which is what makes "https://registry.example.com/v2/" reachable in a test.
type fakeProxy struct {
	// upstream is the host:port every request is really sent to.
	upstream string
	// refuseWith, when non-zero, is the status the proxy answers with instead of connecting:
	// 407 when it wants credentials, 403 when it will not forward to this host, 502 when it
	// cannot reach upstream.
	refuseWith int

	mu       sync.Mutex
	requests []string
}

// serve starts the proxy and returns its address. It is stopped when the test ends.
func (p *fakeProxy) serve(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(p)
	t.Cleanup(server.Close)

	return server.Listener.Addr().String()
}

func (p *fakeProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	p.requests = append(p.requests, r.Method+" "+r.Host)
	p.mu.Unlock()

	if p.refuseWith != 0 {
		// A proxy that will not forward answers the CONNECT itself. Go surfaces a non-2xx
		// answer to CONNECT as a transport error, not as a response, which is exactly the
		// distinction the checks have to cope with.
		w.WriteHeader(p.refuseWith)
		return
	}

	if r.Method == http.MethodConnect {
		p.connect(w)
		return
	}

	p.forward(w, r)
}

func (p *fakeProxy) connect(w http.ResponseWriter) {
	upstream, err := net.Dial("tcp", p.upstream)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	client, _, err := hijacker.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}

	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection established\r\n\r\n"); err != nil {
		return
	}

	splice(client, upstream)
}

// forward relays a plain (non-CONNECT) proxied request, which is what an http:// registry
// produces.
func (p *fakeProxy) forward(w http.ResponseWriter, r *http.Request) {
	outgoing := r.Clone(r.Context())
	outgoing.RequestURI = ""
	outgoing.URL.Scheme = "http"
	outgoing.URL.Host = p.upstream

	resp, err := http.DefaultTransport.RoundTrip(outgoing)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// seen returns the requests the proxy was asked to forward, so a test can tell "the proxy was
// bypassed" from "the proxy answered".
func (p *fakeProxy) seen() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]string(nil), p.requests...)
}

// splice copies between two connections until either side stops, then closes both. Closing both
// is the whole point: a one-way EOF leaves the other copy blocked forever on a peer that has no
// reason to hang up, and the test then hangs in Stop() rather than failing.
func splice(a, b net.Conn) {
	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = a.Close()
			_ = b.Close()
		})
	}

	var wg sync.WaitGroup
	wg.Add(2)
	for _, pair := range [][2]net.Conn{{a, b}, {b, a}} {
		go func(dst, src net.Conn) {
			defer wg.Done()
			defer closeBoth()
			_, _ = io.Copy(dst, src)
		}(pair[0], pair[1])
	}
	wg.Wait()
}

// tunnelPortIsFree reports whether the port the tunnel binds can be bound right now. The port is
// fixed in production, so the fake has to use it too; if something else on the machine holds it,
// skipping beats a failure that says nothing about the code.
func requireTunnelPortFree(t *testing.T) {
	t.Helper()

	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", utils.ProxyTunnelPort))
	if err != nil {
		t.Skipf("port %s is in use on this host, cannot stand up the tunnel", utils.ProxyTunnelPort)
	}
	_ = listener.Close()
}

// isPermanent reports whether the runner has been told not to retry this error.
func isPermanent(err error) bool {
	var permanent *backoff.PermanentError
	return errors.As(err, &permanent)
}
