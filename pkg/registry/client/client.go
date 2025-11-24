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

package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"go.opentelemetry.io/otel"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"
)

const (
	tracerName = "registry-client"
)

var ErrImageNotFound = errors.New("image not found")

// Client provides methods to interact with container registries
type Client struct {
	// e.g., "registry.deckhouse.io"
	registryHost string
	// custom transport for TLS/insecure settings
	transport http.RoundTripper
	// e.g., [deckhouse,ee,modules] (built from chained WithSegment calls)
	segments []string
	// cached joined segments for scope path
	constructedSegments string
	// ensures constructedSegments is computed only once
	constructedSegmentsOnce sync.Once
	// remote options for go-containerregistry
	options []remote.Option

	timeout time.Duration

	logger *log.Logger
}

// NewClientWithOptions creates a new container registry client with advanced options
func NewClientWithOptions(registry string, opts *Options) *Client {
	// Ensure logger first before using it
	logger := ensureLogger(opts.Logger)

	remoteOptions := buildRemoteOptions(opts)

	opts.Insecure = false

	opts.Scheme = strings.ToLower(opts.Scheme)
	if opts.Scheme == "http" {
		opts.Insecure = true
	}

	if opts.TLSSkipVerify {
		logger.Debug("TLS certificate verification disabled",
			slog.String("registry", registry))
	}

	if opts.Insecure {
		logger.Debug("Insecure HTTP mode enabled",
			slog.String("registry", registry))
	}

	registry = strings.TrimSuffix(registry, "/")

	client := &Client{
		registryHost: registry,
		options:      remoteOptions,
		timeout:      opts.Timeout,
		logger:       logger,
	}

	if needsCustomTransport(opts) {
		client.transport = configureTransport(opts)
	}

	return client
}

func (c *Client) withContext(ctx context.Context) remote.Option {
	if c.timeout == 0 {
		c.logger.Debug("Using context without timeout")

		return remote.WithContext(ctx)
	}

	ctxWTO, cancel := context.WithTimeout(ctx, c.timeout)
	// add default timeout to prevent endless request on a huge image
	// Warning!: don't use cancel() in the defer func here. Otherwise *v1.Image outside this function would be inaccessible due to cancelled context, while reading layers, for example.
	_ = cancel

	return remote.WithContext(ctxWTO)
}

// WithSegment creates a new client with an additional scope path segment
// This method can be chained to build complex paths:
// client.WithSegment("deckhouse").WithSegment("ee").WithSegment("modules")
func (c *Client) WithSegment(segments ...string) registry.Client {
	for idx, scope := range segments {
		segments[idx] = strings.TrimSuffix(strings.TrimPrefix(scope, "/"), "/")
	}

	if len(segments) == 0 {
		return c
	}

	return &Client{
		registryHost: c.registryHost,
		segments:     append(append([]string(nil), c.segments...), segments...),
		options:      c.options,
		logger:       c.logger,
		transport:    c.transport,
	}
}

// GetRegistry returns the full registry path (host + scope)
func (c *Client) GetRegistry() string {
	if len(c.segments) == 0 {
		return c.registryHost
	}

	c.constructedSegmentsOnce.Do(func() {
		c.constructedSegments = path.Join(c.segments...)
	})

	return path.Join(c.registryHost, c.constructedSegments)
}

// The repository is determined by the chained WithSegment() calls
func (c *Client) GetDigest(ctx context.Context, tag string) (*v1.Hash, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "GetDigest")
	defer span.End()

	fullRegistry := c.GetRegistry()

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Getting manifest")

	ref, err := name.ParseReference(fullRegistry + ":" + tag)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	opts := append([]remote.Option{}, c.options...)
	opts = append(opts, c.withContext(ctx))

	head, err := remote.Head(ref, opts...)
	if err == nil {
		return &head.Digest, nil
	}

	desc, err := remote.Get(ref, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	logentry.Debug("Manifest retrieved successfully")

	return &desc.Digest, nil
}

// GetManifest retrieves the manifest for a specific image tag
// The repository is determined by the chained WithSegment() calls
func (c *Client) GetManifest(ctx context.Context, tag string) (registry.ManifestResult, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "GetManifest")
	defer span.End()

	fullRegistry := c.GetRegistry()

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Getting manifest")

	ref, err := name.ParseReference(fullRegistry + ":" + tag)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	opts := append([]remote.Option{}, c.options...)
	opts = append(opts, c.withContext(ctx))
	desc, err := remote.Get(ref, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	logentry.Debug("Manifest retrieved successfully")

	return &ManifestResult{
		rawManifest: desc.Manifest,
		descriptor:  &desc.Descriptor,
	}, nil
}

