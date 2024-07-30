// Copyright 2023 Flant JSC
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
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
)

func TestPullTrivyVulnerabilityDatabaseImageSuccessSkipTLS(t *testing.T) {
	blobHandler := registry.NewInMemoryBlobHandler()
	registryHandler := registry.New(registry.WithBlobHandler(blobHandler))
	server := httptest.NewTLSServer(registryHandler)
	nameOpts, remoteOpts := MakeRemoteRegistryRequestOptions(authn.Anonymous, false, true)
	testLayout := prepareEmptyOCILayout(t)

	deckhouseRepo := strings.TrimPrefix(server.URL, "https://") + "/deckhouse/ee"
	trivyDBImageTag := deckhouseRepo + "/security/trivy-db:2"

	ref, err := name.ParseReference(trivyDBImageTag, nameOpts...)
	require.NoError(t, err)
	wantImage, err := random.Image(256, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, wantImage, remoteOpts...))

	err = PullTrivyVulnerabilityDatabaseImageToLayout(deckhouseRepo, authn.Anonymous, testLayout, false, true)
	require.NoError(t, err)

	wantDigest, err := wantImage.Digest()
	require.NoError(t, err)

	gotImage, err := testLayout.Image(wantDigest)
	require.NoError(t, err)

	gotDigest, err := gotImage.Digest()
	require.NoError(t, err)
	require.Equal(t, wantDigest, gotDigest)
}

func TestPullTrivyVulnerabilityDatabaseImageSuccessInsecure(t *testing.T) {
	blobHandler := registry.NewInMemoryBlobHandler()
	registryHandler := registry.New(registry.WithBlobHandler(blobHandler))
	server := httptest.NewServer(registryHandler)
	nameOpts, remoteOpts := MakeRemoteRegistryRequestOptions(authn.Anonymous, true, false)
	testLayout := prepareEmptyOCILayout(t)

	deckhouseRepo := strings.TrimPrefix(server.URL, "http://") + "/deckhouse/ee"
	trivyDBImageTag := deckhouseRepo + "/security/trivy-db:2"

	ref, err := name.ParseReference(trivyDBImageTag, nameOpts...)
	require.NoError(t, err)
	wantImage, err := random.Image(256, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, wantImage, remoteOpts...))

	err = PullTrivyVulnerabilityDatabaseImageToLayout(deckhouseRepo, authn.Anonymous, testLayout, true, false)
	require.NoError(t, err)

	wantDigest, err := wantImage.Digest()
	require.NoError(t, err)

	gotImage, err := testLayout.Image(wantDigest)
	require.NoError(t, err)

	gotDigest, err := gotImage.Digest()
	require.NoError(t, err)
	require.Equal(t, wantDigest, gotDigest)
}

func prepareEmptyOCILayout(t *testing.T) layout.Path {
	t.Helper()
	p, err := os.MkdirTemp(os.TempDir(), "trivy_pull_test")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(p)
	})

	l, err := CreateEmptyImageLayoutAtPath(p)
	require.NoError(t, err)
	return l
}
