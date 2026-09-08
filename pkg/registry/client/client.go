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

const (
	// maxTagsResponseBytes limits the size of a single tags/list JSON response (8 MiB).
	maxTagsResponseBytes = 8 << 20

	// forcedPageSize is the `n` asked for when a registry answered with a page
	// too large to buffer. No `n` is sent otherwise: registries that reject an
	// `n` they do not implement would fail the whole listing, and one that
	// paginates on its own already keeps pages small.
	forcedPageSize = 1000
)

// errPageTooLarge marks a response that exceeded maxTagsResponseBytes, so the
// walk can tell "this registry handed us everything at once" apart from a
// genuinely malformed body and react by demanding pagination.
var errPageTooLarge = errors.New("response too large to buffer")

// sentinelFor maps a registry response onto one of the package's sentinel
// errors, or returns nil when nothing matches.
//
// Callers should not have to re-derive "was this an auth failure" from HTTP
// status codes, OCI error codes and - as consumers ended up doing - substrings
// of the error message. Classifying once, here, where the typed
// *transport.Error is still in hand, keeps errors.Is enough downstream.
//
// Order matters: the OCI error codes are more specific than the status code, so
// a 404 carrying NAME_UNKNOWN is a missing repository rather than a missing
// image, and an authorization failure is never reported as "not found" - a
// token-auth registry denies out-of-scope requests whether or not the target
// exists.
func sentinelFor(err error) error {
	var transportErr *transport.Error
	if !errors.As(err, &transportErr) {
		return nil
	}

	if transportErr.StatusCode == http.StatusUnauthorized || transportErr.StatusCode == http.StatusForbidden {
		return registry.ErrAccessDenied
	}

	for _, diag := range transportErr.Errors {
		switch diag.Code {
		case transport.UnauthorizedErrorCode, transport.DeniedErrorCode:
			return registry.ErrAccessDenied
		case transport.NameUnknownErrorCode:
			return registry.ErrRepositoryNotFound
		case transport.ManifestUnknownErrorCode:
			return registry.ErrImageNotFound
		}
	}

	// A bare 404 with no diagnostic body - what a HEAD response always looks
	// like, since HTTP forbids a body there.
	if transportErr.StatusCode == http.StatusNotFound {
		return registry.ErrImageNotFound
	}

	return nil
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
	// HTTP transports for direct registry requests (StreamTags).
	auth authn.Authenticator
	// keychain is kept for the same reason as auth: the direct HTTP path has to
	// resolve credentials itself, and a client built with WithKeychain has no
	// explicit authenticator to fall back on.
	keychain authn.Keychain
	// userAgent is kept so direct requests carry the same header remote.* sends.
	userAgent string
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
		keychain:      opts.Keychain,
		userAgent:     opts.UserAgent,
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

	// Every field is copied explicitly because Client embeds a sync.Once, which
	// rules out `nc := *c` (go vet's copylocks). Any field added to Client has
	// to be added here too, or it is silently dropped on the first chained call.
	return &Client{
		registryHost:  c.registryHost,
		segments:      append(append([]string(nil), c.segments...), segments...),
		options:       c.options,
		auth:          c.auth,
		keychain:      c.keychain,
		userAgent:     c.userAgent,
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

	// A classified HEAD failure needs no GET retry: a missing or denied target
	// answers the same way to both verbs, so retrying only costs a round trip.
	if sentinel := sentinelFor(err); sentinel != nil {
		return nil, fmt.Errorf("%w: %w", sentinel, err)
	}

	logentry.Debug("HEAD failed, retrying with GET", slog.String("error", err.Error()))

	desc, err := remote.Get(ref, opts...)
	if err != nil {
		if sentinel := sentinelFor(err); sentinel != nil {
			return nil, fmt.Errorf("%w: %w", sentinel, err)
		}

		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	logentry.Debug("Manifest retrieved successfully")

	return &desc.Digest, nil
}

// GetManifest retrieves the manifest for a specific image tag
// The repository is determined by the chained WithSegment() calls
func (c *Client) GetManifest(ctx context.Context, tag string, opts ...registry.ManifestGetOption) (registry.ManifestResult, error) {
	manifestOptions := &registry.ManifestGetOptions{}
	for _, opt := range opts {
		opt.ApplyToManifestGet(manifestOptions)
	}

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

	remoteOpts := append([]remote.Option{}, c.options...)
	remoteOpts = append(remoteOpts, c.withContext(ctx))

	desc, err := remote.Get(ref, remoteOpts...)
	if err != nil {
		if sentinel := sentinelFor(err); sentinel != nil {
			return nil, fmt.Errorf("%w: %w", sentinel, err)
		}

		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}

	// A platform only means something for an index: remote.Get returns the
	// reference as served, so without this the caller asking for linux/arm64
	// would silently receive the index manifest and have to walk it by hand.
	if manifestOptions.Platform != nil && desc.MediaType.IsIndex() {
		return c.childManifest(desc, *manifestOptions.Platform, logentry)
	}

	logentry.Debug("Manifest retrieved successfully")

	return &ManifestResult{
		rawManifest: desc.Manifest,
		descriptor:  &desc.Descriptor,
	}, nil
}

// childManifest resolves an index descriptor down to the manifest of the child
// image matching platform. ErrImageNotFound covers the index that simply has no
// such platform, which is a normal answer rather than a transport failure.
func (c *Client) childManifest(desc *remote.Descriptor, platform v1.Platform, logentry *log.Logger) (registry.ManifestResult, error) {
	idx, err := desc.ImageIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to read index manifest: %w", err)
	}

	indexManifest, err := idx.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to parse index manifest: %w", err)
	}

	for _, child := range indexManifest.Manifests {
		if child.Platform == nil || !child.Platform.Satisfies(platform) {
			continue
		}

		img, err := idx.Image(child.Digest)
		if err != nil {
			return nil, fmt.Errorf("failed to read image %s: %w", child.Digest, err)
		}

		raw, err := img.RawManifest()
		if err != nil {
			return nil, fmt.Errorf("failed to read manifest of %s: %w", child.Digest, err)
		}

		logentry.Debug("Manifest resolved to platform child",
			slog.String("platform", platform.String()),
			slog.String("digest", child.Digest.String()),
		)

		return &ManifestResult{rawManifest: raw, descriptor: &child}, nil
	}

	return nil, fmt.Errorf("%w: index has no manifest for platform %s", registry.ErrImageNotFound, platform.String())
}

