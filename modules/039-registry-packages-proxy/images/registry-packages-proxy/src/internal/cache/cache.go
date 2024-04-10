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
	"io"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/cache"
)

type CacheEntry struct {
	lastAccessTime time.Time
	size           int64
}

type Cache struct {
	storage map[string]*CacheEntry
	sync.RWMutex
	logger        *log.Entry
	root          string
	retentionSize uint64

	metrics *Metrics
}

func New(logger *log.Entry, root string, retentionSize uint64, metrics *Metrics) *Cache {
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
	path := c.digestToPath(digest)

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil, cache.ErrEntryNotFound
		}

		return 0, nil, err
	}

	stat, err := file.Stat()
	c.logger.Infof("found file %s with size %d", stat.Name(), stat.Size())

	if err != nil {
		return 0, nil, err
	}

	c.Lock()
	defer c.Unlock()
	c.storage[digest].lastAccessTime = time.Now()

	return stat.Size(), file, nil
}

func (c *Cache) Set(digest string, size int64, reader io.Reader) error {
	c.logger.Infof("write file with digest %s with size %d to the cache dir", digest, size)
	err := c.copyPackage(digest, reader)
	if err != nil {
		return err
	}

	c.Lock()
	defer c.Unlock()
	c.storage[digest] = &CacheEntry{
		lastAccessTime: time.Now(),
		size:           size,
	}

	c.metrics.CacheSize.Add(float64(size))
	return nil
}

func (c *Cache) Run(ctx context.Context) {
	c.logger.Info("starting cache reconcile loop")

	err := c.ApplyRetentionPolicy()
	if err != nil {
		c.logger.Error(err)
		return
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := c.ApplyRetentionPolicy()
			if err != nil {
				c.logger.Error(err)
				return
			}
		}
	}
}

func (c *Cache) ApplyRetentionPolicy() error {
	return nil
}
