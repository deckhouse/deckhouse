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
	"bytes"
	"errors"
	"io"
	"net"
	"slices"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"kubernetes-api-proxy/internal/firstbyte"
	"kubernetes-api-proxy/internal/upstream"
)

const (
	testTimeout = 2 * time.Second
	floor       = 30 * time.Millisecond
	patience    = testTimeout
)

type fakePicker struct {
	mu        sync.Mutex
	upstreams []*upstream.Upstream
	downs     []string
	silents   []string
	pickNots  []string
}

func newFakePicker(addrs ...string) *fakePicker {
	p := &fakePicker{}
	for _, addr := range addrs {
		p.upstreams = append(p.upstreams, upstream.NewUpstream(addr, nil))
	}

	return p
}

func (p *fakePicker) Pick() (*upstream.Upstream, error) {
	if len(p.upstreams) == 0 {
		return nil, upstream.ErrNoUpstreams
	}

	return p.upstreams[0], nil
}

func (p *fakePicker) PickNot(exclude *upstream.Upstream) (*upstream.Upstream, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.pickNots = append(p.pickNots, exclude.Address())

	for _, u := range p.upstreams {
		if u.Address() != exclude.Address() {
			return u, nil
		}
	}

	return nil, upstream.ErrNoUpstreams
}

func (p *fakePicker) Down(u *upstream.Upstream) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.downs = append(p.downs, u.Address())
}

func (p *fakePicker) Silent(u *upstream.Upstream) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.silents = append(p.silents, u.Address())
}

func (p *fakePicker) calls() (downs, silents, pickNots []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return slices.Clone(p.downs), slices.Clone(p.silents), slices.Clone(p.pickNots)
}

func assertCalls(t *testing.T, p *fakePicker, downs, silents, pickNots []string) {
	t.Helper()

	gotDowns, gotSilents, gotPickNots := p.calls()
	if !slices.Equal(gotDowns, downs) {
		t.Errorf("Down calls = %v, want %v", gotDowns, downs)
	}

	if !slices.Equal(gotSilents, silents) {
		t.Errorf("Silent calls = %v, want %v", gotSilents, silents)
	}

	if !slices.Equal(gotPickNots, pickNots) {
		t.Errorf("PickNot calls = %v, want %v", gotPickNots, pickNots)
	}
}

func waitForSilents(t *testing.T, p *fakePicker, n int) {
	t.Helper()

	deadline := time.Now().Add(testTimeout)

	for {
		if _, silents, _ := p.calls(); len(silents) >= n {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("Silent was not called %d times in time", n)
		}

		time.Sleep(5 * time.Millisecond)
	}
}

func echo(c net.Conn) {
	_, _ = io.Copy(c, c)
}

func hang(c net.Conn) {
	_, _ = io.Copy(io.Discard, c)
}

func answerOn(release <-chan struct{}, msg string) func(net.Conn) {
	return func(c net.Conn) {
		<-release

		if _, err := c.Write([]byte(msg)); err != nil {
			return
		}

		echo(c)
	}
}

func delayed(d time.Duration, then func(net.Conn)) func(net.Conn) {
	return func(c net.Conn) {
		time.Sleep(d)
		then(c)
	}
}

func drain(n int, then func(net.Conn)) func(net.Conn) {
	return func(c net.Conn) {
		if _, err := io.ReadFull(c, make([]byte, n)); err != nil {
			return
		}

		then(c)
	}
}

func releaser(t *testing.T) (<-chan struct{}, func()) {
	ch := make(chan struct{})
	release := sync.OnceFunc(func() { close(ch) })

	t.Cleanup(release)

	return ch, release
}

func listen(t *testing.T, serve func(net.Conn)) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		conns []net.Conn
	)

	track := func(c net.Conn) {
		mu.Lock()
		defer mu.Unlock()

		conns = append(conns, c)
	}

	wg.Go(func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}

			track(c)

			wg.Go(func() {
				defer c.Close()

				serve(c)
			})
		}
	})

	t.Cleanup(func() {
		_ = ln.Close()

		mu.Lock()
		for _, c := range conns {
			_ = c.Close()
		}
		mu.Unlock()

		done := make(chan struct{})

		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(testTimeout):
			if !t.Failed() {
				t.Error("serve goroutines did not exit after the listener closed")
			}
		}
	})

	return ln.Addr().String()
}

func refusedAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	addr := ln.Addr().String()
	_ = ln.Close()

	return addr
}

func testTarget(list upstreamPicker, fallback *upstream.FallbackList, firstByte time.Duration) *lbTarget {
	target := &lbTarget{
		list:         list,
		fallbackList: fallback,
		logger:       log.NewNop(),
		dialTimeout:  200 * time.Millisecond,
	}

	if firstByte > 0 {
		target.firstByte = firstbyte.New(firstByte)
	}

	return target
}

func connect(t *testing.T, target *lbTarget) net.Conn {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	defer ln.Close()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	server, err := ln.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		target.HandleConn(server)
	}()

	t.Cleanup(func() {
		_ = client.Close()

		select {
		case <-done:
		case <-time.After(testTimeout):
			if !t.Failed() {
				t.Error("HandleConn did not return after the client closed")
			}
		}
	})

	_ = client.SetDeadline(time.Now().Add(testTimeout))

	return client
}

func write(t *testing.T, c net.Conn, data string) {
	t.Helper()

	if _, err := io.WriteString(c, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readN(t *testing.T, c net.Conn, n int) string {
	t.Helper()

	buf := make([]byte, n)
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatalf("read: %v", err)
	}

	return string(buf)
}

func TestHandleConn_HappyPath(t *testing.T) {
	const budget = 200 * time.Millisecond

	addr := listen(t, delayed(budget*3/5, echo))
	picker := newFakePicker(addr)
	target := testTarget(picker, nil, budget)
	client := connect(t, target)

	write(t, client, "ping")

	if got := readN(t, client, 4); got != "ping" {
		t.Errorf("read %q, want %q", got, "ping")
	}

	assertCalls(t, picker, nil, nil, nil)

	if got := target.firstByte.Wait(addr); got <= budget {
		t.Errorf("Wait(%s) = %s after a %s answer, want above the floor %s: latency was not observed", addr, got, budget*3/5, budget)
	}
}

func TestHandleConn_HungUpstreamRetriesWithReplay(t *testing.T) {
	closed := make(chan struct{})
	hung := listen(t, func(c net.Conn) {
		hang(c)
		close(closed)
	})
	alive := listen(t, echo)
	picker := newFakePicker(hung, alive)
	client := connect(t, testTarget(picker, nil, floor))

	write(t, client, "ping")

	if got := readN(t, client, 4); got != "ping" {
		t.Errorf("read %q, want %q", got, "ping")
	}

	assertCalls(t, picker, nil, []string{hung}, []string{hung})

	select {
	case <-closed:
	case <-time.After(testTimeout):
		t.Error("the superseded upstream connection was not closed")
	}
}

func TestHandleConn_DialRefusedRetries(t *testing.T) {
	refused := refusedAddr(t)
	alive := listen(t, echo)
	picker := newFakePicker(refused, alive)
	client := connect(t, testTarget(picker, nil, floor))

	write(t, client, "ping")

	if got := readN(t, client, 4); got != "ping" {
		t.Errorf("read %q, want %q", got, "ping")
	}

	assertCalls(t, picker, []string{refused}, nil, []string{refused})
}

func TestHandleConn_BothHungStaysOnSecond(t *testing.T) {
	release, releaseNow := releaser(t)
	hung := listen(t, hang)
	late := listen(t, answerOn(release, "late"))
	picker := newFakePicker(hung, late)
	target := testTarget(picker, nil, floor)
	client := connect(t, target)

	write(t, client, "ping")
	waitForSilents(t, picker, 2)
	assertCalls(t, picker, nil, []string{hung, late}, []string{hung})

	for _, addr := range []string{hung, late} {
		if got := target.firstByte.Wait(addr); got != floor*3/2 {
			t.Errorf("Wait(%s) = %s after one silence, want %s", addr, got, floor*3/2)
		}
	}

	releaseNow()

	if got := readN(t, client, 8); got != "lateping" {
		t.Errorf("read %q, want %q", got, "lateping")
	}
}

func TestHandleConn_SingleUpstreamKeepsWaiting(t *testing.T) {
	release, releaseNow := releaser(t)
	late := listen(t, answerOn(release, "late"))
	picker := newFakePicker(late)
	client := connect(t, testTarget(picker, nil, floor))

	write(t, client, "ping")
	waitForSilents(t, picker, 1)
	releaseNow()

	if got := readN(t, client, 8); got != "lateping" {
		t.Errorf("read %q, want %q", got, "lateping")
	}

	assertCalls(t, picker, nil, []string{late}, []string{late})
}

func TestHandleConn_RecordingCapDisablesRetry(t *testing.T) {
	release, releaseNow := releaser(t)
	payload := bytes.Repeat([]byte("x"), recordCap+1)
	late := listen(t, drain(len(payload), answerOn(release, "late")))
	alive := listen(t, echo)
	picker := newFakePicker(late, alive)
	client := connect(t, testTarget(picker, nil, floor))

	write(t, client, string(payload))
	waitForSilents(t, picker, 1)
	assertCalls(t, picker, nil, []string{late}, nil)
	releaseNow()

	if got := readN(t, client, 4); got != "late" {
		t.Errorf("read %q, want %q", got, "late")
	}
}

func TestHandleConn_AlternativeRefusedStaysOnFirst(t *testing.T) {
	release, releaseNow := releaser(t)
	late := listen(t, answerOn(release, "late"))
	refused := refusedAddr(t)
	picker := newFakePicker(late, refused)
	client := connect(t, testTarget(picker, nil, floor))

	write(t, client, "ping")
	waitForSilents(t, picker, 1)
	time.Sleep(3 * floor)
	assertCalls(t, picker, []string{refused}, []string{late}, []string{late})
	releaseNow()

	if got := readN(t, client, 8); got != "lateping" {
		t.Errorf("read %q, want %q", got, "lateping")
	}
}

func TestHandleConn_DialRetryThenSilenceSwitchesOnce(t *testing.T) {
	release, releaseNow := releaser(t)
	refused := refusedAddr(t)
	late := listen(t, answerOn(release, "late"))
	picker := newFakePicker(refused, late)
	client := connect(t, testTarget(picker, nil, floor))

	write(t, client, "ping")
	waitForSilents(t, picker, 1)
	time.Sleep(3 * floor)
	assertCalls(t, picker, []string{refused}, []string{late}, []string{refused})
	releaseNow()

	if got := readN(t, client, 8); got != "lateping" {
		t.Errorf("read %q, want %q", got, "lateping")
	}
}

func TestHandleConn_ClampedSecondWaitIsNotSilence(t *testing.T) {
	const budget = 100 * time.Millisecond

	release1, releaseFirst := releaser(t)
	release2, releaseNow := releaser(t)
	first := listen(t, answerOn(release1, "first"))
	second := listen(t, answerOn(release2, "second"))
	picker := newFakePicker(first, second)
	target := testTarget(picker, nil, budget)
	target.firstByte.Observe(first, 2*budget)
	target.firstByte.Observe(second, 2*budget)
	client := connect(t, target)

	write(t, client, "ping")
	waitForSilents(t, picker, 1)
	time.Sleep(4 * budget)
	assertCalls(t, picker, nil, []string{first}, []string{first})
	releaseNow()

	if got := readN(t, client, 10); got != "secondping" {
		t.Errorf("read %q, want %q", got, "secondping")
	}

	releaseFirst()
}

func TestHandleConn_IdleClientIsNotSilence(t *testing.T) {
	alive := listen(t, echo)
	other := listen(t, echo)
	picker := newFakePicker(alive, other)
	client := connect(t, testTarget(picker, nil, floor))

	time.Sleep(4 * floor)
	assertCalls(t, picker, nil, nil, nil)

	write(t, client, "ping")

	if got := readN(t, client, 4); got != "ping" {
		t.Errorf("read %q, want %q", got, "ping")
	}

	assertCalls(t, picker, nil, nil, nil)
}

func TestHandleConn_HalfCloseBeforeSwitchIsReplayed(t *testing.T) {
	stuck := make(chan struct{})
	hung := listen(t, func(c net.Conn) {
		hang(c)
		<-stuck
	})

	t.Cleanup(func() { close(stuck) })

	alive := listen(t, func(c net.Conn) {
		data, _ := io.ReadAll(c)
		_, _ = c.Write(data)
	})
	picker := newFakePicker(hung, alive)
	client := connect(t, testTarget(picker, nil, floor))

	write(t, client, "ping")

	if err := client.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatalf("CloseWrite: %v", err)
	}

	if got := readN(t, client, 4); got != "ping" {
		t.Errorf("read %q, want %q", got, "ping")
	}

	assertCalls(t, picker, nil, []string{hung}, []string{hung})
}

