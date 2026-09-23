/*
Copyright 2026 Flant JSC

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
	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks"
)

func TestIDTokenTTLRequirement(t *testing.T) {
	// The hook stores the value under this key; the check reads it under its own copy.
	require.Equal(t, hooks.IDTokenTTLValueKey, idTokenTTLValueKey)

	cases := []struct {
		name   string
		stored any // nil: nothing stored
		want   bool
	}{
		{name: "unset field", stored: "", want: true},
		{name: "default-like value", stored: "10m", want: true},
		{name: "just below the limit", stored: "5h59m", want: true},
		{name: "at the limit", stored: "6h", want: false},
		{name: "a week", stored: "168h", want: false},
		{name: "unparsable legacy value", stored: "1d", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requirements.SaveValue(idTokenTTLValueKey, tc.stored)
			ok, err := requirements.CheckRequirement(idTokenTTLRequirementKey, "6h")
			assert.Equal(t, tc.want, ok)
			if tc.want {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "lower spec.settings.idTokenTTL of ModuleConfig user-authn")
			}
		})
	}

	t.Run("invalid requirement value", func(t *testing.T) {
		requirements.SaveValue(idTokenTTLValueKey, "1h")
		ok, err := requirements.CheckRequirement(idTokenTTLRequirementKey, "six hours")
		assert.False(t, ok)
		require.Error(t, err)
	})
}
