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
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/deckhouse/deckhouse/pkg/registry"
)

// Ensure Client implements registry.Client at compile time.
var _ registry.Client = (*Client)(nil)

var ErrImageNotFound = registry.ErrImageNotFound

// isNotFound reports whether err is an HTTP 404 response from the registry.
func isNotFound(err error) bool {
	var transportErr *transport.Error
	return errors.As(err, &transportErr) && transportErr.StatusCode == http.StatusNotFound
}

// Client provides methods to interact with container registries
type Client struct {
	// e.g., "registry.deckhouse.io"
	registryHost string
	// e.g., [deckhouse,ee,modules] (built from chained WithSegment calls)
	segments []string
	// cached joined segments for scope path
	constructedSegments string
	// ensures constructedSegments is computed only once
	constructedSegmentsOnce sync.Once
	// remote options for go-containerregistry. Auth, CA, TLS and proxy
	// settings are baked into these options at construction time and are
	// also reused by puller-based pagination (ListTags / ListRepositories),
	// so we deliberately do not keep a second copy of authn.Authenticator
	// or http.RoundTripper on the struct.
	options []remote.Option
	// insecure flag for HTTP connections
	insecure bool

	timeout time.Duration

	// logger is an interface; a nil-by-default Logger field is replaced by
	// [resolveLogger] with a slog.Default()-backed adapter at construction
	// time, so this is always safe to call.
	logger Logger
}

// New creates a new container registry client using functional options.
//
//	client.New("registry.example.com",
//		client.WithAuth(auth),
//		client.WithCA(caPEM),
//		client.WithTLSSkipVerify(),
//	)
func New(registry string, opts ...Option) *Client {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}

	return NewClientWithOptions(registry, o)
}

// NewClientWithOptions creates a new container registry client with advanced options.
// Prefer New with functional options for new code.
func NewClientWithOptions(host string, opts *Options) *Client {
	logger := resolveLogger(opts.Logger)

	// Normalize host and scheme before building remote options.
	host = strings.TrimSuffix(host, "/")

	opts.Scheme = strings.ToLower(opts.Scheme)
	if opts.Scheme == "http" {
		opts.Insecure = true
	}

	return &Client{
		registryHost: host,
		options:      buildRemoteOptions(opts, logger, resolveTransport(opts)),
		timeout:      opts.Timeout,
		logger:       logger,
		insecure:     opts.Insecure,
	}
}

// nameOptions returns name.Option slice for parsing references
// Includes name.Insecure if the client is configured for HTTP
func (c *Client) nameOptions() []name.Option {
	if c.insecure {
		return []name.Option{name.Insecure}
	}
	return nil
}

// buildReference constructs a full image reference string from the registry path
// and a tag or digest. Handles both tag references ("v1.0.0") and digest
// references ("@sha256:abc..." or "sha256:abc...").
func (c *Client) buildReference(tag string) string {
	fullRegistry := c.GetRegistry()
	if strings.HasPrefix(tag, "@sha256:") {
		return fullRegistry + tag
	}
	if strings.HasPrefix(tag, "sha256:") {
		return fullRegistry + "@" + tag
	}
	return fullRegistry + ":" + tag
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
		insecure:     c.insecure,
		timeout:      c.timeout,
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
	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Getting manifest")

	ref, err := name.ParseReference(c.buildReference(tag), c.nameOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	opts := append([]remote.Option{}, c.options...)
	opts = append(opts, c.withContext(ctx))

	head, err := remote.Head(ref, opts...)
	if err == nil {
		return &head.Digest, nil
	}

	// If HEAD returned 404, don't bother with GET — the image doesn't exist.
	if isNotFound(err) {
		return nil, fmt.Errorf("%w: %w", ErrImageNotFound, err)
	}

	logentry.Debug("HEAD failed, retrying with GET", slog.String("error", err.Error()))

	desc, err := remote.Get(ref, opts...)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: %w", ErrImageNotFound, err)
		}

		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	logentry.Debug("Manifest retrieved successfully")

	return &desc.Digest, nil
}