type WithPlatform struct {
	Platform *v1.Platform
}

// ApplyToManifestGet lets the same WithPlatform value scope a GetManifest call.
func (w WithPlatform) ApplyToManifestGet(opts *registry.ManifestGetOptions) {
	opts.Platform = w.Platform
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
		if sentinel := sentinelFor(err); sentinel != nil {
			return nil, fmt.Errorf("%w: %w", sentinel, err)
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
// Without options every page of the registry's Link-cursor chain is walked and
// the complete list is returned - never a partial one. WithTagsLimit(n) returns
// at most one page of n tags and WithTagsLast(tag) starts after tag; both can be
// combined. StreamTags is the same walk without accumulating the result.
func (c *Client) ListTags(ctx context.Context, opts ...registry.ListTagsOption) ([]string, error) {
	var tags []string

	err := c.StreamTags(ctx, func(page []string) error {
		tags = append(tags, page...)

		return nil
	}, opts...)
	if err != nil {
		return nil, err
	}

	c.logger.Debug("Tags listed", slog.Int("count", len(tags)))

	return tags, nil
}

// StreamTags invokes visit once per page of tags as it arrives.
//
// This is the single tag-listing path: ListTags is a thin accumulator on top,
// so both share the guarantees below.
//
// The `n` query parameter is sent only when WithTagsLimit asked for a page size.
// Registries that reject an `n` they do not implement answer 400 and would fail
// the whole listing, and asking for large pages buys nothing when the cursor is
// followed to the end anyway.
//
// Returning ErrStopStreaming from visit ends the walk cleanly; any other error
// from visit is propagated unchanged.
func (c *Client) StreamTags(ctx context.Context, visit func(tags []string) error, opts ...registry.ListTagsOption) error {
	listOptions := &registry.ListTagsOptions{}
	for _, opt := range opts {
		opt.ApplyToListTags(listOptions)
	}

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.Int("limit", listOptions.N),
		slog.String("last", listOptions.Last),
	)

	logentry.Debug("Streaming tags")

	ref, err := name.ParseReference(c.GetRegistry(), c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	repo := ref.Context()

	if c.timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	httpClient, err := c.registryHTTPClient(ctx, repo.Registry, repo)
	if err != nil {
		// The /v2/ ping happens here, so this is where a rejected credential
		// surfaces - classify it rather than burying a 401 in "create client".
		if sentinel := sentinelFor(err); sentinel != nil {
			return fmt.Errorf("%w: %w", sentinel, err)
		}

		return fmt.Errorf("create registry client: %w", err)
	}

	var cursors cursorTracker

	pageURL := tagsURL(repo, listOptions.Last, listOptions.N)
	pages := 0

	for pageURL != "" {
		if err := ctx.Err(); err != nil {
			return err
		}

		tags, next, err := c.fetchTagsPageBuffered(ctx, httpClient, pageURL, logentry)
		if err != nil {
			if sentinel := sentinelFor(err); sentinel != nil {
				return fmt.Errorf("%w: %w", sentinel, err)
			}

			return fmt.Errorf("list tags for %s: %w", repo, err)
		}

		pages++

		if err := visit(tags); err != nil {
			if errors.Is(err, registry.ErrStopStreaming) {
				logentry.Debug("Tag streaming stopped by caller", slog.Int("pages", pages))

				return nil
			}

			return err
		}

		// WithTagsLimit asks for exactly one page; continuing is the caller's
		// business, via WithTagsLast on the next call.
		if listOptions.N > 0 {
			break
		}

		if err := cursors.next(next); err != nil {
			return fmt.Errorf("list tags for %s: %w", repo, err)
		}

		pageURL = next
	}

	logentry.Debug("Tag streaming finished", slog.Int("pages", pages))

	return nil
}

// fetchTagsPageBuffered fetches one page, retrying with an explicit `n` when the
// registry answered with more than this client can buffer.
//
// Ordering matters: the plain request comes first because it is the compatible
// one, and `n` is added only once a registry has proved it needs to be told to
// paginate. A registry that ignores `n` fails the retry too, and then the
// original size error is what the caller sees.
func (c *Client) fetchTagsPageBuffered(ctx context.Context, httpClient *http.Client, pageURL string, logentry *log.Logger) ([]string, string, error) {
	tags, next, err := c.fetchTagsPage(ctx, httpClient, pageURL)
	if !errors.Is(err, errPageTooLarge) {
		return tags, next, err
	}

	paged, addErr := withPageSizeParam(pageURL, forcedPageSize)
	if addErr != nil {
		return nil, "", err
	}

	if paged == pageURL {
		return nil, "", err
	}

	logentry.Debug("Tags page too large, asking the registry to paginate",
		slog.Int("n", forcedPageSize),
	)

	tags, next, retryErr := c.fetchTagsPage(ctx, httpClient, paged)
	if retryErr != nil {
		// Report the original oversize failure: a registry that ignores `n`
		// answers the retry the same way, and "response too large" names the
		// actual problem better than a second copy of it.
		return nil, "", err
	}

	return tags, next, nil
}

// withPageSizeParam returns pageURL with n set, or pageURL unchanged when it
// already carries one.
func withPageSizeParam(pageURL string, size int) (string, error) {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}

	q := parsed.Query()
	if q.Get("n") != "" {
		return pageURL, nil
	}

	q.Set("n", strconv.Itoa(size))
	parsed.RawQuery = q.Encode()

	return parsed.String(), nil
}

// scopedResource is what both name.Repository and name.Registry satisfy: an
// authn.Resource that can also name its own registry auth scope.
type scopedResource interface {
	authn.Resource
	Scope(string) string
}

// registryHTTPClient creates an authenticated HTTP client for direct registry requests.
//
// Credentials resolve the way buildRemoteOptions hands them to remote.*: an
// explicit authenticator wins, otherwise the keychain is resolved for this
// repository. Without the keychain branch the direct path went out anonymous
// for every client built with WithKeychain, so a private registry answered 401.
//
// The transport is then wrapped the way remote.* wraps its own, so direct
// requests keep retry-on-temporary-failure and the configured User-Agent.
func (c *Client) registryHTTPClient(ctx context.Context, reg name.Registry, target scopedResource) (*http.Client, error) {
	auth := c.auth

	if auth == nil && c.keychain != nil {
		resolved, err := c.keychain.Resolve(target)
		if err != nil {
			return nil, fmt.Errorf("resolve credentials for %s: %w", target, err)
		}

		auth = resolved
	}

	if auth == nil {
		auth = authn.Anonymous
	}

	// The scope comes from the target: a repository asks for repository:<name>:pull,
	// a registry for registry:catalog:*. Asking for the wrong one gets a token
	// the registry then refuses to honour.
	rt, err := transport.NewWithContext(ctx, reg, auth, c.baseTransport, []string{target.Scope(transport.PullScope)})
	if err != nil {
		return nil, fmt.Errorf("build transport: %w", err)
	}

	rt = transport.NewRetry(rt)

	if c.userAgent != "" {
		rt = transport.NewUserAgent(rt, c.userAgent)
	}

	return &http.Client{Transport: rt}, nil
}

// cursorTracker refuses a pagination cursor that has already been followed.
//
// A registry that ignores `last` and echoes the same Link cursor on every
// response keeps a walk going forever, re-delivering the same page. A cursor
// already followed cannot make progress, so refusing it turns a broken registry
// into an error instead of a hang. Shared by the tag and catalog walks so the
// two cannot drift apart on it.
type cursorTracker struct {
	seen map[string]struct{}
}

func (t *cursorTracker) next(cursor string) error {
	if cursor == "" {
		return nil
	}

	if t.seen == nil {
		t.seen = make(map[string]struct{})
	}

	if _, dup := t.seen[cursor]; dup {
		return fmt.Errorf("registry keeps returning the same pagination cursor %q, refusing to loop", cursor)
	}

	t.seen[cursor] = struct{}{}

	return nil
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

	// Read with one byte of headroom so an oversized page is reported as such:
	// decoding a stream cut at the limit fails with "unexpected EOF", which
	// tells the user nothing about what actually went wrong.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTagsResponseBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read response: %w", err)
	}

	if int64(len(body)) > maxTagsResponseBytes {
		return nil, "", fmt.Errorf("%w: tags response exceeds %d bytes", errPageTooLarge, maxTagsResponseBytes)
	}

	var parsed tagsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, "", fmt.Errorf("decode response: %w", err)
	}

	return parsed.Tags, nextPageURL(resp), nil
}

