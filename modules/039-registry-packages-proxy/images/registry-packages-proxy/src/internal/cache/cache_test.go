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

package cache

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"testing/iotest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/cache"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func newTestCache(t *testing.T, retentionSize uint64) *Cache {
	t.Helper()

	metrics := &Metrics{CacheSize: prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_cache_size"})}

	return NewCache(log.NewNop(), t.TempDir(), retentionSize, metrics)
}

func layerOf(content []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(content))
}

func writeLayerFile(t *testing.T, c *Cache, layerDigest string, content []byte) {
	t.Helper()

	path := c.layerDigestToPath(layerDigest)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, content, 0644))
}

func makeLayerFIFO(t *testing.T, c *Cache, layerDigest string) string {
	t.Helper()

	path := c.layerDigestToPath(layerDigest)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, syscall.Mkfifo(path, 0600))

	return path
}

func tmpFiles(t *testing.T, c *Cache) []os.DirEntry {
	t.Helper()

	entries, err := os.ReadDir(c.tmpDir())
	if os.IsNotExist(err) {
		return nil
	}

	require.NoError(t, err)

	return entries
}

func TestGetEntryDeletedWhileHashingReleasesLock(t *testing.T) {
	c := newTestCache(t, 1<<30)
	layerDigest := layerOf([]byte("expected"))
	path := makeLayerFIFO(t, c, layerDigest)
	c.storage["sha256:pkg"] = &CacheEntry{layerDigest: layerDigest, lastAccessTime: time.Now()}

	type result struct {
		err      error
		panicked any
	}

	done := make(chan result, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- result{panicked: r}
			}
		}()

		_, _, err := c.Get("sha256:pkg")
		done <- result{err: err}
	}()

	writer, err := os.OpenFile(path, os.O_WRONLY, 0)
	require.NoError(t, err)

	c.Lock()
	delete(c.storage, "sha256:pkg")
	c.Unlock()

	_, err = writer.Write([]byte("corrupted"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	res := <-done
	require.Nil(t, res.panicked)
	require.ErrorIs(t, res.err, cache.ErrEntryNotFound)
	require.True(t, c.TryLock(), "a failed Get must not leave the cache locked")
	c.Unlock()
}

func TestSetConcurrentDigestsSharingLayer(t *testing.T) {
	c := newTestCache(t, 1<<30)
	content := bytes.Repeat([]byte("layer"), 64<<10)
	layerDigest := layerOf(content)

	var wg sync.WaitGroup

	errs := make([]error, 8)
	for i := range errs {
		wg.Go(func() {
			errs[i] = c.Set("sha256:pkg"+strconv.Itoa(i), layerDigest, bytes.NewReader(content))
		})
	}

	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}

	assert.NotEmpty(t, c.storage)
	assert.True(t, c.checkHashIsOK(layerDigest))
	assert.Empty(t, tmpFiles(t, c))
	assert.Empty(t, c.inflight)
}

func TestSetSkipsLayerAlreadyBeingWritten(t *testing.T) {
	c := newTestCache(t, 1<<30)
	content := bytes.Repeat([]byte("layer"), 64<<10)
	layerDigest := layerOf(content)
	pipeReader, pipeWriter := io.Pipe()

	done := make(chan error, 1)

	go func() {
		done <- c.Set("sha256:first", layerDigest, pipeReader)
	}()

	_, err := pipeWriter.Write(content[:1])
	require.NoError(t, err)

	second := bytes.NewReader(content)
	require.NoError(t, c.Set("sha256:second", layerDigest, second))
	assert.Equal(t, len(content), second.Len(), "a concurrent fill of the same layer must not be read")

	_, err = pipeWriter.Write(content[1:])
	require.NoError(t, err)
	require.NoError(t, pipeWriter.Close())
	require.NoError(t, <-done)

	assert.Contains(t, c.storage, "sha256:first")
	assert.NotContains(t, c.storage, "sha256:second")
	assert.Empty(t, tmpFiles(t, c))
	assert.Empty(t, c.inflight)
}

