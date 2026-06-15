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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/pkg/registry"
)

// Ensure Client implements registry.Client at compile time.
var _ registry.Client = (*Client)(nil)

var ErrImageNotFound = registry.ErrImageNotFound

// maxTagsResponseBytes limits the size of a single tags/list JSON response (8 MiB).
const maxTagsResponseBytes = 8 << 20

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
	// remote options for go-containerregistry
	options []remote.Option
	// auth is stored separately from remote options to build authenticated
	// HTTP transports for direct registry requests (listTagsPage).
	auth authn.Authenticator
	// baseTransport carries CA/TLS/proxy settings for direct HTTP requests.
	baseTransport http.RoundTripper
	// insecure flag for HTTP connections
	insecure bool

	timeout time.Duration

	logger *log.Logger
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

	baseTransport := resolveTransport(opts)

	return &Client{
		registryHost:  host,
		options:       buildRemoteOptions(opts, logger, baseTransport),
		auth:          opts.Auth,
		baseTransport: baseTransport,
		timeout:       opts.Timeout,
		logger:        logger,
		insecure:      opts.Insecure,
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
		registryHost:  c.registryHost,
		segments:      append(append([]string(nil), c.segments...), segments...),
		options:       c.options,
		auth:          c.auth,
		baseTransport: c.baseTransport,
		logger:        c.logger,
		insecure:      c.insecure,
		timeout:       c.timeout,
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

// GetManifest retrieves the manifest for a specific image tag
// The repository is determined by the chained WithSegment() calls
func (c *Client) GetManifest(ctx context.Context, tag string) (registry.ManifestResult, error) {
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

// PushImage pushes an image to the registry at the specified tag
// The repository is determined by the chained WithSegment() calls
func (c *Client) PushImage(ctx context.Context, tag string, img v1.Image, opts ...registry.ImagePushOption) error {
	putImageOptions := &registry.ImagePushOptions{}

	for _, opt := range opts {
		opt.ApplyToImagePush(putImageOptions)
	}

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Pushing image")

	ref, err := name.ParseReference(c.buildReference(tag), c.nameOptions()...)
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

// ListTags returns tags for the repository built by WithSegment calls.
//
// Without options, all tags are returned. WithTagsLimit(n) returns at most one page
// of n tags. WithTagsLast(tag) returns tags lexicographically after tag.
// Both options can be combined.
func (c *Client) ListTags(ctx context.Context, opts ...registry.ListTagsOption) ([]string, error) {
	listOptions := &registry.ListTagsOptions{}
	for _, opt := range opts {
		opt.ApplyToListTags(listOptions)
	}

	c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.Int("limit", listOptions.N),
		slog.String("last", listOptions.Last),
	).Debug("Listing tags")

	ref, err := name.ParseReference(c.GetRegistry(), c.nameOptions()...)
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	repo := ref.Context()

	if listOptions.N > 0 || listOptions.Last != "" {
		if c.timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.timeout)
			defer cancel()
		}

		tags, err := c.listTagsPage(ctx, repo, listOptions.Last, listOptions.N)
		if err != nil {
			return nil, err
		}

		c.logger.Debug("Tags listed", slog.Int("count", len(tags)))

		return tags, nil
	}

	remoteOpts := append(append([]remote.Option{}, c.options...), c.withContext(ctx))
	tags, err := remote.List(repo, remoteOpts...)
	if err != nil {
		return nil, err
	}

	c.logger.Debug("Tags listed", slog.Int("count", len(tags)))

	return tags, nil
}

// listTagsPage fetches a single page of tags (pageSize > 0) or all remaining pages via direct HTTP.
// When pageSize is 0, the registry picks its own page size and all pages are collected via Link headers.
func (c *Client) listTagsPage(ctx context.Context, repo name.Repository, last string, pageSize int) ([]string, error) {
	httpClient, err := c.registryHTTPClient(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("create registry client: %w", err)
	}

	nextURL := tagsURL(repo, last, pageSize)
	var allTags []string

	for nextURL != "" {
		tags, next, err := c.fetchTagsPage(ctx, httpClient, nextURL)
		if err != nil {
			return nil, err
		}

		allTags = append(allTags, tags...)

		if pageSize > 0 {
			return allTags, nil
		}

		nextURL = next
	}

	return allTags, nil
}

// registryHTTPClient creates an authenticated HTTP client for direct registry requests.
func (c *Client) registryHTTPClient(ctx context.Context, repo name.Repository) (*http.Client, error) {
	auth := c.auth
	if auth == nil {
		auth = authn.Anonymous
	}

	rt, err := transport.NewWithContext(ctx, repo.Registry, auth, c.baseTransport, []string{repo.Scope(transport.PullScope)})
	if err != nil {
		return nil, fmt.Errorf("build transport: %w", err)
	}

	return &http.Client{Transport: rt}, nil
}

