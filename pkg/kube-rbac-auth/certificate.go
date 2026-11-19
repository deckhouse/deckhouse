// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kuberbacauth

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	utilcache "k8s.io/apimachinery/pkg/util/cache"
)

const (
	// authenticatedGroup is the group the API server adds to an authenticated user.
	// A TokenReview returns it; a certificate user gets it here, so both have the same access.
	authenticatedGroup   = "system:authenticated"
	unauthenticatedGroup = "system:unauthenticated"
	anonymousUser        = "system:anonymous"

	// maxClientIntermediates is how many intermediate certificates a client may send.
	// Each one costs signature checks before the client is known, and real chains are short.
	maxClientIntermediates = 4

	// chainCheckTTL is how long the result of a chain check is reused, so a client cannot
	// make the application repeat the signature checks with every request of a connection.
	chainCheckTTL = 10 * time.Second
)

// newClientCAs loads the bundle of the CAs that sign client certificates and keeps it up to date.
func newClientCAs(file string, clock utilcache.Clock, log Logger) (*reloader[x509.CertPool], error) {
	return newReloader("client CA bundle", []string{file}, func(contents [][]byte) (*x509.CertPool, error) {
		return parseCABundle(contents[0])
	}, clock, log)
}

var (
	pemBegin = []byte("-----BEGIN")
	pemEnd   = []byte("-----END")
)

// pemIndent may come before a boundary of a block on its line: indentation and the byte order mark
// that editors on Windows put at the start of a file.
const pemIndent = " \t\uFEFF"

// parseCABundle reads the PEM certificates of a bundle. Text around them, such as comments or the output
// of openssl, is skipped, as RFC 7468 allows. A damaged block or a block of another type fails the whole
// bundle: skipping it would silently drop a CA.
func parseCABundle(data []byte) (*x509.CertPool, error) {
	begins := boundaries(data, pemBegin)
	pool := x509.NewCertPool()
	for i, start := range begins {
		n := i + 1
		block, after := pem.Decode(data[start:])
		// pem.Decode skips a block it cannot decode and takes the next one: the block must end before the next one begins.
		if block == nil || (n < len(begins) && len(data)-len(after) > begins[n]) {
			return nil, fmt.Errorf("certificate %d is not a valid PEM block", n)
		}
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			return nil, fmt.Errorf("PEM block %d is not a certificate", n)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("certificate %d: %w", n, err)
		}
		pool.AddCert(cert)
	}
	// Every block has one end. Another one is left from a block whose beginning is damaged.
	if len(boundaries(data, pemEnd)) != len(begins) {
		return nil, errors.New("a PEM block has a damaged beginning")
	}
	if len(begins) == 0 {
		return nil, errors.New("no certificates")
	}
	return pool, nil
}

// boundaries returns where the boundary is in the lines that begin with it after pemIndent.
// A boundary elsewhere in a line, for example in a commented out certificate, is text.
func boundaries(data, boundary []byte) []int {
	var offsets []int
	offset := 0
	for line := range bytes.Lines(data) {
		trimmed := bytes.TrimLeft(line, pemIndent)
		if bytes.HasPrefix(trimmed, boundary) {
			offsets = append(offsets, offset+len(line)-len(trimmed))
		}
		offset += len(line)
	}
	return offsets
}

// certAuthenticator turns a client certificate into a user the way the API server does:
// the common name is the user name and the organizations are the groups.
type certAuthenticator struct {
	cas   *reloader[x509.CertPool]
	clock utilcache.Clock
	log   Logger
	// checked maps a certificate chain and a bundle version to the user or to nil for a rejected chain.
	checked *utilcache.LRUExpireCache

	// running holds the checks in progress. Concurrent requests with the same chain wait for one check
	// instead of repeating it; requests with other chains are not held up.
	mu      sync.Mutex
	running map[chainID]*chainCheck
}

