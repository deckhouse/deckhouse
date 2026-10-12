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
	"net"
	"sync"
	"time"
)

const recordCap = 64 << 10

var errRecordDropped = errors.New("client sent more than the replay cap before the upstream answered")

type relay struct {
	mu        sync.Mutex
	dst       net.Conn
	record    []byte
	recording bool
	eof       bool
	wait      time.Duration
	armedAt   time.Time
}

type hop struct {
	wait    time.Duration
	armedAt time.Time
}

func newRelay(dst net.Conn, recording bool) *relay {
	return &relay{dst: dst, recording: recording}
}

func (r *relay) Write(p []byte) (int, error) {
	dst, recorded := r.remember(p)

	n, err := dst.Write(p)
	if err != nil && recorded && (r.canRetry() || r.conn() != dst) {
		// The record still carries p, or switchTo has already replayed it:
		// the first-byte reader on dst sees the same failure and either
		// replays the record to another upstream or closes the connection.
		// Once settle() dropped the record nobody replays, so the error
		// must surface.
		return len(p), nil
	}

	return n, err
}

func (r *relay) closeWrite() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.eof = true
	closeWrite(r.dst)
}

func (r *relay) remember(p []byte) (net.Conn, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.wait > 0 && r.armedAt.IsZero() && len(p) > 0 {
		r.armLocked()
	}

	if !r.recording {
		return r.dst, false
	}

	if len(r.record)+len(p) > recordCap {
		r.recording = false
		r.record = nil

		return r.dst, false
	}

	r.record = append(r.record, p...)

	return r.dst, true
}

func (r *relay) arm(wait time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.wait = wait
	r.armedAt = time.Time{}

	if len(r.record) > 0 || !r.recording {
		r.armLocked()
	}
}

func (r *relay) armLocked() {
	r.armedAt = time.Now()
	_ = r.dst.SetReadDeadline(r.armedAt.Add(r.wait))
}

func (r *relay) hop() hop {
	r.mu.Lock()
	defer r.mu.Unlock()

	return hop{wait: r.wait, armedAt: r.armedAt}
}

func (r *relay) settle() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.recording = false
	r.record = nil
	r.wait = 0
	r.armedAt = time.Time{}
	_ = r.dst.SetReadDeadline(time.Time{})
}

func (r *relay) conn() net.Conn {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.dst
}

func (r *relay) canRetry() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.recording
}

func (r *relay) switchTo(dst net.Conn, wait time.Duration) (net.Conn, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.recording {
		return nil, errRecordDropped
	}

	replayed := len(r.record) > 0
	if err := r.replayLocked(dst); err != nil {
		return nil, err
	}

	if r.eof {
		closeWrite(dst)
	}

	old := r.dst
	r.dst = dst
	r.recording = false
	r.record = nil
	r.wait = wait
	r.armedAt = time.Time{}

	if replayed {
		r.armLocked()
	}

	return old, nil
}

func (r *relay) replayLocked(dst net.Conn) error {
	if len(r.record) == 0 {
		return nil
	}

	if _, err := dst.Write(r.record); err != nil {
		r.recording = false
		r.record = nil

		return err
	}

	return nil
}