// nextPageURL extracts the rel="next" URL from a Link header.
//
// RFC 8288 permits several comma-separated values in one header, and registries
// do use that - a page in the middle of a listing may advertise both prev and
// next. Taking the first <...> would then follow prev and walk the listing
// backwards, re-reading pages already seen, so each value is matched on its own
// rel parameter.
func nextPageURL(resp *http.Response) string {
	for _, value := range splitLinkValues(resp.Header.Get("Link")) {
		target, ok := parseLinkValue(value)
		if !ok {
			continue
		}

		linkURL, err := url.Parse(target)
		if err != nil {
			continue
		}

		if resp.Request != nil && resp.Request.URL != nil {
			linkURL = resp.Request.URL.ResolveReference(linkURL)
		}

		return linkURL.String()
	}

	return ""
}

// splitLinkValues splits a Link header on the commas that separate values,
// ignoring those inside the angle-bracketed URI or a quoted parameter.
func splitLinkValues(header string) []string {
	var (
		values   []string
		start    int
		inURI    bool
		inQuotes bool
	)

	for i, r := range header {
		switch r {
		case '<':
			inURI = true
		case '>':
			inURI = false
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inURI && !inQuotes {
				values = append(values, header[start:i])
				start = i + 1
			}
		}
	}

	return append(values, header[start:])
}

