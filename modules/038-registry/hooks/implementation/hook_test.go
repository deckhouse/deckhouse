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

package implementation

import (
	"encoding/json"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	registry_requirements "github.com/deckhouse/deckhouse/modules/038-registry/requirements"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// snapshot is a filter result the way the platform hands one over: encoded, and decoded into
// whatever the reader asks for.
type snapshot struct{ value any }

func (s snapshot) UnmarshalTo(v any) error {
	raw, err := json.Marshal(s.value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func (s snapshot) String() string { return "" }

type snapshots map[string][]sdkpkg.Snapshot

func (s snapshots) Get(key string) []sdkpkg.Snapshot { return s[key] }

// recorded runs the hook over the given snapshots and returns what it wrote for the release check.
func recorded(t *testing.T, snaps snapshots) string {
	t.Helper()

	requirements.RemoveValue(registry_requirements.ImplementationKey)
	require.NoError(t, handleImplementation(t.Context(), &go_hook.HookInput{
		Snapshots: snaps,
		Logger:    log.NewNop(),
	}))

	value, found := requirements.GetValue(registry_requirements.ImplementationKey)
	require.True(t, found, "the hook records a value on every pass, so a release always has one to check")

	return value.(string)
}

func legacyStateSnapshot(state legacyState) snapshots {
	return snapshots{legacyStateSnapName: {snapshot{value: state}}}
}

// TestHandleImplementationTellsAbsenceFromUnreadability is the half of the decision that lives
// outside `decide`: how the snapshots of this hook become its arguments.
//
// The two error paths of the helper mean opposite things. No snapshot is a cluster the previous
// implementation never took, and refusing it would strand every cluster that has never used the
// legacy modes. Anything else is a state that exists and could not be read, which is refused.
func TestHandleImplementationTellsAbsenceFromUnreadability(t *testing.T) {
	t.Run("no state object at all: nothing an upgrade could take away", func(t *testing.T) {
		assert.Equal(t, ImplementationV2, recorded(t, snapshots{}))
	})

	t.Run("a state object that could not be read: refused rather than guessed", func(t *testing.T) {
		// What `filterLegacyState` produces for a Secret it cannot parse — see the test below.
		assert.Equal(t, ImplementationLegacy, recorded(t, legacyStateSnapshot(legacyState{})))
	})

	t.Run("Unmanaged: the previous implementation has let go of the pull path", func(t *testing.T) {
		assert.Equal(t, ImplementationV2, recorded(t, legacyStateSnapshot(legacyState{Mode: "Unmanaged"})))
	})

	t.Run("Direct with nothing configured", func(t *testing.T) {
		assert.Equal(t, ImplementationLegacy, recorded(t, legacyStateSnapshot(legacyState{Mode: "Direct"})))
	})

	t.Run("Direct with the module configured", func(t *testing.T) {
		snaps := legacyStateSnapshot(legacyState{Mode: "Direct"})
		snaps[moduleConfigSnapName] = []sdkpkg.Snapshot{snapshot{value: true}}

		assert.Equal(t, ImplementationV2, recorded(t, snaps))
	})
}

// TestFilterLegacyStateNormalisesWhatItCannotRead pins the reason the hook's last branch is a
// backstop: a Secret this filter cannot parse arrives as a state with no mode and no error, so it
// is refused by the same rule as a state that records no mode.
func TestFilterLegacyStateNormalisesWhatItCannotRead(t *testing.T) {
	stateSecret := func(data map[string]any) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata":   map[string]any{"name": LegacyStateSecretName, "namespace": "d8-system"},
			"data":       data,
		}}
	}

	for _, tc := range []struct {
		name string
		data map[string]any
		want legacyState
	}{
		{
			name: "a state that reads",
			// base64 of "mode: Direct\ntarget_mode: Direct\n"
			data: map[string]any{"state": "bW9kZTogRGlyZWN0CnRhcmdldF9tb2RlOiBEaXJlY3QK"},
			want: legacyState{Mode: "Direct", TargetMode: "Direct"},
		},
		{
			name: "no state key at all",
			data: map[string]any{"other": "dmFsdWU="},
			want: legacyState{},
		},
		{
			name: "a state that is not base64",
			data: map[string]any{"state": "!! not base64 !!"},
			want: legacyState{},
		},
		{
			name: "a state that is not YAML",
			// base64 of "\tmode: [unterminated"
			data: map[string]any{"state": "CW1vZGU6IFt1bnRlcm1pbmF0ZWQ="},
			want: legacyState{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := filterLegacyState(stateSecret(tc.data))
			require.NoError(t, err, "an unreadable Secret is normalised rather than reported")
			assert.Equal(t, tc.want, result)
		})
	}
}
