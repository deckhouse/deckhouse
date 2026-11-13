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

// Package images keeps an index of what this replica's store holds, image by image, and answers
// registry-controller with it — which merges the replicas into the RegistryImage resource.
//
// The index lives in memory and is rebuilt by reading the store: its manifests, and the size of each
// blob they name. The disk is the only source of truth, the index a cache of it as of the last reading,
// and a request never touches the disk. Nothing here reads a blob's content: whether a blob is broken
// is what the integrity checks record, and the index only says which images reference such a blob.
package images

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/integrity"
)

// Set is what the cluster declares: the digests, and what declares each. Known is false when it could
// not be read, and then every image is reported outside the set and none as missing.
type Set struct {
	Known   bool
	Digests map[string]struct{}
	Owners  map[string][]registryimagev1alpha1.ImageOwner
}

// maxCorruptedPerImage is how many broken blobs an image lists.
const maxCorruptedPerImage = 10

// Build reads the store at root into an index. bad are the blobs recorded as broken.
//
// An image is a manifest some repository links, and not one that only an index names: the manifests
// of a multi-platform image are counted in it, not beside it. Its size is everything it consists of,
// each blob once; what removing it would free is the part no other image of the store references.
func Build(root string, set Set, bad map[string]bool) ([]registryimagev1alpha1.IndexedImage, registryv1alpha1.ReplicaImages, error) {
	var counts registryv1alpha1.ReplicaImages

	held, err := fill.HeldManifests(root, "")
	if err != nil {
		return nil, counts, err
	}
	repositories := map[string][]string{}
	for _, manifest := range held {
		repositories[manifest.Digest] = append(repositories[manifest.Digest], manifest.Repository)
	}

	store := &reader{root: root, manifests: map[string]*integrity.Manifest{}, sizes: map[string]int64{}}

	// Which manifests only an index names: they are part of that image.
	children := map[string]bool{}
	for digest := range repositories {
		if manifest := store.manifest(digest); manifest != nil {
			for _, child := range manifest.Children {
				children[child] = true
			}
		}
	}

	type row struct {
		image registryimagev1alpha1.IndexedImage
		blobs []string
	}
	var rows []*row
	references := map[string]int{}
	for digest, repos := range repositories {
		_, declared := set.Digests[digest]
		if children[digest] && !declared {
			continue
		}
		blobs, mediaType, err := store.closure(digest)
		if err != nil {
			return nil, counts, err
		}
		sort.Strings(repos)
		r := &row{blobs: blobs, image: registryimagev1alpha1.IndexedImage{
			Digest:       digest,
			InSet:        declared,
			Owners:       set.Owners[digest],
			Repositories: repos,
			MediaType:    mediaType,
		}}
		for _, blob := range blobs {
			references[blob]++
		}
		rows = append(rows, r)
	}

	images := make([]registryimagev1alpha1.IndexedImage, 0, len(rows)+len(set.Digests))
	for _, r := range rows {
		image := &r.image
		for _, blob := range r.blobs {
			size, present, err := store.size(blob)
			if err != nil {
				return nil, counts, err
			}
			if !present {
				image.MissingBlobs++
				continue
			}
			image.Size += size
			if references[blob] == 1 {
				image.ReclaimableSize += size
			}
			if bad[blob] && len(image.CorruptedBlobs) < maxCorruptedPerImage {
				image.CorruptedBlobs = append(image.CorruptedBlobs, blob)
			}
		}

		switch {
		case len(image.CorruptedBlobs) > 0:
			image.State = registryimagev1alpha1.ImageCorrupted
			counts.Corrupted++
		case image.MissingBlobs > 0:
			image.State = registryimagev1alpha1.ImageIncomplete
		default:
			image.State = registryimagev1alpha1.ImagePresent
		}
		switch {
		case !image.InSet:
			counts.OutOfSet++
		case image.State == registryimagev1alpha1.ImagePresent:
			counts.Present++
		case image.State == registryimagev1alpha1.ImageIncomplete:
			counts.Incomplete++
		}
		images = append(images, *image)
	}

	// What the set declares and no repository links.
	for digest := range set.Digests {
		if _, isHeld := repositories[digest]; isHeld {
			continue
		}
		counts.Missing++
		if !fill.Resolved(digest) {
			// A tag the store could not resolve: no digest to name it by. Counted, not listed.
			continue
		}
		images = append(images, registryimagev1alpha1.IndexedImage{
			Digest: digest,
			State:  registryimagev1alpha1.ImageMissing,
			InSet:  true,
			Owners: set.Owners[digest],
		})
	}

	sort.Slice(images, func(i, j int) bool { return images[i].Digest < images[j].Digest })
	return images, counts, nil
}

// reader reads the store's manifests and the sizes of its blobs, each once.
type reader struct {
	root      string
	manifests map[string]*integrity.Manifest
	sizes     map[string]int64
}

// manifest is the manifest by its digest, nil when the store does not hold its blob or it cannot be
// parsed or read: such a manifest names nothing, and the image it is counts as incomplete or as a blob
// — broken, once a check of the store has read it. One unreadable manifest failing the whole index left
// the listing frozen on the last one built.
func (r *reader) manifest(digest string) *integrity.Manifest {
	if manifest, read := r.manifests[digest]; read {
		return manifest
	}
	manifest, err := integrity.ReadManifest(r.root, digest)
	if err != nil {
		manifest = nil
	}
	r.manifests[digest] = manifest
	return manifest
}

// closure is every blob of the image — its manifest, the manifests of an index, configs and layers —
// sorted, each once, and the media type of its manifest.
func (r *reader) closure(digest string) ([]string, string, error) {
	blobs := map[string]bool{}
	var walk func(digest string, depth int) error
	walk = func(digest string, depth int) error {
		if blobs[digest] {
			return nil
		}
		blobs[digest] = true
		if depth > 4 {
			return nil
		}
		manifest := r.manifest(digest)
		if manifest == nil {
			return nil
		}
		for _, child := range manifest.Children {
			if err := walk(child, depth+1); err != nil {
				return err
			}
		}
		if manifest.Config != "" {
			blobs[manifest.Config] = true
		}
		for _, layer := range manifest.Layers {
			blobs[layer] = true
		}
		return nil
	}
	if err := walk(digest, 0); err != nil {
		return nil, "", err
	}

	mediaType := ""
	if manifest := r.manifest(digest); manifest != nil {
		mediaType = manifest.MediaType
	}
	list := make([]string, 0, len(blobs))
	for blob := range blobs {
		list = append(list, blob)
	}
	sort.Strings(list)
	return list, mediaType, nil
}

// size is the size of a blob's content, and whether the store holds it.
func (r *reader) size(digest string) (int64, bool, error) {
	if size, known := r.sizes[digest]; known {
		return size, size >= 0, nil
	}
	path, ok := integrity.BlobPath(r.root, digest)
	if !ok {
		r.sizes[digest] = -1
		return 0, false, nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		r.sizes[digest] = -1
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("reading the size of %s: %w", digest, err)
	}
	r.sizes[digest] = info.Size()
	return info.Size(), true, nil
}