// parseLinkValue returns the URI of a single Link value when it carries
// rel="next" (quoted or bare, per RFC 8288's case-insensitive rel matching).
func parseLinkValue(value string) (string, bool) {
	value = strings.TrimSpace(value)

	if !strings.HasPrefix(value, "<") {
		return "", false
	}

	end := strings.Index(value, ">")
	if end == -1 {
		return "", false
	}

	uri, params := value[1:end], value[end+1:]

	for _, param := range strings.Split(params, ";") {
		key, val, found := strings.Cut(param, "=")
		if !found || !strings.EqualFold(strings.TrimSpace(key), "rel") {
			continue
		}

		if strings.EqualFold(strings.Trim(strings.TrimSpace(val), `"`), "next") {
			return uri, true
		}
	}

	return "", false
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
	var repos []string

	err := c.StreamRepositories(ctx, func(page []string) error {
		repos = append(repos, page...)

		return nil
	}, opts...)
	if err != nil {
		return nil, err
	}

	c.logger.Debug("Repositories listed", slog.Int("count", len(repos)))

	return repos, nil
}

// StreamRepositories invokes visit once per page of the registry catalog.
//
// It mirrors StreamTags, including the refusal to follow a cursor already seen
// and the decision not to send `n` unless the caller asked for a page size.
func (c *Client) StreamRepositories(ctx context.Context, visit func(repos []string) error, opts ...registry.ListRepositoriesOption) error {
	listOptions := &registry.ListRepositoriesOptions{}
	for _, opt := range opts {
		opt.ApplyToListRepositories(listOptions)
	}

	logentry := c.logger.With(
		slog.String("registry_host", c.registryHost),
		slog.String("segments", c.constructedSegments),
		slog.Int("limit", listOptions.N),
		slog.String("last", listOptions.Last),
	)

	logentry.Debug("Streaming repositories")

	// The catalog is registry-wide, so the host is parsed as a registry. Going
	// through name.ParseReference on host+segments - as this used to - reads a
	// bare "host:port" as a repository:tag under the default registry, and the
	// request then went to Docker Hub instead of the configured host.
	reg, err := name.NewRegistry(c.registryHost, c.nameOptions()...)
	if err != nil {
		return fmt.Errorf("failed to parse registry %q: %w", c.registryHost, err)
	}

	if c.timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	httpClient, err := c.registryHTTPClient(ctx, reg, reg)
	if err != nil {
		if sentinel := catalogSentinelFor(err); sentinel != nil {
			return fmt.Errorf("%w: %w", sentinel, err)
		}

		return fmt.Errorf("create registry client: %w", err)
	}

	var cursors cursorTracker

	pageURL := catalogURL(reg, listOptions.Last, listOptions.N)
	pages := 0

	for pageURL != "" {
		if err := ctx.Err(); err != nil {
			return err
		}

		repos, next, err := c.fetchCatalogPage(ctx, httpClient, pageURL)
		if err != nil {
			if sentinel := catalogSentinelFor(err); sentinel != nil {
				return fmt.Errorf("%w: %w", sentinel, err)
			}

			return fmt.Errorf("failed to list repositories: %w", err)
		}

		pages++

		if err := visit(repos); err != nil {
			if errors.Is(err, registry.ErrStopStreaming) {
				logentry.Debug("Repository streaming stopped by caller", slog.Int("pages", pages))

				return nil
			}

			return err
		}

		if listOptions.N > 0 {
			break
		}

		if err := cursors.next(next); err != nil {
			return fmt.Errorf("list repositories for %s: %w", reg, err)
		}

		pageURL = next
	}

	logentry.Debug("Repository streaming finished", slog.Int("pages", pages))

	return nil
}

