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

package impl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"

	gcv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	gcv1_types "github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/registry"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/utils/set"
)

var _ registry.Registry = (*EmptyImgRegistry)(nil)

// EmptyImgRegistry implements [registry.Registry] serving synthetic empty images as tags.
// Used to expose sub-repositories as tags on a parent path (GET /v2/<repo>/tags/list).
// The image is created once at construction time; all calls produce the same digest.
type EmptyImgRegistry struct {
	repoTags map[string][]string // sorted tag list (pre-sorted at construction).
	repos    []string            // pre-sorted list of repository names
	img      img
}

// NewEmptyImgRegistry builds a [registry.Registry] from repoTags.
// Returns an error when repoTags is empty or any tag set is empty.
func NewEmptyImgRegistry(repoTags map[string]set.Set[string]) (registry.Registry, error) {
	if len(repoTags) == 0 {
		return nil, fmt.Errorf("no repos provided")
	}

	for repo, tags := range repoTags {
		if tags.Len() == 0 {
			return nil, fmt.Errorf("no tags provided for repo %q", repo)
		}
	}

	img, err := newEmptyImg()
	if err != nil {
		return nil, err
	}

	sorted := make(map[string][]string, len(repoTags))
	repos := make([]string, 0, len(repoTags))
	for repo, tags := range repoTags {
		t := tags.Values()
		slices.Sort(t)
		sorted[repo] = t
		repos = append(repos, repo)
	}
	slices.Sort(repos)

	return &EmptyImgRegistry{repoTags: sorted, repos: repos, img: img}, nil
}

func (l *EmptyImgRegistry) Fetch(ctx context.Context, repo string, dgst digest.Digest) (io.ReadCloser, error) {
	if _, ok := l.repoTags[repo]; !ok {
		return nil, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
	}

	blob, err := l.img.fetch(dgst)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(blob)), ctx.Err()
}

func (l *EmptyImgRegistry) Exists(ctx context.Context, repo string, dgst digest.Digest) (bool, int64, error) {
	if _, ok := l.repoTags[repo]; !ok {
		return false, 0, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
	}

	exist, size, err := l.img.exists(dgst)
	if err != nil {
		return false, 0, err
	}
	return exist, size, ctx.Err()
}

func (l *EmptyImgRegistry) Resolve(ctx context.Context, repo string, reference string) (types.ShortDescriptor, io.ReadCloser, error) {
	if reference == "" {
		return types.ShortDescriptor{}, nil, errs.ErrMissingReference
	}

	tags, ok := l.repoTags[repo]
	if !ok {
		return types.ShortDescriptor{}, nil, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
	}

	if !slices.Contains(tags, reference) && l.img.manifest.Digest.String() != reference {
		return types.ShortDescriptor{}, nil, errs.ErrManifestNotFound
	}

	blob, err := l.img.fetch(l.img.manifest.Digest)
	if err != nil {
		return types.ShortDescriptor{}, nil, err
	}
	return l.img.manifest, io.NopCloser(bytes.NewReader(blob)), ctx.Err()
}

func (l *EmptyImgRegistry) Predecessors(ctx context.Context, repo string, dgst digest.Digest) ([]ocispec.Descriptor, error) {
	if _, ok := l.repoTags[repo]; !ok {
		return nil, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
	}

	if dgst != l.img.manifest.Digest {
		return nil, ctx.Err()
	}

	descs, err := types.Successors(l.img.fetch, l.img.manifest)
	if err != nil {
		return nil, err
	}
	return descs, ctx.Err()
}

func (l *EmptyImgRegistry) SortedTags(ctx context.Context, repo string, last string) ([]string, error) {
	tags, ok := l.repoTags[repo]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errs.ErrUnknownRepository, repo)
	}

	if last == "" {
		return tags, ctx.Err()
	}

	i, found := slices.BinarySearch(tags, last)
	if found {
		i++
	}
	return tags[i:], ctx.Err()
}

func (l *EmptyImgRegistry) SortedRepos() []string {
	return l.repos
}

// img holds pre-computed manifest and config blobs for the synthetic empty img.
type img struct {
	manifest types.ShortDescriptor
	blobs    map[digest.Digest][]byte
}

// newEmptyImg creates a minimal deterministic empty OCI image and pre-computes
// all blobs so they can be served without recomputation on each request.
func newEmptyImg() (img, error) {
	emtpyImg := empty.Image
	emtpyImg = mutate.MediaType(emtpyImg, gcv1_types.OCIManifestSchema1)
	emtpyImg = mutate.ConfigMediaType(emtpyImg, gcv1_types.OCIConfigJSON)

	cfg, err := emtpyImg.ConfigFile()
	if err != nil {
		return img{}, fmt.Errorf("config file: %w", err)
	}

	cfg.Architecture = "amd64"
	cfg.OS = "linux"
	cfg.History = []gcv1.History{
		{
			EmptyLayer: true,
			Created:    cfg.Created,
			Author:     "bundle-registry",
			CreatedBy:  "newEmptyImg",
		},
	}

	emtpyImg, err = mutate.ConfigFile(emtpyImg, cfg)
	if err != nil {
		return img{}, fmt.Errorf("set config: %w", err)
	}

	manifestRaw, err := emtpyImg.RawManifest()
	if err != nil {
		return img{}, fmt.Errorf("raw manifest: %w", err)
	}

	manifestHash, err := emtpyImg.Digest()
	if err != nil {
		return img{}, fmt.Errorf("manifest digest: %w", err)
	}

	manifestMediaType, err := emtpyImg.MediaType()
	if err != nil {
		return img{}, fmt.Errorf("media type: %w", err)
	}

	configRaw, err := emtpyImg.RawConfigFile()
	if err != nil {
		return img{}, fmt.Errorf("raw config: %w", err)
	}

	configHash, err := emtpyImg.ConfigName()
	if err != nil {
		return img{}, fmt.Errorf("config digest: %w", err)
	}

	return img{
		manifest: types.ShortDescriptor{
			MediaType: string(manifestMediaType),
			Digest:    digest.Digest(manifestHash.String()),
			Size:      int64(len(manifestRaw)),
		},
		blobs: map[digest.Digest][]byte{
			digest.Digest(manifestHash.String()): manifestRaw,
			digest.Digest(configHash.String()):   configRaw,
		},
	}, nil
}

func (i *img) fetch(dgst digest.Digest) ([]byte, error) {
	if blob, ok := i.blobs[dgst]; ok {
		return blob, nil
	}
	return nil, fmt.Errorf("%w: %s", errs.ErrBlobNotFound, dgst)
}

func (i *img) exists(dgst digest.Digest) (bool, int64, error) {
	if blob, ok := i.blobs[dgst]; ok {
		return true, int64(len(blob)), nil
	}
	return false, 0, fmt.Errorf("%w: %s", errs.ErrBlobNotFound, dgst)
}
