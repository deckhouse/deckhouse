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

package digests_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/digests"
)

// realDeckhouseDigests is a verbatim slice of
// deckhouse/modules/images_digests.json from
// registry.deckhouse.io/deckhouse/fe:v1.76.6, whose full file holds 403 images
// across 56 modules. It pins the mapping against what the build actually
// writes: two levels, lowerCamelCase keys at both, "sha256:" digests as leaves.
const realDeckhouseDigests = `{
  "admissionPolicyEngine": {
    "constraintExporter": "sha256:f1ad7dee5361a3a39022852122f8cf89165ce4a9b30ad363d93c5550c443280c",
    "gatekeeper": "sha256:95da420fe86ac731660d8110f0b4541b7d19b578271bf830029e59b6fdf45270"
  },
  "controlPlaneManager": {
    "controlPlaneManager131": "sha256:17f7eb812ac0d8769bb303aba225018781dd9e96cbacdaeb0b1f1e94413d5547",
    "controlPlaneManager132": "sha256:d1c66417a3e1a4a8cf7ea0dafd91b1540a69ce00dca0b6b0162d19a77fbd1c2c"
  }
}`

func TestParseRealDeckhouseFile(t *testing.T) {
	got, err := digests.Parse([]byte(realDeckhouseDigests))
	require.NoError(t, err)

	require.True(t, got.IsNested())
	assert.ElementsMatch(t, []string{"admissionPolicyEngine", "controlPlaneManager"}, got.Modules())
	assert.Equal(t, 4, got.Count())

	digest, ok := got.Lookup("controlPlaneManager", "controlPlaneManager132")
	require.True(t, ok)
	assert.Equal(t, "sha256:d1c66417a3e1a4a8cf7ea0dafd91b1540a69ce00dca0b6b0162d19a77fbd1c2c", digest)

	// Module keys are lowerCamelCase, not the kebab-case the module is known by
	// in a ModuleSource or on the command line.
	_, ok = got.Lookup("control-plane-manager", "controlPlaneManager132")
	assert.False(t, ok)

	// Every leaf is a full sha256 digest.
	for module, images := range got.ByModule {
		for image, d := range images {
			assert.Len(t, d, 71, "%s/%s", module, image)
			assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, d, "%s/%s", module, image)
		}
	}
}

// realModuleDigests is a verbatim copy of images_digests.json from
// registry.deckhouse.io/deckhouse/fe/modules/stronghold:v1.0.1 — a module image
// bundles only its own images, so the file is flat and sits at the image root.
const realModuleDigests = `{
  "stronghold": "sha256:3c044762cfbf297a95adb5c57ba94929d513c523ead8ed1c614bdeac9f744ac8",
  "strongholdAutomatic": "sha256:338df2156289bc44b9b5f6ae6472a48065f8a8794d2f710f0f776cdf1c2faaf9"
}`

func TestParseRealModuleFile(t *testing.T) {
	got, err := digests.Parse([]byte(realModuleDigests))
	require.NoError(t, err)

	require.False(t, got.IsNested())
	assert.Nil(t, got.Modules())
	assert.Equal(t, 2, got.Count())

	digest, ok := got.Lookup("", "strongholdAutomatic")
	require.True(t, ok)
	assert.Equal(t, "sha256:338df2156289bc44b9b5f6ae6472a48065f8a8794d2f710f0f776cdf1c2faaf9", digest)
}
