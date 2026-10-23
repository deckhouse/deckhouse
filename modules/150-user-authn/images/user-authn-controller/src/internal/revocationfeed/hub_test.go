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
	"errors"
	"sync"
	"testing"
	"time"
)

func newTestHub(t *testing.T, mutate func(*Limits)) *hub {
	t.Helper()
	l := DefaultLimits()
	if mutate != nil {
		mutate(&l)
	}
	h, err := newHub(l)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func grantRecord(clients ...string) record {
	return record{typ: "grant", kind: keyGrant, value: "g", clients: clients}
}

func TestParseCursor(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		in       string
		expected cursor
		ok       bool
	}{
		{in: "0a1b2c3d4e5f6a7b.42", expected: cursor{epoch: "0a1b2c3d4e5f6a7b", seq: 42}, ok: true},
		{in: "0a1b2c3d4e5f6a7b.0", expected: cursor{epoch: "0a1b2c3d4e5f6a7b", seq: 0}, ok: true},
		{in: "", ok: false},
		{in: "42", ok: false},
		{in: ".42", ok: false},
		{in: "epoch.-1", ok: false},
		{in: "epoch.x", ok: false},
	} {
		got, ok := parseCursor(tt.in)
		if ok != tt.ok || (ok && got != tt.expected) {
			t.Errorf("parseCursor(%q) = %+v, %v", tt.in, got, ok)
		}
		if ok && got.String() != tt.in {
			t.Errorf("String = %q, want %q", got.String(), tt.in)
		}
	}
}

