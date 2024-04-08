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
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChunkedFileWriterHappyPath(t *testing.T) {
	workingDir := filepath.Join(os.TempDir(), "chunk_test")
	require.NoError(t, os.MkdirAll(workingDir, 0777))
	t.Cleanup(func() {
		os.RemoveAll(workingDir)
	})

	const testDatasetSize, chunkSize = 10 * 1024 * 1024, 3 * 1024 * 1024
	sourceFile := make([]byte, testDatasetSize)
	bytesGenerated, err := rand.Reader.Read(sourceFile)
	require.NoError(t, err)
	require.Equal(t, testDatasetSize, bytesGenerated)

	bytesWritten, err := io.CopyBuffer(
		newChunkWriter(chunkSize, workingDir, "d8.tar"),
		bytes.NewReader(sourceFile),
		make([]byte, 512*1024),
	)
	require.NoError(t, err)
	require.Equal(t, int64(bytesGenerated), bytesWritten)

	validateSizes(t, workingDir, testDatasetSize, chunkSize)
	compareHashes(t, sourceFile, testDatasetSize, workingDir)
}

func compareHashes(t *testing.T, sourceFile []byte, testDatasetSize int, workingDir string) {
	t.Helper()

	var wantHash, gotHash []byte

	hash := md5.New()
	bytesHashed, err := hash.Write(sourceFile)
	require.NoError(t, err)
	require.Equal(t, testDatasetSize, bytesHashed)
	wantHash = hash.Sum([]byte{})
	hash.Reset()

	catalog, err := os.ReadDir(workingDir)
	require.NoError(t, err)

	streams := make([]io.Reader, 0)
	for _, entry := range catalog {
		if !entry.Type().IsRegular() || filepath.Ext(entry.Name()) != ".chunk" {
			continue
		}

		chunkStream, err := os.Open(filepath.Join(workingDir, entry.Name()))
		require.NoError(t, err)
		defer chunkStream.Close()
		streams = append(streams, chunkStream)
	}

	gotFile, err := io.ReadAll(io.MultiReader(streams...))
	require.NoError(t, err)
	bytesHashed, err = hash.Write(gotFile)
	require.NoError(t, err)
	require.Equal(t, testDatasetSize, bytesHashed)
	gotHash = hash.Sum([]byte{})

	require.Equal(t, wantHash, gotHash)
}

func validateSizes(t *testing.T, workingDir string, totalSize, chunkSize int) {
	t.Helper()
	catalog, err := os.ReadDir(workingDir)
	require.NoError(t, err)

	fullSizeChunks := totalSize / chunkSize
	lastChunkSize := totalSize - chunkSize*fullSizeChunks
	totalChunks := fullSizeChunks
	if lastChunkSize != 0 {
		totalChunks += 1
	}

	require.Len(t, catalog, totalChunks)

	for i, dirEntry := range catalog {
		chunkPath := filepath.Join(workingDir, dirEntry.Name())
		require.FileExists(t, chunkPath)
		s, err := os.Stat(chunkPath)
		require.NoError(t, err)

		if i != len(catalog)-1 {
			require.Equal(t, int64(chunkSize), s.Size())
			continue
		}

		// Last chunk is holds the remainder of the file and in most cases will be smaller than full chunk.
		require.Equal(t, int64(lastChunkSize), s.Size())
	}
}
