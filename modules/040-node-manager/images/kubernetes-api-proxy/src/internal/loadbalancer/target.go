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

package loadbalancer

import (
	"cmp"
	"errors"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/siderolabs/tcpproxy"

	"github.com/deckhouse/deckhouse/pkg/log"

	"kubernetes-api-proxy/internal/firstbyte"
	"kubernetes-api-proxy/internal/upstream"
)

const (
	defaultDialTimeout     = 10 * time.Second
	defaultKeepAlivePeriod = time.Minute
	firstByteBufSize       = 32 << 10
)

var (
	errChainSpent = errors.New("first-byte chain budget spent")
	errSilence    = errors.New("upstream sent nothing within the first-byte budget")
)

type upstreamPicker interface {
	Pick() (*upstream.Upstream, error)
	PickNot(exclude *upstream.Upstream) (*upstream.Upstream, error)
	Down(*upstream.Upstream)
	Silent(*upstream.Upstream)
}

// lbTarget implements tcpproxy.Target and proxies connections to a picked
// upstream chosen from the upstream.List based on current health and tiering.
type lbTarget struct {
	list         upstreamPicker
	fallbackList *upstream.FallbackList
	firstByte    *firstbyte.Budget

	logger          *log.Logger
	route           string
	dialTimeout     time.Duration
	keepAlivePeriod time.Duration
	tcpUserTimeout  time.Duration

	conns liveConns
}

// HandleConn picks an upstream and proxies the connection to it. A main-list
// upstream that refuses the dial, or accepts it and sends nothing within the
// first-byte budget after the client's first byte, is marked down and the
// connection is retried once on another upstream.
func (t *lbTarget) HandleConn(src net.Conn) {
	picked, addr := t.pick()
	if addr == "" {
		_ = src.Close()

		t.logger.Error("no upstreams available to handle connection")

		return
	}

	dst, backend, err := t.connect(picked, addr)
	if err != nil {
		_ = src.Close()

		return
	}

	t.logger.Debug(
		"proxying connection",
		slog.String("route", t.route),
		slog.String("upstream", dst.RemoteAddr().String()),
		slog.String("client", src.RemoteAddr().String()),
	)

	t.tune(src)
	t.proxy(src, dst, backend, backend != picked)
}

func (t *lbTarget) pick() (*upstream.Upstream, string) {
	if t.list != nil {
		if backend, err := t.list.Pick(); err == nil {
			return backend, backend.Address()
		}
	}

	if t.fallbackList != nil {
		if backend, err := t.fallbackList.Pick(); err == nil {
			return nil, backend.Address()
		}
	}

	return nil, t.envUpstream()
}

func (t *lbTarget) envUpstream() string {
	t.logger.Warn("failed to pick upstream from fallback list, falling to KUBERNETES_SERVICE_HOST:KUBERNETES_SERVICE_PORT")

	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return ""
	}

	return net.JoinHostPort(host, port)
}

func (t *lbTarget) connect(backend *upstream.Upstream, addr string) (net.Conn, *upstream.Upstream, error) {
	dst, err := t.dial(addr, t.dialLimit())
	if err == nil {
		return dst, backend, nil
	}

	t.logger.Warn("failed to dial upstream", slog.String("upstream", addr), slog.String("error", err.Error()))

	if backend == nil {
		return nil, nil, err
	}

	t.list.Down(backend)

	next, err := t.alternative(backend)
	if err != nil {
		return nil, nil, err
	}

	dst, err = t.dialNext(next, t.dialLimit())
	if err != nil {
		return nil, nil, err
	}

	return dst, next, nil
}

func (t *lbTarget) alternative(backend *upstream.Upstream) (*upstream.Upstream, error) {
	next, err := t.list.PickNot(backend)
	if err != nil {
		t.logger.Warn("no alternative upstream", slog.String("upstream", backend.Address()), slog.String("error", err.Error()))

		return nil, err
	}

	return next, nil
}

func (t *lbTarget) dialNext(next *upstream.Upstream, limit time.Duration) (net.Conn, error) {
	dst, err := t.dial(next.Address(), limit)
	if err != nil {
		t.list.Down(next)
		t.logger.Warn("failed to dial alternative upstream", slog.String("upstream", next.Address()), slog.String("error", err.Error()))

		return nil, err
	}

	return dst, nil
}

func (t *lbTarget) dial(addr string, limit time.Duration) (net.Conn, error) {
	dst, err := net.DialTimeout("tcp", addr, limit)
	if err != nil {
		return nil, err
	}

	t.tune(dst)

	return dst, nil
}

func (t *lbTarget) dialLimit() time.Duration {
	return cmp.Or(t.dialTimeout, defaultDialTimeout)
}

