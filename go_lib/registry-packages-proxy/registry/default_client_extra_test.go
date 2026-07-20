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

package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	crregistry "github.com/google/go-containerregistry/pkg/registry"
	v1remote "github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLogger struct{}

func (testLogger) Errorf(string, ...interface{}) {}
func (testLogger) Infof(string, ...interface{})  {}
func (testLogger) Warnf(string, ...interface{})  {}
func (testLogger) Debugf(string, ...interface{}) {}
func (testLogger) Error(string, ...interface{})  {}

func newTestRegistry(t *testing.T) (host string) {
	t.Helper()
	srv := httptest.NewServer(crregistry.New())
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

func pushRandomImage(t *testing.T, host, repo, tag string) string {
	t.Helper()
	img, err := random.Image(256, 1)
	require.NoError(t, err)
	ref, err := name.NewTag(host+"/"+repo+":"+tag, name.WeakValidation)
	require.NoError(t, err)
	require.NoError(t, v1remote.Write(ref, img))
	d, err := img.Digest()
	require.NoError(t, err)
	return d.String()
}

func TestDefaultClient_ResolveTag(t *testing.T) {
	host := newTestRegistry(t)
	wantDigest := pushRandomImage(t, host, "deckhouse/deckhouse-cli", "v1.0.1")

	c := &DefaultClient{}
	cfg := &ClientConfig{
		Repository: host + "/deckhouse",
		Scheme:     "http",
	}

	got, err := c.ResolveTag(context.Background(), testLogger{}, cfg, "deckhouse-cli", "v1.0.1", nil)
	require.NoError(t, err)
	assert.Equal(t, wantDigest, got)

	_, err = c.ResolveTag(context.Background(), testLogger{}, cfg, "deckhouse-cli", "missing-tag", nil)
	require.ErrorIs(t, err, ErrPackageNotFound)
}

func TestDefaultClient_ResolveTag_Platform(t *testing.T) {
	host := newTestRegistry(t)

	amd64, err := random.Image(256, 1)
	require.NoError(t, err)
	arm64, err := random.Image(256, 1)
	require.NoError(t, err)

	idx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: amd64, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "amd64"}}},
		mutate.IndexAddendum{Add: arm64, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}}},
	)

	ref, err := name.NewTag(host+"/deckhouse/deckhouse-cli:v2.0.0", name.WeakValidation)
	require.NoError(t, err)
	require.NoError(t, v1remote.WriteIndex(ref, idx))

	indexDigest, err := idx.Digest()
	require.NoError(t, err)
	amd64Digest, err := amd64.Digest()
	require.NoError(t, err)
	arm64Digest, err := arm64.Digest()
	require.NoError(t, err)
	require.NotEqual(t, amd64Digest.String(), arm64Digest.String())

	c := &DefaultClient{}
	cfg := &ClientConfig{Repository: host + "/deckhouse", Scheme: "http"}

	// A requested platform resolves to that child manifest, not the index.
	got, err := c.ResolveTag(context.Background(), testLogger{}, cfg, "deckhouse-cli", "v2.0.0", &v1.Platform{OS: "linux", Architecture: "arm64"})
	require.NoError(t, err)
	assert.Equal(t, arm64Digest.String(), got)

	// Different platforms resolve to different child digests (so the cache can never collide).
	got, err = c.ResolveTag(context.Background(), testLogger{}, cfg, "deckhouse-cli", "v2.0.0", &v1.Platform{OS: "linux", Architecture: "amd64"})
	require.NoError(t, err)
	assert.Equal(t, amd64Digest.String(), got)

	// No platform requested -> the index digest itself (legacy behavior).
	got, err = c.ResolveTag(context.Background(), testLogger{}, cfg, "deckhouse-cli", "v2.0.0", nil)
	require.NoError(t, err)
	assert.Equal(t, indexDigest.String(), got)

	// A platform absent from the index is a clean not-found.
	_, err = c.ResolveTag(context.Background(), testLogger{}, cfg, "deckhouse-cli", "v2.0.0", &v1.Platform{OS: "windows", Architecture: "amd64"})
	require.ErrorIs(t, err, ErrPackageNotFound)
}

func tarLayerWithFile(t *testing.T, fileName, content string) v1.Layer {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: fileName,
		Mode: 0o644,
		Size: int64(len(content)),
	}))
	_, err := tw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	layer, err := tarball.LayerFromReader(&buf)
	require.NoError(t, err)
	return layer
}

func pushImage(t *testing.T, host string, img v1.Image, repo, tag string) string {
	t.Helper()
	ref, err := name.NewTag(host+"/"+repo+":"+tag, name.WeakValidation)
	require.NoError(t, err)
	require.NoError(t, v1remote.Write(ref, img))
	d, err := img.Digest()
	require.NoError(t, err)
	return d.String()
}

func readGzipTarFile(t *testing.T, reader io.Reader, fileName string) string {
	t.Helper()

	gzipReader, err := gzip.NewReader(reader)
	require.NoError(t, err)
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			t.Fatalf("file %q not found in archive", fileName)
		}
		require.NoError(t, err)
		if header.Name != fileName {
			continue
		}
		data, err := io.ReadAll(tarReader)
		require.NoError(t, err)
		return string(data)
	}
}

