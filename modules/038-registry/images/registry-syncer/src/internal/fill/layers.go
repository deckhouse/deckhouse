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

package fill

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A manifest present without its layers is what a pull-through cache leaves behind.
//
// The registry writes a revision link the moment it has served a manifest, and fetches the layers
// only when somebody asks for them — so a store that has answered one metadata request holds a
// manifest it cannot serve. Counting revisions therefore counts intentions rather than images, and
// that count is what authorizes dropping the upstream.
//
// Measured on `ly-mmc`, a three-master cluster reporting `allReplicasFull: true`,
// `safeToDropUpstream: true` and 400 verified digests on every replica: 333 MB on disk against
// gigabytes of platform, 332 manifests, and 61 layer links between them. With the upstream removed,
// no node could pull anything at all — not by tag, not by digest — and the replicas could not even
// replicate from each other: one master answered `404` to the blob requests of another, because
// neither had the data. Putting the upstream back fixed every pull instantly, which is what had
// been hiding it all along.
//
// So an image counts as held when the store has its manifest AND every blob that manifest names,
// down through image indexes to the layers themselves.
type blobs struct {
	root string

	// known caches the answer per digest. Layers are shared heavily between images of one release —
	// without this, a survey of four hundred manifests stats the same base layers hundreds of times.
	known map[string]bool
}

func newBlobs(root string) *blobs {
	return &blobs{root: root, known: make(map[string]bool)}
}

// path is where the registry keeps a blob's content: blobs/<algorithm>/<first two>/<hex>/data.
func (b *blobs) path(digest string) (string, bool) {
	algorithm, hex, found := strings.Cut(digest, ":")
	if !found || algorithm == "" || len(hex) < 2 {
		return "", false
	}
	return filepath.Join(b.root, "docker", "registry", "v2", "blobs", algorithm, hex[:2], hex, "data"), true
}

// present reports whether the blob's content is on disk.
func (b *blobs) present(digest string) bool {
	if answer, cached := b.known[digest]; cached {
		return answer
	}

	path, ok := b.path(digest)
	if !ok {
		b.known[digest] = false
		return false
	}

	_, err := os.Stat(path)
	answer := err == nil
	b.known[digest] = answer
	return answer
}

// descriptor is the part of a manifest this needs: what it points at.
type descriptor struct {
	Digest string `json:"digest"`
}

// manifestBody covers every shape a manifest comes in, because only the references matter here.
//
// An image index and a manifest list differ in media type and agree in structure; an image manifest
// names a config and its layers. Decoding all of them into one struct means an unknown media type
// degrades to "no references", which reads as held — the safe direction is the opposite, so the
// caller treats a manifest it cannot parse as incomplete rather than complete.
type manifestBody struct {
	MediaType string       `json:"mediaType"`
	Manifests []descriptor `json:"manifests"`
	Config    descriptor   `json:"config"`
	Layers    []descriptor `json:"layers"`
}

// complete reports whether the store holds everything this manifest names, recursively.
//
// The manifest itself is read from the blob store rather than from the revision link: the link only
// says the digest was seen. A manifest whose own content is missing is not held at all.
func (b *blobs) complete(digest string, depth int) (bool, error) {
	// Indexes point at manifests which may point at manifests. Bounded because a store is data from
	// elsewhere: a cycle in it must not become a loop here.
	if depth > 4 {
		return false, fmt.Errorf("manifest %s nests deeper than any image does", digest)
	}

	path, ok := b.path(digest)
	if !ok {
		return false, nil
	}

	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("reading manifest %s: %w", digest, err)
	}

	var body manifestBody
	if err := json.Unmarshal(content, &body); err != nil {
		// Unreadable is not the same as absent, and neither is "held". Reported so that a store full
		// of something unexpected is not silently judged complete.
		return false, fmt.Errorf("parsing manifest %s: %w", digest, err)
	}

	for _, child := range body.Manifests {
		held, err := b.complete(child.Digest, depth+1)
		if err != nil || !held {
			return false, err
		}
	}

	// The config is a blob like any other, and an image without it cannot be run.
	if body.Config.Digest != "" && !b.present(body.Config.Digest) {
		return false, nil
	}

	for _, layer := range body.Layers {
		if !b.present(layer.Digest) {
			return false, nil
		}
	}

	// A manifest naming nothing at all — neither children, nor a config, nor layers — is not an image
	// this store can serve. It is what an unrecognised media type decodes to, so it is refused here
	// rather than counted.
	if len(body.Manifests) == 0 && body.Config.Digest == "" && len(body.Layers) == 0 {
		return false, nil
	}

	return true, nil
}
