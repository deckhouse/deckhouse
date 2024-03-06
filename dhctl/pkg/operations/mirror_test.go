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

package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/mirror"
)

func TestMirrorE2E_Insecure(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "mirror_e2e")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})
	workingDir := filepath.Join(tmpDir, "pull")

	sourceHost, sourceRepoPath, sourceBlobHandler := setupEmptyRegistryRepo(false)
	targetHost, targetRepoPath, targetBlobHandler := setupEmptyRegistryRepo(false)

	createDeckhouseControllersAndInstallersInRegistry(t, sourceHost+sourceRepoPath)
	createDeckhouseReleaseChannelsInRegistry(t, sourceHost+sourceRepoPath)

	pullCtx := &mirror.Context{
		Insecure:              true,
		DeckhouseRegistryRepo: sourceHost + sourceRepoPath,
		UnpackedImagesPath:    workingDir,
		ValidationMode:        mirror.NoValidation,
	}
	pushCtx := &mirror.Context{
		Insecure:              true,
		DeckhouseRegistryRepo: sourceHost + sourceRepoPath,
		RegistryHost:          targetHost,
		RegistryPath:          targetRepoPath,
		UnpackedImagesPath:    workingDir,
		ValidationMode:        mirror.NoValidation,
	}

	err = MirrorDeckhouseToLocalFS(
		pullCtx,
		[]semver.Version{
			*semver.MustParse("v1.56.5"),
			*semver.MustParse("v1.55.7"),
		},
	)
	require.NoError(t, err, "Pull should be completed without errors")
	for _, layoutName := range []string{"", "install", "release-channel"} {
		require.DirExists(t, filepath.Join(pullCtx.UnpackedImagesPath, layoutName), "Image layouts should exist after pull")
		require.DirExists(t, filepath.Join(pullCtx.UnpackedImagesPath, layoutName, "blobs"), "Blobs should exist after pull")
	}

	err = PushDeckhouseToRegistry(pushCtx)
	require.NoError(t, err, "Push should be completed without errors")

	require.Subset(t, sourceBlobHandler.ListBlobs(), targetBlobHandler.ListBlobs())
}

func setupEmptyRegistryRepo(useTLS bool) (host, repoPath string, blobHandler *ListableBlobHandler) {
	memBlobHandler := registry.NewInMemoryBlobHandler()
	bh := &ListableBlobHandler{
		BlobHandler:    memBlobHandler,
		BlobPutHandler: memBlobHandler.(registry.BlobPutHandler),
	}
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

	return host, repoPath, bh
}

func createDeckhouseReleaseChannelsInRegistry(t *testing.T, repo string) {
	createDeckhouseReleaseChannelImageInRegistry(t, repo+"/release-channel", "alpha", "v1.56.5")
	createDeckhouseReleaseChannelImageInRegistry(t, repo+"/release-channel", "beta", "v1.56.5")
	createDeckhouseReleaseChannelImageInRegistry(t, repo+"/release-channel", "early-access", "v1.55.7")
	createDeckhouseReleaseChannelImageInRegistry(t, repo+"/release-channel", "stable", "v1.55.7")
	createDeckhouseReleaseChannelImageInRegistry(t, repo+"/release-channel", "rock-solid", "v1.55.7")
	createDeckhouseReleaseChannelImageInRegistry(t, repo+"/release-channel", "v1.55.7", "v1.55.7")
	createDeckhouseReleaseChannelImageInRegistry(t, repo+"/release-channel", "v1.56.5", "v1.56.5")
}

