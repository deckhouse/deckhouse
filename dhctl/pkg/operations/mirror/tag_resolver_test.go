// Copyright 2024 Flant JSC
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

package mirror

import (
	"io"
	"log"
	"maps"
	"math/rand"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
)

func TestTagsResolver_GetTagDigest_HappyPath(t *testing.T) {
	const imageReference = "registry.deckhouse.io/deckhouse/ee/install:stable"

	want := v1.Hash{
		Algorithm: "sha256",
		Hex:       "77af4d6b9913e693e8d0b4b294fa62ade6054e6b2f1ffb617ac955dd63fb0182",
	}
	resolver := &TagsResolver{tagsDigestsMapping: map[string]v1.Hash{
		imageReference: want,
	}}

	got := resolver.GetTagDigest(imageReference)
	require.Equal(t, want, *got)
}

func TestTagsResolver_GetTagDigest_UnknownTag(t *testing.T) {
	const imageReference = "registry.deckhouse.io/deckhouse/ee/install:stable"
	resolver := &TagsResolver{tagsDigestsMapping: map[string]v1.Hash{}}

	got := resolver.GetTagDigest(imageReference)
	require.Nil(t, got)
}

func TestTagsResolver_ResolveTagsDigestsFromImageSet(t *testing.T) {
	registryHost, registryRepoPath := setupEmptyRegistryRepo(false)

	taggedImages := map[string]struct{}{
		registryHost + registryRepoPath + ":alpha": {},
		registryHost + registryRepoPath + ":beta":  {},
	}

	untaggedImages := map[string]struct{}{
		registryHost + registryRepoPath + "@sha256:77af4d6b9913e693e8d0b4b294fa62ade6054e6b2f1ffb617ac955dd63fb0182": {},
		registryHost + registryRepoPath + "@sha256:09ea463141cd441da365200b2933cf9863141cd441da365200b2933cf98b2f1f": {},
	}

	digests := map[string]string{}
	imageSet := map[string]struct{}{}
	maps.Copy(imageSet, taggedImages)
	maps.Copy(imageSet, untaggedImages)

	for imageRef := range imageSet {
		digests[imageRef] = createRandomImageInRegistry(t, imageRef)
	}

	r := NewTagsResolver()
	err := r.ResolveTagsDigestsFromImageSet(imageSet, nil, true, false)
	require.NoError(t, err)

	for imageRef := range taggedImages {
		digest := r.GetTagDigest(imageRef)
		require.NotNil(t, digest)
		require.Equal(t, digests[imageRef], digest.String())
	}
}

func setupEmptyRegistryRepo(useTLS bool) (host, repoPath string) {
	bh := registry.NewInMemoryBlobHandler()
	registryHandler := registry.New(registry.WithBlobHandler(bh), registry.Logger(log.New(io.Discard, "", 0)))

	server := httptest.NewUnstartedServer(registryHandler)
	if useTLS {
		server.StartTLS()
	} else {
		server.Start()
	}

	host = strings.TrimPrefix(server.URL, "http://")
	repoPath = "/deckhouse/ee"
	if useTLS {
		host = strings.TrimPrefix(server.URL, "https://")
	}

	return host, repoPath
}

func createRandomImageInRegistry(t *testing.T, imageRef string) (digest string) {
	t.Helper()

	img, err := random.Image(int64(rand.Intn(1024)+1), int64(rand.Intn(5)+1))
	require.NoError(t, err)

	nameOpts, remoteOpts := MakeRemoteRegistryRequestOptions(nil, true, false)
	ref, err := name.ParseReference(imageRef, nameOpts...)
	require.NoError(t, err)

	err = remote.Write(ref, img, remoteOpts...)
	require.NoError(t, err)

	digestHash, err := img.Digest()
	require.NoError(t, err)

	return digestHash.String()
}
