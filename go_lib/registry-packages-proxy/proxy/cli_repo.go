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

package proxy

import (
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/log"
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

// editionSegments are the per-edition segments that end a Deckhouse registry
// repository, e.g. the "ee" of registry.deckhouse.io/deckhouse/ee.
//
// The list matches Edition.IsValid in the deckhouse-cli client, which decides
// where `d8 mirror pull` reads the CLI artifacts from. Keep them in sync.
//
// "cse" is absent on purpose: the CSE registry keeps the editionless
// artifacts (the installer) under deckhouse/cse, so deckhouse/cse is a root
// of its own, not an edition sub-path.
var editionSegments = map[string]struct{}{
	"ce":      {},
	"be":      {},
	"se":      {},
	"se-plus": {},
	"ee":      {},
	"fe":      {},
}

// cliRegistryRepository returns the edition-trimmed root: where the official
// registry publishes the CLI artifacts (deckhouse-cli and
// deckhouse-cli/plugins/<name>), once for all editions, one level above the
// cluster's edition repository:
//
//	registry.deckhouse.io/deckhouse/ee  ->  registry.deckhouse.io/deckhouse
//
// A repository that does not end with an edition segment (dev registries,
// air-gapped mirrors pushed to a plain path) is returned unchanged. Mirrored
// registries may keep the artifacts under the cluster repository itself, so
// this is one probe candidate, not the only answer; see cliRepoCandidates.
func cliRegistryRepository(clusterRepository string) string {
	repo := strings.TrimRight(clusterRepository, "/")

	idx := strings.LastIndex(repo, "/")
	if idx < 0 {
		return repo
	}

	if _, isEdition := editionSegments[repo[idx+1:]]; !isEdition {
		return repo
	}

	root := repo[:idx]

	// The root keeps the host and at least one path segment. A repository
	// like "registry.example.com/ee" names a project that happens to be
	// called "ee", not an edition of a Deckhouse repository, and its CLI
	// artifacts stay where they are.
	if countPathSegments(root) < 2 {
		return repo
	}

	return root
}

// countPathSegments counts the non-empty slash-separated parts of repo, the
// host included.
func countPathSegments(repo string) int {
	count := 0

	for _, segment := range strings.Split(repo, "/") {
		if segment != "" {
			count++
		}
	}

	return count
}

// cliRepoCandidates returns the repository roots that may hold the CLI
// artifacts, in probe order:
//
//  1. the cluster repository as configured: `d8 mirror push` uploads the
//     bundle layout verbatim, so mirrored artifacts live right under it;
//  2. the edition-trimmed root: the official registry publishes the CLI
//     artifacts once for all editions, above the edition segment.
//
// For a repository without an edition suffix both candidates coincide and
// the list has a single entry.
func cliRepoCandidates(clusterRepository string) []string {
	repo := strings.TrimRight(clusterRepository, "/")

	root := cliRegistryRepository(repo)
	if root == repo {
		return []string{repo}
	}

	return []string{repo, root}
}

// cliRepoMemo remembers which candidate root served the last successful CLI
// request. It is keyed by the cluster repository: the deckhouse-registry
// secret can be rewritten at runtime (registry switch), and a key mismatch
// then discards the remembered root for free.
//
// The memo is only a probe-order hint, correctness never depends on it: a 404
// on the remembered root falls through to the other candidate within the same
// request, and the memo follows the next success.
type cliRepoMemo struct {
	mu          sync.RWMutex
	clusterRepo string
	root        string
}

// get returns the remembered root for clusterRepo, if any.
func (m *cliRepoMemo) get(clusterRepo string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.root == "" || m.clusterRepo != clusterRepo {
		return "", false
	}

	return m.root, true
}

// set remembers root as the working one for clusterRepo and reports whether
// the remembered value changed (callers log only on change).
func (m *cliRepoMemo) set(clusterRepo, root string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clusterRepo == clusterRepo && m.root == root {
		return false
	}

	m.clusterRepo = clusterRepo
	m.root = root

	return true
}

// isDefinitiveMiss reports whether err proves the probed root does not serve
// the artifacts: the registry answered 404, or refused access with 401/403 -
// registries hide foreign paths behind auth errors, and access rules change
// only with the registry configuration. Network failures and 5xx (already
// retried inside the registry client) leave the root's content unknown.
func isDefinitiveMiss(err error) bool {
	if errors.Is(err, registry.ErrPackageNotFound) {
		return true
	}

	e := &transport.Error{}
	if errors.As(err, &e) {
		return e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden
	}

	return false
}

// requestCLIArtifacts runs op against the candidate CLI artifact roots until
// one succeeds and remembers the winner in memo, so later requests try it
// first. base must be a private config copy with Repository set to the
// cluster repository; op receives a copy with Repository rewritten to the
// candidate root. On success the config that served the request is returned,
// so follow-up calls of the same request stay on the same root.
//
// Error policy:
//   - registry.ErrPackageNotFound means "not under this root": the next
//     candidate is tried;
//   - any other error falls through too, so a root hidden behind 401/403 or a
//     network failure does not mask artifacts served by the other root;
//   - when every candidate fails, the first non-NotFound error in probe order
//     is returned, or ErrPackageNotFound when all candidates answered 404.
//
// The memo is updated only when the winner is proven right: every candidate
// skipped before it must have answered a definitive miss (see
// isDefinitiveMiss). A win after a network or 5xx skip serves the request
// without being remembered, so a blip cannot pin the wrong root.
func requestCLIArtifacts[T any](
	memo *cliRepoMemo,
	logger log.Logger,
	base *registry.ClientConfig,
	op func(cfg *registry.ClientConfig) (T, error),
) (T, *registry.ClientConfig, error) {
	clusterRepo := strings.TrimRight(base.Repository, "/")
	candidates := cliRepoCandidates(clusterRepo)

	// The remembered root probes first; the list holds at most two entries.
	if remembered, ok := memo.get(clusterRepo); ok && len(candidates) == 2 && candidates[1] == remembered {
		candidates[0], candidates[1] = candidates[1], candidates[0]
	}

	var firstErr error
	unknownSkip := false

	for _, root := range candidates {
		cfg := *base
		cfg.Repository = root

		result, err := op(&cfg)
		if err == nil {
			if !unknownSkip && memo.set(clusterRepo, root) {
				logger.Infof("CLI artifacts for repository %q are served from %q", clusterRepo, root)
			}
			return result, &cfg, nil
		}

		if !isDefinitiveMiss(err) {
			unknownSkip = true
		}
		if firstErr == nil && !errors.Is(err, registry.ErrPackageNotFound) {
			firstErr = err
		}
	}

	var zero T
	if firstErr != nil {
		return zero, nil, firstErr
	}

	return zero, nil, registry.ErrPackageNotFound
}