// GetManifestRaw returns the manifest bytes exactly as the registry served
// them along with the manifest descriptor. The byte slice is the canonical
// form callers want for signature verification, audit logs and jq pipelines.
// The repository is determined by the chained WithSegment() calls.
//
// Returns [ErrImageNotFound] wrapped around the underlying transport error
// when the registry responds with HTTP 404.
func (c *Client) GetManifestRaw(ctx context.Context, tag string) ([]byte, *v1.Descriptor, error) {
	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Getting raw manifest")

	ref, err := name.ParseReference(c.buildReference(tag), c.nameOptions()...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	opts := append([]remote.Option{}, c.options...)
	opts = append(opts, c.withContext(ctx))

	desc, err := remote.Get(ref, opts...)
	if err != nil {
		if isNotFound(err) {
			return nil, nil, fmt.Errorf("%w: %w", ErrImageNotFound, err)
		}
		return nil, nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	logentry.Debug("Raw manifest retrieved successfully", slog.Int("size", len(desc.Manifest)))

	// Copy the descriptor so callers cannot mutate the puller's internal
	// state through the returned pointer.
	d := desc.Descriptor
	return desc.Manifest, &d, nil
}

// GetManifest retrieves the manifest for a specific image tag and returns it
// as a decoded [ManifestResult]. This is a convenient form on top of
// [Client.GetManifestRaw]; for byte-stable manifest bytes prefer
// GetManifestRaw directly.
//
// The repository is determined by the chained WithSegment() calls.
func (c *Client) GetManifest(ctx context.Context, tag string) (registry.ManifestResult, error) {
	raw, desc, err := c.GetManifestRaw(ctx, tag)
	if err != nil {
		return nil, err
	}
	return &ManifestResult{
		rawManifest: raw,
		descriptor:  desc,
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
	getImageOptions := &registry.ImageGetOptions{}

	for _, opt := range opts {
		opt.ApplyToImageGet(getImageOptions)
	}

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Getting image")

	ref, err := name.ParseReference(c.buildReference(tag), c.nameOptions()...)
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
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: %w", ErrImageNotFound, err)
		}

		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	logentry.Debug("Image retrieved successfully")

	return NewImage(img, ref.String()), nil
}

// Push uploads obj (a v1.Image or v1.ImageIndex, anything implementing
// partial.WithRawManifest) under tag and returns its digest. The repository
// is determined by chained WithSegment() calls.
//
// Returning the digest matters: most callers want it for image-refs files,
// audit logs or downstream Helm values, and recomputing it from the obj
// after Push duplicates work the upstream library already did.
func (c *Client) Push(ctx context.Context, tag string, obj partial.WithRawManifest, opts ...registry.ImagePushOption) (v1.Hash, error) {
	pushOptions := &registry.ImagePushOptions{}
	for _, opt := range opts {
		opt.ApplyToImagePush(pushOptions)
	}

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	// A typed-nil v1.Image / v1.ImageIndex arrives here as a non-nil
	// interface wrapping a nil pointer; the type switch below would still
	// dispatch and remote.Write/WriteIndex would panic deep inside
	// go-containerregistry. Reject both shapes up front for a clean error.
	if obj == nil {
		return v1.Hash{}, fmt.Errorf("push %s: object is nil", tag)
	}

	ref, err := name.ParseReference(c.buildReference(tag), c.nameOptions()...)
	if err != nil {
		return v1.Hash{}, fmt.Errorf("failed to parse reference: %w", err)
	}

	remoteOptions := append([]remote.Option{}, c.options...)
	remoteOptions = append(remoteOptions, c.withContext(ctx))

	switch t := obj.(type) {
	case v1.ImageIndex:
		logentry.Debug("Pushing image index")
		if err := remote.WriteIndex(ref, t, remoteOptions...); err != nil {
			return v1.Hash{}, fmt.Errorf("failed to push image index: %w", err)
		}
		digest, err := t.Digest()
		if err != nil {
			return v1.Hash{}, fmt.Errorf("compute index digest: %w", err)
		}
		logentry.Debug("Image index pushed successfully", slog.String("digest", digest.String()))
		return digest, nil
	case v1.Image:
		logentry.Debug("Pushing image")
		if err := remote.Write(ref, t, remoteOptions...); err != nil {
			return v1.Hash{}, fmt.Errorf("failed to push image: %w", err)
		}
		digest, err := t.Digest()
		if err != nil {
			return v1.Hash{}, fmt.Errorf("compute image digest: %w", err)
		}
		logentry.Debug("Image pushed successfully", slog.String("digest", digest.String()))
		return digest, nil
	default:
		return v1.Hash{}, fmt.Errorf("push %s: unsupported type %T", tag, obj)
	}
}

// PushImage pushes a v1.Image to the registry at the specified tag.
//
// Deprecated: use [Client.Push], which dispatches by media type and also
// returns the resulting digest.
func (c *Client) PushImage(ctx context.Context, tag string, img v1.Image, opts ...registry.ImagePushOption) error {
	_, err := c.Push(ctx, tag, img, opts...)
	return err
}

// GetImageConfig retrieves the image config file containing labels and metadata
// The repository is determined by the chained WithSegment() calls
func (c *Client) GetImageConfig(ctx context.Context, tag string) (*v1.ConfigFile, error) {
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

// WithTagsLast sets the pagination cursor; only tags after last are returned.
func WithTagsLast(last string) registry.ListTagsOption {
	return &withTagsLast{last: last}
}

type withTagsLast struct {
	last string
}

func (w *withTagsLast) ApplyToListTags(opts *registry.ListTagsOptions) {
	opts.Last = w.last
}

// WithTagsLimit caps the number of tags returned to n (single page).
func WithTagsLimit(n int) registry.ListTagsOption {
	return &withTagsLimit{n: n}
}

type withTagsLimit struct {
	n int
}

func (w *withTagsLimit) ApplyToListTags(opts *registry.ListTagsOptions) {
	opts.N = w.n
}

// WalkTags streams tag pages from the registry: visit is invoked once per
// page with the filtered subset of tags as the puller returns them. Returning
// a non-nil error from visit stops iteration and that error becomes the
// return value of WalkTags. The underlying puller walks pages transparently
// via Link headers using the upstream auth/redirect/retry stack.
//
// WithTagsLast(tag) drops tags lexicographically <= tag before visit is
// called; WithTagsLimit(n) stops iteration once a total of n tags have been
// visited (combine to "first N tags after T"). Both filters are applied
// client-side because go-containerregistry's puller does not expose the OCI
// Distribution Spec ?last= cursor.
//
// Prefer WalkTags over ListTags for repositories with thousands of tags so
// callers can stream/filter/log without buffering the full slice.
func (c *Client) WalkTags(ctx context.Context, visit func(tags []string) error, opts ...registry.ListTagsOption) error {
	listOptions := &registry.ListTagsOptions{}
	for _, opt := range opts {
		opt.ApplyToListTags(listOptions)
	}

	c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.Int("limit", listOptions.N),
		slog.String("last", listOptions.Last),
	).Debug("Walking tags")

	ref, err := name.ParseReference(c.GetRegistry(), c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	repo := ref.Context()

	remoteOpts := append([]remote.Option{}, c.options...)
	remoteOpts = append(remoteOpts, c.withContext(ctx))

	// PageSize is forwarded to the registry as ?n=N only when no Last filter
	// is set. Otherwise we need to walk past server-side tags <= Last and
	// keep collecting until we have N post-filter results, which is unrelated
	// to per-page count.
	if listOptions.N > 0 && listOptions.Last == "" {
		remoteOpts = append(remoteOpts, remote.WithPageSize(listOptions.N))
	}

	puller, err := remote.NewPuller(remoteOpts...)
	if err != nil {
		return fmt.Errorf("create puller: %w", err)
	}

	lister, err := puller.Lister(ctx, repo)
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}

	visited := 0
	for lister.HasNext() {
		if err := ctx.Err(); err != nil {
			return err
		}
		page, err := lister.Next(ctx)
		if err != nil {
			return fmt.Errorf("read tag page: %w", err)
		}

		filtered := applyLastAndLimit(page.Tags, listOptions.Last, listOptions.N, visited)
		if len(filtered) > 0 {
			if err := visit(filtered); err != nil {
				return err
			}
			visited += len(filtered)
		}

		// Limit reached: stop. Same condition as the per-page limit inside
		// applyLastAndLimit, but we have to check after visit() to honour
		// "exactly N" rather than "first page only".
		if listOptions.N > 0 && visited >= listOptions.N {
			return nil
		}
		// Without a Last filter, a limit means "first page only" since the
		// registry already capped the page at N via WithPageSize.
		if listOptions.N > 0 && listOptions.Last == "" {
			return nil
		}
	}
	return nil
}

// applyLastAndLimit filters a single page of tags/repos by the Last cursor
// and caps it at the remaining quota toward N (when N > 0 and alreadyVisited
// is the running total handed out before this page).
//
// Returns a fresh slice; the input is never aliased so callers can safely
// pass page.Tags from a puller (which may reuse its internal buffer).
func applyLastAndLimit(items []string, last string, n, alreadyVisited int) []string {
	if last == "" && n <= 0 {
		// Cheap path: still copy so we never alias the puller's buffer.
		out := make([]string, len(items))
		copy(out, items)
		return out
	}
	remaining := -1
	if n > 0 {
		remaining = n - alreadyVisited
		if remaining <= 0 {
			return nil
		}
	}
	out := make([]string, 0, len(items))
	for _, t := range items {
		if last != "" && t <= last {
			continue
		}
		out = append(out, t)
		if remaining > 0 && len(out) >= remaining {
			break
		}
	}
	return out
}

// ListTags returns tags for the repository built by WithSegment calls. Thin
// accumulating wrapper around [Client.WalkTags]; for large repositories
// prefer WalkTags so pages can stream through the caller's pipeline without
// buffering the full slice.
func (c *Client) ListTags(ctx context.Context, opts ...registry.ListTagsOption) ([]string, error) {
	var tags []string
	if err := c.WalkTags(ctx, func(page []string) error {
		tags = append(tags, page...)
		return nil
	}, opts...); err != nil {
		return nil, err
	}
	c.logger.Debug("Tags listed", slog.Int("count", len(tags)))
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

// WalkRepositories streams /v2/_catalog pages: visit is invoked once per
// page with the filtered subset of repositories. Returning a non-nil error
// from visit stops iteration and that error is returned. The underlying
// puller.Catalogger walks pages transparently via Link headers using the
// upstream auth/redirect/retry stack.
//
// Filtering rules for WithReposLast / WithReposLimit mirror [Client.WalkTags].
//
// Note: /v2/_catalog is not implemented by every registry (Docker Hub does
// not expose it, GHCR returns empty). When unsupported, the underlying fetch
// surfaces a 404 / 401 through the error chain.
func (c *Client) WalkRepositories(ctx context.Context, visit func(repos []string) error, opts ...registry.ListRepositoriesOption) error {
	listOptions := &registry.ListRepositoriesOptions{}
	for _, opt := range opts {
		opt.ApplyToListRepositories(listOptions)
	}

	c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.Int("limit", listOptions.N),
		slog.String("last", listOptions.Last),
	).Debug("Walking repositories")

	// name.NewRegistry is the right entry point for /_catalog: it accepts a
	// bare host[:port], whereas name.ParseReference would interpret host:port
	// as "image:tag" and silently fall back to Docker Hub.
	reg, err := name.NewRegistry(c.registryHost, c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse registry: %w", err)
	}

	remoteOpts := append([]remote.Option{}, c.options...)
	remoteOpts = append(remoteOpts, c.withContext(ctx))
	if listOptions.N > 0 && listOptions.Last == "" {
		remoteOpts = append(remoteOpts, remote.WithPageSize(listOptions.N))
	}

	puller, err := remote.NewPuller(remoteOpts...)
	if err != nil {
		return fmt.Errorf("create puller: %w", err)
	}

	catalogger, err := puller.Catalogger(ctx, reg)
	if err != nil {
		return fmt.Errorf("read catalog: %w", err)
	}

	visited := 0
	for catalogger.HasNext() {
		if err := ctx.Err(); err != nil {
			return err
		}
		page, err := catalogger.Next(ctx)
		if err != nil {
			return fmt.Errorf("read catalog page: %w", err)
		}

		filtered := applyLastAndLimit(page.Repos, listOptions.Last, listOptions.N, visited)
		if len(filtered) > 0 {
			if err := visit(filtered); err != nil {
				return err
			}
			visited += len(filtered)
		}

		if listOptions.N > 0 && visited >= listOptions.N {
			return nil
		}
		if listOptions.N > 0 && listOptions.Last == "" {
			return nil
		}
	}
	return nil
}

// ListRepositories lists repositories visible from the registry. Thin
// accumulating wrapper around [Client.WalkRepositories]; for catalogs with
// thousands of repositories prefer WalkRepositories so pages can stream
// through the caller's pipeline.
func (c *Client) ListRepositories(ctx context.Context, opts ...registry.ListRepositoriesOption) ([]string, error) {
	var repos []string
	if err := c.WalkRepositories(ctx, func(page []string) error {
		repos = append(repos, page...)
		return nil
	}, opts...); err != nil {
		return nil, err
	}
	c.logger.Debug("Repositories listed", slog.Int("count", len(repos)))
	return repos, nil
}

// ImageExists reports whether tag resolves to an existing manifest in the
// registry. A 404 is normalised to (false, nil); any other transport-level
// problem is surfaced via the error return so the caller can distinguish
// "not there" from "could not check".
//
// HEAD is tried first since most registries respond with just the digest +
// size headers; on transport problems other than 404 (e.g. registries that
// reject HEAD on manifests) we fall back to a GET so flakiness against
// idiosyncratic registries does not turn into spurious "not found"s.
//
// The repository is determined by the chained WithSegment() calls.
func (c *Client) ImageExists(ctx context.Context, tag string) (bool, error) {
	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Checking if image exists")

	ref, err := name.ParseReference(c.buildReference(tag), c.nameOptions()...)
	if err != nil {
		return false, fmt.Errorf("failed to parse reference: %w", err)
	}

	opts := append([]remote.Option{}, c.options...)
	opts = append(opts, c.withContext(ctx))

	if _, err := remote.Head(ref, opts...); err == nil {
		logentry.Debug("Image exists")
		return true, nil
	} else if isNotFound(err) {
		return false, nil
	} else {
		logentry.Debug("HEAD failed, retrying with GET", slog.String("error", err.Error()))
	}

	if _, err := remote.Get(ref, opts...); err == nil {
		logentry.Debug("Image exists")
		return true, nil
	} else if isNotFound(err) {
		return false, nil
	} else {
		return false, err
	}
}

// CheckImageExists checks if a specific image exists in the registry.
//
// Deprecated: use [Client.ImageExists]. CheckImageExists returns nil for
// "exists" and [ErrImageNotFound] for "does not exist", which inverts the
// usual Go convention and makes call sites harder to read.
func (c *Client) CheckImageExists(ctx context.Context, tag string) error {
	exists, err := c.ImageExists(ctx, tag)
	if err != nil {
		return err
	}
	if !exists {
		return ErrImageNotFound
	}
	return nil
}

// DeleteTag deletes a specific tag from the registry.
// Returns ErrImageNotFound if the tag does not exist.
// The repository is determined by the chained WithSegment() calls.
func (c *Client) DeleteTag(ctx context.Context, tag string) error {
	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Deleting tag")

	ref, err := name.ParseReference(c.buildReference(tag), c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}

	opts := append([]remote.Option{}, c.options...)
	opts = append(opts, c.withContext(ctx))

	if err := remote.Delete(ref, opts...); err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%w: %w", ErrImageNotFound, err)
		}

		return fmt.Errorf("failed to delete tag: %w", err)
	}

	logentry.Debug("Tag deleted successfully")

	return nil
}

