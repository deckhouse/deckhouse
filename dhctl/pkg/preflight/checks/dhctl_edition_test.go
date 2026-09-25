// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_mocks "github.com/deckhouse/deckhouse/dhctl/pkg/config/registrymocks"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// imageAvailableCheck builds the fetching half of the split with a fake registry behind it.
func imageAvailableCheck(t *testing.T, configFile *v1.ConfigFile, fetchErr error) (DeckhouseImageAvailableCheck, string) {
	t.Helper()
	t.Setenv("DHCTL_TEST_VERSION_TAG", "v1.2.3")

	registryCfg := registry_mocks.ConfigBuilder(
		registry_mocks.WithImagesRepo("test.registry.io/test"),
		registry_mocks.WithSchemeHTTPS(),
	)
	installer := &config.DeckhouseInstaller{Registry: registryCfg, DevBranch: "dev-branch"}
	metaCfg := &config.MetaConfig{Registry: registryCfg}

	image, err := installer.GetRemoteImage(t.Context(), true)
	require.NoError(t, err)
	ref, err := name.ParseReference(image)
	require.NoError(t, err)

	provider := NewFakeImageDescriptorProvider(t).ExpectReference(ref).Return(configFile, fetchErr)

	return DeckhouseImageAvailableCheck{
		MetaConfig: metaCfg,
		Installer:  installer,
		Image:      NewDeckhouseImage(),
		descriptor: provider,
	}, image
}

// TestEditionMismatch is the half of the old check that is actually about editions.
func TestEditionMismatch(t *testing.T) {
	fetch, image := imageAvailableCheck(t, &v1.ConfigFile{
		Config: v1.Config{Labels: map[string]string{editionLabel: "BAD"}},
	}, nil)

	detail, err := fetch.Run(t.Context())
	require.NoError(t, err)
	assert.Contains(t, detail, image)

	edition := DhctlEditionCheck{
		BuildInfo: options.BuildInfo{AppVersion: "dev", AppEdition: "test"},
		Image:     fetch.Image,
	}

	_, err = edition.Run(t.Context())
	require.Error(t, err)

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "the installer is test and the image is BAD")
}

func TestEditionMatches(t *testing.T) {
	fetch, _ := imageAvailableCheck(t, &v1.ConfigFile{
		Config: v1.Config{Labels: map[string]string{editionLabel: "test"}},
	}, nil)

	_, err := fetch.Run(t.Context())
	require.NoError(t, err)

	edition := DhctlEditionCheck{
		BuildInfo: options.BuildInfo{AppVersion: "dev", AppEdition: "test"},
		Image:     fetch.Image,
	}

	detail, err := edition.Run(t.Context())
	assert.NoError(t, err)
	assert.Contains(t, detail, "installer edition test matches")
}

// TestEditionWithoutALabel: a development build carries no edition label, which is a different
// thing from carrying the wrong one — the old text printed an empty edition and read as a
// mismatch against "".
func TestEditionWithoutALabel(t *testing.T) {
	fetch, _ := imageAvailableCheck(t, &v1.ConfigFile{Config: v1.Config{Labels: nil}}, nil)

	_, err := fetch.Run(t.Context())
	require.NoError(t, err)

	edition := DhctlEditionCheck{
		BuildInfo: options.BuildInfo{AppVersion: "dev", AppEdition: "test"},
		Image:     fetch.Image,
	}

	_, err = edition.Run(t.Context())
	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "no edition label")
}

// TestEditionOfALocalBuildIsNotApplicable: an installer built outside werf has nothing to
// compare. It used to be disabled at construction, which the report showed as a check the
// operator had skipped with a flag they never passed.
func TestEditionOfALocalBuildIsNotApplicable(t *testing.T) {
	check := DhctlEdition(options.BuildInfo{AppVersion: "local", AppEdition: "local"}, NewDeckhouseImage())
	assert.False(t, check.Disabled, "it is the body that reports this, not a silent disable")

	_, err := check.Run(t.Context())
	assert.ErrorIs(t, err, preflight.ErrNotApplicable)
}

// TestMissingTagIsReportedAsAMissingTag is the failure the split exists for: a tag that was never
// mirrored is the most common air-gapped mistake, and it used to be reported under the name
// "dhctl-edition" with a Go map dump appended.
func TestMissingTagIsReportedAsAMissingTag(t *testing.T) {
	fetchErr := &transport.Error{
		StatusCode: http.StatusNotFound,
		Errors: []transport.Diagnostic{
			{Code: transport.ManifestUnknownErrorCode, Message: "manifest unknown"},
		},
	}
	fetch, image := imageAvailableCheck(t, nil, fetchErr)

	_, err := fetch.Run(t.Context())
	require.Error(t, err)

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "the tag of "+image+" is not in the registry")
	assert.Contains(t, failure.Fix, "d8 mirror pull")
	assert.True(t, errors.Is(err, error(fetchErr)), "the registry's own error stays reachable for a ticket")
}

