/*
Copyright 2025 Flant JSC

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

package helpers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
)

// The documented OpenAPI default for settings.update.mode is AutoPatch
// (modules/002-deckhouse/openapi/config-values.yaml). An unset update.mode must
// resolve to AutoPatch, not Auto — otherwise minor releases auto-apply without
// approval on clusters that never set the mode explicitly.
func TestDefaultDeckhouseSettingsUpdateMode(t *testing.T) {
	require.Equal(t, v1alpha2.UpdateModeAutoPatch.String(), DefaultDeckhouseSettings().Update.Mode)
}

// syncDeckhouseSettings seeds DefaultDeckhouseSettings() and then unmarshals the
// deckhouse ModuleConfig over it. A config that does not set update.mode must
// keep the AutoPatch default rather than silently falling back to Auto.
func TestUnsetUpdateModeStaysAutoPatchAfterConfigOverlay(t *testing.T) {
	settings := DefaultDeckhouseSettings()

	require.NoError(t, json.Unmarshal([]byte(`{"releaseChannel":"Stable"}`), settings))

	require.Equal(t, v1alpha2.UpdateModeAutoPatch.String(), settings.Update.Mode)
}

// An explicitly configured update.mode must still win over the default.
func TestExplicitUpdateModeOverridesDefault(t *testing.T) {
	settings := DefaultDeckhouseSettings()

	require.NoError(t, json.Unmarshal([]byte(`{"update":{"mode":"Auto"}}`), settings))

	require.Equal(t, v1alpha2.UpdateModeAuto.String(), settings.Update.Mode)
}
