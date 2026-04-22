/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"context"
	"io"

	"github.com/opencontainers/go-digest"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
)

// ContentStore provides low-level access to raw blobs by digest.
type ContentStore interface {
	// Fetch opens a content stream for the blob identified by dgst.
	// Caller must close the returned io.ReadCloser.
	//
	// Errors:
	//   - errs.ErrInvalidDigest — dgst is malformed.
	//   - errs.ErrBlobNotFound  — no blob with the given digest exists in the store.
	Fetch(ctx context.Context, dgst digest.Digest) (io.ReadCloser, error)

	// Exists reports whether a blob with the given digest is present in the store,
	// and returns its size in bytes when it does.
	//
	// Errors:
	//   - errs.ErrInvalidDigest — dgst is malformed.
	Exists(ctx context.Context, dgst digest.Digest) (bool, int64, error)
}

// Store resolves references, lists tags, and walks manifest-linked blobs.
type Store interface {
	ContentStore

	// Resolve resolves a reference (tag or digest string) to a descriptor and opens
	// a content stream for the corresponding manifest.
	// Caller must close the returned io.ReadCloser.
	//
	// Errors:
	//   - errs.ErrMissingReference — reference is empty.
	//   - errs.ErrInvalidDigest    — reference is a malformed digest string.
	//   - errs.ErrManifestNotFound — reference does not resolve to a known manifest.
	Resolve(ctx context.Context, reference string) (types.ShortDescriptor, io.ReadCloser, error)

	// Predecessors returns the descriptors of manifests that reference the manifest
	// identified by dgst (e.g. index manifests that list it as a platform entry).
	//
	// Errors:
	//   - errs.ErrInvalidDigest    — dgst is malformed.
	//   - errs.ErrManifestNotFound — no manifest with the given digest exists in the store.
	Predecessors(ctx context.Context, dgst digest.Digest) ([]ociv1.Descriptor, error)

	// SortedTags returns tag names in ascending lexicographic order.
	// If last is non-empty, the list starts after that tag (exclusive), enabling pagination.
	//
	// This method does not return domain errors.
	SortedTags(ctx context.Context, last string) ([]string, error)
}
