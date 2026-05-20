// Copyright 2025 Flant JSC
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

package registry

import (
	"context"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
)

// Client defines the interface for interacting with container registries.
// Implementations must be safe for concurrent use.
type Client interface {
	// WithSegment creates a new client scoped to an additional path segment.
	// This method can be chained: client.WithSegment("org").WithSegment("repo").
	// Multiple segments can be passed at once: client.WithSegment("org", "repo").
	WithSegment(segments ...string) Client

	// GetRegistry returns the full registry path (host + segments).
	GetRegistry() string

	// GetImage retrieves a remote image by tag or digest reference.
	GetImage(ctx context.Context, tag string, opts ...ImageGetOption) (Image, error)

	// Push uploads either a v1.Image or v1.ImageIndex (anything implementing
	// partial.WithRawManifest) to the registry at the specified tag and
	// returns the resulting digest. This is the preferred push API: it
	// dispatches by media type and returns the digest most callers need for
	// audit logs, image-refs files or downstream Helm values.
	Push(ctx context.Context, tag string, obj partial.WithRawManifest, opts ...ImagePushOption) (v1.Hash, error)

	// PushImage pushes a v1.Image to the registry at the specified tag.
	//
	// Deprecated: use [Client.Push], which also returns the resulting digest.
	PushImage(ctx context.Context, tag string, img v1.Image, opts ...ImagePushOption) error

	// PushIndex pushes a v1.ImageIndex (multi-arch manifest list) to the registry.
	//
	// Deprecated: use [Client.Push], which also returns the resulting digest.
	PushIndex(ctx context.Context, tag string, idx v1.ImageIndex, opts ...ImagePushOption) error

	// GetDigest returns the digest hash for the given tag or digest reference.
	GetDigest(ctx context.Context, tag string) (*v1.Hash, error)

	// GetManifest retrieves the manifest for a specific image reference and
	// returns it as a decoded [ManifestResult] for ergonomic field access.
	// For byte-stable manifest bytes (signatures, audit, jq pipelines) prefer
	// [Client.GetManifestRaw].
	GetManifest(ctx context.Context, tag string) (ManifestResult, error)

	// GetManifestRaw returns the manifest bytes exactly as the registry served
	// them along with the manifest descriptor. The bytes are stable and
	// suitable for signature verification, audit logging or piping to jq.
	GetManifestRaw(ctx context.Context, tag string) ([]byte, *v1.Descriptor, error)

	// GetImageConfig retrieves the image config file containing labels and metadata.
	GetImageConfig(ctx context.Context, tag string) (*v1.ConfigFile, error)

	// ImageExists reports whether tag resolves to an existing manifest in
	// the registry. A 404 is normalised to (false, nil); any other
	// transport-level problem is surfaced via the error return so the caller
	// can distinguish "not there" from "could not check".
	ImageExists(ctx context.Context, tag string) (bool, error)

	// CheckImageExists returns nil if tag exists or [ErrImageNotFound] if not.
	//
	// Deprecated: use [Client.ImageExists], whose (bool, error) return type
	// matches the question being asked and never reports ErrImageNotFound as
	// an error.
	CheckImageExists(ctx context.Context, tag string) error

	// WalkTags streams tag pages of the current repository: visit is invoked
	// once per page with the filtered subset of tags. Returning a non-nil
	// error from visit stops iteration and that error is returned. WithTagsLast
	// and WithTagsLimit are honoured: tags lexicographically <= Last are
	// filtered out before visit is called, and iteration stops once a total
	// of N tags have been visited.
	//
	// WalkTags is the streaming primitive; ListTags is a thin accumulating
	// wrapper. Prefer WalkTags for repositories with thousands of tags so
	// that callers can stream/log/filter without buffering the full slice.
	WalkTags(ctx context.Context, visit func(tags []string) error, opts ...ListTagsOption) error

	// ListTags returns tags for the repository built by WithSegment calls.
	// It is a thin wrapper around [Client.WalkTags] that buffers every page
	// into a single slice.
	ListTags(ctx context.Context, opts ...ListTagsOption) ([]string, error)

	// WalkRepositories streams /v2/_catalog pages: visit is invoked once per
	// page with the filtered subset of repositories. Returning a non-nil
	// error from visit stops iteration and that error is returned.
	// WithReposLast and WithReposLimit behave identically to their Tag
	// counterparts.
	WalkRepositories(ctx context.Context, visit func(repos []string) error, opts ...ListRepositoriesOption) error

	// ListRepositories lists repositories visible from the registry. Thin
	// accumulating wrapper around [Client.WalkRepositories].
	ListRepositories(ctx context.Context, opts ...ListRepositoriesOption) ([]string, error)

	// DeleteTag deletes a specific tag from the registry.
	DeleteTag(ctx context.Context, tag string) error

	// DeleteByDigest deletes a manifest by its digest from the registry.
	DeleteByDigest(ctx context.Context, digest v1.Hash) error

	// TagImage adds a new tag pointing to the same manifest as sourceTag.
	TagImage(ctx context.Context, sourceTag, destTag string) error

	// CopyImage copies an image from this client's repository to a destination
	// client's repository, without pulling layers locally when possible.
	CopyImage(ctx context.Context, srcTag string, dest Client, destTag string) error
}