func TestHub_FiltersByClient(t *testing.T) {
	t.Parallel()

	h := newTestHub(t, nil)
	console, err := h.subscribe(feedClient{id: "console"}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	grafana, err := h.subscribe(feedClient{id: "grafana"}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	h.publish(grantRecord("console"))
	h.publish(grantRecord("console", "grafana"))

	if n := len(console.stream.events); n != 2 {
		t.Errorf("console got %d events, want 2", n)
	}
	if n := len(grafana.stream.events); n != 1 {
		t.Errorf("grafana got %d events, want 1", n)
	}
	if rec := <-grafana.stream.events; rec.seq != 2 {
		t.Errorf("grafana got seq %d, want 2", rec.seq)
	}
}

func TestHub_Resume(t *testing.T) {
	t.Parallel()

	h := newTestHub(t, func(l *Limits) { l.Ring = 3 })
	for range 4 {
		h.publish(grantRecord("console"))
	}
	h.publish(grantRecord("grafana"))
	// The ring holds 3, 4, 5.

	tests := []struct {
		name        string
		since       *cursor
		resumed     bool
		backlogSeqs []uint64
	}{
		{name: "no cursor", since: nil},
		{name: "another epoch", since: &cursor{epoch: "0000000000000000", seq: 4}},
		{name: "a position from the future", since: &cursor{epoch: h.epoch, seq: 6}},
		{name: "older than the ring", since: &cursor{epoch: h.epoch, seq: 1}},
		{name: "at the start of the ring", since: &cursor{epoch: h.epoch, seq: 2}, resumed: true, backlogSeqs: []uint64{3, 4}},
		{name: "the latest", since: &cursor{epoch: h.epoch, seq: 5}, resumed: true},
	}
	for _, tt := range tests {
		sub, err := h.subscribe(feedClient{id: "console"}, tt.since, time.Now())
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if sub.resumed != tt.resumed {
			t.Errorf("%s: resumed = %v, want %v", tt.name, sub.resumed, tt.resumed)
		}
		var seqs []uint64
		for _, rec := range sub.backlog {
			seqs = append(seqs, rec.seq)
		}
		if len(seqs) != len(tt.backlogSeqs) || (len(seqs) > 0 && (seqs[0] != tt.backlogSeqs[0] || seqs[len(seqs)-1] != tt.backlogSeqs[len(seqs)-1])) {
			t.Errorf("%s: backlog = %v, want %v", tt.name, seqs, tt.backlogSeqs)
		}
		if !tt.resumed && sub.position != (cursor{epoch: h.epoch, seq: 5}) {
			t.Errorf("%s: snapshot position = %+v", tt.name, sub.position)
		}
		h.unsubscribe(sub.stream)
	}
}

// A consumer that does not read falls behind; once its queue is full the stream is dropped, and the
// others go on.
func TestHub_SlowConsumer(t *testing.T) {
	t.Parallel()

	h := newTestHub(t, func(l *Limits) { l.Queue = 2 })
	slow, err := h.subscribe(feedClient{id: "console"}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	fast, err := h.subscribe(feedClient{id: "console"}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		h.publish(grantRecord("console"))
		<-fast.stream.events
	}
	select {
	case <-slow.stream.done:
	default:
		t.Fatal("the slow stream was not closed")
	}
	select {
	case <-fast.stream.done:
		t.Fatal("the fast stream was closed")
	default:
	}
	perClient, _ := h.stats(time.Now())
	if perClient["console"] != 1 {
		t.Errorf("streams of console = %d, want 1", perClient["console"])
	}
	h.unsubscribe(slow.stream) // a second removal is harmless
}

func TestHub_Limits(t *testing.T) {
	t.Parallel()

	h := newTestHub(t, func(l *Limits) {
		l.StreamsPerClient = 2
		l.Streams = 3
	})
	var subs []subscription
	for range 2 {
		sub, err := h.subscribe(feedClient{id: "console"}, nil, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		subs = append(subs, sub)
	}
	if _, err := h.subscribe(feedClient{id: "console"}, nil, time.Now()); !errors.Is(err, errTooManyClientStreams) {
		t.Errorf("third console stream: err = %v", err)
	}
	if _, err := h.subscribe(feedClient{id: "grafana"}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.subscribe(feedClient{id: "argocd"}, nil, time.Now()); !errors.Is(err, errTooManyStreams) {
		t.Errorf("fourth stream: err = %v", err)
	}
	h.unsubscribe(subs[0].stream)
	if _, err := h.subscribe(feedClient{id: "console"}, nil, time.Now()); err != nil {
		t.Errorf("after a stream ended: %v", err)
	}
}

func TestHub_IdleCursor(t *testing.T) {
	t.Parallel()

	h := newTestHub(t, nil)
	sub, err := h.subscribe(feedClient{id: "console"}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	h.publish(grantRecord("grafana"))
	if got := h.idleCursor(sub.stream, 0); got.seq != 1 {
		t.Errorf("idle cursor = %d, want 1: nothing for console is waiting", got.seq)
	}
	h.publish(grantRecord("console"))
	if got := h.idleCursor(sub.stream, 1); got.seq != 1 {
		t.Errorf("cursor with a queued event = %d, want the last written, 1", got.seq)
	}
}

// Run with -race: publishers, subscribers, and readers at once.
func TestHub_Concurrent(t *testing.T) {
	t.Parallel()

	h := newTestHub(t, func(l *Limits) { l.Queue = 8 })
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 200 {
				h.publish(grantRecord("console", "grafana"))
			}
		})
	}
	for range 4 {
		wg.Go(func() {
			for range 50 {
				sub, err := h.subscribe(feedClient{id: "console"}, &cursor{epoch: h.epoch, seq: 1}, time.Now())
				if err != nil {
					continue
				}
				for range 3 {
					select {
					case <-sub.stream.events:
					case <-sub.stream.done:
					default:
					}
				}
				_ = h.idleCursor(sub.stream, 0)
				_, _ = h.stats(time.Now())
				h.unsubscribe(sub.stream)
			}
		})
	}
	wg.Wait()
	if perClient, _ := h.stats(time.Now()); len(perClient) != 0 {
		t.Errorf("streams left: %v", perClient)
	}
}
