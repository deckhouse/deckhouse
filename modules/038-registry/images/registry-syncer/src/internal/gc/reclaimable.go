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

package gc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/registry-syncer/internal/fill"
)

// Reclaimable is how many bytes a collection would free if it removed the doomed manifests: what every
// blob in the store weighs that no remaining manifest would reference.
//
// The mark is distribution's own, done here because its dry run cannot answer this: it marks from the
// manifests that exist now, and until the collection has removed the doomed ones their blobs are as
// reachable as anything — its dry run says "0 eligible" on a store that is half garbage. So the store
// is read from its files: every manifest that stays marks itself, its config, its layers and the
// manifests an index names; every blob left unmarked is what a sweep would remove, the blobs an
// earlier failed sweep left behind included.
//
// Read-only, and safe beside a fill: what a fill adds is the set, which is never doomed, and a blob
// that lands between the walks is at worst left out of the count.
//
// A remaining manifest that cannot be read fails the estimate rather than counting its blobs as
// garbage: an estimate that says "this would free" about what is still needed is worse than none.
func Reclaimable(dataDir string, doomed []fill.HeldManifest) (int64, error) {
	held, err := fill.HeldManifests(dataDir, "")
	if err != nil {
		return 0, fmt.Errorf("reading the manifests the store holds: %w", err)
	}

	removed := make(map[fill.HeldManifest]bool, len(doomed))
	for _, manifest := range doomed {
		removed[manifest] = true
	}

	marked := map[string]bool{}
	for _, manifest := range held {
		if removed[manifest] {
			continue
		}
		marked[manifest.Digest] = true
		references, err := manifestReferences(dataDir, manifest.Digest)
		if err != nil {
			return 0, fmt.Errorf("reading %s@%s, which stays: %w", manifest.Repository, manifest.Digest, err)
		}
		for _, reference := range references {
			marked[reference] = true
		}
	}

	var total int64
	blobs := filepath.Join(dataDir, "docker", "registry", "v2", "blobs")
	err = filepath.WalkDir(blobs, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() || entry.Name() != "data" {
			return nil
		}
		digest, ok := blobDigest(blobs, path)
		if !ok || marked[digest] {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("measuring the blobs: %w", err)
	}
	return total, nil
}

// manifestReferences are the digests a manifest names: its config and layers, or the manifests of an
// index. A manifest whose blob the store does not hold names nothing it holds either.
func manifestReferences(dataDir, digest string) ([]string, error) {
	manifest, err := readManifest(dataDir, digest)
	if err != nil || manifest == nil {
		return nil, err
	}

	var references []string
	if manifest.Config != nil {
		references = append(references, manifest.Config.Digest)
	}
	if manifest.Subject != nil {
		references = append(references, manifest.Subject.Digest)
	}
	for _, list := range [][]descriptor{manifest.Layers, manifest.Manifests} {
		for _, entry := range list {
			references = append(references, entry.Digest)
		}
	}
	return references, nil
}

// keepChildren adds to the keep-set the manifests every kept index names, and theirs in turn.
//
// What a release or a module declares is the digest a pull asks for, and for an image built for more
// than one platform that is the index alone. The fill copies its children as revisions of their own,
// and judged one by one they are declared by nobody: the manifest pass deleted them, and left an index
// that every pull failed on with MANIFEST_UNKNOWN. Read from the store's files, like the rest of the
// judgement; an index the store does not hold has no children in it either.
func keepChildren(dataDir string, keep map[string]struct{}) error {
	pending := make([]string, 0, len(keep))
	for digest := range keep {
		pending = append(pending, digest)
	}
	for len(pending) > 0 {
		digest := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		manifest, err := readManifest(dataDir, digest)
		if err != nil {
			return fmt.Errorf("reading %s: %w", digest, err)
		}
		if manifest == nil {
			continue
		}
		for _, child := range manifest.Manifests {
			if _, kept := keep[child.Digest]; kept {
				continue
			}
			keep[child.Digest] = struct{}{}
			pending = append(pending, child.Digest)
		}
	}
	return nil
}

// storedManifest is the part of a manifest or an index that names other content.
type storedManifest struct {
	Config    *descriptor  `json:"config"`
	Layers    []descriptor `json:"layers"`
	Manifests []descriptor `json:"manifests"`
	Subject   *descriptor  `json:"subject"`
}

// readManifest reads a manifest out of the store's blobs. Nil when the store does not hold it.
func readManifest(dataDir, digest string) (*storedManifest, error) {
	algorithm, hex, ok := strings.Cut(digest, ":")
	if !ok || len(hex) < 2 {
		return nil, fmt.Errorf("not a digest: %q", digest)
	}
	content, err := os.ReadFile(filepath.Join(dataDir, "docker", "registry", "v2", "blobs", algorithm, hex[:2], hex, "data"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	manifest := &storedManifest{}
	if err := json.Unmarshal(content, manifest); err != nil {
		return nil, fmt.Errorf("parsing the manifest: %w", err)
	}
	return manifest, nil
}

type descriptor struct {
	Digest string `json:"digest"`
}

// blobDigest reads the digest out of a blob's path, .../blobs/<algorithm>/<xx>/<hex>/data.
func blobDigest(blobs, path string) (string, bool) {
	relative, err := filepath.Rel(blobs, path)
	if err != nil {
		return "", false
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	if len(parts) != 4 {
		return "", false
	}
	return parts[0] + ":" + parts[2], true
}
