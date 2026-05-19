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
	"fmt"
	"io"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

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

// GetManifest returns a ManifestResult for the image identified by tag.
func (c *Client) GetManifest(_ context.Context, tag string) (dkpreg.ManifestResult, error) {
	entry, err := c.findImage(tag)
	if err != nil {
		return nil, err
	}
	raw, err := entry.img.RawManifest()
	if err != nil {
		return nil, fmt.Errorf("fake: raw manifest: %w", err)
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

// CheckImageExists returns nil if the image exists or
// [dkpclient.ErrImageNotFound] otherwise.
func (c *Client) CheckImageExists(_ context.Context, tag string) error {
	_, err := c.findImage(tag)
	return err
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

// PushImage stores the image under the current path with the given tag.
// It creates the repository on-the-fly if it does not exist.
func (c *Client) PushImage(_ context.Context, tag string, img v1.Image, _ ...dkpreg.ImagePushOption) error {
	host, repo := c.splitHostRepo()
	if host == "" {
		return fmt.Errorf("fake: PushImage: no registry path set – call WithSegment first")
	}

	reg, ok := c.registries[host]
	if !ok {
		// Auto-create registries for unknown hosts so that push always succeeds.
		reg = NewRegistry(host)
		c.registries[host] = reg
	}

	return reg.AddImage(repo, tag, img)
}

// ListTags returns all tags registered under the current path.
func (c *Client) ListTags(_ context.Context, _ ...dkpreg.ListTagsOption) ([]string, error) {
	host, repo := c.splitHostRepo()
	reg, ok := c.registries[host]
	if !ok {
		return nil, fmt.Errorf("fake: ListTags: registry %q not found", host)
	}
	rs := reg.getRepo(repo)
	if rs == nil {
		return nil, nil
	}
	return rs.listTags(), nil
}

// ListRepositories returns all repository paths registered under the host of
// the current path.  The returned paths are relative to the host.
func (c *Client) ListRepositories(_ context.Context, _ ...dkpreg.ListRepositoriesOption) ([]string, error) {
	host, repoPrefix := c.splitHostRepo()
	reg, ok := c.registries[host]
	if !ok {
		return nil, fmt.Errorf("fake: ListRepositories: registry %q not found", host)
	}

	all := reg.listRepos()
	if repoPrefix == "" {
		return all, nil
	}

	prefix := repoPrefix + "/"
	var filtered []string
	for _, r := range all {
		if strings.HasPrefix(r, prefix) || r == repoPrefix {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
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

// PushIndex stores an image index under the current path with the given tag.
// In the fake implementation this is a no-op that returns nil.
func (c *Client) PushIndex(_ context.Context, _ string, _ v1.ImageIndex, _ ...dkpreg.ImagePushOption) error {
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
	return dest.PushImage(ctx, destTag, entry.img)
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
