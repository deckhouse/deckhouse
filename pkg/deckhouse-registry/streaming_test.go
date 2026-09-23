// Copyright 2026 Flant JSC
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

package dhregistry_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/registry/fake"

	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// releaseRepo fills a module's release repository with one tag per version plus
// the channels, which is the shape Channels has to find its handful of names in.
func releaseRepo(t *testing.T, versions, channels []string) *fake.Registry {
	t.Helper()

	reg := fake.NewRegistry("registry.deckhouse.io")
	scratch := fake.NewImageBuilder().MustBuild()

	for _, tag := range append(append([]string{}, versions...), channels...) {
		reg.MustAddImage("deckhouse/fe/modules/stronghold/release", tag, scratch)
	}

	return reg
}

// TestStreamTags covers the streaming listing: every tag reaches the visitor,
// and the accumulated result matches ListTags.
func TestStreamTags(t *testing.T) {
	reg := releaseRepo(t, []string{"v1.0.0", "v1.0.1", "v1.1.0"}, []string{"alpha", "stable"})
	releases := newFakeRegistry(t, reg).Modules().Module("stronghold").Releases()

	var streamed []string

	require.NoError(t, releases.StreamTags(t.Context(), func(tags []string) error {
		streamed = append(streamed, tags...)

		return nil
	}))

	listed, err := releases.ListTags(t.Context())
	require.NoError(t, err)

	assert.ElementsMatch(t, listed, streamed)
	assert.ElementsMatch(t, []string{"v1.0.0", "v1.0.1", "v1.1.0", "alpha", "stable"}, streamed)
}

// TestStreamTagsStops covers the early exit: ErrStopStreaming ends the walk
// without being reported as a failure, while any other error is propagated.
func TestStreamTagsStops(t *testing.T) {
	reg := releaseRepo(t, []string{"v1.0.0", "v1.0.1"}, []string{"alpha"})
	releases := newFakeRegistry(t, reg).Modules().Module("stronghold").Releases()

	t.Run("stop is not a failure", func(t *testing.T) {
		pages := 0

		err := releases.StreamTags(t.Context(), func([]string) error {
			pages++

			return dhregistry.ErrStopStreaming
		})

		require.NoError(t, err)
		assert.Equal(t, 1, pages)
	})

	t.Run("other errors propagate", func(t *testing.T) {
		sentinel := errors.New("caller failed")

		err := releases.StreamTags(t.Context(), func([]string) error {
			return sentinel
		})

		assert.ErrorIs(t, err, sentinel)
	})
}

// TestChannelsOnVersionsOnlyRepo covers a repository that publishes no channel
// at all: the walk runs to the end and reports nothing rather than failing.
func TestChannelsOnVersionsOnlyRepo(t *testing.T) {
	reg := releaseRepo(t, []string{"v1.0.0", "v1.0.1"}, nil)

	channels, err := newFakeRegistry(t, reg).Modules().Module("stronghold").Releases().Channels(t.Context())
	require.NoError(t, err)

	assert.Empty(t, channels)
}

// TestReaderStreams pins that the streaming methods are part of the read-only
// capability, so a component handed a Reader can walk a repository.
func TestReaderStreams(t *testing.T) {
	reg := releaseRepo(t, []string{"v1.0.0"}, []string{"alpha"})

	count := func(r service.Reader) (int, error) {
		total := 0

		err := r.StreamTags(t.Context(), func(tags []string) error {
			total += len(tags)

			return nil
		})

		return total, err
	}

	total, err := count(newFakeRegistry(t, reg).Modules().Module("stronghold").Releases())
	require.NoError(t, err)
	assert.Equal(t, 2, total)
}