func (t *lbTarget) tune(c net.Conn) {
	tcp, ok := tcpproxy.UnderlyingConn(c).(*net.TCPConn)
	if !ok {
		return
	}

	_ = tcp.SetKeepAlive(true)
	_ = tcp.SetKeepAlivePeriod(cmp.Or(t.keepAlivePeriod, defaultKeepAlivePeriod))

	if t.tcpUserTimeout > 0 {
		_ = tcpproxy.SetTCPUserTimeout(tcp, t.tcpUserTimeout)
	}
}

func (t *lbTarget) proxy(src, dst net.Conn, backend *upstream.Upstream, retried bool) {
	firstByte := backend != nil && t.firstByte != nil
	r := newRelay(dst, firstByte)
	errc := make(chan error, 2)

	go copyToUpstream(errc, r, src)

	final := backend

	if firstByte {
		var err error

		final, err = t.awaitFirstByte(src, r, backend, retried)
		r.settle()

		if err != nil {
			closeBoth(src, r)
			<-errc

			return
		}
	}

	if final != nil {
		defer t.conns.add(final.Address(), src, r)()
	}

	go copyToClient(errc, src, r.conn())

	waitBoth(errc, src, r)
}

func (t *lbTarget) closeConnsTo(u *upstream.Upstream) {
	if n := t.conns.closeAll(u.Address()); n > 0 {
		t.logger.Warn("closed client connections to a stalled upstream", slog.String("upstream", u.Address()), slog.Int("connections", n))
	}
}

func (t *lbTarget) awaitFirstByte(src net.Conn, r *relay, backend *upstream.Upstream, retried bool) (*upstream.Upstream, error) {
	r.arm(t.firstByte.Wait(backend.Address()))

	first, err := t.readFirstByte(src, r, backend)
	if !errors.Is(err, errSilence) {
		return backend, err
	}

	t.expire(backend, first)

	if retried {
		return backend, nil
	}

	return t.retryFirstByte(src, r, backend, first)
}

func (t *lbTarget) retryFirstByte(src net.Conn, r *relay, backend *upstream.Upstream, first hop) (*upstream.Upstream, error) {
	started := first.armedAt
	if started.IsZero() {
		started = time.Now()
	}

	next, full := t.switchUpstream(r, backend, started)
	if next == nil {
		return backend, nil
	}

	second, err := t.readFirstByte(src, r, next)
	if !errors.Is(err, errSilence) {
		return next, err
	}

	if full {
		t.expire(next, second)
	} else {
		t.logger.Warn("first-byte chain budget ran out on the second upstream", slog.String("upstream", next.Address()), slog.Duration("waited", second.wait))
	}

	return next, nil
}

func (t *lbTarget) readFirstByte(src net.Conn, r *relay, backend *upstream.Upstream) (hop, error) {
	buf := make([]byte, firstByteBufSize)

	n, err := r.conn().Read(buf)

	h := r.hop()

	switch {
	case n > 0:
		if !h.armedAt.IsZero() {
			t.firstByte.Observe(backend.Address(), time.Since(h.armedAt))
		}

		_, err = src.Write(buf[:n])

		return h, err
	case isTimeout(err):
		return h, errSilence
	default:
		return h, err
	}
}

func isTimeout(err error) bool {
	var ne net.Error

	return errors.As(err, &ne) && ne.Timeout()
}

func (t *lbTarget) expire(backend *upstream.Upstream, h hop) {
	addr := backend.Address()

	t.logger.Warn("upstream sent nothing within the first-byte budget", slog.String("upstream", addr), slog.Duration("waited", h.wait))
	t.firstByte.Timeout(addr)
	t.list.Silent(backend)
}

func (t *lbTarget) switchUpstream(r *relay, backend *upstream.Upstream, started time.Time) (*upstream.Upstream, bool) {
	if !r.canRetry() {
		return nil, false
	}

	deadline := started.Add(t.firstByte.Chain())

	next, dst, err := t.dialWithin(backend, deadline)
	if err != nil {
		return nil, false
	}

	full := t.firstByte.Wait(next.Address())

	wait := min(full, time.Until(deadline))
	if wait <= 0 {
		_ = dst.Close()

		return nil, false
	}

	old, err := r.switchTo(dst, wait)
	if err != nil {
		_ = dst.Close()

		return nil, false
	}

	_ = old.Close()

	t.logger.Info("switched to another upstream", slog.String("upstream", backend.Address()), slog.String("next", next.Address()), slog.Duration("wait", wait))

	return next, wait == full
}

func (t *lbTarget) dialWithin(backend *upstream.Upstream, deadline time.Time) (*upstream.Upstream, net.Conn, error) {
	limit := time.Until(deadline)
	if limit <= 0 {
		return nil, nil, errChainSpent
	}

	next, err := t.alternative(backend)
	if err != nil {
		return nil, nil, err
	}

	dst, err := t.dialNext(next, min(t.dialLimit(), limit))
	if err != nil {
		return nil, nil, err
	}

	return next, dst, nil
}
