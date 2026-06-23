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

package apps

import (
	"context"
	"testing"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/grants"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values"
)

const grantSettings = `
type: object
properties:
  storageClass:
    type: string
    x-deckhouse-grant:
      resource: storageclasses
`

// stubResolver returns a fixed catalog regardless of namespace/resource.
type stubResolver struct {
	catalog grants.Catalog
}

func (s stubResolver) Resolve(_ context.Context, _, _ string) (grants.Catalog, error) {
	return s.catalog, nil
}

func newTestApp(t *testing.T, resolver grants.Resolver) *Application {
	t.Helper()

	store, err := values.NewStorage("test", nil, []byte(grantSettings), nil)
	require.NoError(t, err)

	return &Application{
		name:          "tenant.app",
		instance:      "app",
		namespace:     "tenant",
		values:        store,
		grantResolver: resolver,
	}
}

func TestResolveGrantDefaults(t *testing.T) {
	t.Run("injects project default into empty field", func(t *testing.T) {
		app := newTestApp(t, stubResolver{catalog: grants.Catalog{
			Found:     true,
			Default:   "ssd",
			Available: []string{"ssd", "hdd"},
		}})

		require.NoError(t, app.resolveGrantDefaults(context.Background()))
		require.NoError(t, app.values.ApplySettings(addonutils.Values{}))

		assert.Equal(t, "ssd", app.values.GetValues()["storageClass"])
	})

	t.Run("skips when catalog has no default", func(t *testing.T) {
		app := newTestApp(t, stubResolver{catalog: grants.Catalog{Found: true}})

		require.NoError(t, app.resolveGrantDefaults(context.Background()))
		require.NoError(t, app.values.ApplySettings(addonutils.Values{}))

		_, present := app.values.GetValues()["storageClass"]
		assert.False(t, present)
	})

	t.Run("skips when feature inactive", func(t *testing.T) {
		app := newTestApp(t, grants.NoopResolver{})

		require.NoError(t, app.resolveGrantDefaults(context.Background()))
		require.NoError(t, app.values.ApplySettings(addonutils.Values{}))

		_, present := app.values.GetValues()["storageClass"]
		assert.False(t, present)
	})
}

func TestValidateGrants(t *testing.T) {
	app := newTestApp(t, stubResolver{catalog: grants.Catalog{
		Found:     true,
		Default:   "ssd",
		Available: []string{"ssd", "hdd"},
	}})

	t.Run("allows available value", func(t *testing.T) {
		err := app.validateGrants(context.Background(), addonutils.Values{"storageClass": "hdd"})
		require.NoError(t, err)
	})

	t.Run("rejects unavailable value", func(t *testing.T) {
		err := app.validateGrants(context.Background(), addonutils.Values{"storageClass": "nvme"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not available")
	})

	t.Run("allows empty value", func(t *testing.T) {
		err := app.validateGrants(context.Background(), addonutils.Values{})
		require.NoError(t, err)
	})

	t.Run("skips validation when feature inactive", func(t *testing.T) {
		inactive := newTestApp(t, grants.NoopResolver{})
		err := inactive.validateGrants(context.Background(), addonutils.Values{"storageClass": "nvme"})
		require.NoError(t, err)
	})
}