// TestUnknownRepositoryIsNotAMissingTag keeps the two apart: one is fixed by mirroring, the other
// by correcting imagesRepo.
func TestUnknownRepositoryIsNotAMissingTag(t *testing.T) {
	fetch, _ := imageAvailableCheck(t, nil, &transport.Error{
		StatusCode: http.StatusNotFound,
		Errors: []transport.Diagnostic{
			{Code: transport.NameUnknownErrorCode, Message: "repository name not known to registry"},
		},
	})

	_, err := fetch.Run(t.Context())
	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "does not exist on the registry")
	assert.Contains(t, failure.Fix, "imagesRepo")
}

// TestTransportErrorKeepsTheCauseReachable: the registry's own error has to stay available for a
// support ticket, under `details:`.
func TestTransportErrorKeepsTheCauseReachable(t *testing.T) {
	cause := &transport.Error{
		StatusCode: http.StatusUnauthorized,
		Errors:     []transport.Diagnostic{{Code: transport.UnauthorizedErrorCode, Message: "authentication required"}},
	}
	fetch, _ := imageAvailableCheck(t, nil, cause)

	_, err := fetch.Run(t.Context())
	require.True(t, errors.Is(err, error(cause)), "the transport error must stay reachable")
}

// TestEditionOfADevBranchBuildIsNotApplicable: an image pulled by branch name is built from that
// branch and labelled with whatever it carried — often nothing. Comparing against it would refuse
// the ordinary way of testing a change: set devBranch, run the installer built from the same
// branch. The check is about a release, where an installer built for version X must not install
// another edition tagged X.
func TestEditionOfADevBranchBuildIsNotApplicable(t *testing.T) {
	// No DHCTL_TEST_VERSION_TAG and no embedded version file: GetImageTag falls through to
	// devBranch, which is what a development build does.
	registryCfg := registry_mocks.ConfigBuilder(
		registry_mocks.WithImagesRepo("test.registry.io/test"),
		registry_mocks.WithSchemeHTTPS(),
	)
	installer := &config.DeckhouseInstaller{Registry: registryCfg, DevBranch: "my-feature"}

	image, err := installer.GetRemoteImage(t.Context(), true)
	require.NoError(t, err)
	require.Contains(t, image, ":my-feature", "with no embedded version the tag is the branch")

	ref, err := name.ParseReference(image)
	require.NoError(t, err)

	fetch := DeckhouseImageAvailableCheck{
		MetaConfig: &config.MetaConfig{Registry: registryCfg},
		Installer:  installer,
		Image:      NewDeckhouseImage(),
		descriptor: NewFakeImageDescriptorProvider(t).ExpectReference(ref).
			Return(&v1.ConfigFile{Config: v1.Config{Labels: map[string]string{editionLabel: "CE"}}}, nil),
	}

	detail, err := fetch.Run(t.Context())
	require.NoError(t, err)
	assert.Contains(t, detail, "a development build of branch my-feature")

	edition := DhctlEditionCheck{
		BuildInfo: options.BuildInfo{AppVersion: "v1.70.3", AppEdition: "EE"},
		Image:     fetch.Image,
	}

	_, err = edition.Run(t.Context())
	assert.ErrorIs(t, err, preflight.ErrNotApplicable)
}

// TestEditionOfAReleaseTagIsStillCompared: with a version tag the comparison is the point.
func TestEditionOfAReleaseTagIsStillCompared(t *testing.T) {
	fetch, _ := imageAvailableCheck(t, &v1.ConfigFile{
		Config: v1.Config{Labels: map[string]string{editionLabel: "CE"}},
	}, nil)

	_, err := fetch.Run(t.Context())
	require.NoError(t, err)
	require.False(t, fetch.Image.FromDevBranch(), "DHCTL_TEST_VERSION_TAG stands in for an embedded version")

	edition := DhctlEditionCheck{
		BuildInfo: options.BuildInfo{AppVersion: "v1.2.3", AppEdition: "EE"},
		Image:     fetch.Image,
	}

	_, err = edition.Run(t.Context())
	require.Error(t, err)
	assert.NotErrorIs(t, err, preflight.ErrNotApplicable)
}