// tagsURL builds the /v2/<repo>/tags/list URL with optional last and n query parameters.
func tagsURL(repo name.Repository, last string, pageSize int) string {
	uri := &url.URL{
		Scheme: repo.Scheme(),
		Host:   repo.RegistryStr(),
		Path:   fmt.Sprintf("/v2/%s/tags/list", repo.RepositoryStr()),
	}

	q := url.Values{}
	if last != "" {
		q.Set("last", last)
	}

	if pageSize > 0 {
		q.Set("n", strconv.Itoa(pageSize))
	}

	uri.RawQuery = q.Encode()

	return uri.String()
}

// tagsResponse represents the JSON body of GET /v2/<name>/tags/list (OCI Distribution Spec).
type tagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// fetchTagsPage performs a single GET and returns tags with the next-page URL from the Link header.
func (c *Client) fetchTagsPage(ctx context.Context, httpClient *http.Client, pageURL string) ([]string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if err := transport.CheckError(resp, http.StatusOK); err != nil {
		return nil, "", err
	}

	var parsed tagsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxTagsResponseBytes)).Decode(&parsed); err != nil {
		return nil, "", fmt.Errorf("decode response: %w", err)
	}

	return parsed.Tags, nextPageURL(resp), nil
}

// nextPageURL extracts the URL from a Link: <url>; rel="next" header.
// Handles the common OCI registry format only; RFC 8288 multi-value headers are not supported.
func nextPageURL(resp *http.Response) string {
	link := resp.Header.Get("Link")
	if link == "" || link[0] != '<' {
		return ""
	}

	end := strings.Index(link, ">")
	if end == -1 {
		return ""
	}

	linkURL, err := url.Parse(link[1:end])
	if err != nil {
		return ""
	}

	if resp.Request != nil && resp.Request.URL != nil {
		linkURL = resp.Request.URL.ResolveReference(linkURL)
	}

	return linkURL.String()
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

	ref, err := name.ParseReference(fullRegistry, c.nameOptions()...)
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
	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Checking if image exists")

	ref, err := name.ParseReference(c.buildReference(tag), c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}

	opts := append([]remote.Option{}, c.options...)
	opts = append(opts, c.withContext(ctx))

	_, err = remote.Head(ref, opts...)
	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%w: %w", ErrImageNotFound, err)
		}

		logentry.Debug("HEAD failed, retrying with GET", log.Err(err))

		_, err = remote.Get(ref, opts...)
	}

	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%w: %w", ErrImageNotFound, err)
		}

		return err
	}

	logentry.Debug("Image exists")

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

// PushIndex pushes a multi-architecture image index to the registry at the specified tag.
// The repository is determined by the chained WithSegment() calls.
func (c *Client) PushIndex(ctx context.Context, tag string, idx v1.ImageIndex, opts ...registry.ImagePushOption) error {
	pushOptions := &registry.ImagePushOptions{}
	for _, opt := range opts {
		opt.ApplyToImagePush(pushOptions)
	}

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.String("tag", tag),
	)

	logentry.Debug("Pushing image index")

	ref, err := name.ParseReference(c.buildReference(tag), c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}

	remoteOptions := append([]remote.Option{}, c.options...)
	remoteOptions = append(remoteOptions, c.withContext(ctx))

	if err := remote.WriteIndex(ref, idx, remoteOptions...); err != nil {
		return fmt.Errorf("failed to push image index: %w", err)
	}

	logentry.Debug("Image index pushed successfully")

	return nil
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

// CopyImage copies an image from this client's repository to a destination
// client's repository. It fetches the remote descriptor and writes it to the
// destination without pulling layers through the local machine when possible
// (server-side mount). Both source and destination must be accessible.
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

	// If the destination is our concrete Client type, we can use its remote options
	// directly for an efficient server-side copy.
	destClient, ok := dest.(*Client)
	if !ok {
		// Fallback: pull the image and push it via the interface.
		img, err := desc.Image()
		if err != nil {
			return fmt.Errorf("failed to read source image: %w", err)
		}

		return dest.PushImage(ctx, destTag, img)
	}

	dstRef, err := name.ParseReference(destClient.buildReference(destTag), destClient.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse destination reference: %w", err)
	}

	destOpts := append([]remote.Option{}, destClient.options...)
	destOpts = append(destOpts, destClient.withContext(ctx))

	// Use the appropriate write method based on media type.
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

	logentry.Debug("Image copied successfully")

	return nil
}
