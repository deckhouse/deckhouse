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

package values

import (
	"testing"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const grantSettingsSchema = `
type: object
properties:
  storageClass:
    type: string
    x-deckhouse-grant:
      resource: storageclasses
  postgres:
    type: object
    default: {}
    properties:
      storageClass:
        type: string
        x-deckhouse-grant:
          resource: postgresclasses
`

func TestDynamicDefaults(t *testing.T) {
	t.Run("fills empty fields", func(t *testing.T) {
		s, err := NewStorage("test", nil, []byte(grantSettingsSchema), nil)
		require.NoError(t, err)

		require.NoError(t, s.SetDynamicDefaults([]DynamicDefault{
			{Path: []string{"storageClass"}, Value: "ssd"},
			{Path: []string{"postgres", "storageClass"}, Value: "fast"},
		}))

		vals := s.GetValues()
		assert.Equal(t, "ssd", vals["storageClass"])
		postgres, ok := vals["postgres"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "fast", postgres["storageClass"])
	})

	t.Run("user settings override the default", func(t *testing.T) {
		s, err := NewStorage("test", nil, []byte(grantSettingsSchema), nil)
		require.NoError(t, err)

		require.NoError(t, s.SetDynamicDefaults([]DynamicDefault{
			{Path: []string{"storageClass"}, Value: "ssd"},
		}))
		require.NoError(t, s.ApplySettings(addonutils.Values{"storageClass": "hdd"}))

		assert.Equal(t, "hdd", s.GetValues()["storageClass"])
	})

	t.Run("empty default value is skipped", func(t *testing.T) {
		s, err := NewStorage("test", nil, []byte(grantSettingsSchema), nil)
		require.NoError(t, err)

		require.NoError(t, s.SetDynamicDefaults([]DynamicDefault{
			{Path: []string{"storageClass"}, Value: ""},
		}))

		_, present := s.GetValues()["storageClass"]
		assert.False(t, present)
	})

	t.Run("static values are not overridden", func(t *testing.T) {
		static := addonutils.Values{"storageClass": "preset"}
		s, err := NewStorage("test", static, []byte(grantSettingsSchema), nil)
		require.NoError(t, err)

		require.NoError(t, s.SetDynamicDefaults([]DynamicDefault{
			{Path: []string{"storageClass"}, Value: "ssd"},
		}))

		assert.Equal(t, "preset", s.GetValues()["storageClass"])
	})
}
