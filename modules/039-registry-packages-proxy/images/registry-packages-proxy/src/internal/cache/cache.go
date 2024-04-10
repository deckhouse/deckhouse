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
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/cache"
)

type Cache struct {
	root            string
	retentionSize   uint64
	retentionPeriod time.Duration

	metrics *Metrics
}

func New(root string, retentionSize uint64, retentionPeriod time.Duration, metrics *Metrics) (*Cache, error) {
	return &Cache{
		root:            root,
		retentionSize:   retentionSize,
		retentionPeriod: retentionPeriod,
		metrics:         metrics,
	}, nil
}

func (c *Cache) Close() error {
	return nil
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
		return errors.Wrap(err, "failed to apply retention policy")
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

func (c *Cache) applyRetentionPolicy(tx *bolt.Tx, size uint64) error {
	sizeBucket := tx.Bucket(sizeBucketName)

	newSizeBytes := sizeBucket.Get(sizeBucketName)

	newSize := binary.BigEndian.Uint64(newSizeBytes) + size

	cursor := tx.Bucket(retentionBucketName).Cursor()

	minTimestamp := timeToTimestampBytes(time.Time{})

	for timestamp, value := cursor.Seek(minTimestamp); timestamp != nil && c.retentionCondition(timestamp, newSize); timestamp, value = cursor.Next() {
		metadata := &retentionBucketValue{}

		err := gob.NewDecoder(bytes.NewReader(value)).Decode(metadata)
		if err != nil {
			return errors.Wrap(err, "failed to decode retention bucket value")
		}

		newSize -= uint64(metadata.Size)

		path := c.digestToPath(metadata.Digest)

		err = os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err, "failed to remove package")
		}

		c.metrics.CacheSize.Sub(float64(metadata.Size))

		err = cursor.Delete()
		if err != nil {
			return errors.Wrap(err, "failed to delete retention bucket value")
		}
	}

	if size > 0 && newSize != size {
		err := sizeBucket.Put(sizeBucketName, binary.BigEndian.AppendUint64(nil, newSize))
		if err != nil {
			return errors.Wrap(err, "failed to update cache size")
		}
	}

	return nil
}

func (c *Cache) retentionCondition(timestampBytes []byte, size uint64) bool {
	timestamp := time.Unix(int64(binary.BigEndian.Uint64(timestampBytes)), 0)

	return timestamp.Add(c.retentionPeriod).Before(time.Now()) || size > c.retentionSize
}

var (
	sizeBucketName      = []byte("size")
	retentionBucketName = []byte("retention")
)

func timeToTimestampBytes(t time.Time) []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(t.Unix()))
}

type retentionBucketValue struct {
	Digest string
	Size   int64
}