// catalogSentinelFor classifies a /v2/_catalog failure.
//
// A registry that does not implement the endpoint answers 404 or UNSUPPORTED,
// which the generic classifier would read as a missing image - a misleading
// answer for a request that never named one.
func catalogSentinelFor(err error) error {
	var transportErr *transport.Error
	if !errors.As(err, &transportErr) {
		return nil
	}

	if transportErr.StatusCode == http.StatusNotFound {
		return registry.ErrCatalogNotSupported
	}

	for _, diag := range transportErr.Errors {
		if diag.Code == transport.UnsupportedErrorCode {
			return registry.ErrCatalogNotSupported
		}
	}

	return sentinelFor(err)
}

// catalogURL builds the /v2/_catalog URL with optional last and n parameters.
func catalogURL(reg name.Registry, last string, pageSize int) string {
	uri := &url.URL{
		Scheme: reg.Scheme(),
		Host:   reg.RegistryStr(),
		Path:   "/v2/_catalog",
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

// catalogResponse represents the JSON body of GET /v2/_catalog.
type catalogResponse struct {
	Repositories []string `json:"repositories"`
}

// fetchCatalogPage performs a single GET and returns repositories with the
// next-page URL from the Link header.
func (c *Client) fetchCatalogPage(ctx context.Context, httpClient *http.Client, pageURL string) ([]string, string, error) {
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTagsResponseBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read response: %w", err)
	}

	if int64(len(body)) > maxTagsResponseBytes {
		return nil, "", fmt.Errorf("%w: catalog response exceeds %d bytes", errPageTooLarge, maxTagsResponseBytes)
	}

	var parsed catalogResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, "", fmt.Errorf("decode response: %w", err)
	}

	return parsed.Repositories, nextPageURL(resp), nil
}

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
		if sentinel := sentinelFor(err); sentinel != nil {
			return fmt.Errorf("%w: %w", sentinel, err)
		}

		logentry.Debug("HEAD failed, retrying with GET", log.Err(err))

		_, err = remote.Get(ref, opts...)
	}

	if err != nil {
		if sentinel := sentinelFor(err); sentinel != nil {
			return fmt.Errorf("%w: %w", sentinel, err)
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
		if sentinel := sentinelFor(err); sentinel != nil {
			return fmt.Errorf("%w: %w", sentinel, err)
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
		if sentinel := sentinelFor(err); sentinel != nil {
			return fmt.Errorf("%w: %w", sentinel, err)
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
		if sentinel := sentinelFor(err); sentinel != nil {
			return fmt.Errorf("%w: %w", sentinel, err)
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
		if sentinel := sentinelFor(err); sentinel != nil {
			return fmt.Errorf("%w: %w", sentinel, err)
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