func TestHandleConn_ClientResetDuringWaitIsNotADown(t *testing.T) {
	stuck := make(chan struct{})
	hung := listen(t, func(c net.Conn) {
		hang(c)
		<-stuck
	})

	t.Cleanup(func() { close(stuck) })

	alive := listen(t, echo)
	picker := newFakePicker(hung, alive)
	client := connect(t, testTarget(picker, nil, floor))

	write(t, client, "ping")

	if err := client.(*net.TCPConn).SetLinger(0); err != nil {
		t.Fatalf("SetLinger: %v", err)
	}

	_ = client.Close()

	time.Sleep(3 * floor)
	assertCalls(t, picker, nil, nil, nil)
}

func TestHandleConn_FirstByteDisabled(t *testing.T) {
	t.Run("dial retry still works", func(t *testing.T) {
		refused := refusedAddr(t)
		alive := listen(t, echo)
		picker := newFakePicker(refused, alive)
		client := connect(t, testTarget(picker, nil, 0))

		write(t, client, "ping")

		if got := readN(t, client, 4); got != "ping" {
			t.Errorf("read %q, want %q", got, "ping")
		}

		assertCalls(t, picker, []string{refused}, nil, []string{refused})
	})

	t.Run("silence is not retried", func(t *testing.T) {
		release, releaseNow := releaser(t)
		late := listen(t, answerOn(release, "late"))
		alive := listen(t, echo)
		picker := newFakePicker(late, alive)
		client := connect(t, testTarget(picker, nil, 0))

		write(t, client, "ping")
		time.Sleep(3 * floor)
		releaseNow()

		if got := readN(t, client, 8); got != "lateping" {
			t.Errorf("read %q, want %q", got, "lateping")
		}

		assertCalls(t, picker, nil, nil, nil)
	})
}

