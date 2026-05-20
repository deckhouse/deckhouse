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

package fake

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"

	dkpreg "github.com/deckhouse/deckhouse/pkg/registry"
	dkpclient "github.com/deckhouse/deckhouse/pkg/registry/client"
)

// registryImage wraps a v1.Image and adds the Extract() method required by
// [dkpreg.Image].
type registryImage struct {
	v1.Image
}

func (ri *registryImage) Extract() io.ReadCloser {
	return mutate.Extract(ri.Image)
}

// Client is a fully in-memory fake that implements [localreg.Client].
// It routes all operations to the appropriate [Registry] based on the URL
// path built up by [WithSegment] calls.
//
// A typical test initialises a Client like this:
//
//	src := fake.NewRegistry("registry.example.com")
//	src.MustAddImage("deckhouse/ee", "v1.65.0",
//	    fake.NewImageBuilder().
//	        WithFile("version.json", `{"version":"v1.65.0"}`).
//	        MustBuild(),
//	)
//
//	client := fake.NewClient(src)
//	svc := client.WithSegment("deckhouse", "ee")
type Client struct {
	// registries holds all registered [Registry] instances keyed by their host.
	registries map[string]*Registry

	// currentPath is the full path (host + repo segments) accumulated by
	// successive WithSegment calls.  Empty means "no path set yet".
	currentPath string

	// defaultHost is the host used when currentPath is empty.
	// It is set to the first registry's host.
	defaultHost string
}

// NewClient creates a new fake [Client] backed by the supplied registries.
// At least one registry should be provided.  When no current path is set,
// operations use the default host (first registry's host).
func NewClient(registries ...*Registry) *Client {
	c := &Client{
		registries: make(map[string]*Registry, len(registries)),
	}
	for i, r := range registries {
		c.registries[r.Host()] = r
		if i == 0 {
			c.defaultHost = r.Host()
		}
	}
	return c
}

// ----- localreg.Client interface -----

// WithSegment returns a new Client whose currentPath is extended by segments.
func (c *Client) WithSegment(segments ...string) dkpreg.Client {
	base := c.currentPath
	if base == "" {
		base = c.defaultHost
	}

	nc := c.clone()
	if len(segments) == 0 {
		nc.currentPath = base
		return nc
	}
	nc.currentPath = base + "/" + strings.Join(segments, "/")
	return nc
}

// GetRegistry returns the host part of the current path.
func (c *Client) GetRegistry() string {
	host, _ := c.splitHostRepo()
	if host == "" {
		return c.defaultHost
	}
	return host
}

// GetDigest returns the digest of the image identified by tag.
func (c *Client) GetDigest(_ context.Context, tag string) (*v1.Hash, error) {
	entry, err := c.findImage(tag)
	if err != nil {
		return nil, err
	}
	h := entry.digest
	return &h, nil
}

// GetManifestRaw returns the raw manifest bytes and a synthesized descriptor
// for the image identified by tag.
func (c *Client) GetManifestRaw(_ context.Context, tag string) ([]byte, *v1.Descriptor, error) {
	entry, err := c.findImage(tag)
	if err != nil {
		return nil, nil, err
	}
	raw, err := entry.img.RawManifest()
	if err != nil {
		return nil, nil, fmt.Errorf("fake: raw manifest: %w", err)
	}
	mediaType, err := entry.img.MediaType()
	if err != nil {
		return nil, nil, fmt.Errorf("fake: media type: %w", err)
	}
	desc := &v1.Descriptor{
		MediaType: mediaType,
		Size:      int64(len(raw)),
		Digest:    entry.digest,
	}
	return raw, desc, nil
}

// GetManifest returns a ManifestResult for the image identified by tag. Thin
// wrapper around [Client.GetManifestRaw] that wires the bytes + descriptor
// into the decoded form.
func (c *Client) GetManifest(ctx context.Context, tag string) (dkpreg.ManifestResult, error) {
	raw, _, err := c.GetManifestRaw(ctx, tag)
	if err != nil {
		return nil, err
	}
	return dkpclient.NewManifestResultFromBytes(raw), nil
}

// GetImageConfig returns the v1.ConfigFile for the image identified by tag.
func (c *Client) GetImageConfig(_ context.Context, tag string) (*v1.ConfigFile, error) {
	entry, err := c.findImage(tag)
	if err != nil {
		return nil, err
	}
	return entry.img.ConfigFile()
}