// TagImage adds a new tag pointing to the same manifest as sourceTag without
// re-uploading any layers. This is a single manifest PUT — the standard
// promotion pattern (e.g. :latest → :v1.2.3).
// The repository is determined by the chained WithSegment() calls.
func (c *Client) TagImage(ctx context.Context, sourceTag, destTag string) error {
	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("source_tag", sourceTag),
		slog.String("dest_tag", destTag),
	)

	logentry.Debug("Retagging image")

	srcRef, err := name.ParseReference(c.buildReference(sourceTag), c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse source reference: %w", err)
	}

	opts := append([]remote.Option{}, c.options...)
	opts = append(opts, c.withContext(ctx))

	// Fetch the manifest descriptor without downloading any layers.
	desc, err := remote.Get(srcRef, opts...)
	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%w: %w", ErrImageNotFound, err)
		}

		return fmt.Errorf("failed to get source manifest: %w", err)
	}

	dstTag, err := name.NewTag(c.buildReference(destTag), c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse destination tag: %w", err)
	}

	// remote.Tag performs a single manifest PUT with the same bytes — no layer uploads.
	if err := remote.Tag(dstTag, desc, opts...); err != nil {
		return fmt.Errorf("failed to tag image: %w", err)
	}

	logentry.Debug("Image retagged successfully", slog.String("dest_tag", destTag))

	return nil
}

