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

package ondemand

import (
	"context"
	"io"

	"github.com/flant/docs-builder/internal/s3cache"
)

// ColdCache keeps archives of rendered versions outside the pod, so they
// survive a restart and are shared by replicas: an S3 bucket, be it the
// system storage of the cluster, a bucket of the user or the one of the
// public site.
//
// An object that is not there is an error wrapping ErrColdNotFound; a cold
// cache that is not set up yet answers every call with one wrapping
// ErrColdNotReady. The store takes either for an archive missing and renders
// the version; a follower takes only the first for an answer, and reads the
// registry itself while the cold cache is not ready.
type ColdCache interface {
	// Get writes the archive to w.
	Get(ctx context.Context, key string, w io.Writer) error
	Put(ctx context.Context, key, path string) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]ColdObject, error)
}

// ColdObject is an archive in the cold cache: an object of the bucket, so
// the bucket is the cold cache as it is.
type ColdObject = s3cache.Object
