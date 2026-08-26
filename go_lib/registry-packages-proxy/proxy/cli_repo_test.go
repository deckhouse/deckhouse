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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCLIRegistryRepository(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		// Edition repositories collapse to the shared root.
		"registry.deckhouse.io/deckhouse/ee":            "registry.deckhouse.io/deckhouse",
		"registry.deckhouse.io/deckhouse/ce":            "registry.deckhouse.io/deckhouse",
		"registry.deckhouse.io/deckhouse/be":            "registry.deckhouse.io/deckhouse",
		"registry.deckhouse.io/deckhouse/se":            "registry.deckhouse.io/deckhouse",
		"registry.deckhouse.io/deckhouse/se-plus":       "registry.deckhouse.io/deckhouse",
		"registry.deckhouse.io/deckhouse/fe/":           "registry.deckhouse.io/deckhouse",
		"registry.company.com:5000/mirror/deckhouse/ee": "registry.company.com:5000/mirror/deckhouse",
		// Repositories without an edition segment are the root already.
		"dev-registry.deckhouse.io/sys/deckhouse-oss": "dev-registry.deckhouse.io/sys/deckhouse-oss",
		"registry.company.com/deckhouse":              "registry.company.com/deckhouse",
		"registry.company.com/ee-mirror":              "registry.company.com/ee-mirror",
		"registry.company.com/deckhouse/EE":           "registry.company.com/deckhouse/EE",
		"registry.local:5000":                         "registry.local:5000",
		// CSE keeps its editionless artifacts (the installer) under
		// deckhouse/cse, so that path is a root of its own.
		"registry-cse.deckhouse.ru/deckhouse/cse": "registry-cse.deckhouse.ru/deckhouse/cse",
		// Stripping never eats the last path segment and leaves a bare host,
		// nor empties the repository outright.
		"registry.example.com/ee": "registry.example.com/ee",
		"registry.local/se":       "registry.local/se",
		"/ee":                     "/ee",
		"//ee":                    "//ee",
	}

	for in, want := range cases {
		in, want := in, want
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, cliRegistryRepository(in))
		})
	}
}
