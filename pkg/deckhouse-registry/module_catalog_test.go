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

package dhregistry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry/client"
	"github.com/deckhouse/deckhouse/pkg/registry/fake"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/module"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// TestNewCatalogWrapsRepo pins that NewCatalog treats its service's repository
// as the catalog itself, at whatever path — unlike New(...).Modules(), which
// scopes /modules onto an edition root.
func TestNewCatalogWrapsRepo(t *testing.T) {
	for _, repo := range []string{
		"registry.deckhouse.io/deckhouse/fe/modules",
		"registry.example.io/external-modules",
		"registry.example.io/modules-source",
	} {
		t.Run(repo, func(t *testing.T) {
			host, path := splitRepo(repo)
			cli := client.New(host).WithSegment(path...)

			catalog := module.NewCatalog(service.NewBasicService(module.CatalogServiceName, cli, log.NewNop()))

			assert.Equal(t, repo, catalog.Path())
			assert.Equal(t, repo+"/stronghold", catalog.Module("stronghold").Path())
			assert.Equal(t, repo+"/stronghold/release", catalog.Module("stronghold").Releases().Path())
		})
	}
}

// TestNewCatalogReads drives a wrapped catalog against a fake registry to
// confirm it lists modules and resolves a module's release.
func TestNewCatalogReads(t *testing.T) {
	reg := fake.NewRegistry("registry.example.io")
	scratch := fake.NewImageBuilder().MustBuild()
	reg.MustAddImage("external-modules", "stronghold", scratch)
	reg.MustAddImage("external-modules", "neuvector", scratch)
	reg.MustAddImage("external-modules/stronghold/release", "alpha",
		fake.NewImageBuilder().WithFile("version.json", `{"version":"v1.0.1"}`).MustBuild())

	cli := fake.NewClient(reg).WithSegment("external-modules")
	catalog := module.NewCatalog(service.NewBasicService(module.CatalogServiceName, cli, log.NewNop()))

	names, err := catalog.List(t.Context())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"stronghold", "neuvector"}, names)

	rel, err := catalog.Module("stronghold").Releases().Fetch(t.Context(), "alpha")
	require.NoError(t, err)

	version, err := rel.Version()
	require.NoError(t, err)
	assert.Equal(t, "v1.0.1", version)
}

// splitRepo splits "host/a/b" into host and its path segments.
func splitRepo(repo string) (string, []string) {
	host := repo
	var segs []string

	for i := 0; i < len(repo); i++ {
		if repo[i] == '/' {
			host = repo[:i]
			segs = splitSlash(repo[i+1:])

			break
		}
	}

	return host, segs
}

func splitSlash(p string) []string {
	var out []string

	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			out = append(out, p[start:i])
			start = i + 1
		}
	}

	return append(out, p[start:])
}
