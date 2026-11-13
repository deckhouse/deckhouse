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

package fill

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"
)

// TestTheSetSaysWhatDeclaresEachImage: the keys of images_digests.json — the module and the image —
// are what lists the cache by component, and the set is read once for both.
func TestTheSetSaysWhatDeclaresEachImage(t *testing.T) {
	source := startRegistry(t)
	source.Repository = "system/deckhouse"

	controller := pushByDigest(t, source, "system/deckhouse")
	shared := pushByDigest(t, source, "system/deckhouse")
	for _, version := range []string{"v1.70.1", "v1.71.0"} {
		pushInstaller(t, source, "system/deckhouse:"+version, map[string]any{
			platformDigestsFile: map[string]map[string]string{
				"registry":   {"controller": controller.String()},
				"deckhouse":  {"webhook": shared.String()},
				"monitoring": {"exporter": shared.String()},
			},
		})
	}
	moduleImage := pushByDigest(t, source, "system/deckhouse/modules/foo")
	pushInstaller(t, source, "system/deckhouse/modules/foo:v1.2.0", map[string]any{
		moduleDigestsFile: map[string]string{"app": moduleImage.String()},
	})

	declared, err := DeclareSet(t.Context(), source, []string{"v1.70.1", "v1.71.0"},
		[]ModuleRef{{Name: "foo", Version: "v1.2.0"}})
	require.NoError(t, err)

	assert.Equal(t, []registryimagev1alpha1.ImageOwner{{
		Kind: registryimagev1alpha1.OwnerPlatform, Module: "registry", Image: "controller",
		Versions: []string{"v1.70.1", "v1.71.0"},
	}}, declared.Owners[controller.String()], "one image of two releases is one owner with two versions")

	assert.Equal(t, []registryimagev1alpha1.ImageOwner{
		{Kind: registryimagev1alpha1.OwnerPlatform, Module: "deckhouse", Image: "webhook", Versions: []string{"v1.70.1", "v1.71.0"}},
		{Kind: registryimagev1alpha1.OwnerPlatform, Module: "monitoring", Image: "exporter", Versions: []string{"v1.70.1", "v1.71.0"}},
	}, declared.Owners[shared.String()], "a digest two modules use names both")

	assert.Equal(t, []registryimagev1alpha1.ImageOwner{{
		Kind: registryimagev1alpha1.OwnerModule, Module: "foo", Image: "app", Versions: []string{"v1.2.0"},
	}}, declared.Owners[moduleImage.String()])

	// Every digest of the set has an owner, the release images and the module package included.
	for digest := range declared.Digests {
		assert.NotEmpty(t, declared.Owners[digest], digest)
	}
	digests, err := DeclaredDigests(t.Context(), source, []string{"v1.70.1", "v1.71.0"},
		[]ModuleRef{{Name: "foo", Version: "v1.2.0"}})
	require.NoError(t, err)
	assert.Equal(t, digests, declared.Digests, "the same set, with or without its owners")
}

func TestTakeCountsAnImageWithABrokenBlobAsNotHeld(t *testing.T) {
	const (
		scope  = "system/deckhouse"
		config = "sha256:aaaa111111111111111111111111111111111111111111111111111111111111"
		layer  = "sha256:bbbb222222222222222222222222222222222222222222222222222222222222"
	)
	root := store(t)
	blob(t, root, config, []byte(`{}`))
	blob(t, root, layer, []byte("layer"))
	manifest := imageManifest(config, layer)
	blob(t, root, digestOne, manifest)
	revisionLink(t, root, scope, digestOne)
	declared := map[string]struct{}{digestOne: {}}

	survey, err := Take(root, scope, declared, nil, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 1, survey.Declared)

	survey, err = Take(root, scope, declared, nil, map[string]bool{layer: true})
	require.NoError(t, err)
	assert.Zero(t, survey.Declared, "a broken blob serves content every client refuses")

	affected, err := CorruptedOfSet(root, declared, map[string]bool{layer: true})
	require.NoError(t, err)
	assert.Equal(t, 1, affected)
	affected, err = CorruptedOfSet(root, declared, map[string]bool{digestThree: true})
	require.NoError(t, err)
	assert.Zero(t, affected, "a broken blob no image of the set names does not count")
}
