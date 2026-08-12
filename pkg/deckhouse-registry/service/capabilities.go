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

package service

import (
	"context"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"
)

// The interfaces below slice a repository into the three things one can do to
// it — read, push, delete — so a component can be handed exactly the capability
// it needs and no more. *BasicService implements all of them, so the concrete
// service the tree hands out satisfies any of these without wrapping: a cleanup
// routine takes a Deleter, a mirror takes a Pusher, a resolver takes a Reader,
// and passing the service in narrows it at the call boundary.
//
// Compose wider capabilities by embedding — ReadWriter and ReadDeleter are the
// two the library itself needs, and a caller can declare its own combination
// the same way.
//
// Repository is deliberately thin: it exposes identity and reference building
// but neither Client nor Logger, so a narrowed view cannot be widened back into
// full registry access through the underlying client.

// Repository is the identity every capability shares: the repository path, the
// references built under it, and the log entries tagged with it. It carries no
// registry access of its own — it is what a Reader, Pusher or Deleter is "of".
type Repository interface {
	// Name is the service name used in log records.
	Name() string
	// Path is the full repository path this addresses, without a tag.
	Path() string
	// Ref is the fully-qualified reference for a tag or digest under it.
	Ref(tag string) string
	// Entry is a log entry annotated with this service and tag.
	Entry(tag string) *log.Logger
}

// Reader is the read-only capability: pulling images and their metadata, and
// listing what a repository holds. It never mutates the registry.
type Reader interface {
	Repository

	GetImage(ctx context.Context, tag string, opts ...registry.ImageGetOption) (registry.Image, error)
	GetDigest(ctx context.Context, tag string) (*v1.Hash, error)
	GetManifest(ctx context.Context, tag string) (registry.ManifestResult, error)
	GetImageConfig(ctx context.Context, tag string) (*v1.ConfigFile, error)
	CheckImageExists(ctx context.Context, tag string) error
	Exists(ctx context.Context, tag string) (bool, error)
	ListTags(ctx context.Context, opts ...registry.ListTagsOption) ([]string, error)
	ListRepositories(ctx context.Context, opts ...registry.ListRepositoriesOption) ([]string, error)
}

// Pusher is the write capability: publishing images and indexes, promoting a
// manifest to another tag, and copying an image into another repository.
type Pusher interface {
	Repository

	PushImage(ctx context.Context, tag string, img v1.Image, opts ...registry.ImagePushOption) error
	PushIndex(ctx context.Context, tag string, idx v1.ImageIndex, opts ...registry.ImagePushOption) error
	TagImage(ctx context.Context, sourceTag, destTag string) error
	CopyImage(ctx context.Context, srcTag string, dest registry.Client, destTag string) error
}

// Deleter is the delete capability: removing a tag, or a manifest by digest.
type Deleter interface {
	Repository

	DeleteTag(ctx context.Context, tag string) error
	DeleteByDigest(ctx context.Context, digest v1.Hash) error
}

// ReadWriter reads and publishes — the capability a build-and-push flow needs.
type ReadWriter interface {
	Reader
	Pusher
}

// ReadDeleter reads and deletes — the capability the module and package delete
// flows need: read the bundle's images_digests, then delete by digest and tag.
type ReadDeleter interface {
	Reader
	Deleter
}

// *BasicService provides every capability. The tree hands out the concrete
// type; callers narrow it to the interface they accept.
var (
	_ Reader  = (*BasicService)(nil)
	_ Pusher  = (*BasicService)(nil)
	_ Deleter = (*BasicService)(nil)
)
