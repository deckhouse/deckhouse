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
	"errors"
	"io"
	"net"
	"syscall"
	"testing"
	"time"
)

type brokenConn struct {
	net.Conn
}

func (brokenConn) Write([]byte) (int, error) {
	return 0, errors.New("write: broken pipe")
}

func (brokenConn) SetReadDeadline(time.Time) error { return nil }

func (brokenConn) Close() error { return nil }

func TestRelay_SettleStopsRecording(t *testing.T) {
	a, b := net.Pipe()

	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })

	r := newRelay(a, true)
	r.remember([]byte("ping"))
	r.settle()

	if r.canRetry() {
		t.Fatal("canRetry() after settle = true, want false")
	}

	r.remember([]byte("more"))

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.record != nil || r.recording {
		t.Errorf("after settle record = %q recording = %v, want nil and false", r.record, r.recording)
	}
}

func TestRelay_WriteFailureOnRecordedBytesIsNotFatal(t *testing.T) {
	r := newRelay(brokenConn{}, true)

	n, err := r.Write([]byte("ping"))
	if err != nil || n != 4 {
		t.Errorf("Write on a recording relay = (%d, %v), want (4, nil): the record keeps the bytes for the replay", n, err)
	}

	r.settle()

	if _, err := r.Write([]byte("ping")); err == nil {
		t.Error("Write after settle swallowed the error, want it returned")
	}
}

func TestRelay_SwitchToFailureStopsRecording(t *testing.T) {
	a, b := net.Pipe()

	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })

	r := newRelay(a, true)
	r.remember([]byte("ping"))

	if _, err := r.switchTo(brokenConn{}, floor); err == nil {
		t.Fatal("switchTo with a broken replay returned nil error")
	}

	if r.canRetry() {
		t.Error("canRetry() after a failed replay = true, want false")
	}

	if got := r.conn(); got != a {
		t.Error("switchTo with a broken replay replaced dst")
	}
}

type swapConn struct {
	net.Conn
	onWrite func()
}

func (c swapConn) Write([]byte) (int, error) {
	c.onWrite()

	return 0, errors.New("write: broken pipe")
}

func (swapConn) SetReadDeadline(time.Time) error { return nil }

func (swapConn) Close() error { return nil }

func TestRelay_RecordCapBoundary(t *testing.T) {
	a, b := net.Pipe()

	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })

	r := newRelay(a, true)

	if _, recorded := r.remember(make([]byte, recordCap)); !recorded || !r.canRetry() {
		t.Fatal("exactly recordCap bytes dropped the record")
	}

	if _, recorded := r.remember([]byte{0}); recorded || r.canRetry() {
		t.Fatal("one byte over the cap kept recording")
	}
}

func TestRelay_SwitchToKeepsHalfClose(t *testing.T) {
	a, b := net.Pipe()

	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })

	client, server := pair(t)

	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })

	_ = server.SetDeadline(time.Now().Add(testTimeout))

	r := newRelay(a, true)
	r.remember([]byte("ping"))
	r.closeWrite()

	if _, err := r.switchTo(client, floor); err != nil {
		t.Fatalf("switchTo: %v", err)
	}

	data, err := io.ReadAll(server)
	if err != nil || string(data) != "ping" {
		t.Errorf("peer read %q, %v; want ping then EOF", data, err)
	}
}

func TestRelay_WriteToSupersededConnIsNotFatal(t *testing.T) {
	a, b := net.Pipe()

	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })

	r := newRelay(nil, true)
	r.dst = swapConn{onWrite: func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		r.dst = a
		r.recording = false
		r.record = nil
	}}

	if n, err := r.Write([]byte("ping")); err != nil || n != 4 {
		t.Errorf("Write superseded by a switch = (%d, %v), want (4, nil)", n, err)
	}
}

type settleOnWriteConn struct {
	net.Conn
	r *relay
}

func (c settleOnWriteConn) Write([]byte) (int, error) {
	c.r.settle()

	return 0, &net.OpError{Op: "write", Net: "tcp", Err: syscall.EPIPE}
}

func (settleOnWriteConn) SetReadDeadline(time.Time) error { return nil }

func (settleOnWriteConn) Close() error { return nil }

func TestRelay_WriteErrorAfterSettleIsReturned(t *testing.T) {
	r := newRelay(nil, true)
	r.dst = settleOnWriteConn{r: r}

	if _, err := r.Write([]byte("ping")); err == nil {
		t.Error("Write swallowed the error although settle() had dropped the record")
	}
}

func TestRelay_Arm(t *testing.T) {
	tests := []struct {
		name   string
		before func(r *relay)
		armed  bool
	}{
		{name: "nothing recorded yet", before: func(*relay) {}, armed: false},
		{name: "bytes recorded", before: func(r *relay) { r.remember([]byte("ping")) }, armed: true},
		{
			name: "record dropped over the cap",
			before: func(r *relay) {
				r.remember(make([]byte, recordCap))
				r.remember([]byte{0})
			},
			armed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := net.Pipe()

			t.Cleanup(func() { _ = a.Close(); _ = b.Close() })

			r := newRelay(a, true)
			tt.before(r)
			r.arm(floor)

			if got := !r.hop().armedAt.IsZero(); got != tt.armed {
				t.Errorf("armed = %v, want %v", got, tt.armed)
			}
		})
	}
}

func TestRelay_RememberArmsAfterArm(t *testing.T) {
	a, b := net.Pipe()

	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })

	r := newRelay(a, true)
	r.arm(floor)
	r.remember([]byte("ping"))

	if r.hop().armedAt.IsZero() {
		t.Error("the client's first byte did not arm the deadline")
	}
}

func TestRelay_SwitchToWithEmptyRecordArmsOnFirstClientByte(t *testing.T) {
	a, b := net.Pipe()
	c, d := net.Pipe()

	t.Cleanup(func() { _ = a.Close(); _ = b.Close(); _ = c.Close(); _ = d.Close() })

	r := newRelay(a, true)
	if _, err := r.switchTo(c, floor); err != nil {
		t.Fatalf("switchTo: %v", err)
	}

	if !r.hop().armedAt.IsZero() {
		t.Error("deadline armed before the client sent anything")
	}

	r.remember([]byte("ping"))

	if r.hop().armedAt.IsZero() {
		t.Error("deadline not armed by the client's first byte")
	}
}
