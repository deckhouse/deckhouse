/*
Copyright The ORAS Authors.
Copyright 2026 Flant JSC

Modifications made by Flant JSC as part of the Deckhouse project.

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

// Package oci implements [store.Store] for an OCI image layout on [io/fs.FS].
package oci

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"

	"github.com/opencontainers/go-digest"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/store"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
)

var _ store.Store = (*LayoutStore)(nil)

// LayoutStore is a [store.Store] backed by an OCI image layout.
type LayoutStore struct {
	*ContentStore
	tags      *Tags
	manifests *Manifests
}

// NewLayoutStore builds a [LayoutStore] for the OCI layout rooted at fsys.
// It does not verify blobs; call [ValidateLayout] first if you need that.
func NewLayoutStore(fsys fs.FS) (*LayoutStore, error) {
	client := NewContentClient(fsys)
	tags := NewTags()
	manifests := NewManifests()

	index, err := client.ReadIndex()
	if err != nil {
		return nil, fmt.Errorf("index.json: %w", err)
	}

	contentStore := NewContentStore(client)
	for _, desc := range index.Manifests {
		short := types.ShortDescriptor{}
		short.FromDescriptor(desc)
		manifests.Set(desc.Digest, short)

		if tag := types.GetTagFromAnnotation(desc); tag != "" {
			tags.Set(desc.Digest, tag)
		}

		// Reading local files, which nothing cancels.
		if err := registerIndexChildren(context.Background(), contentStore, manifests, short, 0); err != nil {
			return nil, fmt.Errorf("manifest %q: %w", desc.Digest, err)
		}
	}

	return &LayoutStore{
		ContentStore: contentStore,
		tags:         tags,
		manifests:    manifests,
	}, nil
}

// registerIndexChildren makes the manifests an image index points at resolvable by digest.
//
// index.json names an image index by its tag, and a client pulling that tag reads the index, picks the
// child for its platform and asks for it BY DIGEST. Only index.json was registered, so that second
// request found nothing. Children absent from the layout are left out: ValidateLayout has already
// decided which absences are acceptable.
func registerIndexChildren(
	ctx context.Context, contentStore *ContentStore, manifests *Manifests, desc types.ShortDescriptor, depth int,
) error {
	if !types.IsManifestList(desc.MediaType) {
		return nil
	}
	if depth >= maxIndexDepth {
		return fmt.Errorf("image indexes nested deeper than %d", maxIndexDepth)
	}

	children, err := types.ManifestListSuccessors(fetchAll(ctx, contentStore), desc)
	if err != nil {
		return fmt.Errorf("children of index %q: %w", desc.Digest, err)
	}
	for _, child := range children {
		ok, _, err := contentStore.Exists(ctx, child.Digest)
		if err != nil {
			return fmt.Errorf("child %q: %w", child.Digest, err)
		}
		if !ok {
			continue
		}
		short := types.ShortDescriptor{}
		short.FromDescriptor(child)
		manifests.Set(child.Digest, short)
		if err := registerIndexChildren(ctx, contentStore, manifests, short, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// Resolve implements [store.Store].
func (s *LayoutStore) Resolve(ctx context.Context, reference string) (types.ShortDescriptor, io.ReadCloser, error) {
	if reference == "" {
		return types.ShortDescriptor{}, nil, errs.ErrMissingReference
	}

	dgst, err := digest.Parse(reference)
	if err != nil {
		dgst, err = s.tags.Resolve(reference)
	}
	if err != nil {
		return types.ShortDescriptor{}, nil, err
	}

	desc, err := s.manifests.Resolve(dgst)
	if err != nil {
		return types.ShortDescriptor{}, nil, err
	}

	rc, err := s.Fetch(ctx, desc.Digest)
	return desc, rc, err
}

// Predecessors implements [store.Store].
// If dgst is not listed in the index as a manifest, it returns (nil, ctx.Err()).
// Otherwise it returns linked blobs (config, layers, subject) via [store.ManifestSuccessors].
func (s *LayoutStore) Predecessors(ctx context.Context, dgst digest.Digest) ([]ociv1.Descriptor, error) {
	desc, err := s.manifests.Resolve(dgst)
	if err != nil {
		if errors.Is(err, errs.ErrManifestNotFound) {
			return nil, ctx.Err()
		}
		return nil, err
	}
	return types.Successors(fetchAll(ctx, s.ContentStore), desc)
}

// SortedTags implements [store.Store].
func (s *LayoutStore) SortedTags(ctx context.Context, last string) ([]string, error) {
	return tagNames(s.tags, last), ctx.Err()
}

// HasTags implements [store.Store]: true when at least one tag entry was loaded from the index.
func (s *LayoutStore) HasTags() bool {
	return s.tags.Len() != 0
}

// ValidateLayout checks oci-layout version, reads index.json, and ensures every blob
// reachable from each index manifest (via [store.ManifestSuccessors]) exists under fsys.
// An image index is followed into the manifests it lists; see validateDescriptor.
func ValidateLayout(ctx context.Context, fsys fs.FS) error {
	client := NewContentClient(fsys)
	contentStore := NewContentStore(client)

	layout, err := client.ReadLayout()
	if err != nil {
		return fmt.Errorf("oci-layout: %w", err)
	}

	if layout.Version != ociv1.ImageLayoutVersion {
		return fmt.Errorf("oci-layout version %q, want %q", layout.Version, ociv1.ImageLayoutVersion)
	}

	index, err := client.ReadIndex()
	if err != nil {
		return fmt.Errorf("index.json: %w", err)
	}

	for _, manifest := range index.Manifests {
		if err := validateDescriptor(ctx, manifest, contentStore, 0); err != nil {
			return fmt.Errorf("manifest %q: %w", manifest.Digest, err)
		}
	}
	return ctx.Err()
}

const (
	// maxIndexDepth bounds how deeply image indexes may nest. One level is what a multi-platform image
	// is; the bound exists so that an index naming itself ends in an error rather than a stack overflow.
	maxIndexDepth = 4

	// referenceTypeAnnotation and attestationManifest mark the provenance manifest buildx attaches to an
	// image index beside the platform images.
	referenceTypeAnnotation = "vnd.docker.reference.type"
	attestationManifest     = "attestation-manifest"
)

// validateDescriptor checks one entry of a layout: a manifest and every blob it references, or an image
// index and every manifest it lists.
//
// An image index is a legitimate top-level entry, and the bundle has one: `d8 mirror pull` copies the d8
// CLI image (`deckhouse-cli`) as it is published, and since it is built with buildx that is an index of
// one platform image and one attestation manifest. Refused as an "invalid media type", every installation
// from a bundle stopped before it began (`process deckhouse-cli.tar: ... invalid media type
// "application/vnd.oci.image.index.v1+json"`, e2e-registry `bundle` and `dhctl-airgap`, 2026-10-05).
//
// Every platform image an index lists has to be present and complete — that is what a pull of it reads.
// The one absence tolerated is an attestation manifest: it describes how the image was built, no runtime
// pulls it, and a mirror that leaves it behind still serves the image whole.
func validateDescriptor(ctx context.Context, desc ociv1.Descriptor, contentStore *ContentStore, depth int) error {
	if types.IsManifestList(desc.MediaType) {
		return validateIndex(ctx, desc, contentStore, depth)
	}
	return validateManifest(ctx, desc, contentStore)
}

func validateIndex(ctx context.Context, desc ociv1.Descriptor, contentStore *ContentStore, depth int) error {
	if depth >= maxIndexDepth {
		return fmt.Errorf("image indexes nested deeper than %d", maxIndexDepth)
	}

	ok, _, err := contentStore.Exists(ctx, desc.Digest)
	if err != nil {
		return fmt.Errorf("index %q: %w", desc.Digest, err)
	}
	if !ok {
		return fmt.Errorf("index %q: %w", desc.Digest, errs.ErrBlobNotFound)
	}

	shortDesc := types.ShortDescriptor{}
	shortDesc.FromDescriptor(desc)
	children, err := types.ManifestListSuccessors(fetchAll(ctx, contentStore), shortDesc)
	if err != nil {
		return fmt.Errorf("index %q: %w", desc.Digest, err)
	}

	for _, child := range children {
		if child.Annotations[referenceTypeAnnotation] == attestationManifest {
			present, _, err := contentStore.Exists(ctx, child.Digest)
			if err != nil {
				return fmt.Errorf("attestation %q: %w", child.Digest, err)
			}
			if !present {
				continue
			}
		}
		if err := validateDescriptor(ctx, child, contentStore, depth+1); err != nil {
			return fmt.Errorf("in index %q: %w", desc.Digest, err)
		}
	}
	return ctx.Err()
}

func validateManifest(ctx context.Context, desc ociv1.Descriptor, contentStore *ContentStore) error {
	if !types.IsManifest(desc.MediaType) {
		return fmt.Errorf("invalid media type %q", desc.MediaType)
	}

	shortDesc := types.ShortDescriptor{}
	shortDesc.FromDescriptor(desc)
	blobs, err := types.ManifestSuccessors(fetchAll(ctx, contentStore), shortDesc)
	if err != nil {
		return fmt.Errorf("blobs: %w", err)
	}

	for _, blob := range append(blobs, desc) {
		ok, _, err := contentStore.Exists(ctx, blob.Digest)
		if err != nil {
			return fmt.Errorf("blob %q: %w", blob.Digest, err)
		}

		if !ok {
			return fmt.Errorf("blob %q: %w", blob.Digest, errs.ErrBlobNotFound)
		}
	}
	return ctx.Err()
}

// tagNames returns sorted tag names for [Store.Tags], listing rules:
// skip entries where tag equals the digest string; if last is non-empty, omit names ≤ last.
func tagNames(t *Tags, last string) []string {
	var out []string
	for tag, dgst := range t.Map() {
		if tag == dgst.String() {
			continue
		}
		if last != "" && tag <= last {
			continue
		}
		out = append(out, tag)
	}
	slices.Sort(out)
	return out
}

// fetchAll returns a fetch function that reads the full content for the given descriptor from the store.
func fetchAll(ctx context.Context, contentStore store.ContentStore) func(dgst digest.Digest) ([]byte, error) {
	return func(dgst digest.Digest) ([]byte, error) {
		rc, err := contentStore.Fetch(ctx, dgst)
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		ret, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("read failed: %w", err)
		}
		return ret, nil
	}
}
