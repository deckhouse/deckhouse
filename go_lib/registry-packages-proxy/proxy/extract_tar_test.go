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

package proxy

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type extractTestLogger struct{}

func (extractTestLogger) Errorf(string, ...interface{}) {}
func (extractTestLogger) Infof(string, ...interface{})  {}
func (extractTestLogger) Warnf(string, ...interface{})  {}
func (extractTestLogger) Debugf(string, ...interface{}) {}
func (extractTestLogger) Error(string, ...interface{})  {}

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

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	})
	require.NoError(t, err)
	return layer
}

func flattenedPackageReader(t *testing.T, img v1.Image) io.ReadCloser {
	t.Helper()

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return mutate.Extract(img), nil
	})
	require.NoError(t, err)

	reader, err := layer.Compressed()
	require.NoError(t, err)
	return reader
}

func TestExtractTarGzFile_extractsFromIntermediateLayer(t *testing.T) {
	const iconPath = "docs/icon.svg"
	const iconContent = "<svg>icon</svg>"

	layer1 := tarLayerWithFile(t, "base/README", "base layer")
	layer2 := tarLayerWithFile(t, iconPath, iconContent)
	layer3 := tarLayerWithFile(t, "meta/version", "v1.0.0")

	img, err := mutate.AppendLayers(empty.Image, layer1, layer2, layer3)
	require.NoError(t, err)

	t.Run("flattened image", func(t *testing.T) {
		reader := flattenedPackageReader(t, img)
		defer reader.Close()

		got, err := extractTarGzFile(reader, exactNameMatcher(iconPath), maxIconBytes)
		require.NoError(t, err)
		assert.Equal(t, iconContent, string(got))
	})

	t.Run("last layer only", func(t *testing.T) {
		layers, err := img.Layers()
		require.NoError(t, err)
		require.Len(t, layers, 3)

		reader, err := layers[2].Compressed()
		require.NoError(t, err)
		defer reader.Close()

		_, err = extractTarGzFile(reader, exactNameMatcher(iconPath), maxIconBytes)
		require.ErrorIs(t, err, errFileNotFoundInArchive)
	})
}

func TestExtractTarGzFile_normalizesLeadingDotSlash(t *testing.T) {
	// Many tar producers prefix entries with "./"; the matcher should accept
	// both shapes.
	const target = "docs/icon.svg"
	layer := tarLayerWithFile(t, "./"+target, "<svg/>")
	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	reader := flattenedPackageReader(t, img)
	defer reader.Close()

	got, err := extractTarGzFile(reader, exactNameMatcher(target), maxIconBytes)
	require.NoError(t, err)
	assert.Equal(t, "<svg/>", string(got))
}

func TestExtractTarGzFile_rejectsOversizedEntry(t *testing.T) {
	const target = "docs/icon.svg"
	big := make([]byte, 16)
	layer := tarLayerWithFile(t, target, string(big))
	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	reader := flattenedPackageReader(t, img)
	defer reader.Close()

	_, err = extractTarGzFile(reader, exactNameMatcher(target), 8)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestExtractIcon_picksHighestPriorityCandidate(t *testing.T) {
	// All four formats present at once: SVG must win because it's first
	// in iconCandidates.
	layers := []v1.Layer{
		tarLayerWithFile(t, "docs/icon.png", "PNG"),
		tarLayerWithFile(t, "docs/icon.jpeg", "JPEG"),
		tarLayerWithFile(t, "docs/icon.svg", "SVG"),
		tarLayerWithFile(t, "docs/icon.jpg", "JPG"),
	}
	img, err := mutate.AppendLayers(empty.Image, layers...)
	require.NoError(t, err)

	reader := flattenedPackageReader(t, img)
	defer reader.Close()

	data, cand, err := extractIcon(reader)
	require.NoError(t, err)
	assert.Equal(t, "SVG", string(data))
	assert.Equal(t, "image/svg+xml", cand.contentType)
	assert.Equal(t, "svg", cand.ext)
}

func TestExtractIcon_fallsBackThroughRasters(t *testing.T) {
	// No SVG: PNG should win over JPG/JPEG.
	layers := []v1.Layer{
		tarLayerWithFile(t, "docs/icon.jpg", "JPG"),
		tarLayerWithFile(t, "docs/icon.jpeg", "JPEG"),
		tarLayerWithFile(t, "docs/icon.png", "PNG"),
	}
	img, err := mutate.AppendLayers(empty.Image, layers...)
	require.NoError(t, err)

	reader := flattenedPackageReader(t, img)
	defer reader.Close()

	data, cand, err := extractIcon(reader)
	require.NoError(t, err)
	assert.Equal(t, "PNG", string(data))
	assert.Equal(t, "image/png", cand.contentType)
	assert.Equal(t, "png", cand.ext)
}

func TestExtractIcon_returnsSentinelWhenAbsent(t *testing.T) {
	// Only unrelated files - the sentinel is what tells fetchIcon to
	// answer 404 instead of 502.
	layer := tarLayerWithFile(t, "README", "no icon here")
	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	reader := flattenedPackageReader(t, img)
	defer reader.Close()

	_, _, err = extractIcon(reader)
	require.ErrorIs(t, err, errFileNotFoundInArchive)
}

func TestExtractIcon_normalizesLeadingDotSlash(t *testing.T) {
	// Tar producers (e.g. ko, BuildKit) often prefix entries with "./".
	// extractIcon must treat "./docs/icon.png" as a match.
	layer := tarLayerWithFile(t, "./docs/icon.png", "PNG")
	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	reader := flattenedPackageReader(t, img)
	defer reader.Close()

	data, cand, err := extractIcon(reader)
	require.NoError(t, err)
	assert.Equal(t, "PNG", string(data))
	assert.Equal(t, "png", cand.ext)
}
