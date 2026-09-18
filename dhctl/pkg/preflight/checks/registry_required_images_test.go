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

package checks

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/digests"
	registry_mocks "github.com/deckhouse/deckhouse/dhctl/pkg/config/registrymocks"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// releaseDigests builds an image list of the shape images_digests.json has.
func releaseDigests(count int) digests.ImagesDigests {
	all := digests.ImagesDigests{"common": map[string]any{}, "controlPlaneManager": map[string]any{}}
	for i := range count {
		section := "common"
		if i%2 == 1 {
			section = "controlPlaneManager"
		}
		all[section][fmt.Sprintf("image%02d", i)] = fmt.Sprintf("sha256:%064x", i)
	}
	return all
}

func requiredImagesCheck(all digests.ImagesDigests, head func(name.Reference, ...remote.Option) error) RegistryRequiredImagesCheck {
	registryCfg := registry_mocks.ConfigBuilder(
		registry_mocks.WithImagesRepo("registry.example.com/deckhouse/ee"),
		registry_mocks.WithSchemeHTTPS(),
	)
	return RegistryRequiredImagesCheck{
		MetaConfig: &config.MetaConfig{Registry: registryCfg},
		digests:    func() (digests.ImagesDigests, error) { return all, nil },
		head:       head,
	}
}

// absent is what a registry says about an image it does not hold.
func absent() error {
	return &transport.Error{
		StatusCode: http.StatusNotFound,
		Errors:     []transport.Diagnostic{{Code: transport.ManifestUnknownErrorCode, Message: "manifest unknown"}},
	}
}

// TestRegistryRequiredImagesFindsAMirrorWithOnlyTheTag is the failure this check exists for: a
// registry filled with `crane copy` or `skopeo copy` holds the Deckhouse image and none of the
// images it refers to. deckhouse-image-available is happy with it, and the bootstrap then fails
// with bashible looping on "072 etcd not running after 200s".
func TestRegistryRequiredImagesFindsAMirrorWithOnlyTheTag(t *testing.T) {
	check := requiredImagesCheck(releaseDigests(40), func(name.Reference, ...remote.Option) error {
		return absent()
	})

	_, err := check.Run(t.Context())
	require.Error(t, err)

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "5 of them are not there")
	assert.Contains(t, failure.Fix, "d8 mirror pull")
	assert.Contains(t, failure.Fix, "crane copy")
}

func TestRegistryRequiredImagesAcceptsAProperMirror(t *testing.T) {
	asked := 0
	check := requiredImagesCheck(releaseDigests(40), func(ref name.Reference, _ ...remote.Option) error {
		asked++
		// Every image is asked for by digest, never by tag: a tag can point anywhere.
		assert.Contains(t, ref.String(), "@sha256:")
		return nil
	})

	detail, err := check.Run(t.Context())
	require.NoError(t, err)
	assert.Equal(t, requiredImagesSample, asked, "a few images, not the several hundred a release has")
	assert.Contains(t, detail, "are present in registry.example.com/deckhouse/ee")
}

// TestRegistryRequiredImagesReportsOnlyWhatIsMissing: a partially mirrored registry is a real
// state, and naming the images that are absent is what tells the operator the mirror is partial
// rather than empty.
func TestRegistryRequiredImagesReportsOnlyWhatIsMissing(t *testing.T) {
	check := requiredImagesCheck(releaseDigests(40), func(ref name.Reference, _ ...remote.Option) error {
		if strings.HasSuffix(ref.String(), fmt.Sprintf("%064x", 0)) {
			return absent()
		}
		return nil
	})

	_, err := check.Run(t.Context())
	require.Error(t, err)

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "1 of them are not there")
	assert.Contains(t, failure.Observed, "sha256:000000000000", "the digest is shortened to something readable")
}

// TestRegistryRequiredImagesLeavesRegistryErrorsToRegistryReachable: a registry that cannot answer
// is not a registry missing images, and reporting it here would be the second failure for one
// cause.
func TestRegistryRequiredImagesLeavesRegistryErrorsToRegistryReachable(t *testing.T) {
	cause := errors.New("dial tcp: i/o timeout")
	check := requiredImagesCheck(releaseDigests(40), func(name.Reference, ...remote.Option) error {
		return cause
	})

	_, err := check.Run(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, cause)

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.NotContains(t, failure.Fix, "d8 mirror", "this one is not about mirroring")
}

// TestRegistryRequiredImagesWithoutDigests: a development installer carries no image list, so
// there is nothing to look for.
func TestRegistryRequiredImagesWithoutDigests(t *testing.T) {
	check := requiredImagesCheck(digests.ImagesDigests{}, func(name.Reference, ...remote.Option) error {
		t.Fatal("nothing should be asked of the registry")
		return nil
	})

	_, err := check.Run(t.Context())
	assert.ErrorIs(t, err, preflight.ErrNotApplicable)
}

// TestSampleDigestsIsStableAndSpread: an unstable sample makes the check flaky, and one taken
// from the head of the list would test a single section of the release.
func TestSampleDigestsIsStableAndSpread(t *testing.T) {
	all := releaseDigests(40)

	first := sampleDigests(all)
	second := sampleDigests(all)
	assert.Equal(t, first, second, "the same installer must ask for the same images every run")

	sections := map[string]struct{}{}
	for _, image := range first {
		sections[strings.SplitN(image.name, "/", 2)[0]] = struct{}{}
	}
	assert.Greater(t, len(sections), 1, "the sample must not sit in one section of the release")

	// Fewer images than the sample size is not an error; it is a small release.
	assert.Len(t, sampleDigests(releaseDigests(3)), 3)
}
