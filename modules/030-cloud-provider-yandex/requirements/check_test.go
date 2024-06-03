/*
Copyright 2022 Flant JSC

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

package requirements

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

func TestKubernetesVersionRequirement(t *testing.T) {
	t.Run("in-config requirement met", func(t *testing.T) {
		requirements.SaveValue(yandexDeprecatedZoneInConfigKey, false)
		ok, err := requirements.CheckRequirement(yandexDeprecatedZoneInConfigRequirementsKey, "")
		assert.True(t, ok)
		require.NoError(t, err)
	})
	t.Run("in-nodes requirement met", func(t *testing.T) {
		requirements.SaveValue(yandexDeprecatedZoneInNodesKey, false)
		ok, err := requirements.CheckRequirement(yandexDeprecatedZoneInNodesRequirementsKey, "")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("in-config requirement failed", func(t *testing.T) {
		requirements.SaveValue(yandexDeprecatedZoneInConfigKey, true)
		ok, err := requirements.CheckRequirement(yandexDeprecatedZoneInConfigRequirementsKey, "")
		assert.False(t, ok)
		require.Error(t, err)
	})
	t.Run("in-nodes requirement failed", func(t *testing.T) {
		requirements.SaveValue(yandexDeprecatedZoneInNodesKey, true)
		ok, err := requirements.CheckRequirement(yandexDeprecatedZoneInNodesRequirementsKey, "")
		assert.False(t, ok)
		require.Error(t, err)
	})
}
