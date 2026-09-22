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
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultTrustDir is where Deckhouse stages the certificate authorities of registries the
// cluster was told to trust, one PEM file per registry.
//
// Written by a bashible step that knows nothing about which registry implementation is in
// charge — a ModuleSource with its own authority puts it there on every node — so it is
// the node's answer to "whose certificates does this cluster accept", and the agent needs
// it for the same reason the runtime used to: every registry now arrives at the agent.
const DefaultTrustDir = "/opt/deckhouse/share/ca-certificates"

// trustBundle is the staged authorities as one PEM blob, with a digest so that a refresh
// can tell an unchanged directory from a changed one without rebuilding anything.
type trustBundle struct {
	pem    []byte
	digest string
}

// loadTrustBundle concatenates every certificate file in the directory.
//
// A fixed order and nothing else in it, so an unchanged directory yields an identical
// bundle: the client cache is keyed by the digest, and a bundle that differed by file
// order would rebuild every client on every pass.
//
// An absent directory is not an error. A cluster whose module sources all use public
// authorities never has one, and that is the ordinary case.
func loadTrustBundle(directory string) ([]byte, error) {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", directory, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".crt") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	var bundle []byte
	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", filepath.Join(directory, name), err)
		}
		bundle = append(bundle, content...)
	}

	return bundle, nil
}

// RefreshTrust re-reads the staged authorities.
//
// Called from the agent's loop rather than once at startup, because the directory is a
// mount of the node's: a module source added to a running cluster puts its authority
// there, and nothing restarts a static pod for it. An agent that read the bundle once
// would refuse that registry until something else happened to restart it.
//
// A read failure leaves the previous bundle in place and says so. The alternative —
// dropping the authorities because a directory listing failed — would turn a transient
// filesystem error into every pull from those registries failing.
func (s *Server) RefreshTrust() {
	if s.TrustDir == "" {
		return
	}

	bundle, err := loadTrustBundle(s.TrustDir)
	if err != nil {
		s.Log.Warn("cannot read the certificate authorities staged on the node",
			"directory", s.TrustDir, "error", err.Error())
		return
	}

	sum := sha256.Sum256(bundle)
	digest := hex.EncodeToString(sum[:])

	if previous := s.trust.Load(); previous != nil && previous.digest == digest {
		return
	}

	s.trust.Store(&trustBundle{pem: bundle, digest: digest})

	// The clients built against the previous bundle are no longer the right answer for
	// their key, so the cache is replaced wholesale and their idle connections closed
	// with it. Rare enough to be cheap: this happens when an operator changes what the
	// cluster trusts, not on the pull path.
	if replaced := s.clients.Swap(new(clientCache)); replaced != nil {
		replaced.CloseIdle()
	}

	s.Log.Info("the certificate authorities staged on the node were reloaded",
		"directory", s.TrustDir, "bytes", len(bundle))
}
