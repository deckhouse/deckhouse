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
	t.Run("requirement met", func(t *testing.T) {
		requirements.SaveValue(yandexDeprecatedZoneKey, false)
		ok, err := requirements.CheckRequirement(yandexDeprecatedZoneRequirementsKey, "")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("requirement failed", func(t *testing.T) {
		requirements.SaveValue(yandexDeprecatedZoneKey, true)
		ok, err := requirements.CheckRequirement(yandexDeprecatedZoneRequirementsKey, "")
		assert.False(t, ok)
		require.Error(t, err)
	})
}
