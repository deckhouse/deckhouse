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

package machineclass

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderChecksumForInstanceClass_ParityWithFullContext proves the production switch to the
// narrow context is byte-neutral: for every provider template, rendering from just
// {instanceClass, manualRolloutID} equals rendering from a full resolved-NodeGroup-shaped map
// carrying the same two fields plus everything else the resolver adds.
func TestRenderChecksumForInstanceClass_ParityWithFullContext(t *testing.T) {
	for name, path := range allChecksumTemplates {
		t.Run(name, func(t *testing.T) {
			tmpl, err := os.ReadFile(path)
			require.NoError(t, err)

			ic := everyProviderField()

			full := buildChecksumElement(everyProviderField(), "rollout-1")
			full["name"] = "worker"
			full["nodeType"] = "CloudEphemeral"
			full["engine"] = "CAPI"
			full["kubernetesVersion"] = "1.34"
			full["cri"] = map[string]interface{}{"type": "Containerd"}
			full["zones"] = []interface{}{"a", "b"}
			full["updateEpoch"] = "1746532947"
			full["serializedLabels"] = "a=b"
			full["serializedTaints"] = ""
			fromFull, err := RenderChecksum(tmpl, full, minimalCloudProvider())
			require.NoError(t, err)

			fromNarrow, err := RenderChecksumForInstanceClass(tmpl, ic, "rollout-1", minimalCloudProvider())
			require.NoError(t, err)

			assert.Equal(t, fromFull, fromNarrow,
				"narrow checksum context must reproduce the full-context bytes — a diff here renames MachineClass/MachineTemplate and rolls nodes")
		})
	}
}

// TestFuncMap_NondeterministicFunctionsRemoved: a template using a clock/random/network
// function must fail at Parse. If one of these leaked into a checksum template, the checksum
// would move on every resync and silently roll every node in the group.
func TestFuncMap_NondeterministicFunctionsRemoved(t *testing.T) {
	for _, fn := range []string{"now", "ago", "randAlphaNum", "uuidv4", "getHostByName", "randBytes", "genPrivateKey", "bcrypt"} {
		t.Run(fn, func(t *testing.T) {
			_, err := RenderChecksum([]byte("{{ "+fn+" }}"), buildChecksumElement(map[string]interface{}{}, ""), nil)
			require.Error(t, err, "template using %s must fail to parse", fn)
			assert.Contains(t, err.Error(), "not defined")
		})
	}
	// Deterministic sprig stays available.
	out, err := RenderChecksum([]byte(`{{ printf "%s" "x" | sha256sum }}`), buildChecksumElement(map[string]interface{}{}, ""), nil)
	require.NoError(t, err)
	assert.Len(t, out, 64)
}
