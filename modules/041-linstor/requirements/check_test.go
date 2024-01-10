/*
Copyright 2023 Flant JSC
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

func TestDisabledLinstorRequirement(t *testing.T) {
	requirements.RemoveValue(linstorEnabled)

	t.Run("linstor is disabled", func(t *testing.T) {
		ok, err := requirements.CheckRequirement(requirementsKey, "true")
		assert.True(t, ok)
		require.NoError(t, err)
	})

	t.Run("linstor is enabled", func(t *testing.T) {
		requirements.SaveValue(linstorEnabled, "true")
		ok, err := requirements.CheckRequirement(requirementsKey, "true")
		assert.False(t, ok)
		require.Error(t, err)
	})
}