// PushIndex pushes a v1.ImageIndex (multi-arch manifest list) to the registry
// at the specified tag.
//
// Deprecated: use [Client.Push], which dispatches by media type and also
// returns the resulting digest.
func (c *Client) PushIndex(ctx context.Context, tag string, idx v1.ImageIndex, opts ...registry.ImagePushOption) error {
	_, err := c.Push(ctx, tag, idx, opts...)
	return err
}

// DeleteByDigest deletes a manifest by its digest from the registry.
// The repository is determined by the chained WithSegment() calls.
func (c *Client) DeleteByDigest(ctx context.Context, digest v1.Hash) error {
	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("digest", digest.String()),
	)

	logentry.Debug("Deleting manifest by digest")

	ref, err := name.ParseReference(c.GetRegistry()+"@"+digest.String(), c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse digest reference: %w", err)
	}

	opts := append([]remote.Option{}, c.options...)
	opts = append(opts, c.withContext(ctx))

	if err := remote.Delete(ref, opts...); err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%w: %w", ErrImageNotFound, err)
		}

		return fmt.Errorf("failed to delete manifest: %w", err)
	}

	logentry.Debug("Manifest deleted successfully")

	return nil
}

// CopyImage copies an image (or multi-arch image index) from this client's
// repository to dest's repository.
//
// When source and destination are on the same registry host and the
// destination's credentials carry pull access to the source repository
// (always true when dest is derived from this client via WithSegment),
// go-containerregistry attempts a server-side cross-repository blob mount:
// remote.Get(...).Image()/.ImageIndex() returns objects whose layers are
// wrapped in remote.MountableLayer with the source name.Reference baked in,
// and remote.Write recognises that wrapper and issues
// `POST /v2/<dst>/blobs/uploads/?mount=<digest>&from=<src-repo>` per layer.
// Mountable blobs never traverse the local machine. Blobs the destination
// registry cannot mount (cross-host copy, or registry without mount support)
// fall back to a regular pull-from-source / push-to-destination stream.
//
// Multi-arch indices are preserved on the fast path (dest is a *Client) and
// also when dest implements PushIndex; the slow fallback through PushImage
// stays a last-resort for custom dest implementations.
//
// Returns [ErrImageNotFound] if the source tag does not exist.
func (c *Client) CopyImage(ctx context.Context, srcTag string, dest registry.Client, destTag string) error {
	logentry := c.logger.With(
		slog.String("src_registry", c.GetRegistry()),
		slog.String("src_tag", srcTag),
		slog.String("dest_registry", dest.GetRegistry()),
		slog.String("dest_tag", destTag),
	)

	logentry.Debug("Copying image")

	srcRef, err := name.ParseReference(c.buildReference(srcTag), c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse source reference: %w", err)
	}

	opts := append([]remote.Option{}, c.options...)
	opts = append(opts, c.withContext(ctx))

	desc, err := remote.Get(srcRef, opts...)
	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%w: %w", ErrImageNotFound, err)
		}

		return fmt.Errorf("failed to get source image: %w", err)
	}

	// Fast path: dest is *Client - call remote.Write/WriteIndex directly so
	// MountableLayer wrappers from desc.Image()/desc.ImageIndex() reach the
	// upstream uploader and trigger ?mount=<digest>&from=<src> on every blob.
	if destClient, ok := dest.(*Client); ok {
		return c.copyViaRemote(ctx, srcRef, desc, destClient, destTag, logentry)
	}

	// Fallback for custom registry.Client implementations: dispatch by media
	// type to preserve multi-arch indices. The layers still carry the
	// MountableLayer wrapper, so a dest that ultimately calls remote.Write
	// internally (our own Client included) still benefits from mount.
	if desc.MediaType.IsIndex() {
		idx, err := desc.ImageIndex()
		if err != nil {
			return fmt.Errorf("failed to read source image index: %w", err)
		}
		if err := dest.PushIndex(ctx, destTag, idx); err != nil {
			return fmt.Errorf("failed to push index to destination: %w", err)
		}
		logentry.Debug("Image copied successfully")
		return nil
	}

	img, err := desc.Image()
	if err != nil {
		return fmt.Errorf("failed to read source image: %w", err)
	}
	if err := dest.PushImage(ctx, destTag, img); err != nil {
		return fmt.Errorf("failed to push image to destination: %w", err)
	}
	logentry.Debug("Image copied successfully")
	return nil
}

