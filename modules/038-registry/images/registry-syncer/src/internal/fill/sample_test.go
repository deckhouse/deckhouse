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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestASampleReadsTheReleaseAndTheSetOffTheDisk: what a leader looks at to tell whether its store lost
// the set — the release's tag in the scope's own repository, and a few manifests of the set held whole.
func TestASampleReadsTheReleaseAndTheSetOffTheDisk(t *testing.T) {
	const (
		scope   = "system/deckhouse"
		release = "v1.70.1"
		config  = "sha256:aaaa111111111111111111111111111111111111111111111111111111111111"
		layer   = "sha256:bbbb222222222222222222222222222222222222222222222222222222222222"
		missing = "sha256:cccc333333333333333333333333333333333333333333333333333333333333"
		gone    = "sha256:dddd444444444444444444444444444444444444444444444444444444444444"
	)
	whole := func(t *testing.T, root string, digests ...string) {
		t.Helper()
		blob(t, root, config, []byte(`{}`))
		blob(t, root, layer, []byte("layer"))
		for _, digest := range digests {
			blob(t, root, digest, imageManifest(config, layer))
		}
	}

	t.Run("everything held", func(t *testing.T) {
		root := store(t)
		whole(t, root, digestOne, digestTwo, digestThree)
		tagLink(t, root, scope, release, digestOne)

		result, err := Sample(root, "/"+scope+"/", release, []string{digestOne, digestTwo, digestThree})
		require.NoError(t, err)
		assert.Equal(t, SampleResult{ReleaseHeld: true, Checked: 3}, result)
	})

	t.Run("a store that was wiped", func(t *testing.T) {
		root := store(t)

		result, err := Sample(root, scope, release, []string{digestOne, digestTwo})
		require.NoError(t, err)
		assert.Equal(t, SampleResult{Checked: 2, Missing: 2}, result,
			"no repositories at all: the release is not held, and nothing of the set is")
	})

	t.Run("the tag without its manifest", func(t *testing.T) {
		root := store(t)
		whole(t, root, digestTwo)
		tagLink(t, root, scope, release, gone)

		result, err := Sample(root, scope, release, nil)
		require.NoError(t, err)
		assert.False(t, result.ReleaseHeld, "a tag naming content the store does not have resolves to nothing")
	})

	t.Run("the tag in another repository", func(t *testing.T) {
		root := store(t)
		whole(t, root, digestOne)
		tagLink(t, root, scope+"/install", release, digestOne)

		result, err := Sample(root, scope, release, nil)
		require.NoError(t, err)
		assert.False(t, result.ReleaseHeld, "the release is read from the scope's own repository")
	})

	t.Run("half the sample missing", func(t *testing.T) {
		root := store(t)
		whole(t, root, digestOne, digestTwo)
		blob(t, root, digestThree, imageManifest(config, missing))
		tagLink(t, root, scope, release, digestOne)

		result, err := Sample(root, scope, release, []string{digestOne, digestTwo, digestThree, gone})
		require.NoError(t, err)
		assert.Equal(t, SampleResult{ReleaseHeld: true, Checked: 4, Missing: 2}, result,
			"a manifest without its layer is not held, nor is one without its own content")
	})

	t.Run("members the store could not resolve are not looked at", func(t *testing.T) {
		root := store(t)
		whole(t, root, digestOne)

		result, err := Sample(root, scope, "", []string{digestOne, missingPrefix + "registry/system/deckhouse/modules/foo:v1"})
		require.NoError(t, err)
		assert.Equal(t, SampleResult{ReleaseHeld: true, Checked: 1}, result,
			"no release to look for is held; an unresolved member was never in the store")
		assert.False(t, Resolved(missingPrefix+"x"))
		assert.True(t, Resolved(digestOne))
	})

	t.Run("a manifest that cannot be parsed counts as missing", func(t *testing.T) {
		root := store(t)
		blob(t, root, digestOne, []byte("not a manifest"))

		result, err := Sample(root, scope, "", []string{digestOne})
		require.NoError(t, err)
		assert.Equal(t, 1, result.Missing)
	})

	t.Run("a store that cannot be read is no evidence", func(t *testing.T) {
		root := store(t)
		// A directory where the manifest's content belongs: reading it fails, and not with ENOENT.
		algorithm, hex, _ := strings.Cut(digestOne, ":")
		require.NoError(t, os.MkdirAll(filepath.Join(root, "docker", "registry", "v2", "blobs",
			algorithm, hex[:2], hex, "data"), 0o755))

		_, err := Sample(root, scope, "", []string{digestOne})
		assert.Error(t, err)

		root = store(t)
		require.NoError(t, os.MkdirAll(filepath.Join(root, "docker", "registry", "v2", "repositories",
			scope, "_manifests", "tags", release, "current", "link"), 0o755))
		_, err = Sample(root, scope, release, nil)
		assert.Error(t, err, "nor is a tag link that cannot be read")
	})
}
