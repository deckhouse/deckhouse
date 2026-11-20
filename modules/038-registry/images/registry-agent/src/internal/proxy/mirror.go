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

package proxy

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/deckhouse/registry-agent/internal/metrics"
)

const (
	// MirrorRefusalWindow is how long a digest a mirror served wrong content for is not asked of it.
	//
	// Long enough for the pull that hit it to finish from the targets behind the mirror, retries
	// included; short enough that a mirror whose administrator fixed the content is asked again
	// the same day without anyone restarting anything.
	MirrorRefusalWindow = time.Hour

	// maxRefusedDigests bounds the set. A mirror that serves everything wrong refuses every digest
	// in it, and the set must not grow with the size of the image set; past the bound the oldest
	// refusal goes first, which at worst asks the mirror for one digest too many.
	maxRefusedDigests = 4096
)

// errDigestMismatch is what a mirror's transfer ends with when its content is not what it was
// asked for.
var errDigestMismatch = errors.New("the mirror's content does not match the digest it was asked for")

// mirrorTargetsFor drops the targets of a mirror a request must not be sent to.
//
// Only reads: a mirror holds a copy, and anything else sent there — were the runtime ever to send
// it — would be a write to the wrong registry. And not a digest the mirror served wrong content for
// a moment ago: asked again, it serves the same, and the pull fails the same way every time.
//
// Nor the agent itself. The controller refuses a mirror at the agent's address, but this is the last
// place it can be stopped: asked, the agent would route the request to the mirror again.
func (s *Server) mirrorTargetsFor(request *http.Request, targets []Target) []Target {
	read := request.Method == http.MethodGet || request.Method == http.MethodHead

	kept := targets[:0:0]
	for i := range targets {
		target := &targets[i]
		if target.Mirror != "" &&
			(!read || equalHost(target.Host, s.Self) || s.mirrorRefused.refused(target)) {
			continue
		}
		kept = append(kept, *target)
	}
	return kept
}

// passOverMirror consumes a mirror's answer that is not the content, and records what it says
// about the mirror.
//
// A 404 is a miss: the mirror is up, and simply does not hold this. Anything else is a mirror
// that does not work — a server error, or a refusal that the targets behind it will not repeat —
// and is reported the way any target's failure is, and demoted: a refusal too, unlike a backend's,
// since the pull succeeds past it and nothing else would stop every request by digest paying for
// it again.
func (s *Server) passOverMirror(target *Target, response *http.Response, started time.Time) {
	defer func() { _ = response.Body.Close() }()

	switch status := response.StatusCode; {
	case status == http.StatusNotFound:
		s.demoted.succeed(target)
		s.mirrorResult(target, metrics.MirrorMiss)

	case status >= http.StatusInternalServerError:
		s.demoted.fail(target, started)
		s.markFailing(target.Name)
		s.failover(target.Name, metrics.ReasonServerError)
		s.mirrorResult(target, metrics.MirrorUnavailable)
		s.Log.Warn("a mirror failed, trying the next target",
			"mirror", target.Mirror, "host", target.Host, "status", response.Status)

	default:
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			// The token it refused, ours or an anonymous one, is not offered to it again. See forward.
			s.auth.forget(target, response.Request.Header.Get("Authorization"))
		}
		s.demoted.fail(target, started)
		s.markFailing(target.Name)
		s.failover(target.Name, metrics.ReasonRefused)
		s.mirrorResult(target, metrics.MirrorRefused)
		s.Log.Warn("a mirror refused the request, trying the next target",
			"mirror", target.Mirror, "host", target.Host, "status", response.Status,
			"challenge", response.Header.Get("WWW-Authenticate"))
	}
}

// mirrorResult counts what a mirror's target came to. Nothing for any other target.
func (s *Server) mirrorResult(target *Target, result string) {
	if s.Metrics == nil || target.Mirror == "" {
		return
	}
	s.Metrics.MirrorRequests.WithLabelValues(target.Mirror, result).Inc()
}

