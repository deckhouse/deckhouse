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

package main

import (
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	apilb "kubernetes-api-proxy/internal/apiserver"
	"kubernetes-api-proxy/internal/app"
	uopts "kubernetes-api-proxy/internal/upstream"
)

// testProxy is a started proxy with a health server. Connections go to an
// echo backend through KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT,
// because the upstream list is empty.
type testProxy struct {
	lb       *apilb.LoadBalancer
	list     *uopts.List
	srv      *http.Server
	health   string
	canceled atomic.Bool
}

func startTestProxy(t *testing.T) *testProxy {
	t.Helper()

	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	go func() {
		for {
			c, err := backend.Accept()
			if err != nil {
				return
			}

			go func() {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}()
		}
	}()

	host, port, _ := net.SplitHostPort(backend.Addr().String())
	t.Setenv("KUBERNETES_SERVICE_HOST", host)
	t.Setenv("KUBERNETES_SERVICE_PORT", port)

	list, err := uopts.NewList(nil, uopts.WithHealthcheckInterval(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	lb, err := apilb.NewLoadBalancer("127.0.0.1", freePort(t), log.NewNop(), apilb.WithMainUpstreamList(list))
	if err != nil {
		t.Fatal(err)
	}

	if err := lb.Start(); err != nil {
		t.Fatal(err)
	}

	healthLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	p := &testProxy{lb: lb, list: list, srv: app.NewHealthServer("", lb), health: healthLn.Addr().String()}
	go func() { _ = p.srv.Serve(healthLn) }()

	t.Cleanup(func() {
		_ = p.srv.Close()
		_ = lb.Shutdown()
	})

	return p
}

// run starts stop in the background. The returned channel closes when stop returns.
func (p *testProxy) run(sigCh <-chan os.Signal, delay time.Duration) <-chan error {
	done := make(chan error, 1)

	go func() {
		done <- stop(sigCh, delay, log.NewNop(), func() { p.canceled.Store(true) }, p.srv, p.lb)
	}()

	return done
}

func freePort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	return ln.Addr().(*net.TCPAddr).Port
}

// echo sends msg on c and reads the same bytes back.
func echo(c net.Conn, msg string) error {
	_ = c.SetDeadline(time.Now().Add(time.Second))

	if _, err := c.Write([]byte(msg)); err != nil {
		return err
	}

	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(c, buf); err != nil {
		return err
	}

	if string(buf) != msg {
		return errors.New("got " + strconv.Quote(string(buf)) + ", want " + strconv.Quote(msg))
	}

	return nil
}

// proxyEcho opens a new connection through the proxy and sends one message.
func (p *testProxy) proxyEcho(msg string) error {
	c, err := net.DialTimeout("tcp", p.lb.Endpoint(), time.Second)
	if err != nil {
		return err
	}
	defer c.Close()

	return echo(c, msg)
}

func (p *testProxy) get(path string) (int, error) {
	// Without keep-alive the client leaves no unused connection that holds
	// the shutdown of the health server.
	client := http.Client{Timeout: time.Second, Transport: &http.Transport{DisableKeepAlives: true}}

	resp, err := client.Get("http://" + p.health + path)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

// After a stop signal the proxy continues to serve for the shutdown delay:
// on node shutdown kubelet stops the proxy and other node-critical pods at the
// same time, and these pods write their last state through the proxy. After
// the delay the proxy closes the listener.
func TestStop_ServesDuringShutdownDelay(t *testing.T) {
	p := startTestProxy(t)

	inFlight, err := net.DialTimeout("tcp", p.lb.Endpoint(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer inFlight.Close()

	if err := echo(inFlight, "before"); err != nil {
		t.Fatalf("connection before the signal: %v", err)
	}

	const delay = time.Second

	sigCh := make(chan os.Signal, 2)
	done := p.run(sigCh, delay)
	start := time.Now()
	sigCh <- syscall.SIGTERM

	time.Sleep(100 * time.Millisecond)
	seq, _ := p.list.Probe()

	if err := p.proxyEcho("during the delay"); err != nil {
		t.Errorf("new connection during the shutdown delay: %v", err)
	}

	if err := echo(inFlight, "in flight"); err != nil {
		t.Errorf("connection opened before the signal, during the shutdown delay: %v", err)
	}

	for _, path := range []string{"/readyz", "/healthz", "/upstreams"} {
		if code, err := p.get(path); err != nil || code != http.StatusOK {
			t.Errorf("GET %s during the shutdown delay = %d, %v; want 200", path, code, err)
		}
	}

	if p.canceled.Load() {
		t.Error("discovery stopped during the shutdown delay")
	}

	time.Sleep(200 * time.Millisecond)

	if next, _ := p.list.Probe(); next <= seq {
		t.Errorf("probe number did not grow during the shutdown delay: %d, then %d", seq, next)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("stop() = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stop() did not return after the shutdown delay")
	}

	if elapsed := time.Since(start); elapsed < delay {
		t.Errorf("stop() returned after %v, before the shutdown delay %v", elapsed, delay)
	}

	if err := p.proxyEcho("after the delay"); err == nil {
		t.Error("new connection after the shutdown delay succeeded, want an error")
	}

	if !p.canceled.Load() {
		t.Error("discovery did not stop after the shutdown delay")
	}
}

// A second stop signal ends the shutdown delay at once.
func TestStop_SecondSignalEndsShutdownDelay(t *testing.T) {
	p := startTestProxy(t)

	sigCh := make(chan os.Signal, 2)
	done := p.run(sigCh, time.Minute)
	sigCh <- syscall.SIGTERM

	time.Sleep(100 * time.Millisecond)

	if err := p.proxyEcho("during the delay"); err != nil {
		t.Errorf("new connection during the shutdown delay: %v", err)
	}

	sigCh <- syscall.SIGINT

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("stop() = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stop() did not return after the second signal")
	}

	if err := p.proxyEcho("after the second signal"); err == nil {
		t.Error("new connection after the second signal succeeded, want an error")
	}
}

// With a zero delay the proxy closes the listener at once, as without the delay.
func TestStop_ZeroDelayStopsAtOnce(t *testing.T) {
	p := startTestProxy(t)

	sigCh := make(chan os.Signal, 2)
	done := p.run(sigCh, 0)
	sigCh <- syscall.SIGTERM

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("stop() = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stop() with a zero delay did not return at once")
	}

	if err := p.proxyEcho("after the signal"); err == nil {
		t.Error("new connection after the signal succeeded, want an error")
	}

	if !p.canceled.Load() {
		t.Error("discovery did not stop")
	}
}

// A connection to the health server without a request does not hold the
// shutdown: the proxy must exit soon after the shutdown delay.
func TestStop_UnusedHealthConnectionDoesNotHoldShutdown(t *testing.T) {
	p := startTestProxy(t)

	c, err := net.DialTimeout("tcp", p.health, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	sigCh := make(chan os.Signal, 2)
	done := p.run(sigCh, 0)
	start := time.Now()
	sigCh <- syscall.SIGTERM

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("stop() = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("stop() did not return")
	}

	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Errorf("stop() returned after %v, want at most 1.5s", elapsed)
	}
}
