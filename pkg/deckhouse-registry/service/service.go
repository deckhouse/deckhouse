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

// GetManifest retrieves the raw manifest (or index) for a tag or digest.
func (s *BasicService) GetManifest(ctx context.Context, tag string) (registry.ManifestResult, error) {
	entry := s.Entry(tag)

	entry.Debug("Getting manifest")

	manifest, err := s.client.GetManifest(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("get manifest %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Manifest retrieved successfully")

	return manifest, nil
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

// ListTags returns the tags of this repository.
func (s *BasicService) ListTags(ctx context.Context, opts ...registry.ListTagsOption) ([]string, error) {
	entry := s.logger.With(slog.String("service", s.name))

	entry.Debug("Listing tags")

	tags, err := s.client.ListTags(ctx, opts...)
	if err != nil {
		// A repository that does not exist answers NAME_UNKNOWN or 404 here.
		// Mapping it to the sentinel keeps callers from having to recognize
		// transport-level codes themselves.
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: %s", ErrImageNotFound, s.Path())
		}

		return nil, fmt.Errorf("list tags of %s: %w", s.Path(), err)
	}

	entry.Debug("Tags listed successfully", slog.Int("count", len(tags)))

	return tags, nil
}

// ListRepositories lists the repositories the registry catalog exposes.
func (s *BasicService) ListRepositories(ctx context.Context, opts ...registry.ListRepositoriesOption) ([]string, error) {
	entry := s.logger.With(slog.String("service", s.name))

	entry.Debug("Listing repositories")

	repos, err := s.client.ListRepositories(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("list repositories of %s: %w", s.Path(), err)
	}

	entry.Debug("Repositories listed successfully", slog.Int("count", len(repos)))

	return repos, nil
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