// verifiedBody checks a mirror's content against the digest the request named, while it streams.
//
// The runtime verifies the digest as well, and a mismatch would never reach a container either
// way. What it cannot do is stop asking the mirror: it retries the same request, the agent sends it
// to the same mirror, and the pull fails on every attempt for as long as the mirror holds the bad
// copy. So the agent checks too, and when the content is wrong it cuts the transfer short — the
// runtime then sees a broken body rather than a complete wrong one — and stops asking this mirror
// for this digest. The retry goes to the targets behind it.
//
// Only a whole body can be checked: not a range, which a runtime resuming a transfer asks for, and
// not a digest algorithm this agent does not know. Those are relayed as they are, with the runtime's
// own check behind them.
//
// Nor content the mirror encoded, which the agent relays as it is and the runtime decodes: its bytes
// are not the ones the digest is of. The agent does not ask for an encoding (see attempt), so this is
// a proxy in front of the mirror compressing on its own; refusing correct content for an hour over it
// would cost a failed pull for every new image.
//
// Counted as a hit once the content is known to be right, and at once where it cannot be checked: a
// transfer that turns out wrong is a digest_mismatch and nothing else.
func (s *Server) verifiedBody(request *http.Request, response *http.Response, target *Target) io.ReadCloser {
	unchecked := func() io.ReadCloser {
		s.mirrorResult(target, metrics.MirrorHit)
		return response.Body
	}

	if request.Method != http.MethodGet || response.StatusCode != http.StatusOK ||
		request.Header.Get("Range") != "" {
		return unchecked()
	}
	if encoding := response.Header.Get("Content-Encoding"); encoding != "" && !strings.EqualFold(encoding, "identity") {
		return unchecked()
	}

	algorithm, expected, _ := strings.Cut(target.Digest, ":")
	var digest hash.Hash
	switch algorithm {
	case "sha256":
		digest = sha256.New()
	case "sha512":
		digest = sha512.New()
	default:
		return unchecked()
	}

	return &verifyingBody{
		body:     response.Body,
		hash:     digest,
		expected: strings.ToLower(expected),
		verified: func() { s.mirrorResult(target, metrics.MirrorHit) },
		mismatch: func() {
			s.mirrorRefused.refuse(target)
			s.mirrorResult(target, metrics.MirrorDigestMismatch)
			s.Log.Error("a mirror served content that does not match its digest; "+
				"not asking it for this digest again for a while",
				"mirror", target.Mirror, "host", target.Host, "digest", target.Digest,
				"for", MirrorRefusalWindow.String())
		},
	}
}

// verifyingBody hashes what passes through it and fails the read that ends it when the hash is not
// the one expected.
//
// The last byte read is always held back until the mirror's body has ended, and released only when
// the hash is right. Without that a body with a declared length is complete at the client before
// the end of it is known here: over HTTP/2 the last data comes with no error and the end of the
// stream only with the next read, so all of a wrong body went out, the client took it as finished
// and hung up, and the next read here ended with the client's cancellation instead of the end of
// the body — no check, no refusal, and the runtime's retry came back to the same mirror.
type verifyingBody struct {
	body     io.ReadCloser
	hash     hash.Hash
	expected string
	verified func()
	mismatch func()
	failed   bool
	done     bool

	// held is the last byte read, not yet passed on when holding.
	held    byte
	holding bool
}

func (b *verifyingBody) Read(p []byte) (int, error) {
	if b.failed {
		return 0, errDigestMismatch
	}
	if len(p) == 0 {
		return 0, nil
	}
	if b.done {
		if b.holding {
			p[0], b.holding = b.held, false
			return 1, io.EOF
		}
		return 0, io.EOF
	}

	n, err := b.body.Read(p)
	b.hash.Write(p[:n])

	// What goes out is the byte held from before and all of this read but its last byte, which is
	// held in its turn.
	out := n
	if n > 0 {
		last := p[n-1]
		if b.holding {
			copy(p[1:n], p[:n-1])
			p[0] = b.held
		} else {
			out = n - 1
		}
		b.held, b.holding = last, true
	}

	if !errors.Is(err, io.EOF) {
		return out, err
	}

	b.done = true
	if hex.EncodeToString(b.hash.Sum(nil)) == b.expected {
		b.verified()
		if !b.holding {
			return out, io.EOF
		}
		if out < len(p) {
			p[out], b.holding = b.held, false
			return out + 1, io.EOF
		}
		// No room for the held byte: it goes out with the next read.
		return out, nil
	}

	// Nothing of this read goes out, nor the byte held, so a body with a declared length comes out
	// short and the runtime sees the transfer fail rather than finish. What came before is already
	// sent; the runtime discards it with the broken transfer.
	b.failed = true
	b.mismatch()
	return 0, errDigestMismatch
}

func (b *verifyingBody) Close() error { return b.body.Close() }

// refusedDigests remembers, per address of a mirror, the digests it served wrong content for, each
// until its window closes.
//
// Per address rather than per mirror: the spare addresses in source.mirrors may be replicas of their
// own, and one of them holding a bad copy says nothing about the others. Where they are aliases of
// one store, each serves the bad copy once before it is refused there too.
type refusedDigests struct {
	// now is the clock, replaceable in tests.
	now func() time.Time

	mu      sync.Mutex
	entries map[string]time.Time
}

func (r *refusedDigests) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

func refusalKey(target *Target) string {
	return demotionKey(target) + "@" + target.Digest
}

// refuse stops asking the target's address of its mirror for its digest until the window closes.
func (r *refusedDigests) refuse(target *Target) {
	now := r.clock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.entries == nil {
		r.entries = map[string]time.Time{}
	}
	for key, until := range r.entries {
		if !now.Before(until) {
			delete(r.entries, key)
		}
	}
	for len(r.entries) >= maxRefusedDigests {
		oldest, oldestUntil := "", time.Time{}
		for key, until := range r.entries {
			if oldest == "" || until.Before(oldestUntil) {
				oldest, oldestUntil = key, until
			}
		}
		delete(r.entries, oldest)
	}
	r.entries[refusalKey(target)] = now.Add(MirrorRefusalWindow)
}

// refused reports whether the target's address of its mirror is not to be asked for its digest now.
func (r *refusedDigests) refused(target *Target) bool {
	now := r.clock()

	r.mu.Lock()
	defer r.mu.Unlock()

	until, known := r.entries[refusalKey(target)]
	return known && now.Before(until)
}
