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
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// versionCheckAgainst runs the fetching half against an image with the given labels and returns
// the version check reading its result — the same split dhctl-edition uses, so no network is
// touched here either.
func versionCheckAgainst(t *testing.T, labels map[string]string, buildInfo options.BuildInfo) DhctlVersionCheck {
	t.Helper()

	fetch, _ := imageAvailableCheck(t, &v1.ConfigFile{Config: v1.Config{Labels: labels}}, nil)
	_, err := fetch.Run(t.Context())
	require.NoError(t, err)

	return DhctlVersionCheck{BuildInfo: buildInfo, Image: fetch.Image}
}

// TestDhctlVersion: the documentation has promised a version check for years and the label was
// never read. The installer writes the manifests the image's controllers then read, so the two
// are not interchangeable.
func TestDhctlVersion(t *testing.T) {
	t.Run("the versions differ", func(t *testing.T) {
		check := versionCheckAgainst(t,
			map[string]string{versionLabel: "v1.77.2"},
			options.BuildInfo{AppVersion: "v1.77.3", AppEdition: "EE"})

		_, err := check.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "the installer is v1.77.3 and the image is v1.77.2")
		assert.Contains(t, failure.Fix, "v1.77.2")
	})

	t.Run("the versions match", func(t *testing.T) {
		check := versionCheckAgainst(t,
			map[string]string{versionLabel: "v1.77.3"},
			options.BuildInfo{AppVersion: "v1.77.3", AppEdition: "EE"})

		detail, err := check.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "installer version v1.77.3 matches")
	})

	t.Run("an image from before the label existed", func(t *testing.T) {
		// Unlike the edition label this one is recent, so an older release legitimately has
		// none. Failing would make those images uninstallable.
		check := versionCheckAgainst(t,
			map[string]string{editionLabel: "EE"},
			options.BuildInfo{AppVersion: "v1.77.3", AppEdition: "EE"})

		_, err := check.Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
		assert.Contains(t, err.Error(), versionLabel)
	})

	t.Run("an installer that was not built by werf", func(t *testing.T) {
		// Building dhctl locally to test a change is an ordinary thing to do, and the binary
		// then carries no version to compare.
		for _, version := range []string{"", "local", "dev"} {
			check := versionCheckAgainst(t,
				map[string]string{versionLabel: "v1.77.3"},
				options.BuildInfo{AppVersion: version})

			_, err := check.Run(t.Context())

			require.ErrorIs(t, err, preflight.ErrNotApplicable, "version %q", version)
		}
	})

	t.Run("the image was never read", func(t *testing.T) {
		// deckhouse-image-available failed, so this check has nothing to compare and must not
		// invent an answer.
		check := DhctlVersionCheck{
			BuildInfo: options.BuildInfo{AppVersion: "v1.77.3"},
			Image:     NewDeckhouseImage(),
		}

		_, err := check.Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
	})
}