// TestDefaultClient_GetPackage_lastLayerOnly_byDefault pins down the
// historical contract: without FlattenLayers, callers see only the bytes of
// the LAST layer. The icon (in the earlier layer) must NOT be visible.
func TestDefaultClient_GetPackage_lastLayerOnly_byDefault(t *testing.T) {
	host := newTestRegistry(t)

	iconLayer := tarLayerWithFile(t, "docs/icon.svg", "<svg>icon</svg>")
	topLayer := tarLayerWithFile(t, "meta/version", "v1.0.0")
	img, err := mutate.AppendLayers(empty.Image, iconLayer, topLayer)
	require.NoError(t, err)

	digest := pushImage(t, host, img, "deckhouse/test-package", "v1.0.0")

	c := &DefaultClient{}
	cfg := &ClientConfig{
		Repository: host + "/deckhouse",
		Scheme:     "http",
	}

	_, _, reader, err := c.GetPackage(context.Background(), testLogger{}, cfg, digest, "test-package")
	require.NoError(t, err)
	defer reader.Close()

	assert.Equal(t, "v1.0.0", readGzipTarFile(t, reader, "meta/version"))
}

// TestDefaultClient_GetPackage_returnsFlattenedLayers exercises the opt-in
// flatten path used by the icon handler: every file from every layer must be
// reachable in the returned tar.
func TestDefaultClient_GetPackage_returnsFlattenedLayers(t *testing.T) {
	host := newTestRegistry(t)

	iconLayer := tarLayerWithFile(t, "docs/icon.svg", "<svg>icon</svg>")
	topLayer := tarLayerWithFile(t, "meta/version", "v1.0.0")
	img, err := mutate.AppendLayers(empty.Image, iconLayer, topLayer)
	require.NoError(t, err)

	digest := pushImage(t, host, img, "deckhouse/test-package", "v1.0.0")

	c := &DefaultClient{}
	cfg := &ClientConfig{
		Repository:    host + "/deckhouse",
		Scheme:        "http",
		FlattenLayers: true,
	}

	_, _, reader, err := c.GetPackage(context.Background(), testLogger{}, cfg, digest, "test-package")
	require.NoError(t, err)
	defer reader.Close()

	assert.Equal(t, "<svg>icon</svg>", readGzipTarFile(t, reader, "docs/icon.svg"))
}

func TestDefaultClient_ListTags(t *testing.T) {
	host := newTestRegistry(t)
	pushRandomImage(t, host, "deckhouse/deckhouse-cli", "v1.0.0")
	pushRandomImage(t, host, "deckhouse/deckhouse-cli", "v1.0.1")
	pushRandomImage(t, host, "deckhouse/deckhouse-cli", "v1.1.0")

	c := &DefaultClient{}
	cfg := &ClientConfig{
		Repository: host + "/deckhouse",
		Scheme:     "http",
	}

	tags, err := c.ListTags(context.Background(), testLogger{}, cfg, "deckhouse-cli")
	require.NoError(t, err)
	sort.Strings(tags)
	assert.Equal(t, []string{"v1.0.0", "v1.0.1", "v1.1.0"}, tags)

	_, err = c.ListTags(context.Background(), testLogger{}, cfg, "unknown-image")
	require.ErrorIs(t, err, ErrPackageNotFound)
}

func TestDefaultClient_GetRawManifest(t *testing.T) {
	host := newTestRegistry(t)

	c := &DefaultClient{}
	cfg := &ClientConfig{Repository: host + "/deckhouse", Scheme: "http"}

	// Single image: the raw manifest is returned verbatim, annotations and all, with
	// no layers pulled.
	img, err := random.Image(256, 1)
	require.NoError(t, err)
	img, ok := mutate.Annotations(img, map[string]string{"contract": "Zm9v"}).(v1.Image)
	require.True(t, ok)
	pushImage(t, host, img, "deckhouse/deckhouse-cli/plugins/single", "v1.0.0")

	raw, mediaType, err := c.GetRawManifest(context.Background(), testLogger{}, cfg, "deckhouse-cli/plugins/single", "v1.0.0")
	require.NoError(t, err)
	assert.NotEmpty(t, mediaType)
	assert.Contains(t, string(raw), "Zm9v")

	// Multi-platform index: the contract is a top-level index annotation. The raw
	// index manifest is returned as-is - the proxy never descends into a child.
	amd64, err := random.Image(256, 1)
	require.NoError(t, err)
	arm64, err := random.Image(256, 1)
	require.NoError(t, err)
	idx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: amd64, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "amd64"}}},
		mutate.IndexAddendum{Add: arm64, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}}},
	)
	idx, ok = mutate.Annotations(idx, map[string]string{"contract": "YmFy"}).(v1.ImageIndex)
	require.True(t, ok)
	ref, err := name.NewTag(host+"/deckhouse/deckhouse-cli/plugins/multi:v2.0.0", name.WeakValidation)
	require.NoError(t, err)
	require.NoError(t, v1remote.WriteIndex(ref, idx))

	raw, _, err = c.GetRawManifest(context.Background(), testLogger{}, cfg, "deckhouse-cli/plugins/multi", "v2.0.0")
	require.NoError(t, err)
	assert.Contains(t, string(raw), "YmFy")

	// Missing tag -> not found.
	_, _, err = c.GetRawManifest(context.Background(), testLogger{}, cfg, "deckhouse-cli/plugins/single", "v9.9.9")
	require.ErrorIs(t, err, ErrPackageNotFound)
}
