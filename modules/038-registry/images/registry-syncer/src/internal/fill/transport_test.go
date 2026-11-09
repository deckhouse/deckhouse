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

package fill

import (
	"bytes"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// silencingProxy forwards TCP between a client and a server until it is silenced. Then it forwards
// nothing more in either direction and keeps both connections open: what a peer whose machine lost
// power looks like from the client — no data, no reset, no error.
type silencingProxy struct {
	address string
	silent  atomic.Bool
	stop    chan struct{}

	mu    sync.Mutex
	conns []net.Conn
}

func startSilencingProxy(t *testing.T, target string) *silencingProxy {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	p := &silencingProxy{address: listener.Addr().String(), stop: make(chan struct{})}
	t.Cleanup(func() {
		close(p.stop)
		_ = listener.Close()
		p.mu.Lock()
		defer p.mu.Unlock()
		for _, conn := range p.conns {
			_ = conn.Close()
		}
	})

	go func() {
		for {
			in, err := listener.Accept()
			if err != nil {
				return
			}
			out, err := net.Dial("tcp", target)
			if err != nil {
				_ = in.Close()
				return
			}
			p.mu.Lock()
			p.conns = append(p.conns, in, out)
			p.mu.Unlock()
			go p.pump(out, in)
			go p.pump(in, out)
		}
	}()
	return p
}

// pump copies from one side to the other until the proxy is silenced, and then holds what it read.
func (p *silencingProxy) pump(to, from net.Conn) {
	buffer := make([]byte, 32<<10)
	for {
		n, err := from.Read(buffer)
		if err != nil {
			return
		}
		if p.silent.Load() {
			<-p.stop
			return
		}
		if _, err := to.Write(buffer[:n]); err != nil {
			return
		}
	}
}

// startTLSPeer runs handler over HTTPS with HTTP/2, as a registry serves its neighbours, and returns its
// authority.
func startTLSPeer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, string) {
	t.Helper()

	server := httptest.NewUnstartedServer(handler)
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)

	authority := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	return server, string(authority)
}

// TestAPeerThatStopsAnsweringIsDropped is the follower of a leader whose machine lost power, mid-blob.
// Nothing comes back — no reset, no unreachable — and on a DVP stand the connection hung for sixteen
// minutes, until the kernel gave up retransmitting, while the replica that had taken the lease filled
// nothing. A ping nobody answers ends it in seconds.
func TestAPeerThatStopsAnsweringIsDropped(t *testing.T) {
	first := bytes.Repeat([]byte("a"), 1<<10)
	released := make(chan struct{})
	server, authority := startTLSPeer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(first)
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-released:
		}
	})
	// Before the proxy's cleanup, which runs first and lets go of the server's side.
	t.Cleanup(func() { close(released) })
	proxy := startSilencingProxy(t, server.Listener.Addr().String())

	transport, err := newTransport(authority, 100*time.Millisecond, 100*time.Millisecond)
	require.NoError(t, err)
	client := &http.Client{Transport: transport}

	response, err := client.Get("https://" + proxy.address + "/v2/system/deckhouse/blobs/sha256:0")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, 2, response.ProtoMajor, "the copy between replicas is TLS, so HTTP/2")

	got := make([]byte, len(first))
	_, err = io.ReadFull(response.Body, got)
	require.NoError(t, err)

	proxy.silent.Store(true)
	failed := make(chan error, 1)
	go func() {
		_, err := io.ReadAll(response.Body)
		failed <- err
	}()
	select {
	case err := <-failed:
		assert.Error(t, err, "the rest of the body never came, and the read has to say so")
	case <-time.After(2 * time.Second):
		t.Fatal("a peer that stopped answering is still waited for")
	}
}

// TestASilentButLiveServerIsKept: the bound is on the peer, not on the request. A registry that takes
// its time — a mirror fetching what it does not hold yet, a disk under load — still answers pings, and
// its blob arrives whole however long it takes.
func TestASilentButLiveServerIsKept(t *testing.T) {
	half := bytes.Repeat([]byte("b"), 1<<10)
	server, authority := startTLSPeer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(half)
		w.(http.Flusher).Flush()
		time.Sleep(time.Second)
		_, _ = w.Write(half)
	})
	proxy := startSilencingProxy(t, server.Listener.Addr().String())

	transport, err := newTransport(authority, 100*time.Millisecond, 100*time.Millisecond)
	require.NoError(t, err)
	client := &http.Client{Transport: transport}

	response, err := client.Get("https://" + proxy.address + "/v2/system/deckhouse/blobs/sha256:0")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, 2, response.ProtoMajor)

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err, "ten silences of the ping interval, each answered")
	assert.Equal(t, append(append([]byte(nil), half...), half...), body)
}

// TestTheTransportBoundsASilentPeer: every registry the syncer talks to — the upstream, the leader, its
// own — goes through Transport, so the bounds are there for each of them, and no bound on a whole
// request is: a blob takes as long as the link needs.
func TestTheTransportBoundsASilentPeer(t *testing.T) {
	transport, err := Transport("")
	require.NoError(t, err)

	require.NotNil(t, transport.HTTP2)
	assert.Equal(t, peerSilentAfter, transport.HTTP2.SendPingTimeout)
	assert.Equal(t, peerPingTimeout, transport.HTTP2.PingTimeout)
	assert.Equal(t, responseHeaderTimeout, transport.ResponseHeaderTimeout)
	assert.True(t, transport.ForceAttemptHTTP2, "the ping is HTTP/2's, and a custom authority must not lose it")
}
