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

// Package service provides BasicService, one node of the Deckhouse registry
// tree: a single OCI repository plus the logging and reference-building
// conventions shared by every node.
//
// Every sub-tree package (deckhouse, module, packages, security, cli, release,
// extra) embeds a BasicService and adds only the segments and metadata specific
// to it.
//
// BasicService can read, push and delete. Those three groups are also exposed
// as the Reader, Pusher and Deleter interfaces (and the ReadWriter and
// ReadDeleter compositions), so a component can be handed exactly the
// capability it needs; BasicService implements all of them. See capabilities.go.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"
)

// BasicService addresses exactly one OCI repository.
type BasicService struct {
	name   string
	client registry.Client
	logger *log.Logger
}

// NewBasicService creates a BasicService named name over the repository the
// client points at.
func NewBasicService(name string, client registry.Client, logger *log.Logger) *BasicService {
	return &BasicService{
		name:   name,
		client: client,
		logger: logger,
	}
}

// Name returns the service name used in log records (e.g. "module_release").
func (s *BasicService) Name() string {
	return s.name
}

// Client returns the registry client scoped to this service's repository.
func (s *BasicService) Client() registry.Client {
	return s.client
}

// Logger returns the logger this service and its children write to.
func (s *BasicService) Logger() *log.Logger {
	return s.logger
}

// Path returns the full repository path this service addresses, without a tag —
// e.g. "registry.deckhouse.io/deckhouse/fe/modules/stronghold/release".
func (s *BasicService) Path() string {
	return s.client.GetRegistry()
}

// Ref returns the fully-qualified reference for a tag or digest under this
// repository. Digests (with or without a leading "@") produce "path@sha256:…",
// everything else produces "path:tag".
func (s *BasicService) Ref(tag string) string {
	path := s.Path()

	switch {
	case strings.HasPrefix(tag, "@sha256:"):
		return path + tag
	case strings.HasPrefix(tag, "sha256:"):
		return path + "@" + tag
	default:
		return path + ":" + tag
	}
}

// Entry returns a log entry annotated with this service and the given tag.
// Sub-tree packages use it so their records carry the same fields.
func (s *BasicService) Entry(tag string) *log.Logger {
	return s.logger.With(slog.String("service", s.name), slog.String("tag", tag))
}