func TestHandleConn_FallbackPathIsPlain(t *testing.T) {
	release, releaseNow := releaser(t)
	late := listen(t, answerOn(release, "late"))

	fallback, err := upstream.NewFallbackList(upstream.WithUpstreamsFromArgs([]string{late}))
	if err != nil {
		t.Fatalf("NewFallbackList: %v", err)
	}

	picker := newFakePicker()
	target := testTarget(picker, fallback, floor)
	client := connect(t, target)

	write(t, client, "ping")
	time.Sleep(3 * floor)
	releaseNow()

	if got := readN(t, client, 8); got != "lateping" {
		t.Errorf("read %q, want %q", got, "lateping")
	}

	assertCalls(t, picker, nil, nil, nil)

	if got := target.firstByte.Wait(late); got != floor {
		t.Errorf("Wait(%s) = %s, want the untouched floor %s: the fallback path fed the budget", late, got, floor)
	}
}

func TestHandleConn_MainListBeatsFallback(t *testing.T) {
	released := make(chan struct{})
	close(released)

	main := listen(t, echo)
	late := listen(t, answerOn(released, "late"))

	fallback, err := upstream.NewFallbackList(upstream.WithUpstreamsFromArgs([]string{late}))
	if err != nil {
		t.Fatalf("NewFallbackList: %v", err)
	}

	picker := newFakePicker(main)
	client := connect(t, testTarget(picker, fallback, patience))

	write(t, client, "ping")

	if got := readN(t, client, 4); got != "ping" {
		t.Errorf("read %q, want %q (fallback answered instead of the main list)", got, "ping")
	}

	assertCalls(t, picker, nil, nil, nil)
}

func TestHandleConn_UpstreamClosesBeforeFirstByteIsNotRetried(t *testing.T) {
	closer := listen(t, func(net.Conn) {})
	alive := listen(t, echo)
	picker := newFakePicker(closer, alive)
	client := connect(t, testTarget(picker, nil, floor))

	write(t, client, "ping")

	if _, err := client.Read(make([]byte, 1)); err == nil {
		t.Error("read succeeded, want the connection closed")
	}

	assertCalls(t, picker, nil, nil, nil)
}

func TestHandleConn_NoUpstreamsClosesClient(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	client := connect(t, testTarget(newFakePicker(), nil, floor))

	if _, err := client.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Errorf("read error = %v, want %v", err, io.EOF)
	}
}

type kernelTimeoutConn struct {
	net.Conn
	wrote chan struct{}
	once  sync.Once
}

