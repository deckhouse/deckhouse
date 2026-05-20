/*
Copyright 2026 Flant JSC

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

// Package registry defines the HTTP-facing repository abstraction: a set of named
// repositories, each backed by content-addressable storage ([Registry.Resolve], [Registry.Fetch], …).
package registry

import (
	"context"
	"io"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
)

// Registry is a multi-repository API: operations are scoped by repo name.
type Registry interface {
	// Resolve resolves a reference (tag or digest string) within the given repository
	// to a descriptor and opens a content stream for the manifest.
	// Caller must close the returned io.ReadCloser.
	//
	// Errors:
	//   - errs.ErrUnknownRepository — repo does not exist.
	//   - errs.ErrMissingReference  — reference is empty.
	//   - errs.ErrManifestNotFound  — reference does not resolve to a known manifest.
	//   - errs.ErrInvalidDigest     — reference is a malformed digest string.
	Resolve(ctx context.Context, repo string, reference string) (types.ShortDescriptor, io.ReadCloser, error)

	// Fetch opens a content stream for the blob identified by dgst within the given repository.
	// Caller must close the returned io.ReadCloser.
	//
	// Errors:
	//   - errs.ErrUnknownRepository — repo does not exist.
	//   - errs.ErrInvalidDigest     — dgst is malformed.
	//   - errs.ErrBlobNotFound      — no blob with the given digest exists in the repository.
	Fetch(ctx context.Context, repo string, dgst digest.Digest) (io.ReadCloser, error)

	// Exists reports whether a blob with the given digest exists in the repository,
	// and returns its size in bytes when it does.
	//
	// Errors:
	//   - errs.ErrUnknownRepository — repo does not exist.
	//   - errs.ErrInvalidDigest     — dgst is malformed.
	Exists(ctx context.Context, repo string, dgst digest.Digest) (bool, int64, error)

	// Predecessors returns the descriptors of manifests that reference the manifest
	// identified by dgst (e.g. index manifests that list it as a platform entry).
	//
	// Errors:
	//   - errs.ErrUnknownRepository — repo does not exist.
	//   - errs.ErrInvalidDigest     — dgst is malformed.
	//   - errs.ErrManifestNotFound  — no manifest with the given digest exists in the repository.
	Predecessors(ctx context.Context, repo string, dgst digest.Digest) ([]ocispec.Descriptor, error)

	// SortedTags returns tag names for the given repository in ascending lexicographic order.
	// If last is non-empty, the list starts after that tag (exclusive), enabling pagination.
	//
	// Errors:
	//   - errs.ErrUnknownRepository — repo does not exist.
	SortedTags(ctx context.Context, repo string, last string) ([]string, error)

	// SortedRepos returns the names of all known repositories in ascending lexicographic order.
	// This method never returns an error.
	SortedRepos() []string
}
