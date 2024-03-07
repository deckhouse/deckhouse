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
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/cache"
)

type Cache struct {
	root            string
	retentionSize   uint64
	retentionPeriod time.Duration

	db *bolt.DB
}

func New(root string, retentionSize uint64, retentionPeriod time.Duration) (*Cache, error) {
	db, err := bolt.Open(filepath.Join(root, "retention.db"), 0600, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open retention database")
	}

	err = db.Update(func(tx *bolt.Tx) error {
		sizeBucket, err := tx.CreateBucketIfNotExists(sizeBucketName)
		if err != nil {
			return errors.Wrap(err, "failed to create size bucket")
		}

		newSizeBytes := sizeBucket.Get(sizeBucketName)
		if newSizeBytes == nil {
			err := sizeBucket.Put(sizeBucketName, binary.BigEndian.AppendUint64(nil, 0))
			if err != nil {
				return errors.Wrap(err, "failed to initialize size bucket")
			}
		}

		_, err = tx.CreateBucketIfNotExists(retentionBucketName)
		if err != nil {
			return errors.Wrap(err, "failed to create retention bucket")
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize retention database")
	}

	return &Cache{
		root:            root,
		retentionSize:   retentionSize,
		retentionPeriod: retentionPeriod,
		db:              db,
	}, nil
}

func (c *Cache) Close() error {
	return c.db.Close()
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
	return c.db.Update(func(tx *bolt.Tx) error {
		err := c.applyRetentionPolicy(tx, uint64(size))
		if err != nil {
			return errors.Wrap(err, "failed to apply retention policy")
		}

		retentionBucket := tx.Bucket(retentionBucketName)

		value := &retentionBucketValue{
			Digest: digest,
			Size:   size,
		}

		valueBuffer := &bytes.Buffer{}

		err = gob.NewEncoder(valueBuffer).Encode(value)
		if err != nil {
			return errors.Wrap(err, "failed to encode retention bucket value")
		}

		err = retentionBucket.Put(timeToKey(time.Now()), valueBuffer.Bytes())
		if err != nil {
			return errors.Wrap(err, "failed to put retention bucket value")
		}

		err = c.copyPackage(digest, reader)
		if err != nil {
			return errors.Wrap(err, "failed to copy package")
		}

		return nil
	})
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
	return c.db.Update(func(tx *bolt.Tx) error {
		return c.applyRetentionPolicy(tx, 0)
	})
}

func (c *Cache) applyRetentionPolicy(tx *bolt.Tx, size uint64) error {
	sizeBucket := tx.Bucket(sizeBucketName)

	newSizeBytes := sizeBucket.Get(sizeBucketName)

	newSize := binary.BigEndian.Uint64(newSizeBytes) + size

	cursor := tx.Bucket(retentionBucketName).Cursor()

	min := timeToKey(time.Time{})

	for key, value := cursor.Seek(min); key != nil && c.retentionCondition(key, newSize); key, value = cursor.Next() {
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

func (c *Cache) retentionCondition(key []byte, size uint64) bool {
	max := timeToKey(time.Now().Add(-c.retentionPeriod))

	return bytes.Compare(key, max) <= 0 || size > c.retentionSize
}

var (
	sizeBucketName      = []byte("size")
	retentionBucketName = []byte("retention")
)

func timeToKey(t time.Time) []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(t.Unix()))
}

type retentionBucketValue struct {
	Digest string
	Size   int64
}
