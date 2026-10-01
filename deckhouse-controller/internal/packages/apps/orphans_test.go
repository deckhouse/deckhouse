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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/orphans"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

const orphanValues = `
type: object
properties:
  orphanResources:
    type: array
    default:
      - kind: Job
        version: v1
      - kind: CronJob
        version: v1
      - kind: Pod
        version: v1
      - kind: Secret
        version: v1
    items:
      type: object
      properties:
        kind:
          type: string
        version:
          type: string
`

func newValuesApp(t *testing.T, static addonutils.Values, valuesSchema string) *Application {
	t.Helper()

	store, err := values.NewStorage("test", static, nil, []byte(valuesSchema))
	require.NoError(t, err)

	return &Application{
		name:      "tenant.app",
		instance:  "app",
		namespace: "tenant",
		values:    store,
	}
}

func TestGetOrphanResources(t *testing.T) {
	t.Run("reads the kinds the values schema defaults", func(t *testing.T) {
		resources, err := newValuesApp(t, nil, orphanValues).GetOrphanResources()
		require.NoError(t, err)

		assert.Equal(t, []orphans.Resource{
			{Kind: "Job", Version: "v1"},
			{Kind: "CronJob", Version: "v1"},
			{Kind: "Pod", Version: "v1"},
			{Kind: "Secret", Version: "v1"},
		}, resources)
	})

	t.Run("declares none without the field", func(t *testing.T) {
		resources, err := newValuesApp(t, nil, "type: object\n").GetOrphanResources()
		require.NoError(t, err)
		assert.Empty(t, resources)
	})

	t.Run("rejects a declaration that is not a list of kinds", func(t *testing.T) {
		_, err := newValuesApp(t, addonutils.Values{"orphanResources": "Job"}, "").GetOrphanResources()
		require.Error(t, err)
	})
}
