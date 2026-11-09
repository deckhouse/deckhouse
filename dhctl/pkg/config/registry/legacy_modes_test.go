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

package registry

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
)

func TestRefusePreviousImplementationModes(t *testing.T) {
	t.Run("no registry section", func(t *testing.T) {
		require.NoError(t, RefusePreviousImplementationModes(nil))
	})

	for _, mode := range []constant.ModeType{constant.ModeDirect, constant.ModeUnmanaged} {
		t.Run("accepted: "+string(mode), func(t *testing.T) {
			require.NoError(t, RefusePreviousImplementationModes(&module_config.DeckhouseSettings{Mode: mode}))
		})
	}

	for _, mode := range []constant.ModeType{constant.ModeProxy, constant.ModeLocal} {
		t.Run("refused: "+string(mode), func(t *testing.T) {
			err := RefusePreviousImplementationModes(&module_config.DeckhouseSettings{Mode: mode})
			require.Error(t, err)

			// The message has to name the mode and say what to write instead: the operator reads it
			// before anything is created, and it is the only place they learn about the registry module.
			require.Contains(t, err.Error(), `registry.mode "`+string(mode)+`"`)
			require.Contains(t, err.Error(), "previous registry implementation")
			require.Contains(t, err.Error(), "mode: Managed with primary.upstream")
			require.Contains(t, err.Error(), "--img-bundle-path")
		})
	}
}

// Commander's check, converge and destroy parse an existing cluster's resources: the refusal must
// tell the user to bring those resources in line with the cluster, not how to install from scratch.
func TestExistingClusterRefusal(t *testing.T) {
	t.Run("other errors unchanged", func(t *testing.T) {
		err := errors.New("unrelated")
		require.Same(t, err, ExistingClusterRefusal(err))
		require.NoError(t, ExistingClusterRefusal(nil))
	})

	for _, mode := range []constant.ModeType{constant.ModeProxy, constant.ModeLocal} {
		t.Run("reworded: "+string(mode), func(t *testing.T) {
			refusal := RefusePreviousImplementationModes(&module_config.DeckhouseSettings{Mode: mode})
			err := ExistingClusterRefusal(fmt.Errorf("unable to initialize registry config: %w", refusal))
			require.Error(t, err)

			require.Contains(t, err.Error(), `registry.mode "`+string(mode)+`"`)
			require.Contains(t, err.Error(), "previous registry implementation")
			require.Contains(t, err.Error(), "this cluster's resources")
			require.Contains(t, err.Error(), "d8 k get mc deckhouse -o jsonpath='{.spec.settings.registry}'")
			require.Contains(t, err.Error(), "d8 k get mc registry -o jsonpath='{.spec}'")
			require.NotContains(t, err.Error(), "-o yaml", "metadata and status of the cluster's objects do not belong in the resources")
			// A switched cluster can still store the mode in its own deckhouse ModuleConfig: copying
			// that back would be refused again, so the advice has to cover it.
			require.Contains(t, err.Error(), "remove settings.registry, also when the cluster's own still names Proxy or Local")
			require.NotContains(t, err.Error(), "--img-bundle-path", "installation advice does not fit an existing cluster")
		})
	}
}

// The same refusal over one deckhouse ModuleConfig as a document, for the callers that check every
// copy of it: YAML from the configuration, JSON from a parsed object.
func TestRefusePreviousImplementationModesInModuleConfig(t *testing.T) {
	const yamlLocal = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    registry:
      mode: Local
`
	err := RefusePreviousImplementationModesInModuleConfig([]byte(yamlLocal))
	var refusal *PreviousImplementationModeError
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, constant.ModeLocal, refusal.Mode)

	jsonProxy := `{"kind":"ModuleConfig","metadata":{"name":"deckhouse"},"spec":{"settings":{"registry":{"mode":"Proxy"}}}}`
	require.ErrorAs(t, RefusePreviousImplementationModesInModuleConfig([]byte(jsonProxy)), &refusal)
	require.Equal(t, constant.ModeProxy, refusal.Mode)

	require.NoError(t, RefusePreviousImplementationModesInModuleConfig(
		[]byte(`{"kind":"ModuleConfig","metadata":{"name":"deckhouse"},"spec":{"settings":{"bundle":"Default"}}}`)))
	require.NoError(t, RefusePreviousImplementationModesInModuleConfig(
		[]byte(`{"spec":{"settings":{"registry":{"mode":"Direct"}}}}`)))

	require.Error(t, RefusePreviousImplementationModesInModuleConfig([]byte(`{"spec":{"settings":{"registry":"Proxy"}}}`)),
		"a registry section that cannot be read is an error, not a pass")
}