func TestSetAbortedFillKeepsCachedLayer(t *testing.T) {
	c := newTestCache(t, 1<<30)
	content := bytes.Repeat([]byte("layer"), 64<<10)
	layerDigest := layerOf(content)
	require.NoError(t, c.Set("sha256:first", layerDigest, bytes.NewReader(content)))

	aborted := io.MultiReader(bytes.NewReader(content[:len(content)/2]), iotest.ErrReader(io.ErrUnexpectedEOF))
	require.Error(t, c.Set("sha256:second", layerDigest, aborted))

	assert.True(t, c.checkHashIsOK(layerDigest), "an aborted fill must not touch the cached file")
	assert.NotContains(t, c.storage, "sha256:second")
	assert.Empty(t, tmpFiles(t, c))

	size, reader, err := c.Get("sha256:first")
	require.NoError(t, err)
	defer reader.Close()

	assert.Equal(t, int64(len(content)), size)
}

func TestSetRejectsContentNotMatchingLayerDigest(t *testing.T) {
	c := newTestCache(t, 1<<30)
	layerDigest := layerOf([]byte("expected"))

	require.Error(t, c.Set("sha256:pkg", layerDigest, bytes.NewReader([]byte("truncated"))))

	assert.NoFileExists(t, c.layerDigestToPath(layerDigest))
	assert.NotContains(t, c.storage, "sha256:pkg")
	assert.Empty(t, tmpFiles(t, c))
}

func TestCheckFilesHashDoesNotHoldLockWhileHashing(t *testing.T) {
	c := newTestCache(t, 1<<30)
	layerDigest := layerOf([]byte("expected"))
	path := makeLayerFIFO(t, c, layerDigest)
	c.storage["sha256:pkg"] = &CacheEntry{layerDigest: layerDigest, lastAccessTime: time.Now()}

	done := make(chan struct{})
	go func() {
		defer close(done)

		c.checkFilesHash()
	}()

	writer, err := os.OpenFile(path, os.O_WRONLY, 0)
	require.NoError(t, err)

	locked := c.TryLock()
	if locked {
		c.Unlock()
	}

	_, err = writer.Write([]byte("corrupted"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	<-done

	assert.True(t, locked, "hashing must not block requests")
	assert.True(t, c.storage["sha256:pkg"].isCorrupted)
}

func TestApplyRetentionPolicyEvictsLeastRecentlyUsed(t *testing.T) {
	c := newTestCache(t, 100)
	oldContent := bytes.Repeat([]byte("o"), 60)
	newContent := bytes.Repeat([]byte("n"), 60)
	oldLayer, newLayer := layerOf(oldContent), layerOf(newContent)
	writeLayerFile(t, c, oldLayer, oldContent)
	writeLayerFile(t, c, newLayer, newContent)
	c.storage["sha256:old"] = &CacheEntry{layerDigest: oldLayer, lastAccessTime: time.Now().Add(-time.Hour)}
	c.storage["sha256:new"] = &CacheEntry{layerDigest: newLayer, lastAccessTime: time.Now()}

	done := make(chan struct{})
	go func() {
		defer close(done)

		c.applyRetentionPolicy()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("applyRetentionPolicy did not return")
	}

	assert.NotContains(t, c.storage, "sha256:old")
	assert.Contains(t, c.storage, "sha256:new")
}

func TestApplyRetentionPolicyIgnoresOrphanFiles(t *testing.T) {
	c := newTestCache(t, 100)
	orphanContent := bytes.Repeat([]byte("o"), 85)
	liveContent := bytes.Repeat([]byte("l"), 5)
	liveLayer := layerOf(liveContent)
	writeLayerFile(t, c, layerOf(orphanContent), orphanContent)
	writeLayerFile(t, c, liveLayer, liveContent)
	c.storage["sha256:live"] = &CacheEntry{layerDigest: liveLayer, lastAccessTime: time.Now()}

	c.applyRetentionPolicy()

	assert.Contains(t, c.storage, "sha256:live")
}

func TestDeleteFilesKeepsTemporaryFiles(t *testing.T) {
	c := newTestCache(t, 1<<30)
	orphan := layerOf([]byte("orphan"))
	writeLayerFile(t, c, orphan, []byte("orphan"))
	require.NoError(t, os.MkdirAll(c.tmpDir(), 0755))
	inFlight := filepath.Join(c.tmpDir(), "in-flight")
	require.NoError(t, os.WriteFile(inFlight, []byte("partial"), 0644))

	c.deleteFiles()

	assert.NoFileExists(t, c.layerDigestToPath(orphan))
	assert.FileExists(t, inFlight)
}