// GetImage retrieves an image by tag or digest.
//
// A multi-arch reference resolves to one child image. Without
// client.WithPlatform that child is linux/amd64 — a hardcoded default in the
// underlying library, not the host's platform — so pass the option explicitly
// whenever the architecture matters.
func (s *BasicService) GetImage(ctx context.Context, tag string, opts ...registry.ImageGetOption) (registry.Image, error) {
	entry := s.Entry(tag)

	entry.Debug("Getting image")

	img, err := s.client.GetImage(ctx, tag, opts...)
	if err != nil {
		return nil, fmt.Errorf("get image %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Image retrieved successfully")

	return img, nil
}

// GetDigest returns the digest of the manifest a tag points at.
func (s *BasicService) GetDigest(ctx context.Context, tag string) (*v1.Hash, error) {
	entry := s.Entry(tag)

	entry.Debug("Getting digest")

	hash, err := s.client.GetDigest(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("get digest %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Digest retrieved successfully")

	return hash, nil
}

// GetManifest retrieves the manifest for a tag or digest, as the registry
// served it — for a multi-arch reference that is the index itself.
//
// Pass client.WithPlatform to resolve an index down to one child image's
// manifest instead. Unlike GetImage, omitting it resolves nothing rather than
// defaulting to linux/amd64.
func (s *BasicService) GetManifest(ctx context.Context, tag string, opts ...registry.ManifestGetOption) (registry.ManifestResult, error) {
	entry := s.Entry(tag)

	entry.Debug("Getting manifest")

	manifest, err := s.client.GetManifest(ctx, tag, opts...)
	if err != nil {
		return nil, fmt.Errorf("get manifest %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Manifest retrieved successfully")

	return manifest, nil
}

// GetIndex retrieves a multi-arch index whole, resolving nothing — what a
// caller copying or inspecting every platform needs. A reference that is a
// plain image rather than an index is an error.
func (s *BasicService) GetIndex(ctx context.Context, tag string) (v1.ImageIndex, error) {
	entry := s.Entry(tag)

	entry.Debug("Getting index")

	idx, err := s.client.GetIndex(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("get index %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Index retrieved successfully")

	return idx, nil
}

// GetImageConfig retrieves the image config file, which carries labels and
// other build-time metadata.
func (s *BasicService) GetImageConfig(ctx context.Context, tag string) (*v1.ConfigFile, error) {
	entry := s.Entry(tag)

	entry.Debug("Getting image config")

	cfg, err := s.client.GetImageConfig(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("get image config %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Image config retrieved successfully")

	return cfg, nil
}

// CheckImageExists returns nil when the tag exists and ErrImageNotFound when it
// does not. Prefer Exists when a boolean is more convenient.
func (s *BasicService) CheckImageExists(ctx context.Context, tag string) error {
	entry := s.Entry(tag)

	entry.Debug("Checking if image exists")

	if err := s.client.CheckImageExists(ctx, tag); err != nil {
		return fmt.Errorf("check image %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Image exists")

	return nil
}

// Exists reports whether a tag exists. A missing image is not an error; any
// other registry failure is returned as one.
func (s *BasicService) Exists(ctx context.Context, tag string) (bool, error) {
	err := s.CheckImageExists(ctx, tag)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, ErrImageNotFound) {
		return false, nil
	}

	return false, err
}

// PushImage publishes img at tag in this repository.
func (s *BasicService) PushImage(ctx context.Context, tag string, img v1.Image, opts ...registry.ImagePushOption) error {
	entry := s.Entry(tag)

	entry.Debug("Pushing image")

	if err := s.client.PushImage(ctx, tag, img, opts...); err != nil {
		return fmt.Errorf("push image %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Image pushed successfully")

	return nil
}

// PushIndex publishes a multi-arch image index at tag in this repository.
func (s *BasicService) PushIndex(ctx context.Context, tag string, idx v1.ImageIndex, opts ...registry.ImagePushOption) error {
	entry := s.Entry(tag)

	entry.Debug("Pushing image index")

	if err := s.client.PushIndex(ctx, tag, idx, opts...); err != nil {
		return fmt.Errorf("push index %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Image index pushed successfully")

	return nil
}

// TagImage points destTag at the manifest sourceTag already resolves to. It is
// a manifest-only promotion — no layers are re-uploaded.
func (s *BasicService) TagImage(ctx context.Context, sourceTag, destTag string) error {
	entry := s.logger.With(
		slog.String("service", s.name),
		slog.String("source_tag", sourceTag),
		slog.String("dest_tag", destTag),
	)

	entry.Debug("Tagging image")

	if err := s.client.TagImage(ctx, sourceTag, destTag); err != nil {
		return fmt.Errorf("tag %s as %s: %w", s.Ref(sourceTag), s.Ref(destTag), err)
	}

	entry.Debug("Image tagged successfully")

	return nil
}

// CopyImage copies srcTag from this repository into dest at destTag, without
// pulling layers through the local machine when the registry supports it.
func (s *BasicService) CopyImage(ctx context.Context, srcTag string, dest registry.Client, destTag string) error {
	entry := s.logger.With(
		slog.String("service", s.name),
		slog.String("src_tag", srcTag),
		slog.String("dest", dest.GetRegistry()),
		slog.String("dest_tag", destTag),
	)

	entry.Debug("Copying image")

	if err := s.client.CopyImage(ctx, srcTag, dest, destTag); err != nil {
		return fmt.Errorf("copy %s to %s: %w", s.Ref(srcTag), dest.GetRegistry()+":"+destTag, err)
	}

	entry.Debug("Image copied successfully")

	return nil
}

// DeleteTag removes a tag from this repository. Returns ErrImageNotFound when
// the tag does not exist, so callers can treat deleting something already gone
// as a no-op.
func (s *BasicService) DeleteTag(ctx context.Context, tag string) error {
	entry := s.Entry(tag)

	entry.Debug("Deleting tag")

	if err := s.client.DeleteTag(ctx, tag); err != nil {
		return fmt.Errorf("delete tag %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Tag deleted successfully")

	return nil
}

// DeleteByDigest removes a manifest by its digest from this repository. Returns
// ErrImageNotFound when no manifest with that digest exists.
func (s *BasicService) DeleteByDigest(ctx context.Context, digest v1.Hash) error {
	entry := s.Entry(digest.String())

	entry.Debug("Deleting manifest by digest")

	if err := s.client.DeleteByDigest(ctx, digest); err != nil {
		return fmt.Errorf("delete digest %s: %w", s.Ref(digest.String()), err)
	}

	entry.Debug("Manifest deleted successfully")

	return nil
}

// ListTags returns every tag of this repository: the client walks the
// registry's cursor to the end, so the result is complete or an error, never a
// silently truncated page. Ask for one page explicitly with
// client.WithTagsLimit and client.WithTagsLast, or use StreamTags to walk
// without buffering.
func (s *BasicService) ListTags(ctx context.Context, opts ...registry.ListTagsOption) ([]string, error) {
	entry := s.logger.With(slog.String("service", s.name))

	entry.Debug("Listing tags")

	tags, err := s.client.ListTags(ctx, opts...)
	if err != nil {
		return nil, s.listError("list tags of %s", err)
	}

	entry.Debug("Tags listed successfully", slog.Int("count", len(tags)))

	return tags, nil
}

// StreamTags hands each page of tags to visit as it arrives, so a repository
// with very many tags can be walked without holding them all in memory. It is
// ListTags without the accumulation, and takes the same options.
//
// Return ErrStopStreaming from visit to end the walk early — once the caller
// has seen what it needs, the remaining pages are wasted round trips. It is not
// reported as a failure; any other error from visit is returned unchanged.
func (s *BasicService) StreamTags(ctx context.Context, visit func(tags []string) error, opts ...registry.ListTagsOption) error {
	entry := s.logger.With(slog.String("service", s.name))

	entry.Debug("Streaming tags")

	if err := s.client.StreamTags(ctx, visit, opts...); err != nil {
		return s.listError("list tags of %s", err)
	}

	entry.Debug("Tags streamed successfully")

	return nil
}

// ListRepositories lists the repositories the registry catalog exposes.
//
// Registries that do not implement the catalog endpoint — Docker Hub, GCR and
// Artifact Registry among them — report ErrCatalogNotSupported.
func (s *BasicService) ListRepositories(ctx context.Context, opts ...registry.ListRepositoriesOption) ([]string, error) {
	entry := s.logger.With(slog.String("service", s.name))

	entry.Debug("Listing repositories")

	repos, err := s.client.ListRepositories(ctx, opts...)
	if err != nil {
		return nil, s.listError("list repositories of %s", err)
	}

	entry.Debug("Repositories listed successfully", slog.Int("count", len(repos)))

	return repos, nil
}

// StreamRepositories is to ListRepositories what StreamTags is to ListTags:
// visit is called once per page, and ErrStopStreaming ends the walk without
// being reported as a failure.
func (s *BasicService) StreamRepositories(ctx context.Context, visit func(repos []string) error, opts ...registry.ListRepositoriesOption) error {
	entry := s.logger.With(slog.String("service", s.name))

	entry.Debug("Streaming repositories")

	if err := s.client.StreamRepositories(ctx, visit, opts...); err != nil {
		return s.listError("list repositories of %s", err)
	}

	entry.Debug("Repositories streamed successfully")

	return nil
}

// listError annotates a listing failure with the repository it came from.
//
// The client classifies its own failures — ErrRepositoryNotFound,
// ErrAccessDenied, ErrCatalogNotSupported — so the sentinel travels out with
// the error rather than being replaced by it. Only a client that classifies
// nothing gets the transport-level fallback, and even then the original error
// is kept: "not found" that hides which HTTP answer produced it is a bad
// diagnostic.
func (s *BasicService) listError(format string, err error) error {
	if sentinel := notFoundSentinel(err); sentinel != nil {
		return fmt.Errorf(format+": %w: %w", s.Path(), sentinel, err)
	}

	return fmt.Errorf(format+": %w", s.Path(), err)
}

// Sub returns a BasicService for one or more fixed child segments of this
// repository. Fixed segments are declared by the sub-tree packages, so they are
// never empty.
func (s *BasicService) Sub(name string, segments ...string) *BasicService {
	return NewBasicService(name, s.client.WithSegment(segments...), s.logger)
}

// Named returns a BasicService for a caller-supplied path segment (a module,
// package, plugin, extra or security image name).
//
// The name is not validated here: an empty or malformed segment collapses out
// of the path and silently addresses the parent repository, so callers taking
// names from user input or a CR should check them with ValidateName first.
func (s *BasicService) Named(serviceName, segment string) *BasicService {
	return s.Sub(serviceName+"/"+segment, segment)
}
