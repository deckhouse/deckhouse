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

package images

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/storetest"
)

func byDigest(images []registryimagev1alpha1.IndexedImage) map[string]registryimagev1alpha1.IndexedImage {
	out := map[string]registryimagev1alpha1.IndexedImage{}
	for _, image := range images {
		out[image.Digest] = image
	}
	return out
}

func size(t *testing.T, root string, blobs ...string) int64 {
	t.Helper()
	var total int64
	for _, blob := range blobs {
		info, err := os.Stat(storetest.BlobPath(root, blob))
		require.NoError(t, err)
		total += info.Size()
	}
	return total
}

func TestTheIndexSaysWhatEachImageIsAndWhatItWeighs(t *testing.T) {
	root := storetest.New(t)

	present, presentBlobs := storetest.Image(t, root, "system/deckhouse", "shared-base", "present-top")
	incomplete, incompleteBlobs := storetest.Image(t, root, "system/deckhouse", "incomplete-top")
	storetest.Remove(t, root, incompleteBlobs[1])
	corrupted, corruptedBlobs := storetest.Image(t, root, "system/deckhouse/modules/foo", "shared-base", "corrupted-top")
	pulled, _ := storetest.Image(t, root, "system/deckhouse/pulled", "pulled-top")
	childA, _ := storetest.Image(t, root, "system/deckhouse/multi", "amd64")
	childB, _ := storetest.Image(t, root, "system/deckhouse/multi", "arm64")
	index := storetest.Index(t, root, "system/deckhouse/multi", childA, childB)
	missing := storetest.Digest([]byte("never arrived"))

	owner := registryimagev1alpha1.ImageOwner{Kind: registryimagev1alpha1.OwnerPlatform, Module: "foo", Image: "app", Versions: []string{"v1.70.1"}}
	set := Set{
		Known: true,
		Digests: map[string]struct{}{
			present: {}, incomplete: {}, corrupted: {}, index: {}, missing: {}, "missing:registry/system/deckhouse:v1.70.2": {},
		},
		Owners: map[string][]registryimagev1alpha1.ImageOwner{present: {owner}, missing: {owner}},
	}
	images, counts, err := Build(root, set, map[string]bool{corruptedBlobs[2]: true})
	require.NoError(t, err)

	all := byDigest(images)
	assert.Len(t, all, 6, "the children of an index are counted in it, not beside it; an unresolved tag has no digest to list")

	assert.Equal(t, registryimagev1alpha1.ImagePresent, all[present].State)
	assert.Equal(t, []registryimagev1alpha1.ImageOwner{owner}, all[present].Owners)
	assert.Equal(t, size(t, root, append(presentBlobs, present)...), all[present].Size)
	// The base layer is shared with another image, so removing this one does not free it.
	assert.Equal(t, all[present].Size-size(t, root, presentBlobs[1]), all[present].ReclaimableSize)
	assert.Equal(t, "application/vnd.oci.image.manifest.v1+json", all[present].MediaType)

	assert.Equal(t, registryimagev1alpha1.ImageIncomplete, all[incomplete].State)
	assert.EqualValues(t, 1, all[incomplete].MissingBlobs)

	assert.Equal(t, registryimagev1alpha1.ImageCorrupted, all[corrupted].State)
	assert.Equal(t, []string{corruptedBlobs[2]}, all[corrupted].CorruptedBlobs)

	assert.False(t, all[pulled].InSet)
	assert.Equal(t, registryimagev1alpha1.ImagePresent, all[pulled].State)

	assert.True(t, all[index].InSet)
	assert.Equal(t, "application/vnd.oci.image.index.v1+json", all[index].MediaType)

	assert.Equal(t, registryimagev1alpha1.ImageMissing, all[missing].State)
	assert.True(t, all[missing].InSet)

	assert.EqualValues(t, 2, counts.Present, "the image and the index")
	assert.EqualValues(t, 2, counts.Missing, "a digest and an unresolved tag")
	assert.EqualValues(t, 1, counts.Incomplete)
	assert.EqualValues(t, 1, counts.Corrupted)
	assert.EqualValues(t, 1, counts.OutOfSet)
}

func TestWithoutTheSetEverythingIsOutsideItAndNothingIsMissing(t *testing.T) {
	root := storetest.New(t)
	image, _ := storetest.Image(t, root, "system/deckhouse", "layer")

	images, counts, err := Build(root, Set{}, nil)
	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, image, images[0].Digest)
	assert.False(t, images[0].InSet)
	assert.EqualValues(t, 1, counts.OutOfSet)
	assert.Zero(t, counts.Missing)
}

func TestTheIndexIsServedFromMemory(t *testing.T) {
	root := storetest.New(t)
	image, _ := storetest.Image(t, root, "system/deckhouse", "layer")

	indexer := NewIndexer()
	indexer.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	indexer.Node = "master-0"
	indexer.Root = root
	indexer.Set = func(context.Context) (Set, error) {
		return Set{Known: true, Digests: map[string]struct{}{image: {}}}, nil
	}

	ask := func() registryimagev1alpha1.ReplicaIndex {
		recorder := httptest.NewRecorder()
		indexer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, registryimagev1alpha1.IndexPath, nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		var index registryimagev1alpha1.ReplicaIndex
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &index))
		return index
	}

	before := ask()
	assert.False(t, before.Ready, "nothing is said before the store has been read")
	assert.Equal(t, "master-0", before.Node)

	indexer.build(t.Context())
	// The store changes; the answer is the last reading until the next one.
	require.NoError(t, os.RemoveAll(storetest.BlobPath(root, image)))
	after := ask()
	assert.True(t, after.Ready)
	assert.True(t, after.SetKnown)
	require.Len(t, after.Images, 1)
	assert.Equal(t, registryimagev1alpha1.ImagePresent, after.Images[0].State)
	assert.NotNil(t, after.IndexedAt)
}
