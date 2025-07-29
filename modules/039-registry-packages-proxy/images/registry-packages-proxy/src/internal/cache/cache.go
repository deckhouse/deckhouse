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

package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/pkg/errors"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/cache"
)

const HighUsagePercent = 80

type CacheEntry struct {
	lastAccessTime time.Time
	layerDigest    string
	isCorrupted    bool
}

type Cache struct {
	storage map[string]*CacheEntry
	sync.RWMutex
	logger        *log.Logger
	root          string
	retentionSize uint64

	metrics *Metrics
}

func NewCache(logger *log.Logger, root string, retentionSize uint64, metrics *Metrics) *Cache {
	storage := make(map[string]*CacheEntry)
	return &Cache{
		storage:       storage,
		logger:        logger,
		root:          root,
		retentionSize: retentionSize,
		metrics:       metrics,
	}
}

func (c *Cache) Get(digest string) (int64, io.ReadCloser, error) {
	// check if cache entry exists
	entry, ok := c.storageGetOK(digest)
	if !ok {
		return 0, nil, cache.ErrEntryNotFound
	}

	path := c.layerDigestToPath(entry.layerDigest)

	// check if file hash is correct
	if !c.checkHashIsOK(entry.layerDigest) {
		c.logger.Warn("entry with digest is corrupted, marking it", slog.String("digest", digest))
		c.Lock()
		c.storage[digest].isCorrupted = true
		c.Unlock()
		return 0, nil, cache.ErrEntryNotFound
	}

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil, cache.ErrEntryNotFound
		}

		return 0, nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		return 0, nil, err
	}

	c.logger.Info("found file with size in the cache", slog.String("digest", digest), slog.String("name", stat.Name()), slog.Int64("size", stat.Size()))

	c.Lock()
	c.storage[digest].lastAccessTime = time.Now()
	c.Unlock()

	return stat.Size(), file, nil
}

func (c *Cache) Set(digest string, layerDigest string, reader io.Reader) error {

	if digest == "" {
		c.logger.Warn("digest is empty, skipping", slog.String("digest", digest))
		return nil
	}

	if layerDigest == "" {
		c.logger.Warn("layer digest is empty, skipping", slog.String("layerDigest", layerDigest))
		return nil
	}

	// check if cache entry exists
	if _, ok := c.storageGetOK(digest); ok {
		c.logger.Info("entry with digest already exists, skipping", slog.String("digest", digest))
		return nil
	}

	path := c.layerDigestToPath(layerDigest)

	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	file, err := os.Create(path)
	defer file.Close()
	if err != nil {
		return err
	}
	size, err := io.Copy(file, reader)
	if err != nil {
		return err
	}

	c.logger.Info("wrote file with digest to the cache dir", slog.String("digest", digest), slog.String("path", path), slog.Int64("size", size))

	c.RLock()
	defer c.RUnlock()
	c.storage[digest] = &CacheEntry{
		lastAccessTime: time.Now(),
		layerDigest:    layerDigest,
		isCorrupted:    false,
	}

	c.metrics.CacheSize.Add(float64(size))
	return nil
}

func (c *Cache) Reconcile(ctx context.Context) {
	c.logger.Info("starting cache reconcile loop")

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(c.storage) == 0 {
				c.logger.Info("storage map is empty")
				continue
			}

			c.applyRetentionPolicy()
			c.checkFilesHash()
			c.deleteOrphanedOrCorruptedEntries()
			c.deleteFiles()
		}
	}
}

func (c *Cache) deleteOrphanedOrCorruptedEntries() {
	c.logger.Info("starting cache delete orphaned or corrupted entries")
	c.Lock()
	defer c.Unlock()
	// delete corrupted entries
	for k, v := range c.storage {
		if v.isCorrupted || v.layerDigest == "" {
			c.logger.Info("delete corrupted entry", slog.String("digest", k), slog.String("layerDigest", v.layerDigest))
			delete(c.storage, k)
		}
	}
}

