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
	"context"
	"io"
	"log"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startRegistry runs a real registry in memory, so the copy and its accounting are
// exercised against actual registry behaviour rather than a mock of it.
func startRegistry(t *testing.T) Registry {
	t.Helper()

	server := httptest.NewServer(registry.New(
		// The registry logs every request; a fill of a few hundred images would bury
		// the actual test output.
		registry.Logger(log.New(io.Discard, "", 0)),
	))
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)

	return Registry{Address: parsed.Host, Insecure: true}
}

func pushImage(t *testing.T, target Registry, reference string) v1.Hash {
	t.Helper()

	image, err := random.Image(256, 1)
	require.NoError(t, err)

	tag, err := name.NewTag(target.Address+"/"+reference, name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))

	digest, err := image.Digest()
	require.NoError(t, err)
	return digest
}

func digestOf(t *testing.T, target Registry, reference string) (v1.Hash, error) {
	t.Helper()

	tag, err := name.NewTag(target.Address+"/"+reference, name.Insecure)
	require.NoError(t, err)

	descriptor, err := remote.Get(tag)
	if err != nil {
		return v1.Hash{}, err
	}
	return descriptor.Digest, nil
}

func TestCopierFillsAnEmptyStorage(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)

	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	first := pushImage(t, source, "deckhouse/ee/registry-controller:v1")
	second := pushImage(t, source, "deckhouse/ee/release-channel:stable")

	copier := &Copier{Source: source, Destination: destination, Discover: Catalogue{}}
	report, err := copier.Run(context.Background())
	require.NoError(t, err)

	assert.EqualValues(t, 2, report.Written)
	assert.EqualValues(t, 0, report.Skipped)
	assert.Empty(t, report.Failed)

	// The repository prefix is rewritten, so the cluster refers to images under a
	// fixed prefix no matter where they came from.
	got, err := digestOf(t, destination, "system/deckhouse/registry-controller:v1")
	require.NoError(t, err)
	assert.Equal(t, first, got)

	got, err = digestOf(t, destination, "system/deckhouse/release-channel:stable")
	require.NoError(t, err)
	assert.Equal(t, second, got)
}

// TestCopierIsIncrementalAndStillCountsWhatIsHeld is the accounting property the
// air-gap gate depends on: a second run copies nothing but must still report the
// storage as holding everything, because completeness is about what is THERE and
// not about what this run moved.
func TestCopierIsIncrementalAndStillCountsWhatIsHeld(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	pushImage(t, source, "deckhouse/ee/one:v1")
	pushImage(t, source, "deckhouse/ee/two:v1")

	copier := &Copier{Source: source, Destination: destination, Discover: Catalogue{}}

	first, err := copier.Run(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 2, first.Written)

	second, err := copier.Run(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 2, second.Written, "the storage still holds both")
	assert.EqualValues(t, 2, second.Skipped, "and neither had to be moved again")
	assert.True(t, second.Complete(2))
}

// TestCopierReplacesAChangedTag covers an update arriving on an existing tag: the
// digest differs, so it has to be written even though the tag is already there.
func TestCopierReplacesAChangedTag(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	pushImage(t, source, "deckhouse/ee/release-channel:stable")
	copier := &Copier{Source: source, Destination: destination, Discover: Catalogue{}}
	_, err := copier.Run(context.Background())
	require.NoError(t, err)

	updated := pushImage(t, source, "deckhouse/ee/release-channel:stable")

	report, err := copier.Run(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 1, report.Written)
	assert.EqualValues(t, 0, report.Skipped)

	got, err := digestOf(t, destination, "system/deckhouse/release-channel:stable")
	require.NoError(t, err)
	assert.Equal(t, updated, got)
}

// TestCopierStaysWithinItsPrefix keeps a shared upstream from dragging everything
// on it into a cache sized for Deckhouse.
func TestCopierStaysWithinItsPrefix(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	pushImage(t, source, "deckhouse/ee/wanted:v1")
	pushImage(t, source, "someone-else/unrelated:v1")
	// A prefix that merely starts with the same characters is a different repository.
	pushImage(t, source, "deckhouse/ee-staging/unwanted:v1")

	report, err := (&Copier{Source: source, Destination: destination, Discover: Catalogue{}}).Run(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 1, report.Written)

	_, err = digestOf(t, destination, "system/deckhouse/wanted:v1")
	assert.NoError(t, err)

	_, err = digestOf(t, destination, "system/deckhouse/unrelated:v1")
	assert.Error(t, err)
}

func TestCopierReportsProgress(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"

	for _, reference := range []string{"one:v1", "two:v1", "three:v1"} {
		pushImage(t, source, "deckhouse/ee/"+reference)
	}

	var totals []int32
	copier := &Copier{
		Source: source, Destination: destination, Discover: Catalogue{},
		OnProgress: func(done, _ int32) { totals = append(totals, done) },
	}

	_, err := copier.Run(context.Background())
	require.NoError(t, err)

	// A long fill has to report progress rather than going silent for however long
	// it takes.
	assert.Equal(t, []int32{1, 2, 3}, totals)
}

