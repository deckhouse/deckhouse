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

	// PushImage pushes a v1.Image to the registry at the specified tag.
	PushImage(ctx context.Context, tag string, img v1.Image, opts ...ImagePushOption) error

	// PushIndex pushes a v1.ImageIndex (multi-arch manifest list) to the registry.
	PushIndex(ctx context.Context, tag string, idx v1.ImageIndex, opts ...ImagePushOption) error

	// GetDigest returns the digest hash for the given tag or digest reference.
	GetDigest(ctx context.Context, tag string) (*v1.Hash, error)

	// GetManifest retrieves the manifest for a specific image reference.
	GetManifest(ctx context.Context, tag string) (ManifestResult, error)

	// GetImageConfig retrieves the image config file containing labels and metadata.
	GetImageConfig(ctx context.Context, tag string) (*v1.ConfigFile, error)

	// CheckImageExists checks whether an image exists in the registry.
	// Returns ErrImageNotFound if the image does not exist.
	CheckImageExists(ctx context.Context, tag string) error

	// ListTags returns tags for the repository built by WithSegment calls.
	ListTags(ctx context.Context, opts ...ListTagsOption) ([]string, error)

	// ListRepositories lists repositories visible from the registry.
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
