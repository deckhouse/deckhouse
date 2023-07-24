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

package transport

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/blobinfocache/memory"
	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/util"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDestinationReference(t *testing.T) {
	ref, tmpFile := refToTempFile(t)

	dest, err := ref.NewImageDestination(context.Background(), nil)
	require.NoError(t, err)
	defer dest.Close()
	ref2 := dest.Reference()
	assert.Equal(t, tmpFile, ref2.StringWithinTransport())
}

func TestGetPutManifest(t *testing.T) {
	ref, _ := refToTempFile(t)

	man := []byte("test-manifest")
	list := []byte("test-manifest-list")
	md, err := manifest.Digest(man)
	require.NoError(t, err)

	func() {
		dest, err := ref.NewImageDestination(context.Background(), nil)
		require.NoError(t, err)
		defer dest.Close()
		err = dest.PutManifest(context.Background(), man, &md)
		assert.NoError(t, err)
		err = dest.PutManifest(context.Background(), list, nil)
		assert.NoError(t, err)
		err = dest.Commit(context.Background(), nil) // nil unparsedToplevel is invalid, we don’t currently use the value
		assert.NoError(t, err)
	}()

	src, err := ref.NewImageSource(context.Background(), nil)
	require.NoError(t, err)
	defer src.Close()
	m, mt, err := src.GetManifest(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, list, m)
	assert.Equal(t, "", mt)

	m, mt, err = src.GetManifest(context.Background(), &md)
	assert.NoError(t, err)
	assert.Equal(t, man, m)
	assert.Equal(t, "", mt)
}

func TestGetPutBlob(t *testing.T) {
	computedBlob := []byte("test-blob")
	providedBlob := []byte("provided-blob")
	providedDigest := digest.Digest("sha256:provided-test-digest")

	ref, _ := refToTempFile(t)
	cache := memory.New()

	var providedInfo, computedInfo types.BlobInfo
	func() {
		dest, err := ref.NewImageDestination(context.Background(), nil)
		require.NoError(t, err)
		defer dest.Close()
		assert.Equal(t, types.PreserveOriginal, dest.DesiredLayerCompression())
		// PutBlob with caller-provided data
		providedInfo, err = dest.PutBlob(context.Background(), bytes.NewReader(providedBlob), types.BlobInfo{Digest: providedDigest, Size: int64(len(providedBlob))}, cache, false)
		assert.NoError(t, err)
		assert.Equal(t, int64(len(providedBlob)), providedInfo.Size)
		assert.Equal(t, providedDigest, providedInfo.Digest)
		// PutBlob with unknown data
		computedInfo, err = dest.PutBlob(context.Background(), bytes.NewReader(computedBlob), types.BlobInfo{Digest: "", Size: int64(-1)}, cache, false)
		assert.NoError(t, err)
		assert.Equal(t, int64(len(computedBlob)), computedInfo.Size)
		assert.Equal(t, digest.FromBytes(computedBlob), computedInfo.Digest)
		err = dest.Commit(context.Background(), nil) // nil unparsedToplevel is invalid, we don’t currently use the value
		assert.NoError(t, err)
	}()

	src, err := ref.NewImageSource(context.Background(), nil)
	require.NoError(t, err)
	defer src.Close()
	for digest, expectedBlob := range map[digest.Digest][]byte{
		providedInfo.Digest: providedBlob,
		computedInfo.Digest: computedBlob,
	} {
		rc, size, err := src.GetBlob(context.Background(), types.BlobInfo{Digest: digest, Size: int64(len(expectedBlob))}, cache)
		assert.NoError(t, err)
		defer rc.Close()
		b, err := io.ReadAll(rc)
		assert.NoError(t, err)
		assert.Equal(t, expectedBlob, b)
		assert.Equal(t, int64(len(expectedBlob)), size)
	}
}

// readerFromFunc allows implementing Reader by any function, e.g. a closure.
type readerFromFunc func([]byte) (int, error)

func (fn readerFromFunc) Read(p []byte) (int, error) {
	return fn(p)
}

// TestPutBlobDigestFailure simulates behavior on digest verification failure.
func TestPutBlobDigestFailure(t *testing.T) {
	const digestErrorString = "Simulated digest error"
	const blobDigest = digest.Digest("sha256:test-digest")

	ref, _ := refToTempFile(t)
	fileRef, ok := ref.(fileReference)
	require.True(t, ok)
	blobPath := filepath.Join(util.TrimTarGzExt(fileRef.StringWithinTransport()), blobDigest.String())
	cache := memory.New()

	firstRead := true
	reader := readerFromFunc(func(p []byte) (int, error) {
		_, err := os.Lstat(blobPath)
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
		if firstRead {
			if len(p) > 0 {
				firstRead = false
			}
			for i := 0; i < len(p); i++ {
				p[i] = 0xAA
			}
			return len(p), nil
		}
		return 0, errors.Errorf(digestErrorString)
	})

	func() {
		dest, err := ref.NewImageDestination(context.Background(), nil)
		require.NoError(t, err)
		defer dest.Close()
		_, err = dest.PutBlob(context.Background(), reader, types.BlobInfo{Digest: blobDigest, Size: -1}, cache, false)
		assert.ErrorContains(t, err, digestErrorString)
		err = dest.Commit(context.Background(), nil) // nil unparsedToplevel is invalid, we don’t currently use the value
		assert.NoError(t, err)
	}()

	_, err := os.Lstat(blobPath)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}

func TestGetPutSignatures(t *testing.T) {
	ref, _ := refToTempFile(t)

	man := []byte("test-manifest")
	list := []byte("test-manifest-list")
	md, err := manifest.Digest(man)
	require.NoError(t, err)
	signatures := [][]byte{
		[]byte("sig1"),
		[]byte("sig2"),
	}
	listSignatures := [][]byte{
		[]byte("sig3"),
		[]byte("sig4"),
	}

	func() {
		dest, err := ref.NewImageDestination(context.Background(), nil)
		require.NoError(t, err)
		defer dest.Close()
		err = dest.SupportsSignatures(context.Background())
		assert.NoError(t, err)

		err = dest.PutManifest(context.Background(), man, &md)
		require.NoError(t, err)
		err = dest.PutManifest(context.Background(), list, nil)
		require.NoError(t, err)

		err = dest.PutSignatures(context.Background(), signatures, &md)
		assert.NoError(t, err)
		err = dest.PutSignatures(context.Background(), listSignatures, nil)
		assert.NoError(t, err)
		err = dest.Commit(context.Background(), nil) // nil unparsedToplevel is invalid, we don’t currently use the value
		assert.NoError(t, err)
	}()

	src, err := ref.NewImageSource(context.Background(), nil)
	require.NoError(t, err)
	defer src.Close()
	sigs, err := src.GetSignatures(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, listSignatures, sigs)

	sigs, err = src.GetSignatures(context.Background(), &md)
	assert.NoError(t, err)
	assert.Equal(t, signatures, sigs)
}

func TestSourceReference(t *testing.T) {
	ref, tmpFile := refToTempFile(t)

	src, err := ref.NewImageSource(context.Background(), nil)
	require.NoError(t, err)
	defer src.Close()
	ref2 := src.Reference()
	assert.Equal(t, tmpFile, ref2.StringWithinTransport())
}