func (c *Cache) deleteFiles() {
	c.logger.Info("starting cache delete files")
	c.RLock()
	layerDigests := make(map[string]struct{}, len(c.storage))
	for _, v := range c.storage {
		layerDigests[v.layerDigest] = struct{}{}
	}
	c.RUnlock()

	files := c.getFileList()
	for _, file := range files {
		if _, ok := layerDigests[filepath.Base(file)]; ok {
			continue
		}
		stat, err := os.Stat(file)
		if err != nil {
			c.logger.Warn("failed to stat file", slog.String("path", file), log.Err(err))
			continue
		}
		err = os.Remove(file)
		if err != nil {
			c.logger.Warn("failed to delete orphaned file", slog.String("path", file), log.Err(err))
			continue
		}
		c.metrics.CacheSize.Sub(float64(stat.Size()))
		c.logger.Info("delete orphaned file", slog.String("path", file))
	}
}

func (c *Cache) checkFilesHash() {
	c.logger.Info("starting cache files hash check")
	c.Lock()
	defer c.Unlock()
	for k, v := range c.storage {
		if !c.checkHashIsOK(v.layerDigest) {
			c.logger.Warn("entry with digest is corrupted, marking it", slog.String("digest", k))
			c.storage[k].isCorrupted = true
		}
	}
}

func (c *Cache) applyRetentionPolicy() {
	c.logger.Info("starting cache retention policy")
	for {
		usagePercent := int(float64(c.calculateCacheSize()) / float64(c.retentionSize) * 100)
		if usagePercent < HighUsagePercent {
			c.logger.Info("current cache usage low, compaction is not needed", slog.Int("usagePercent", usagePercent), slog.Int("HighUsagePercent", HighUsagePercent))
			return
		}

		c.logger.Info("need to compact cache, current usage is high", slog.Int("usagePercent", usagePercent), slog.Int("HighUsagePercent", HighUsagePercent))

		// sort descending by last access time
		var oldestDigest string
		lowestTime := time.Now()

		c.Lock()
		defer c.Unlock()
		for k, v := range c.storage {
			if lowestTime.Compare(v.lastAccessTime) >= 0 {
				oldestDigest = k
			}
		}
		delete(c.storage, oldestDigest)
	}
}

func (c *Cache) calculateCacheSize() int64 {
	files := c.getFileList()
	var size int64

	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			c.logger.Warn("failed to stat file", slog.String("path", file), log.Err(err))
			continue
		}
		size += stat.Size()
	}

	return size
}

func (c *Cache) layerDigestToPath(digest string) string {
	return filepath.Join(c.root, "packages", digest[:2], digest)
}

func (c *Cache) storageGetOK(digest string) (*CacheEntry, bool) {
	if digest == "" {
		c.logger.Info("digest is empty, skipping", slog.String("digest", digest))
		return nil, false
	}
	c.RLock()
	defer c.RUnlock()
	entry, ok := c.storage[digest]

	if !ok {
		c.logger.Info("entry with digest is not found in the cache", slog.String("digest", digest))
		return nil, false
	}

	if entry.isCorrupted {
		c.logger.Warn("entry with digest is corrupted, skipping", slog.String("digest", digest))
		return nil, false
	}

	if entry.layerDigest == "" {
		c.logger.Warn("entry with digest doesn't have layer digest, skipping", slog.String("digest", digest))
		return nil, false
	}

	// deepcopy
	ret := &CacheEntry{
		lastAccessTime: entry.lastAccessTime,
		layerDigest:    entry.layerDigest,
		isCorrupted:    entry.isCorrupted,
	}
	return ret, true
}

func (c *Cache) checkHashIsOK(layerDigest string) bool {
	path := c.layerDigestToPath(layerDigest)
	c.logger.Info("checking hash sum of file in the cache", slog.String("layerDigest", layerDigest), slog.String("path", path))
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		c.logger.Warn("failed to open file", slog.String("path", path), log.Err(err))
		return false
	}

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		c.logger.Warn("failed to calculate hash sum", slog.String("path", path), log.Err(err))
		return false
	}
	hsum := fmt.Sprintf("%x", h.Sum(nil))
	if hsum != layerDigest {
		c.logger.Warn("entry with layer digest corrupted in the cache", slog.String("path", path), slog.String("hash", hsum), slog.String("layerHash", layerDigest))
		return false
	}

	return true
}

func (c *Cache) getFileList() []string {
	var files []string
	err := filepath.Walk(c.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		c.logger.Warn("failed to walk cache dir", log.Err(err))
	}
	return files
}
