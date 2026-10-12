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
	"io"
	"net"
	"testing"
	"time"
)

func (c *liveConns) count(addr string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.by[addr])
}

func (c *liveConns) empty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.by) == 0
}

func waitConns(t *testing.T, target *lbTarget, addr string, n int) {
	t.Helper()

	deadline := time.Now().Add(testTimeout)

	for target.conns.count(addr) != n {
		if time.Now().After(deadline) {
			t.Fatalf("connections registered to %s = %d, want %d", addr, target.conns.count(addr), n)
		}

		time.Sleep(time.Millisecond)
	}
}

func answerOnceThenHang(closed chan<- struct{}) func(net.Conn) {
	return func(c net.Conn) {
		if _, err := io.ReadFull(c, make([]byte, 4)); err != nil {
			return
		}

		if _, err := c.Write([]byte("pong")); err != nil {
			return
		}

		hang(c)
		close(closed)
	}
}

func roundTrip(t *testing.T, c net.Conn, send, want string) {
	t.Helper()

	write(t, c, send)

	if got := readN(t, c, len(want)); got != want {
		t.Fatalf("read %q, want %q", got, want)
	}
}

func TestCloseConnsTo_ClosesSettledConn(t *testing.T) {
	closed := make(chan struct{})
	stalled := listen(t, answerOnceThenHang(closed))
	picker := newFakePicker(stalled)
	target := testTarget(picker, nil, floor)
	client := connect(t, target)

	roundTrip(t, client, "ping", "pong")
	waitConns(t, target, stalled, 1)
	target.closeConnsTo(picker.upstreams[0])

	_ = client.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

	if _, err := client.Read(make([]byte, 1)); err == nil || isTimeout(err) {
		t.Errorf("client read error = %v, want EOF or reset", err)
	}

	select {
	case <-closed:
	case <-time.After(testTimeout):
		t.Error("the stalled upstream did not see the close")
	}
}

func TestCloseConnsTo_SkipsFirstByteWait(t *testing.T) {
	const budget = 300 * time.Millisecond

	hung := listen(t, hang)
	alive := listen(t, echo)
	picker := newFakePicker(hung, alive)
	target := testTarget(picker, nil, budget)
	client := connect(t, target)

	write(t, client, "ping")
	time.Sleep(budget / 6)

	if n := target.conns.closeAll(hung); n != 0 {
		t.Errorf("closed %d connections during the first-byte wait, want 0", n)
	}

	if got := readN(t, client, 4); got != "ping" {
		t.Errorf("read %q, want %q", got, "ping")
	}
}

func TestCloseConnsTo_KeyedByFinalUpstream(t *testing.T) {
	hung := listen(t, hang)
	alive := listen(t, echo)
	picker := newFakePicker(hung, alive)
	target := testTarget(picker, nil, floor)
	client := connect(t, target)

	roundTrip(t, client, "ping", "ping")
	waitConns(t, target, alive, 1)

	if n := target.conns.closeAll(hung); n != 0 {
		t.Errorf("closeAll(first upstream) = %d, want 0", n)
	}

	if n := target.conns.closeAll(alive); n != 1 {
		t.Errorf("closeAll(final upstream) = %d, want 1", n)
	}
}

func TestCloseConnsTo_LeavesOtherUpstream(t *testing.T) {
	alive := listen(t, echo)
	other := listen(t, echo)
	picker := newFakePicker(alive, other)
	target := testTarget(picker, nil, floor)
	client := connect(t, target)

	roundTrip(t, client, "ping", "ping")
	waitConns(t, target, alive, 1)
	target.closeConnsTo(picker.upstreams[1])
	roundTrip(t, client, "pong", "pong")
}

func TestCloseConnsTo_EnvNotTracked(t *testing.T) {
	alive := listen(t, echo)

	host, port, err := net.SplitHostPort(alive)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("KUBERNETES_SERVICE_HOST", host)
	t.Setenv("KUBERNETES_SERVICE_PORT", port)

	target := testTarget(nil, nil, floor)
	client := connect(t, target)

	roundTrip(t, client, "ping", "ping")

	if !target.conns.empty() {
		t.Error("an env upstream connection was registered")
	}
}

func TestCloseConnsTo_RegistryEmptyAfterClose(t *testing.T) {
	tests := []struct {
		name  string
		close func(client net.Conn, target *lbTarget, addr string)
	}{
		{name: "client closes", close: func(client net.Conn, _ *lbTarget, _ string) { _ = client.Close() }},
		{name: "stall kill", close: func(_ net.Conn, target *lbTarget, addr string) { target.conns.closeAll(addr) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alive := listen(t, echo)
			target := testTarget(newFakePicker(alive), nil, floor)
			client := connect(t, target)

			roundTrip(t, client, "ping", "ping")
			waitConns(t, target, alive, 1)
			tt.close(client, target, alive)
			waitConns(t, target, alive, 0)

			if !target.conns.empty() {
				t.Error("registry keeps an empty entry")
			}
		})
	}
}

func TestCloseConnsTo_RaceWithClientEOF(t *testing.T) {
	alive := listen(t, echo)
	picker := newFakePicker(alive)
	target := testTarget(picker, nil, floor)

	for range 20 {
		client := connect(t, target)

		roundTrip(t, client, "ping", "ping")
		waitConns(t, target, alive, 1)

		done := make(chan struct{})

		go func() {
			defer close(done)

			_ = client.Close()
		}()

		target.closeConnsTo(picker.upstreams[0])
		<-done
		waitConns(t, target, alive, 0)
	}
}