type WithPlatform struct {
	Platform *v1.Platform
}

func (w WithPlatform) ApplyToImageGet(opts *registry.ImageGetOptions) {
	opts.Platform = w.Platform
}

// GetImage retrieves an remote image for a specific reference
// Do not return remote image to avoid drop connection with context cancelation.
// It will be in use while passed context will be alive.
// The repository is determined by the chained WithSegment() calls
func (c *Client) GetImage(ctx context.Context, tag string, opts ...registry.ImageGetOption) (registry.Image, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "GetImage")
	defer span.End()

	getImageOptions := &registry.ImageGetOptions{}

	for _, opt := range opts {
		opt.ApplyToImageGet(getImageOptions)
	}

	fullRegistry := c.GetRegistry()

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Getting image")

	imagepath := fullRegistry + ":" + tag
	if strings.HasPrefix(tag, "@sha256:") {
		logentry.Debug("tag contains digest reference")
		imagepath = fullRegistry + tag
	}

	ref, err := name.ParseReference(imagepath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	imageOptions := []remote.Option{c.withContext(ctx)}
	imageOptions = append(imageOptions, c.options...)

	if getImageOptions.Platform != nil {
		imageOptions = append(imageOptions, remote.WithPlatform(*getImageOptions.Platform))
	}

	img, err := remote.Image(ref, imageOptions...)
	if err != nil {
		var transportErr *transport.Error
		if errors.As(err, &transportErr) && transportErr.StatusCode == http.StatusNotFound {
			// Image not found, which is expected for non-vulnerable images
			return nil, fmt.Errorf("%w: %w", ErrImageNotFound, err)
		}

		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	logentry.Debug("Image retrieved successfully")

	return NewImage(img, ref.String()), nil
}

// PushImage pushes an image to the registry at the specified tag
// The repository is determined by the chained WithSegment() calls
func (c *Client) PushImage(ctx context.Context, tag string, img v1.Image, opts ...registry.ImagePushOption) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "PushImage")
	defer span.End()

	putImageOptions := &registry.ImagePushOptions{}

	for _, opt := range opts {
		opt.ApplyToImagePush(putImageOptions)
	}

	fullRegistry := c.GetRegistry()

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Pushing image")

	ref, err := name.ParseReference(fullRegistry + ":" + tag)
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}

	remoteOptions := append([]remote.Option{}, c.options...)
	remoteOptions = append(remoteOptions, c.withContext(ctx))

	if err := remote.Write(ref, img, remoteOptions...); err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}

	logentry.Debug("Image pushed successfully")

	return nil
}

// GetImageConfig retrieves the image config file containing labels and metadata
// The repository is determined by the chained WithSegment() calls
func (c *Client) GetImageConfig(ctx context.Context, tag string) (*v1.ConfigFile, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "GetImageConfig")
	defer span.End()

	_ = c.GetRegistry()

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Getting image config")

	img, err := c.GetImage(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	configFile, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get image config: %w", err)
	}

	logentry.Debug("Image config retrieved successfully")

	return configFile, nil
}

// WithLast sets the pagination continuation token for tags
func WithTagsLast(last string) registry.ListTagsOption {
	return &withTagsLast{last: last}
}

type withTagsLast struct {
	last string
}

func (w *withTagsLast) ApplyToListTags(opts *registry.ListTagsOptions) {
	opts.Last = w.last
}

// WithTagsLimit sets the maximum number of tag results to return
func WithTagsLimit(n int) registry.ListTagsOption {
	return &withTagsLimit{n: n}
}

type withTagsLimit struct {
	n int
}

func (w *withTagsLimit) ApplyToListTags(opts *registry.ListTagsOptions) {
	opts.N = w.n
}