func createDeckhouseControllersAndInstallersInRegistry(t *testing.T, repo string) {
	t.Helper()

	nameOpts, remoteOpts := mirror.MakeRemoteRegistryRequestOptions(nil, true, false)

	createRandomImageInRegistry(t, repo+":alpha")
	createRandomImageInRegistry(t, repo+":beta")
	createRandomImageInRegistry(t, repo+":early-access")
	createRandomImageInRegistry(t, repo+":stable")
	createRandomImageInRegistry(t, repo+":rock-solid")
	createRandomImageInRegistry(t, repo+":v1.56.5")
	createRandomImageInRegistry(t, repo+":v1.55.7")

	installers := map[string]v1.Image{
		"v1.56.5": createSyntheticInstallerImage(t, "v1.56.5", repo),
		"v1.55.7": createSyntheticInstallerImage(t, "v1.55.7", repo),
	}
	installers["alpha"] = installers["v1.56.5"]
	installers["beta"] = installers["v1.56.5"]
	installers["early-access"] = installers["v1.55.7"]
	installers["stable"] = installers["v1.55.7"]
	installers["rock-solid"] = installers["v1.55.7"]

	for shortTag, installer := range installers {
		ref, err := name.ParseReference(repo+"/install:"+shortTag, nameOpts...)
		require.NoError(t, err)

		err = remote.Write(ref, installer, remoteOpts...)
		require.NoError(t, err)
	}
}

func createSyntheticInstallerImage(t *testing.T, version, repo string) v1.Image {
	t.Helper()

	// FROM scratch
	base := empty.Image
	layers := make([]v1.Layer, 0)

	// COPY ./version /deckhouse/version
	// COPY ./images_digests.json /deckhouse/candi/images_digests.json
	imagesDigests, err := json.Marshal(
		map[string]map[string]string{
			"common": {
				"alpine": createRandomImageInRegistry(t, repo+":alpine"+version),
			},
			"nodeManager": {
				"bashibleApiserver": createRandomImageInRegistry(t, repo+":bashibleApiserver"+version),
			},
		})
	require.NoError(t, err)
	l, err := crane.Layer(map[string][]byte{
		"deckhouse/version":                   []byte(version),
		"deckhouse/candi/images_digests.json": imagesDigests,
	})
	require.NoError(t, err)
	layers = append(layers, l)

	img, err := mutate.AppendLayers(base, layers...)
	require.NoError(t, err)

	// ENTRYPOINT ["/bin/bash"]
	img, err = mutate.Config(img, v1.Config{
		Entrypoint: []string{"/bin/bash"},
	})
	require.NoError(t, err)

	return img
}

func createRandomImageInRegistry(t *testing.T, tag string) (digest string) {
	t.Helper()

	img, err := random.Image(int64(rand.Intn(1024)+1), int64(rand.Intn(5)+1))
	require.NoError(t, err)

	nameOpts, remoteOpts := mirror.MakeRemoteRegistryRequestOptions(nil, true, false)
	ref, err := name.ParseReference(tag, nameOpts...)
	require.NoError(t, err)

	err = remote.Write(ref, img, remoteOpts...)
	require.NoError(t, err)

	digestHash, err := img.Digest()
	require.NoError(t, err)

	return digestHash.String()
}

func createDeckhouseReleaseChannelImageInRegistry(t *testing.T, repo, tag, version string) (digest string) {
	t.Helper()

	// FROM scratch
	base := empty.Image
	layers := make([]v1.Layer, 0)

	// COPY ./version.json /version.json
	l, err := crane.Layer(map[string][]byte{
		"version.json": []byte(fmt.Sprintf(`{"version": %q}`, version)),
	})
	require.NoError(t, err)
	layers = append(layers, l)

	img, err := mutate.AppendLayers(base, layers...)
	require.NoError(t, err)

	nameOpts, remoteOpts := mirror.MakeRemoteRegistryRequestOptions(nil, true, false)
	ref, err := name.ParseReference(repo+":"+tag, nameOpts...)
	require.NoError(t, err)

	err = remote.Write(ref, img, remoteOpts...)
	require.NoError(t, err)

	digestHash, err := img.Digest()
	require.NoError(t, err)

	return digestHash.String()
}

type ListableBlobHandler struct {
	registry.BlobHandler
	registry.BlobPutHandler

	mu            sync.Mutex
	ingestedBlobs []string
}

func (h *ListableBlobHandler) Get(ctx context.Context, repo string, hash v1.Hash) (io.ReadCloser, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ingestedBlobs = append(h.ingestedBlobs, hash.String())

	return h.BlobHandler.Get(ctx, repo, hash)
}

func (h *ListableBlobHandler) ListBlobs() []string {
	return h.ingestedBlobs
}