// copyViaRemote drives the *Client → *Client fast path: parses the destination
// reference once and dispatches the upload by media type, keeping the
// MountableLayer hints from desc.Image()/desc.ImageIndex() intact so that
// cross-repository mount kicks in for free.
func (c *Client) copyViaRemote(ctx context.Context, srcRef name.Reference, desc *remote.Descriptor, destClient *Client, destTag string, logentry Logger) error {
	dstRef, err := name.ParseReference(destClient.buildReference(destTag), destClient.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse destination reference: %w", err)
	}

	destOpts := append([]remote.Option{}, destClient.options...)
	destOpts = append(destOpts, destClient.withContext(ctx))

	if desc.MediaType.IsIndex() {
		idx, err := desc.ImageIndex()
		if err != nil {
			return fmt.Errorf("failed to read source image index: %w", err)
		}
		if err := remote.WriteIndex(dstRef, idx, destOpts...); err != nil {
			return fmt.Errorf("failed to write index to destination: %w", err)
		}
	} else {
		img, err := desc.Image()
		if err != nil {
			return fmt.Errorf("failed to read source image: %w", err)
		}
		if err := remote.Write(dstRef, img, destOpts...); err != nil {
			return fmt.Errorf("failed to write image to destination: %w", err)
		}
	}

	logentry.Debug(
		"Image copied successfully",
		slog.String("src_ref", srcRef.String()),
		slog.String("dst_ref", dstRef.String()),
	)
	return nil
}
