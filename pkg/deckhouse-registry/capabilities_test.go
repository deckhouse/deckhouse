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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/registry/fake"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/bundle"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// TestTreeNodesNarrowToCapabilities pins that the services the tree hands out
// satisfy the capability interfaces, so a caller can accept exactly the subset
// it needs. The tree itself stays full-featured — narrowing happens here, at
// the boundary, by the interface the parameter is declared as.
func TestTreeNodesNarrowToCapabilities(t *testing.T) {
	reg := newFE(t)

	var (
		_ service.Reader      = reg.Modules().Module("stronghold")
		_ service.Deleter     = reg.Modules().Module("stronghold")
		_ service.ReadDeleter = reg.Modules().Module("stronghold")
		_ service.ReadWriter  = reg.Modules().Module("stronghold")

		_ service.ReadDeleter = reg.Packages().Package("elma")
		_ service.Reader      = reg.Deckhouse().Releases()
		_ service.Reader      = reg.Security().Image("trivy-db")
	)
}

// TestNarrowedCapabilityUsage shows the point of the split: a helper is handed
// only the delete capability, yet a full tree node is accepted for it.
func TestNarrowedCapabilityUsage(t *testing.T) {
	// purge cannot read or push — it is given a Deleter and nothing else.
	purge := func(ctx context.Context, d service.Deleter, tag string) error {
		return d.DeleteTag(ctx, tag)
	}

	reg := fake.NewRegistry("registry.deckhouse.io")
	reg.MustAddImage("deckhouse/fe/modules/stronghold", "v1.0.1",
		fake.NewImageBuilder().WithFile(bundle.RootPath, `{}`).MustBuild())

	module := newFakeRegistry(t, reg).Modules().Module("stronghold")

	require.NoError(t, purge(t.Context(), module, "v1.0.1"))

	ok, err := module.Exists(t.Context(), "v1.0.1")
	require.NoError(t, err)
	assert.False(t, ok)
}