func TestCopierWithNothingToCopy(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)

	report, err := (&Copier{Source: source, Destination: destination, Discover: Catalogue{}}).Run(context.Background())
	require.NoError(t, err)

	assert.EqualValues(t, 0, report.Written)
	assert.Empty(t, report.Failed)
	// An empty source is not a complete cache. Without this, an unreachable or
	// misconfigured upstream that lists nothing would look like a finished fill.
	assert.False(t, report.Complete(459))
}

func TestCopierHonoursCancellation(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"

	for _, reference := range []string{"one:v1", "two:v1", "three:v1"} {
		pushImage(t, source, "deckhouse/ee/"+reference)
	}

	ctx, cancel := context.WithCancel(context.Background())
	copier := &Copier{
		Source: source, Destination: destination, Discover: Catalogue{},
		OnProgress: func(done, _ int32) {
			if done == 1 {
				cancel()
			}
		},
	}

	_, err := copier.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestReportComplete(t *testing.T) {
	tests := []struct {
		name     string
		report   Report
		expected int32
		want     bool
	}{
		{
			name:     "everything written",
			report:   Report{Written: 459},
			expected: 459,
			want:     true,
		},
		{
			name:     "more than expected, which a stale expectation looks like",
			report:   Report{Written: 460},
			expected: 459,
			want:     true,
		},
		{
			name:     "partially written",
			report:   Report{Written: 312},
			expected: 459,
			want:     false,
		},
		{
			name:     "everything written but something failed",
			report:   Report{Written: 459, Failed: []string{"registry.example.com/one:v1"}},
			expected: 459,
			want:     false,
		},
		{
			name:   "no expectation at all",
			report: Report{Written: 459},
			// Reporting completeness here would authorize dropping the upstream on no
			// evidence whatsoever.
			expected: 0,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.report.Complete(tt.expected))
		})
	}
}

// TestCountCatalogue covers the air-gap accounting path: content that arrived out
// of band through `d8 mirror push` is invisible to the copier, so completeness has
// to come from reading what the registry holds.
func TestCountCatalogue(t *testing.T) {
	storage := startRegistry(t)
	storage.Repository = "system/deckhouse"

	assertCount := func(want int32) {
		t.Helper()
		got, err := CountCatalogue(context.Background(), storage)
		require.NoError(t, err)
		assert.EqualValues(t, want, got)
	}

	assertCount(0)

	pushImage(t, storage, "system/deckhouse/one:v1")
	pushImage(t, storage, "system/deckhouse/two:v1")
	pushImage(t, storage, "system/deckhouse/two:v2")
	assertCount(3)

	// Anything outside the prefix is somebody else's content and must not be
	// counted towards this cache being complete.
	pushImage(t, storage, "unrelated/image:v1")
	assertCount(3)
}

func TestRegistryOptions(t *testing.T) {
	options, err := RegistryOptions("", "", "")
	require.NoError(t, err)
	assert.Len(t, options, 1, "the transport is always configured")

	options, err = RegistryOptions("", "user", "password")
	require.NoError(t, err)
	assert.Len(t, options, 2)

	_, err = RegistryOptions("this is not a certificate", "", "")
	assert.Error(t, err, "an unusable certificate authority must fail loudly, not silently fall back")
}

func TestRewriteRepository(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		destination string
		repository  string
		want        string
	}{
		{
			name:        "prefix swapped",
			source:      "deckhouse/ee",
			destination: "system/deckhouse",
			repository:  "deckhouse/ee/registry-controller",
			want:        "system/deckhouse/registry-controller",
		},
		{
			name:        "the prefix itself",
			source:      "deckhouse/ee",
			destination: "system/deckhouse",
			repository:  "deckhouse/ee",
			want:        "system/deckhouse",
		},
		{
			name:        "no source prefix",
			destination: "system/deckhouse",
			repository:  "registry-controller",
			want:        "system/deckhouse/registry-controller",
		},
		{
			name:       "no destination prefix",
			source:     "deckhouse/ee",
			repository: "deckhouse/ee/registry-controller",
			want:       "registry-controller",
		},
		{
			name:        "a lookalike prefix is left alone",
			source:      "deckhouse/ee",
			destination: "system/deckhouse",
			repository:  "deckhouse/ee-staging/thing",
			want:        "system/deckhouse/deckhouse/ee-staging/thing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copier := &Copier{
				Source:      Registry{Repository: tt.source},
				Destination: Registry{Repository: tt.destination},
			}
			assert.Equal(t, tt.want, copier.rewriteRepository(tt.repository))
		})
	}
}