// ImageExists reports whether tag resolves to an image in the fake registry.
// Lookup failures other than "not found" are surfaced via the error return.
func (c *Client) ImageExists(_ context.Context, tag string) (bool, error) {
	if _, err := c.findImage(tag); err != nil {
		if errors.Is(err, dkpclient.ErrImageNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CheckImageExists returns nil if the image exists or
// [dkpclient.ErrImageNotFound] otherwise.
//
// Deprecated: use [Client.ImageExists].
func (c *Client) CheckImageExists(ctx context.Context, tag string) error {
	exists, err := c.ImageExists(ctx, tag)
	if err != nil {
		return err
	}
	if !exists {
		return dkpclient.ErrImageNotFound
	}
	return nil
}

// GetImage returns a [dkpreg.Image] for the given tag or digest reference.
// Digest references start with "@sha256:".
func (c *Client) GetImage(_ context.Context, ref string, _ ...dkpreg.ImageGetOption) (dkpreg.Image, error) {
	var entry *imageEntry
	var err error

	if strings.HasPrefix(ref, "@sha256:") {
		entry, err = c.findImageByDigest(strings.TrimPrefix(ref, "@"))
	} else {
		entry, err = c.findImage(ref)
	}
	if err != nil {
		return nil, err
	}
	return &registryImage{Image: entry.img}, nil
}

// Push stores obj under the current path with the given tag and returns its
// digest. v1.Image objects are stored in the in-memory registry; v1.ImageIndex
// objects are accepted but kept opaque (the fake does not yet model indexes).
func (c *Client) Push(_ context.Context, tag string, obj partial.WithRawManifest, _ ...dkpreg.ImagePushOption) (v1.Hash, error) {
	if obj == nil {
		return v1.Hash{}, fmt.Errorf("fake: Push: object is nil")
	}

	host, repo := c.splitHostRepo()
	if host == "" {
		return v1.Hash{}, fmt.Errorf("fake: Push: no registry path set – call WithSegment first")
	}

	reg, ok := c.registries[host]
	if !ok {
		// Auto-create registries for unknown hosts so that push always succeeds.
		reg = NewRegistry(host)
		c.registries[host] = reg
	}

	switch t := obj.(type) {
	case v1.ImageIndex:
		// The fake registry does not store indexes yet; return the digest so
		// callers that depend on Push's contract still work in tests.
		return t.Digest()
	case v1.Image:
		if err := reg.AddImage(repo, tag, t); err != nil {
			return v1.Hash{}, err
		}
		return t.Digest()
	default:
		return v1.Hash{}, fmt.Errorf("fake: Push: unsupported type %T", obj)
	}
}

// PushImage stores the image under the current path with the given tag.
//
// Deprecated: use [Client.Push].
func (c *Client) PushImage(ctx context.Context, tag string, img v1.Image, opts ...dkpreg.ImagePushOption) error {
	_, err := c.Push(ctx, tag, img, opts...)
	return err
}

// PushIndex stores an image index under the current path with the given tag.
//
// Deprecated: use [Client.Push].
func (c *Client) PushIndex(ctx context.Context, tag string, idx v1.ImageIndex, opts ...dkpreg.ImagePushOption) error {
	_, err := c.Push(ctx, tag, idx, opts...)
	return err
}

// WalkTags streams a single in-memory page of tags through visit. The fake
// holds everything in memory, so paging is degenerate; the Last/Limit
// semantics still apply for parity with the real client.
func (c *Client) WalkTags(_ context.Context, visit func(tags []string) error, opts ...dkpreg.ListTagsOption) error {
	listOpts := &dkpreg.ListTagsOptions{}
	for _, opt := range opts {
		opt.ApplyToListTags(listOpts)
	}

	host, repo := c.splitHostRepo()
	reg, ok := c.registries[host]
	if !ok {
		return fmt.Errorf("fake: WalkTags: registry %q not found", host)
	}
	rs := reg.getRepo(repo)
	if rs == nil {
		return nil
	}

	tags := rs.listTags()
	filtered := applyLastAndLimit(tags, listOpts.Last, listOpts.N, 0)
	if len(filtered) == 0 {
		return nil
	}
	return visit(filtered)
}

// ListTags returns all tags registered under the current path. Thin
// accumulating wrapper around [Client.WalkTags].
func (c *Client) ListTags(ctx context.Context, opts ...dkpreg.ListTagsOption) ([]string, error) {
	var tags []string
	if err := c.WalkTags(ctx, func(page []string) error {
		tags = append(tags, page...)
		return nil
	}, opts...); err != nil {
		return nil, err
	}
	return tags, nil
}

// WalkRepositories streams a single in-memory page of repository paths
// (relative to the host) through visit.
func (c *Client) WalkRepositories(_ context.Context, visit func(repos []string) error, opts ...dkpreg.ListRepositoriesOption) error {
	listOpts := &dkpreg.ListRepositoriesOptions{}
	for _, opt := range opts {
		opt.ApplyToListRepositories(listOpts)
	}

	host, repoPrefix := c.splitHostRepo()
	reg, ok := c.registries[host]
	if !ok {
		return fmt.Errorf("fake: WalkRepositories: registry %q not found", host)
	}

	all := reg.listRepos()
	if repoPrefix != "" {
		prefix := repoPrefix + "/"
		filtered := all[:0:0]
		for _, r := range all {
			if strings.HasPrefix(r, prefix) || r == repoPrefix {
				filtered = append(filtered, r)
			}
		}
		all = filtered
	}

	page := applyLastAndLimit(all, listOpts.Last, listOpts.N, 0)
	if len(page) == 0 {
		return nil
	}
	return visit(page)
}

// ListRepositories returns all repository paths registered under the host of
// the current path. Thin accumulating wrapper around [Client.WalkRepositories].
func (c *Client) ListRepositories(ctx context.Context, opts ...dkpreg.ListRepositoriesOption) ([]string, error) {
	var repos []string
	if err := c.WalkRepositories(ctx, func(page []string) error {
		repos = append(repos, page...)
		return nil
	}, opts...); err != nil {
		return nil, err
	}
	return repos, nil
}

// applyLastAndLimit is a tiny helper shared between WalkTags and
// WalkRepositories: it drops items <= last and caps the slice at n-visited.
func applyLastAndLimit(items []string, last string, n, alreadyVisited int) []string {
	if last == "" && n <= 0 {
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

// DeleteTag removes a tag from the current repository.
func (c *Client) DeleteTag(_ context.Context, tag string) error {
	host, repo := c.splitHostRepo()
	reg, ok := c.registries[host]
	if !ok {
		return fmt.Errorf("%w: registry %q", dkpclient.ErrImageNotFound, host)
	}
	rs := reg.getRepo(repo)
	if rs == nil {
		return fmt.Errorf("%w: repo %q", dkpclient.ErrImageNotFound, repo)
	}
	if !rs.deleteTag(tag) {
		return fmt.Errorf("%w: tag %q", dkpclient.ErrImageNotFound, tag)
	}
	return nil
}

// DeleteByDigest removes images matching the given digest from the current
// repository.
func (c *Client) DeleteByDigest(_ context.Context, digest v1.Hash) error {
	host, repo := c.splitHostRepo()
	reg, ok := c.registries[host]
	if !ok {
		return fmt.Errorf("%w: registry %q", dkpclient.ErrImageNotFound, host)
	}
	rs := reg.getRepo(repo)
	if rs == nil {
		return fmt.Errorf("%w: repo %q", dkpclient.ErrImageNotFound, repo)
	}
	if !rs.deleteByDigest(digest.String()) {
		return fmt.Errorf("%w: digest %q", dkpclient.ErrImageNotFound, digest)
	}
	return nil
}

// CopyImage copies an image from this client's repository to a destination
// client's repository.
func (c *Client) CopyImage(ctx context.Context, srcTag string, dest dkpreg.Client, destTag string) error {
	entry, err := c.findImage(srcTag)
	if err != nil {
		return err
	}
	_, err = dest.Push(ctx, destTag, entry.img)
	return err
}

// TagImage copies the manifest of sourceTag to destTag.
func (c *Client) TagImage(_ context.Context, sourceTag, destTag string) error {
	host, repo := c.splitHostRepo()
	reg, ok := c.registries[host]
	if !ok {
		return fmt.Errorf("%w: registry %q", dkpclient.ErrImageNotFound, host)
	}
	rs := reg.getRepo(repo)
	if rs == nil {
		return fmt.Errorf("%w: repo %q", dkpclient.ErrImageNotFound, repo)
	}
	entry, ok := rs.getByTag(sourceTag)
	if !ok {
		return fmt.Errorf("%w: tag %q", dkpclient.ErrImageNotFound, sourceTag)
	}
	return rs.addImage(destTag, entry.img)
}

// ----- internal helpers -----

// clone returns a shallow copy of c with a shared registries map.
func (c *Client) clone() *Client {
	return &Client{
		registries:  c.registries,
		defaultHost: c.defaultHost,
	}
}

// splitHostRepo splits c.currentPath into (host, repoPath).
// On empty currentPath the defaultHost is used and repoPath is "".
func (c *Client) splitHostRepo() (string, string) {
	path := c.currentPath
	if path == "" {
		return c.defaultHost, ""
	}

	// Try to match the longest known registered host prefix first.
	for h := range c.registries {
		if path == h {
			return h, ""
		}
		if strings.HasPrefix(path, h+"/") {
			return h, strings.TrimPrefix(path, h+"/")
		}
	}

	// Fallback: treat everything up to the first "/" as the host.
	idx := strings.Index(path, "/")
	if idx == -1 {
		return path, ""
	}
	return path[:idx], path[idx+1:]
}

// findImage locates an imageEntry by tag within the scope of the current path.
func (c *Client) findImage(tag string) (*imageEntry, error) {
	host, repo := c.splitHostRepo()

	reg, ok := c.registries[host]
	if !ok {
		return nil, fmt.Errorf("%w: registry %q not found", dkpclient.ErrImageNotFound, host)
	}

	rs := reg.getRepo(repo)
	if rs == nil {
		return nil, fmt.Errorf("%w: repository %q not found in %q", dkpclient.ErrImageNotFound, repo, host)
	}

	entry, ok := rs.getByTag(tag)
	if !ok {
		return nil, fmt.Errorf("%w: tag %q not found in %s/%s", dkpclient.ErrImageNotFound, tag, host, repo)
	}
	return entry, nil
}

// findImageByDigest searches all registries and repositories for the given digest.
func (c *Client) findImageByDigest(digestStr string) (*imageEntry, error) {
	for _, reg := range c.registries {
		for _, rp := range reg.listRepos() {
			rs := reg.getRepo(rp)
			if rs == nil {
				continue
			}
			if entry, ok := rs.getByDigest(digestStr); ok {
				return entry, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: digest %q not found", dkpclient.ErrImageNotFound, digestStr)
}
