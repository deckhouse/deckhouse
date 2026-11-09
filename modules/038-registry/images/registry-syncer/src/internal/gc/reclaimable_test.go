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

package gc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/registry-syncer/internal/fill"
)

// writeBlob puts content where distribution keeps a blob and returns its digest.
func writeBlob(t *testing.T, dataDir string, content []byte) string {
	t.Helper()

	sum := sha256.Sum256(content)
	encoded := hex.EncodeToString(sum[:])
	dir := filepath.Join(dataDir, "docker", "registry", "v2", "blobs", "sha256", encoded[:2], encoded)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data"), content, 0o644))
	return "sha256:" + encoded
}

// writeManifest stores a manifest naming the given config and layers, and records it as a revision of
// the repository, which is what makes distribution's mark — and Reclaimable — count it as held.
func writeManifest(t *testing.T, dataDir, repository, config string, layers ...string) (string, int64) {
	t.Helper()

	type entry struct {
		Digest string `json:"digest"`
	}
	manifest := struct {
		Config entry   `json:"config"`
		Layers []entry `json:"layers"`
	}{Config: entry{Digest: config}}
	for _, layer := range layers {
		manifest.Layers = append(manifest.Layers, entry{Digest: layer})
	}
	content, err := json.Marshal(manifest)
	require.NoError(t, err)

	digest := writeBlob(t, dataDir, content)
	revisionLink(t, dataDir, repository, digest)
	return digest, int64(len(content))
}

func TestReclaimableWeighsWhatOnlyTheDoomedReference(t *testing.T) {
	dataDir := t.TempDir()

	shared := writeBlob(t, dataDir, []byte("a layer both releases share"))
	keptConfig := writeBlob(t, dataDir, []byte("the config of the release kept"))
	keptLayer := writeBlob(t, dataDir, []byte("a layer of the release kept"))
	doomedConfig := []byte("the config of the release moved past")
	doomedLayer := []byte("a layer of the release moved past, the heavy part")
	orphan := []byte("a blob an earlier failed sweep left behind")

	writeManifest(t, dataDir, "system/deckhouse", keptConfig, keptLayer, shared)
	doomedDigest, doomedSize := writeManifest(t, dataDir, "system/deckhouse",
		writeBlob(t, dataDir, doomedConfig), writeBlob(t, dataDir, doomedLayer), shared)
	writeBlob(t, dataDir, orphan)

	reclaimable, err := Reclaimable(dataDir, []fill.HeldManifest{
		{Repository: "system/deckhouse", Digest: doomedDigest},
	})
	require.NoError(t, err)

	assert.Equal(t, doomedSize+int64(len(doomedConfig)+len(doomedLayer)+len(orphan)), reclaimable,
		"the doomed manifest, its own config and layer, and the orphan; not the layer the kept release shares")
}

// A manifest another repository still holds is still referenced, whatever happens to it here — and so
// is everything it names.
func TestReclaimableKeepsAManifestHeldElsewhere(t *testing.T) {
	dataDir := t.TempDir()

	config := writeBlob(t, dataDir, []byte("config"))
	layer := writeBlob(t, dataDir, []byte("layer"))
	digest, _ := writeManifest(t, dataDir, "system/deckhouse/a", config, layer)
	revisionLink(t, dataDir, "system/deckhouse/b", digest)

	reclaimable, err := Reclaimable(dataDir, []fill.HeldManifest{
		{Repository: "system/deckhouse/a", Digest: digest},
	})
	require.NoError(t, err)
	assert.Zero(t, reclaimable)
}

// The answer that matters most: a store with nothing doomed and nothing left over frees nothing, so a
// collection started by hand would be pointless.
func TestReclaimableOfACleanStoreIsZero(t *testing.T) {
	dataDir := t.TempDir()
	writeManifest(t, dataDir, "system/deckhouse",
		writeBlob(t, dataDir, []byte("config")), writeBlob(t, dataDir, []byte("layer")))

	reclaimable, err := Reclaimable(dataDir, nil)
	require.NoError(t, err)
	assert.Zero(t, reclaimable)

	empty, err := Reclaimable(t.TempDir(), nil)
	require.NoError(t, err)
	assert.Zero(t, empty, "a store never written to")
}

// Counting the blobs of a manifest that stays as garbage would promise space a collection will not
// free, so a kept manifest that cannot be read fails the estimate.
func TestReclaimableRefusesToGuessAboutAKeptManifest(t *testing.T) {
	dataDir := t.TempDir()
	digest := writeBlob(t, dataDir, []byte("not a manifest"))
	revisionLink(t, dataDir, "system/deckhouse", digest)

	_, err := Reclaimable(dataDir, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "which stays")
}

// What the estimate weighs is what the same judgement, run dry, would delete — and only that.
func TestADryRunNamesTheManifestsItWouldDelete(t *testing.T) {
	target := startRegistry(t)

	current := pushHeld(t, target, "system/deckhouse")
	first := pushHeld(t, target, "system/deckhouse")
	second := pushHeld(t, target, "system/deckhouse")
	push(t, target, "system/deckhouse:v1.76.6")
	pushInstaller(t, target, "v1.76.6", []string{current})

	collector := newCollector(t, target, Releases{Deployed: "v1.76.6"}, &countingSweeper{})
	collector.DryRun = true
	report, err := collector.Run(context.Background())
	require.NoError(t, err)

	assert.ElementsMatch(t, []fill.HeldManifest{
		{Repository: "system/deckhouse", Digest: first},
		{Repository: "system/deckhouse", Digest: second},
	}, report.Doomed)
}