// ListTags lists tags for the current scope with pagination
// The repository is determined by the chained WithSegment() calls
func (c *Client) ListTags(ctx context.Context, opts ...registry.ListTagsOption) ([]string, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "ListTags")
	defer span.End()

	listOptions := &registry.ListTagsOptions{}

	for _, opt := range opts {
		opt.ApplyToListTags(listOptions)
	}

	fullRegistry := c.GetRegistry()

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.Int("limit", listOptions.N),
		slog.String("last", listOptions.Last),
	)

	logentry.Debug("Listing tags")

	ref, err := name.ParseReference(fullRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	repo := ref.Context()
	remoteOpts := append([]remote.Option{}, c.options...)
	remoteOpts = append(remoteOpts, c.withContext(ctx))

	// Add pagination options
	if listOptions.N > 0 {
		remoteOpts = append(remoteOpts, remote.WithPageSize(listOptions.N))
	}
	if listOptions.Last != "" {
		remoteOpts = append(remoteOpts, remote.WithFilter("last", listOptions.Last))
	}

	// Get tags with server-side pagination
	tags, err := remote.List(repo, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	logentry.Debug("Tags retrieved", slog.Int("returned_count", len(tags)))

	return tags, nil
}

// WithReposLast sets the pagination continuation token for repositories
func WithReposLast(last string) registry.ListRepositoriesOption {
	return &withReposLast{last: last}
}

type withReposLast struct {
	last string
}

func (w *withReposLast) ApplyToListRepositories(opts *registry.ListRepositoriesOptions) {
	opts.Last = w.last
}

// WithReposLimit sets the maximum number of repository results to return
func WithReposLimit(n int) registry.ListRepositoriesOption {
	return &withReposLimit{n: n}
}

type withReposLimit struct {
	n int
}

func (w *withReposLimit) ApplyToListRepositories(opts *registry.ListRepositoriesOptions) {
	opts.N = w.n
}

// ListRepositories lists sub-repositories under the current scope with pagination
// The scope is determined by the chained WithSegment() calls
// Returns repository names under the current scope
func (c *Client) ListRepositories(ctx context.Context, opts ...registry.ListRepositoriesOption) ([]string, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "ListRepositories")
	defer span.End()

	listOptions := &registry.ListRepositoriesOptions{}

	for _, opt := range opts {
		opt.ApplyToListRepositories(listOptions)
	}

	fullRegistry := c.GetRegistry()

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.Int("limit", listOptions.N),
		slog.String("last", listOptions.Last),
	)

	logentry.Debug("Listing repositories")

	ref, err := name.ParseReference(fullRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to parse registry reference: %w", err)
	}

	repo := ref.Context()

	logentry.Debug("Listing repositories for base repository", slog.String("repository", repo.String()))

	remoteOpts := append([]remote.Option{}, c.options...)
	remoteOpts = append(remoteOpts, c.withContext(ctx))

	// Use CatalogPage for server-side pagination if supported
	if listOptions.N > 0 || listOptions.Last != "" {
		repos, err := remote.CatalogPage(repo.Registry, listOptions.Last, listOptions.N, remoteOpts...)
		if err != nil {
			logentry.Debug("Failed to list repositories with pagination", slog.String("error", err.Error()))

			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		logentry.Debug("Repositories retrieved with pagination", slog.Int("returned_count", len(repos)))

		return repos, nil
	}

	// Fallback to regular catalog listing
	result, err := remote.Catalog(ctx, repo.Registry, remoteOpts...)
	if err != nil {
		logentry.Debug("Failed to list repositories", slog.String("error", err.Error()))

		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	logentry.Debug("Repositories retrieved", slog.Int("total_repositories", len(result)))

	return result, nil
}

// CheckImageExists checks if a specific image exists in the registry
// If image not found, return an error
// The repository is determined by the chained WithSegment() calls
func (c *Client) CheckImageExists(ctx context.Context, tag string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "CheckImageExists")
	defer span.End()

	fullRegistry := c.GetRegistry()

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Checking if image exists")

	ref, err := name.ParseReference(fullRegistry + ":" + tag)
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}

	opts := append([]remote.Option{}, c.options...)
	opts = append(opts, c.withContext(ctx))

	_, err = remote.Head(ref, opts...)
	if err != nil {
		var transportErr *transport.Error
		if errors.As(err, &transportErr) && transportErr.StatusCode == http.StatusNotFound {
			// Image not found, which is expected for non-vulnerable images
			return ErrImageNotFound
		}

		logentry.Debug("get Head error", log.Err(err))
	}

	if err != nil {
		_, err = remote.Get(ref, opts...)
	}

	if err != nil {
		var transportErr *transport.Error
		if errors.As(err, &transportErr) && transportErr.StatusCode == http.StatusNotFound {
			// Image not found, which is expected for non-vulnerable images
			return ErrImageNotFound
		}

		logentry.Debug("get Get error", log.Err(err))

		return err
	}

	logentry.Debug("Image exists")

	return nil
}
