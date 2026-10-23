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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	errTooManyStreams       = errors.New("too many streams on this replica")
	errTooManyClientStreams = errors.New("too many streams of the client on this replica")
)

// record is a revocation the feed has seen since it started, with the number it got.
type record struct {
	seq uint64
	// typ is the type of the revocation (pair, session, grant), kind the kind of its key (pair,
	// sid, grant), and value what the key is made of.
	typ        string
	kind       string
	value      string
	generation uint64
	exp        int64
	at         int64
	// created is when the revocation happened, for the delivery time.
	created time.Time
	clients []string
}

// cursor is a position in the feed: the epoch of the process and the number of the last record.
type cursor struct {
	epoch string
	seq   uint64
}

func (c cursor) String() string {
	return c.epoch + "." + strconv.FormatUint(c.seq, 10)
}

// parseCursor reads "<epoch>.<seq>".
func parseCursor(s string) (cursor, bool) {
	epoch, seq, ok := strings.Cut(s, ".")
	if !ok || epoch == "" {
		return cursor{}, false
	}
	n, err := strconv.ParseUint(seq, 10, 64)
	if err != nil {
		return cursor{}, false
	}
	return cursor{epoch: epoch, seq: n}, true
}

// stream is one subscriber connection.
type stream struct {
	client feedClient
	events chan record
	// done is closed when the stream must end: it fell behind, or it was unsubscribed.
	done      chan struct{}
	closeOnce sync.Once
	// lastWrite is when the stream last wrote to its client, Unix nanoseconds.
	lastWrite atomic.Int64
}

func (s *stream) close() {
	s.closeOnce.Do(func() { close(s.done) })
}

// hub numbers the revocations the informer sees and hands them to the streams of the clients they
// name. It keeps the last of them in a ring, so that a client that reconnects to the same process
// resumes where it stopped instead of loading a snapshot.
type hub struct {
	mu        sync.Mutex
	epoch     string
	seq       uint64
	ring      []record
	ringSize  int
	queueSize int
	streams   map[*stream]struct{}
	perClient map[string]int

	maxStreams          int
	maxStreamsPerClient int
}

func newHub(l Limits) (*hub, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, fmt.Errorf("choose the epoch of the feed: %w", err)
	}
	return &hub{
		epoch:               hex.EncodeToString(b[:]),
		ring:                make([]record, 0, l.Ring),
		ringSize:            l.Ring,
		queueSize:           l.Queue,
		streams:             map[*stream]struct{}{},
		perClient:           map[string]int{},
		maxStreams:          l.Streams,
		maxStreamsPerClient: l.StreamsPerClient,
	}, nil
}

// publish numbers rec and queues it to the streams of the clients it names. A stream whose queue is
// full is dropped: its client reconnects and catches up.
func (h *hub) publish(rec record) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	rec.seq = h.seq
	if len(h.ring) == h.ringSize {
		copy(h.ring, h.ring[1:])
		h.ring = h.ring[:len(h.ring)-1]
	}
	h.ring = append(h.ring, rec)
	for st := range h.streams {
		if !slices.Contains(rec.clients, st.client.id) {
			continue
		}
		select {
		case st.events <- rec:
		default:
			h.dropLocked(st)
		}
	}
}

// subscription is what a new stream starts with.
type subscription struct {
	stream *stream
	// resumed means the stream continues from the cursor of the client: backlog holds the records
	// after it. Otherwise the client gets a snapshot at position.
	resumed  bool
	backlog  []record
	position cursor
}

// subscribe registers a stream of c. The stream receives every record published from now on; what
// came before is either the backlog after since, when since is in this epoch and the ring still
// holds everything after it, or a snapshot the caller builds at position.
func (h *hub) subscribe(c feedClient, since *cursor, now time.Time) (subscription, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.streams) >= h.maxStreams {
		return subscription{}, errTooManyStreams
	}
	if h.perClient[c.id] >= h.maxStreamsPerClient {
		return subscription{}, errTooManyClientStreams
	}
	st := &stream{client: c, events: make(chan record, h.queueSize), done: make(chan struct{})}
	st.lastWrite.Store(now.UnixNano())
	h.streams[st] = struct{}{}
	h.perClient[c.id]++

	if since != nil && since.epoch == h.epoch && since.seq <= h.seq && since.seq+uint64(len(h.ring)) >= h.seq {
		var backlog []record
		for _, rec := range h.ring {
			if rec.seq > since.seq && slices.Contains(rec.clients, c.id) {
				backlog = append(backlog, rec)
			}
		}
		return subscription{stream: st, resumed: true, backlog: backlog, position: *since}, nil
	}
	return subscription{stream: st, position: cursor{epoch: h.epoch, seq: h.seq}}, nil
}

// unsubscribe removes st; it can be called more than once.
func (h *hub) unsubscribe(st *stream) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dropLocked(st)
}

func (h *hub) dropLocked(st *stream) {
	if _, ok := h.streams[st]; !ok {
		return
	}
	delete(h.streams, st)
	h.perClient[st.client.id]--
	if h.perClient[st.client.id] == 0 {
		delete(h.perClient, st.client.id)
	}
	st.close()
}

// idleCursor is the position of st for a heartbeat: the latest number when nothing is waiting in its
// queue, since then every record before it that names the client has been written; last otherwise.
func (h *hub) idleCursor(st *stream, last uint64) cursor {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(st.events) == 0 {
		return cursor{epoch: h.epoch, seq: h.seq}
	}
	return cursor{epoch: h.epoch, seq: last}
}

// stats returns the streams per client and the longest time a stream has not written, for the
// metrics.
func (h *hub) stats(now time.Time) (map[string]int, time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	perClient := make(map[string]int, len(h.perClient))
	for id, n := range h.perClient {
		perClient[id] = n
	}
	var lag time.Duration
	for st := range h.streams {
		lag = max(lag, now.Sub(time.Unix(0, st.lastWrite.Load())))
	}
	return perClient, lag
}
