/*
Copyright The ORAS Authors.
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

package oci

import (
	"fmt"
	"maps"

	"github.com/opencontainers/go-digest"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
)

// Tags maps a tag string to a manifest digest (from index.json annotations).
// Do not call [Tags.Set] concurrently with other methods.
type Tags struct {
	byTag map[string]digest.Digest
}

// NewTags returns an empty tag map.
func NewTags() *Tags {
	return &Tags{byTag: make(map[string]digest.Digest)}
}

// Resolve returns the digest for ref, or an error wrapping [errs.ErrManifestNotFound].
func (t *Tags) Resolve(ref string) (digest.Digest, error) {
	dgst, ok := t.byTag[ref]
	if !ok {
		return "", fmt.Errorf("%s: %w", ref, errs.ErrManifestNotFound)
	}
	return dgst, nil
}

// Set records that ref (typically a tag) selects manifest digest dgst.
func (t *Tags) Set(dgst digest.Digest, ref string) {
	t.byTag[ref] = dgst
}

// Map returns a copy of the tag→digest map.
func (t *Tags) Map() map[string]digest.Digest {
	return maps.Clone(t.byTag)
}

// Len returns the number of tag entries.
func (t *Tags) Len() int {
	return len(t.byTag)
}

// Manifests maps manifest content digest to its descriptor from index.json.
// Do not call [types.Set] concurrently with other methods.
type Manifests struct {
	byDigest map[digest.Digest]types.ShortDescriptor
}

// NewManifests returns an empty manifest index.
func NewManifests() *Manifests {
	return &Manifests{byDigest: make(map[digest.Digest]types.ShortDescriptor)}
}

// Resolve returns the descriptor for dgst, or an error wrapping [errs.ErrManifestNotFound].
func (m *Manifests) Resolve(dgst digest.Digest) (types.ShortDescriptor, error) {
	desc, ok := m.byDigest[dgst]
	if !ok {
		return types.ShortDescriptor{}, fmt.Errorf("%s: %w", dgst, errs.ErrManifestNotFound)
	}
	return desc, nil
}

// Set records the index descriptor for manifest content digest dgst.
func (m *Manifests) Set(dgst digest.Digest, desc types.ShortDescriptor) {
	m.byDigest[dgst] = desc
}

// Len returns the number of indexed types.
func (m *Manifests) Len() int {
	return len(m.byDigest)
}
