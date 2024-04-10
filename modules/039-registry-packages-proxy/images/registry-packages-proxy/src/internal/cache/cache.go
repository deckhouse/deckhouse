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
	"time"

	"github.com/pkg/errors"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/cache"
)

type Cache struct {
	root          string
	retentionSize uint64

	metrics *Metrics
}

func New(root string, retentionSize uint64, metrics *Metrics) (*Cache, error) {
	return &Cache{
		root:          root,
		retentionSize: retentionSize,
		metrics:       metrics,
	}, nil
}

func (c *Cache) Get(digest string) (int64, io.ReadCloser, error) {
	path := c.digestToPath(digest)

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil, cache.ErrEntryNotFound
		}

		return 0, nil, errors.Wrap(err, "failed to open package file")
	}

	stat, err := file.Stat()
	if err != nil {
		return 0, nil, errors.Wrap(err, "failed to get stat of package file")
	}

	return stat.Size(), file, nil
}

func (c *Cache) Set(digest string, size int64, reader io.Reader) error {
	err := c.copyPackage(digest, reader)
	if err != nil {
		return err
	}
	c.metrics.CacheSize.Add(float64(size))
	return nil
}

func (c *Cache) Run(ctx context.Context) error {

	err := c.ApplyRetentionPolicy()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			err := c.ApplyRetentionPolicy()
			if err != nil {
				return errors.Wrap(err, "failed to apply retention policy")
			}
		}
	}
}

func (c *Cache) ApplyRetentionPolicy() error {
	return nil
}
