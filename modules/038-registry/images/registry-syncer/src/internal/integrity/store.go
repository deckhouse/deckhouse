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

package integrity

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"

	"github.com/deckhouse/registry-syncer/internal/fill"
)

// Manifest is what a manifest of the store names, read off its blob.
type Manifest struct {
	// MediaType of the manifest, as it says itself. Empty for one that does not say.
	MediaType string

	// Children are the manifests of an image index.
	Children []string

	// Config and Layers are the blobs of an image manifest.
	Config string
	Layers []string
}

type manifestBody struct {
	MediaType string `json:"mediaType"`
	Manifests []struct {
		Digest string `json:"digest"`
	} `json:"manifests"`
	Config struct {
		Digest string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		Digest string `json:"digest"`
	} `json:"layers"`
}

// ReadManifest reads a manifest out of the store's blobs. Nil when the store does not hold its blob.
func ReadManifest(root, digest string) (*Manifest, error) {
	path, ok := BlobPath(root, digest)
	if !ok {
		return nil, fmt.Errorf("not a digest: %q", digest)
	}
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", digest, err)
	}
	var body manifestBody
	if err := json.Unmarshal(content, &body); err != nil {
		return nil, fmt.Errorf("parsing manifest %s: %w", digest, err)
	}
	manifest := &Manifest{MediaType: body.MediaType, Config: body.Config.Digest}
	for _, child := range body.Manifests {
		if child.Digest != "" {
			manifest.Children = append(manifest.Children, child.Digest)
		}
	}
	for _, layer := range body.Layers {
		if layer.Digest != "" {
			manifest.Layers = append(manifest.Layers, layer.Digest)
		}
	}
	return manifest, nil
}

// Closure is every blob an image consists of: its manifest, the manifests of an index down to the
// images, their configs and layers. Sorted, each once. Also returns the manifests among them, which
// are fetched differently from the other blobs.
//
// A manifest the store does not hold is in the closure and names nothing: what it names is unknown
// until it is fetched. One that cannot be parsed is in it too, and names nothing either.
func Closure(root, digest string) ([]string, map[string]bool, error) {
	blobs := map[string]bool{}
	manifests := map[string]bool{}
	var walk func(digest string, depth int) error
	walk = func(digest string, depth int) error {
		if blobs[digest] {
			return nil
		}
		blobs[digest] = true
		manifests[digest] = true
		if depth > 4 {
			return fmt.Errorf("manifest %s nests deeper than any image does", digest)
		}
		manifest, err := ReadManifest(root, digest)
		if err != nil {
			// Unparseable, or unreadable — the disk failing under it: a blob like any other here,
			// checked against its digest, found broken and replaced by a Repair. Failing the whole
			// closure instead left the manifest unrepairable and every Verify of the image failing.
			return nil
		}
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
		return nil, nil, err
	}

	list := make([]string, 0, len(blobs))
	for blob := range blobs {
		list = append(list, blob)
	}
	sort.Strings(list)
	return list, manifests, nil
}

// Target is one digest a Verify or a Repair names, as this store holds it.
type Target struct {
	// Digest as named.
	Digest string

	// Repository the image is linked from, or that links an image the blob belongs to: where a
	// whole copy is asked for. Empty when the store links nothing that names the digest.
	Repository string

	// Blobs are what the digest consists of — see Closure — and Manifests the manifests among them.
	// Empty when the store holds nothing by that digest.
	Blobs     []string
	Manifests map[string]bool
}

// Resolve finds each digest in the store: an image linked from a repository, with every blob it
// consists of, or a single blob, with an image that names it. A digest the store holds neither as
// an image nor as a blob resolves to no blobs.
func Resolve(root string, digests []string) ([]Target, error) {
	held, err := fill.HeldManifests(root, "")
	if err != nil {
		return nil, err
	}
	repositories := map[string]string{}
	for _, manifest := range held {
		if current, ok := repositories[manifest.Digest]; !ok || manifest.Repository < current {
			repositories[manifest.Digest] = manifest.Repository
		}
	}

	// Which image names a bare blob is read only if a bare blob is asked about, since it means
	// reading every manifest of the store.
	var owners map[string]string
	ownerOf := func(blob string) (string, error) {
		if owners == nil {
			owners = map[string]string{}
			for digest, repository := range repositories {
				blobs, _, err := Closure(root, digest)
				if err != nil {
					return "", err
				}
				for _, member := range blobs {
					if current, ok := owners[member]; !ok || repository < current {
						owners[member] = repository
					}
				}
			}
		}
		return owners[blob], nil
	}

	targets := make([]Target, 0, len(digests))
	for _, digest := range digests {
		target := Target{Digest: digest}
		if repository, isImage := repositories[digest]; isImage {
			blobs, manifests, err := Closure(root, digest)
			if err != nil {
				return nil, err
			}
			target.Repository, target.Blobs, target.Manifests = repository, blobs, manifests
			targets = append(targets, target)
			continue
		}

		path, ok := BlobPath(root, digest)
		if !ok {
			targets = append(targets, target)
			continue
		}
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			targets = append(targets, target)
			continue
		} else if err != nil {
			return nil, fmt.Errorf("reading %s: %w", digest, err)
		}
		repository, err := ownerOf(digest)
		if err != nil {
			return nil, err
		}
		target.Repository, target.Blobs = repository, []string{digest}
		target.Manifests = map[string]bool{}
		if _, isManifest := repositories[digest]; isManifest {
			target.Manifests[digest] = true
		}
		targets = append(targets, target)
	}
	return targets, nil
}