func (c *kernelTimeoutConn) Write(p []byte) (int, error) {
	c.once.Do(func() { close(c.wrote) })

	return len(p), nil
}

func (c *kernelTimeoutConn) Read([]byte) (int, error) {
	<-c.wrote

	return 0, &net.OpError{Op: "read", Net: "tcp", Err: syscall.ETIMEDOUT}
}

func (*kernelTimeoutConn) SetReadDeadline(time.Time) error { return nil }

func (*kernelTimeoutConn) Close() error { return nil }

func pair(t *testing.T) (client, server net.Conn) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	defer ln.Close()

	client, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	server, err = ln.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}

	_ = client.SetDeadline(time.Now().Add(testTimeout))

	return client, server
}

func TestProxy_KernelTimeoutBeforeFirstByteIsSilence(t *testing.T) {
	alive := listen(t, echo)
	picker := newFakePicker("fake", alive)
	target := testTarget(picker, nil, floor)
	client, server := pair(t)
	stalled := &kernelTimeoutConn{wrote: make(chan struct{})}
	done := make(chan struct{})

	go func() {
		defer close(done)

		target.proxy(server, stalled, picker.upstreams[0], false)
	}()

	t.Cleanup(func() {
		_ = client.Close()

		select {
		case <-done:
		case <-time.After(testTimeout):
			t.Error("proxy did not return after the client closed")
		}
	})

	write(t, client, "ping")

	if got := readN(t, client, 4); got != "ping" {
		t.Errorf("read %q, want %q", got, "ping")
	}

	assertCalls(t, picker, nil, []string{"fake"}, []string{"fake"})
}

type clientGoneConn struct {
	net.Conn
	closed chan struct{}
	once   sync.Once
}

func (*clientGoneConn) Write([]byte) (int, error) {
	return 0, &net.OpError{Op: "write", Net: "tcp", Err: syscall.ETIMEDOUT}
}

func (c *clientGoneConn) Read([]byte) (int, error) {
	<-c.closed

	return 0, net.ErrClosed
}

func (c *clientGoneConn) Close() error {
	c.once.Do(func() { close(c.closed) })

	return nil
}

func (*clientGoneConn) SetReadDeadline(time.Time) error { return nil }

type deadDst struct {
	net.Conn
}

func (deadDst) Read([]byte) (int, error) {
	return 0, &net.OpError{Op: "read", Net: "tcp", Err: syscall.ETIMEDOUT}
}

func (deadDst) Write(p []byte) (int, error) { return len(p), nil }

func (deadDst) SetReadDeadline(time.Time) error { return nil }

func (deadDst) Close() error { return nil }

func TestAwaitFirstByte_ClientWriteTimeoutIsNotSilence(t *testing.T) {
	a := listen(t, echo)
	b := listen(t, echo)
	picker := newFakePicker(a, b)
	target := testTarget(picker, nil, floor)

	dst, err := net.Dial("tcp", a)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	t.Cleanup(func() { _ = dst.Close() })

	r := newRelay(dst, true)
	if _, err := r.Write([]byte("ping")); err != nil {
		t.Fatalf("relay write: %v", err)
	}

	src := &clientGoneConn{closed: make(chan struct{})}
	if _, err := target.awaitFirstByte(src, r, picker.upstreams[0], false); err == nil {
		t.Error("awaitFirstByte returned nil after the client write failed, want the write error")
	}

	assertCalls(t, picker, nil, nil, nil)
}

func TestProxy_KernelTimeoutBeforeClientByteSwitchesAndWaits(t *testing.T) {
	alive := listen(t, echo)
	picker := newFakePicker("dead", alive)
	target := testTarget(picker, nil, floor)
	client, server := pair(t)
	done := make(chan struct{})

	go func() {
		defer close(done)

		target.proxy(server, deadDst{}, picker.upstreams[0], false)
	}()

	t.Cleanup(func() {
		_ = client.Close()

		select {
		case <-done:
		case <-time.After(testTimeout):
			t.Error("proxy did not return after the client closed")
		}
	})

	time.Sleep(2 * floor)
	write(t, client, "ping")

	if got := readN(t, client, 4); got != "ping" {
		t.Errorf("read %q, want %q", got, "ping")
	}

	assertCalls(t, picker, nil, []string{"dead"}, []string{"dead"})
}
