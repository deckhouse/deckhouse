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

package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoRootFromPackage is this package's distance to the repository root; the live
// candi templates and the provider modules are read from there.
const repoRootFromPackage = "../../../../../../.."

// providerScriptGlobs find every provider's bootstrap-networks.sh.tpl where the editions
// keep them. The chart globs them out of the assembled candi/cloud-providers tree
// instead (bootstrap-templates-cm.yaml:17), which a checkout does not have.
var providerScriptGlobs = []string{
	"modules/030-cloud-provider-*/candi/bashible/bootstrap-networks.sh.tpl",
	"ee/modules/030-cloud-provider-*/candi/bashible/bootstrap-networks.sh.tpl",
	"ee/*/modules/030-cloud-provider-*/candi/bashible/bootstrap-networks.sh.tpl",
}

// The ConfigMap is now the only way a provider network script reaches a node, and only
// yandex is rendered anywhere else in these tests. Reading the live files means a
// template function the Go func map lacks fails here, not on a node with CI green.
func TestRenderLiveProviderNetworkScripts(t *testing.T) {
	scripts := liveProviderNetworkScripts(t)
	require.GreaterOrEqual(t, len(scripts), 5, "every edition's provider scripts must be found")

	lib := liveTemplate(t, "candi/bashible/lib.sh.tpl")
	prerequisites := liveTemplate(t, "candi/bashible/bootstrap/01-bootstrap-prerequisites.sh.tpl")

	for provider, script := range scripts {
		t.Run(provider, func(t *testing.T) {
			files := &Files{text: map[string]string{
				"lib.sh.tpl":                                 lib,
				"01-bootstrap-prerequisites.sh.tpl":          prerequisites,
				"bootstrap-networks-" + provider + ".sh.tpl": script,
			}}

			in := baseInput(files)
			in.NodeGroup = map[string]any{"name": "worker", "nodeType": "CloudEphemeral"}
			in.BootstrapToken = "myworker"
			in.Provider = provider

			rendered, err := RenderScript(in)
			require.NoError(t, err)

			// Without this the spec would pass on a script that was never found: the
			// prerequisites template skips the block on an empty .Files.Get and renders
			// happily (01-bootstrap-prerequisites.sh.tpl:41).
			in.Provider = ""
			withoutProvider, err := RenderScript(in)
			require.NoError(t, err)
			assert.NotEqual(t, string(withoutProvider), string(rendered),
				"the provider network script must be inlined, not skipped")
		})
	}
}

// liveProviderNetworkScripts returns the live bootstrap-networks.sh.tpl of every
// provider, keyed by the provider name the render looks it up under.
func liveProviderNetworkScripts(t *testing.T) map[string]string {
	t.Helper()

	scripts := map[string]string{}
	for _, glob := range providerScriptGlobs {
		matches, err := filepath.Glob(filepath.Join(repoRootFromPackage, glob))
		require.NoError(t, err)
		for _, match := range matches {
			module := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(match))))
			provider := strings.TrimPrefix(module, "030-cloud-provider-")
			require.NotEqual(t, module, provider, "cannot read the provider name out of %s", match)

			raw, err := os.ReadFile(match)
			require.NoError(t, err)
			scripts[provider] = string(raw)
		}
	}
	return scripts
}

func liveTemplate(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(repoRootFromPackage, path))
	require.NoError(t, err)
	return string(raw)
}
