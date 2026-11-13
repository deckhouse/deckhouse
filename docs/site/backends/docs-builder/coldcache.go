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

package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/s3cache"
)

// coldCache keeps a nil bucket a nil interface, so the store sees no cold
// cache at all.
func coldCache(bucket *s3cache.Cache) ondemand.ColdCache {
	if bucket == nil {
		return nil
	}
	return coldBucket{bucket: bucket}
}

// coldBucket is the S3 bucket as the store sees it: its errors in the terms
// of ondemand.ColdCache.
type coldBucket struct {
	bucket *s3cache.Cache
}

func (c coldBucket) Get(ctx context.Context, key string, w io.Writer) error {
	return coldError(c.bucket.Get(ctx, key, w))
}

func (c coldBucket) Put(ctx context.Context, key, path string) error {
	return coldError(c.bucket.Put(ctx, key, path))
}

func (c coldBucket) Delete(ctx context.Context, key string) error {
	return coldError(c.bucket.Delete(ctx, key))
}

func (c coldBucket) List(ctx context.Context, prefix string) ([]ondemand.ColdObject, error) {
	objects, err := c.bucket.List(ctx, prefix)

	return objects, coldError(err)
}

// coldError marks an error of the bucket with the one of the cold cache it
// means: an object that is not there, or a bucket whose connection is not
// mounted yet.
func coldError(err error) error {
	switch {
	case errors.Is(err, s3cache.ErrNotFound):
		return fmt.Errorf("%w: %w", ondemand.ErrColdNotFound, err)
	case errors.Is(err, s3cache.ErrNotReady):
		return fmt.Errorf("%w: %w", ondemand.ErrColdNotReady, err)
	}

	return err
}
