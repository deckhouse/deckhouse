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
	"sync"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_ConcurrentUse is a race-detector test: it asserts nothing the other
// tests do not, and exists to be run under -race.
//
// A Client is documented as safe for concurrent use, and callers rely on that —
// a whole service tree is built over one client and shared across goroutines.
// The scope path is what makes that non-trivial: it is derived from the
// segments and read by GetRegistry and by every request method, which puts it
// in its log record. Deriving it lazily on first use therefore wrote a field
// that other goroutines were already reading.
//
// Every goroutine works through the one shared scoped client, which is the
// arrangement that matters: a client scoped per call would derive its path in
// private and hide the problem.
func TestClient_ConcurrentUse(t *testing.T) {
	_, c := newTestServer(t)

	// The module catalog shape: the repository's tags are the module names.
	pushRandomImage(t, c, "deckhouse/fe/modules", "stronghold")

	scoped := c.WithSegment("deckhouse", "fe", "modules")

	const goroutines = 8

	var wg sync.WaitGroup

	wg.Add(goroutines * 4)

	// Readers of the scope path alone.
	for range goroutines {
		go func() {
			defer wg.Done()

			for range 25 {
				assert.Equal(t, c.GetRegistry()+"/deckhouse/fe/modules", scoped.GetRegistry())
			}
		}()
	}

	// Requests on that same client. Each reads the scope path for its log
	// record before it does anything else, which is the read that has to be
	// safe against another goroutine deriving the path.
	for range goroutines {
		go func() {
			defer wg.Done()

			for range 5 {
				_, err := scoped.GetDigest(context.Background(), "stronghold")
				assert.NoError(t, err)
			}
		}()
	}

	for range goroutines {
		go func() {
			defer wg.Done()

			for range 5 {
				tags, err := scoped.ListTags(context.Background())
				assert.NoError(t, err)
				assert.Contains(t, tags, "stronghold")
			}
		}()
	}

	// Scoping further down, the other path into the same derivation.
	for range goroutines {
		go func() {
			defer wg.Done()

			for range 25 {
				assert.Equal(t, c.GetRegistry()+"/deckhouse/fe/modules/stronghold", scoped.WithSegment("stronghold").GetRegistry())
			}
		}()
	}

	wg.Wait()
}

// TestManifestResult_ConcurrentUse is the second race-detector test, for the
// object a request hands back rather than the client that made it.
//
// A ManifestResult travels beyond the call that produced it — a caller may keep
// one and read it from wherever — so decoding it must not write into it. This
// drives the image and the index shape at once, since they decode through
// separate accessors.
func TestManifestResult_ConcurrentUse(t *testing.T) {
	img, err := random.Image(256, 1)
	require.NoError(t, err)

	imageManifest, err := img.RawManifest()
	require.NoError(t, err)

	idx := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
		Add:        img,
		Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "amd64"}},
	})

	indexManifest, err := idx.RawManifest()
	require.NoError(t, err)

	tests := map[string]struct {
		raw  []byte
		read func(t *testing.T, m *ManifestResult)
	}{
		"image": {
			raw: imageManifest,
			read: func(t *testing.T, m *ManifestResult) {
				parsed, err := m.GetManifest()
				require.NoError(t, err)
				assert.Len(t, parsed.GetLayers(), 1)

				_, err = m.GetIndexManifest()
				assert.ErrorIs(t, err, ErrIsNotIndexManifest)
			},
		},
		"index": {
			raw: indexManifest,
			read: func(t *testing.T, m *ManifestResult) {
				parsed, err := m.GetIndexManifest()
				require.NoError(t, err)
				assert.Len(t, parsed.GetManifests(), 1)

				_, err = m.GetManifest()
				assert.ErrorIs(t, err, ErrIsIndexManifest)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := NewManifestResultFromBytes(tt.raw)

			const goroutines = 8

			var wg sync.WaitGroup

			wg.Add(goroutines)

			for range goroutines {
				go func() {
					defer wg.Done()

					for range 20 {
						tt.read(t, result)

						assert.NotEmpty(t, result.GetRaw())
						assert.NotEmpty(t, result.GetDescriptor().GetDigest().String())
					}
				}()
			}

			wg.Wait()
		})
	}
}

// TestClient_WithSegmentDoesNotMutateCaller pins that scoping copies the
// caller's slice instead of trimming it in place. A caller that passes a slice
// it keeps — a module name list, say — would otherwise find it rewritten.
func TestClient_WithSegmentDoesNotMutateCaller(t *testing.T) {
	c := New("registry.example.com")

	segments := []string{"/deckhouse/", "/fe"}
	scoped := c.WithSegment(segments...)

	assert.Equal(t, []string{"/deckhouse/", "/fe"}, segments, "the caller's slice must be untouched")
	assert.Equal(t, "registry.example.com/deckhouse/fe", scoped.GetRegistry())
}

// TestClient_ScopeIsIndependentOfCallOrder pins that a client reports the same
// path whether or not it was asked for it before being scoped further — the
// property a memoized path is easy to break.
func TestClient_ScopeIsIndependentOfCallOrder(t *testing.T) {
	base := New("registry.example.com").WithSegment("deckhouse")

	// Ask the parent first, then scope.
	require.Equal(t, "registry.example.com/deckhouse", base.GetRegistry())
	assert.Equal(t, "registry.example.com/deckhouse/fe/modules", base.WithSegment("fe").WithSegment("modules").GetRegistry())

	// Scope first, ask the parent afterwards.
	fresh := New("registry.example.com").WithSegment("deckhouse")
	assert.Equal(t, "registry.example.com/deckhouse/fe/modules", fresh.WithSegment("fe").WithSegment("modules").GetRegistry())
	assert.Equal(t, "registry.example.com/deckhouse", fresh.GetRegistry())
}