// chainID identifies the certificates sent by a client and the bundle they are checked against.
type chainID [sha256.Size]byte

// chainCheck is a check in progress. user is set before done is closed.
type chainCheck struct {
	done chan struct{}
	user *User
}

func newCertAuthenticator(cas *reloader[x509.CertPool], cacheSize int, clock utilcache.Clock, log Logger) *certAuthenticator {
	return &certAuthenticator{
		cas:     cas,
		clock:   clock,
		log:     log,
		checked: utilcache.NewLRUExpireCacheWithClock(cacheSize, clock),
		running: make(map[chainID]*chainCheck),
	}
}

// authenticate returns nil when the request has no certificate signed by a trusted CA.
func (a *certAuthenticator) authenticate(r *http.Request) *User {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return nil
	}
	chain := r.TLS.PeerCertificates
	if len(chain)-1 > maxClientIntermediates {
		a.log.Debug("client certificate chain is too long", slog.Int("intermediates", len(chain)-1))
		return nil
	}
	// Without roots the chain would be checked against the CAs of the operating system.
	roots, version := a.cas.get()
	if roots == nil {
		return nil
	}

	id := newChainID(chain, version)
	if cached, ok := a.checked.Get(id); ok {
		return cached.(*User)
	}

	a.mu.Lock()
	if running, ok := a.running[id]; ok {
		a.mu.Unlock()
		select {
		case <-running.done:
			return running.user
		case <-r.Context().Done():
			return nil
		}
	}
	// The check may have finished between the two lookups.
	if cached, ok := a.checked.Get(id); ok {
		a.mu.Unlock()
		return cached.(*User)
	}
	current := &chainCheck{done: make(chan struct{})}
	a.running[id] = current
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		delete(a.running, id)
		a.mu.Unlock()
		close(current.done)
	}()
	user, ttl := a.check(chain, roots)
	current.user = user
	a.checked.Add(id, user, ttl)
	return user
}

// check verifies the chain and returns the user with the time the answer may be reused for.
func (a *certAuthenticator) check(chain []*x509.Certificate, roots *x509.CertPool) (*User, time.Duration) {
	leaf := chain[0]
	intermediates := x509.NewCertPool()
	for _, cert := range chain[1:] {
		intermediates.AddCert(cert)
	}
	now := a.clock.Now()
	verified, err := leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		a.log.Debug("client certificate is not trusted", slog.String("error", err.Error()))
		// The rejection must end before a certificate that is not valid yet becomes valid.
		ttl := chainCheckTTL
		for _, cert := range chain {
			if cert.NotBefore.After(now) {
				ttl = min(ttl, cert.NotBefore.Sub(now)-time.Nanosecond)
			}
		}
		return nil, ttl
	}
	if leaf.Subject.CommonName == "" {
		return nil, chainCheckTTL
	}

	// The answer must not outlive any certificate of the verified chain.
	ttl := chainCheckTTL
	for _, cert := range verified[0] {
		ttl = min(ttl, cert.NotAfter.Sub(now))
	}
	return certificateUser(leaf), ttl
}

func newChainID(chain []*x509.Certificate, casVersion uint64) chainID {
	h := sha256.New()
	h.Write(binary.BigEndian.AppendUint64(nil, casVersion))
	for _, cert := range chain {
		h.Write(cert.Raw)
	}
	var id chainID
	h.Sum(id[:0])
	return id
}

// certificateUser adds system:authenticated the way the API server does: not to the anonymous user
// and not to a user that already has it or system:unauthenticated.
func certificateUser(leaf *x509.Certificate) *User {
	user := &User{Name: leaf.Subject.CommonName, Groups: slices.Clone(leaf.Subject.Organization)}
	if user.Name == anonymousUser ||
		slices.Contains(user.Groups, authenticatedGroup) ||
		slices.Contains(user.Groups, unauthenticatedGroup) {
		return user
	}
	user.Groups = append(user.Groups, authenticatedGroup)
	return user
}
