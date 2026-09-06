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

package cr

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/gojuno/minimock/v3"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/require"
)

func imageWith(t *testing.T, files map[string]string) crv1.Image {
	t.Helper()

	buf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(buf)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	})
	require.NoError(t, err)

	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	return img
}

func clientFor(t *testing.T, img crv1.Image) Client {
	t.Helper()

	return NewClientMock(minimock.NewController(t)).ImageMock.Return(img, nil)
}

func TestResolveChannel(t *testing.T) {
	// A development build ships a non-semver version - ResolveChannel must pass it through
	// untouched instead of choking on it.
	info, err := ResolveChannel(context.Background(), clientFor(t, imageWith(t, map[string]string{
		"version.json":   `{"version": "mr1"}`,
		"changelog.yaml": "feat:\n- Added new feature\n",
		"module.yaml":    "name: cloud-provider-dvp\nweight: 960\n",
	})), "mr1")
	require.NoError(t, err)
	require.Equal(t, "mr1", info.Version)
	require.NotEmpty(t, info.Digest)
	require.Equal(t, map[string]any{"feat": []any{"Added new feature"}}, info.Changelog)
	require.Contains(t, string(info.ModuleYAML), "cloud-provider-dvp")
}

func TestResolveChannelNoVersion(t *testing.T) {
	_, err := ResolveChannel(context.Background(), clientFor(t, imageWith(t, map[string]string{
		"changelog.yaml": "feat: []\n",
	})), "stable")
	require.ErrorContains(t, err, "no version found")
}

func TestImagesDigests(t *testing.T) {
	digests, err := ImagesDigests(context.Background(), clientFor(t, imageWith(t, map[string]string{
		"images_digests.json": `{"validator": "sha256:aaa", "terraformManager": "sha256:bbb"}`,
	})), "vmr1")
	require.NoError(t, err)
	require.Equal(t, "sha256:bbb", digests["terraformManager"])
}

func TestModuleImageTag(t *testing.T) {
	require.Equal(t, "v1.2.3", ModuleImageTag("1.2.3"))
	require.Equal(t, "v1.2.3", ModuleImageTag("v1.2.3"))
	require.Equal(t, "vmr1", ModuleImageTag("mr1"))
}
