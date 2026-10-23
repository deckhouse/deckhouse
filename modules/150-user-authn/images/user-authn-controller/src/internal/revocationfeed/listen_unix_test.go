//go:build linux || darwin

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

package revocationfeed

import (
	"net"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// acceptedConn listens with listen, dials the listener, and returns the connection it accepted.
func acceptedConn(t *testing.T) syscall.RawConn {
	t.Helper()
	ln, err := listen(t.Context(), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		t.Fatalf("accepted %T, want a TCP connection", conn)
	}
	raw, err := tcp.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func sockoptInt(t *testing.T, raw syscall.RawConn, level, opt int) int {
	t.Helper()
	var (
		value  int
		getErr error
	)
	if err := raw.Control(func(fd uintptr) {
		value, getErr = unix.GetsockoptInt(int(fd), level, opt)
	}); err != nil {
		t.Fatal(err)
	}
	if getErr != nil {
		t.Fatalf("getsockopt %d/%d: %v", level, opt, getErr)
	}
	return value
}

// An accepted connection with nothing to send probes its peer after 15 seconds, every 5 seconds,
// and gives up after 3 probes.
func TestListen_KeepAlive(t *testing.T) {
	t.Parallel()

	raw := acceptedConn(t)
	if v := sockoptInt(t, raw, unix.SOL_SOCKET, unix.SO_KEEPALIVE); v == 0 {
		t.Error("SO_KEEPALIVE is off")
	}
	if v := sockoptInt(t, raw, unix.IPPROTO_TCP, unix.TCP_KEEPINTVL); v != int(keepAliveInterval.Seconds()) {
		t.Errorf("TCP_KEEPINTVL = %d, want %d", v, int(keepAliveInterval.Seconds()))
	}
	if v := sockoptInt(t, raw, unix.IPPROTO_TCP, unix.TCP_KEEPCNT); v != keepAliveCount {
		t.Errorf("TCP_KEEPCNT = %d, want %d", v, keepAliveCount)
	}
}
