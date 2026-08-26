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
	"strings"
)

// editionSegments are the per-edition segments that end a Deckhouse registry
// repository, e.g. the "ee" of registry.deckhouse.io/deckhouse/ee.
//
// The list matches Edition.IsValid in the deckhouse-cli client, which decides
// where `d8 mirror pull` reads the CLI artifacts from and where `d8 mirror
// push` writes them. Both sides must agree, so keep them in sync.
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

// cliRegistryRepository returns the repository holding the Deckhouse CLI
// artifacts: deckhouse-cli and deckhouse-cli/plugins/<name>. They are
// published once for all editions at the registry root, one level above the
// cluster's edition repository:
//
//	registry.deckhouse.io/deckhouse/ee  ->  registry.deckhouse.io/deckhouse
//
// A repository that does not end with an edition segment (dev registries,
// air-gapped mirrors pushed to a plain path) is that root already.
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
