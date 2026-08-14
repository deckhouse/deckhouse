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

package registry

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
	"github.com/deckhouse/deckhouse/pkg/registry/fake"
)

// fakeRegistry builds the library tree over an in-memory fake registry, scoped
// to the fe edition. Images are added under "deckhouse/fe/…", matching the
// paths the tree addresses.
func fakeRegistry(reg *fake.Registry) *dhregistry.Registry {
	return dhregistry.New(
		fake.NewClient(reg).WithSegment("deckhouse"),
		dhregistry.WithEdition(dhregistry.FEEdition),
	)
}

// versionImage builds a release image carrying a version.json declaring version.
func versionImage(version string) *fake.ImageBuilder {
	return fake.NewImageBuilder().WithFile("version.json", `{"version":"`+version+`"}`)
}

// captureStdout redirects os.Stdout for the duration of fn and returns what it
// wrote. It is not safe for parallel use, so these tests do not call t.Parallel.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	orig := os.Stdout
	os.Stdout = w

	runErr := fn()

	require.NoError(t, w.Close())
	os.Stdout = orig

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	return string(out), runErr
}

func TestValidateEnumFlag(t *testing.T) {
	allowed := []string{"alpha", "beta", "stable"}

	// Empty is treated as unset and skips the check.
	assert.NoError(t, validateEnumFlag(nil, "channel", "", allowed...))
	assert.NoError(t, validateEnumFlag(nil, "channel", "stable", allowed...))

	err := validateEnumFlag(nil, "channel", "nightly", allowed...)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel")
	assert.Contains(t, err.Error(), "alpha, beta, stable")
}

func TestSemVerRegex(t *testing.T) {
	for _, tag := range []string{"v1.73.0", "1.73.0", "v1.73", "v1", "1.2.3-rc.1", "v1.2.3+build.4"} {
		assert.True(t, semVerRegex.MatchString(tag), tag)
	}

	for _, tag := range []string{"stable", "alpha", "early-access", "latest", "meta-123"} {
		assert.False(t, semVerRegex.MatchString(tag), tag)
	}
}

func TestHandleListDeckhouseReleases(t *testing.T) {
	reg := fake.NewRegistry("registry.example.com")
	img := versionImage("v1.73.0").MustBuild()
	// The list command reads the Deckhouse image repository, whose tags are
	// versions and channel names alike.
	for _, tag := range []string{"v1.73.0", "v1.72.0", "stable", "rock-solid"} {
		reg.MustAddImage("deckhouse/fe", tag, img)
	}

	svc := fakeRegistry(reg).Deckhouse().BasicService

	t.Run("semver only by default", func(t *testing.T) {
		out, err := captureStdout(t, func() error {
			return handleListDeckhouseReleases(t.Context(), svc, false)
		})
		require.NoError(t, err)

		lines := nonEmptyLines(out)
		assert.ElementsMatch(t, []string{"v1.73.0", "v1.72.0"}, lines)
	})

	t.Run("all tags with --all", func(t *testing.T) {
		out, err := captureStdout(t, func() error {
			return handleListDeckhouseReleases(t.Context(), svc, true)
		})
		require.NoError(t, err)

		lines := nonEmptyLines(out)
		assert.ElementsMatch(t, []string{"v1.73.0", "v1.72.0", "stable", "rock-solid"}, lines)
	})
}

func TestHandleGetDeckhouseRelease(t *testing.T) {
	reg := fake.NewRegistry("registry.example.com")
	reg.MustAddImage("deckhouse/fe/release-channel", "stable", versionImage("v1.73.0").MustBuild())

	releases := fakeRegistry(reg).Deckhouse().Releases()

	t.Run("version line by default", func(t *testing.T) {
		out, err := captureStdout(t, func() error {
			return handleGetDeckhouseRelease(t.Context(), releases, "stable", false)
		})
		require.NoError(t, err)
		assert.Equal(t, "Deckhouse version in channel 'stable': v1.73.0", strings.TrimSpace(out))
	})

	t.Run("raw version.json with --all", func(t *testing.T) {
		out, err := captureStdout(t, func() error {
			return handleGetDeckhouseRelease(t.Context(), releases, "stable", true)
		})
		require.NoError(t, err)

		var decoded struct {
			Version string `json:"version"`
		}
		require.NoError(t, json.Unmarshal([]byte(out), &decoded))
		assert.Equal(t, "v1.73.0", decoded.Version)
	})

	t.Run("missing channel", func(t *testing.T) {
		_, err := captureStdout(t, func() error {
			return handleGetDeckhouseRelease(t.Context(), releases, "alpha", false)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "channel 'alpha' is not found")
	})
}

func TestHandleListModulesNames(t *testing.T) {
	reg := fake.NewRegistry("registry.example.com")
	scratch := fake.NewImageBuilder().MustBuild()

	t.Run("names found", func(t *testing.T) {
		reg.MustAddImage("deckhouse/fe/modules", "stronghold", scratch)
		reg.MustAddImage("deckhouse/fe/modules", "neuvector", scratch)

		out, err := captureStdout(t, func() error {
			return handleListModulesNames(t.Context(), fakeRegistry(reg).Modules(), false)
		})
		require.NoError(t, err)

		assert.Contains(t, out, "Modules found (2)")
		lines := nonEmptyLines(out)
		assert.Contains(t, lines, "stronghold")
		assert.Contains(t, lines, "neuvector")
	})

	t.Run("empty catalog", func(t *testing.T) {
		empty := fake.NewRegistry("registry.example.com")
		out, err := captureStdout(t, func() error {
			return handleListModulesNames(t.Context(), fakeRegistry(empty).Modules(), true)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "Modules not found")
	})
}

func TestHandleListModulesVersions(t *testing.T) {
	reg := fake.NewRegistry("registry.example.com")
	img := versionImage("v1.0.1").MustBuild()
	for _, tag := range []string{"v1.0.0", "v1.0.1", "alpha"} {
		reg.MustAddImage("deckhouse/fe/modules/stronghold", tag, img)
	}

	catalog := fakeRegistry(reg).Modules()

	t.Run("semver only by default", func(t *testing.T) {
		out, err := captureStdout(t, func() error {
			return handleListModulesVersions(t.Context(), catalog, "stronghold", false)
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"v1.0.0", "v1.0.1"}, nonEmptyLines(out))
	})

	t.Run("all tags with --all", func(t *testing.T) {
		out, err := captureStdout(t, func() error {
			return handleListModulesVersions(t.Context(), catalog, "stronghold", true)
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"v1.0.0", "v1.0.1", "alpha"}, nonEmptyLines(out))
	})

	t.Run("no semver tags", func(t *testing.T) {
		other := fake.NewRegistry("registry.example.com")
		other.MustAddImage("deckhouse/fe/modules/legacy", "alpha", img)

		out, err := captureStdout(t, func() error {
			return handleListModulesVersions(t.Context(), fakeRegistry(other).Modules(), "legacy", false)
		})
		require.NoError(t, err)
		assert.Contains(t, out, "Module releases with semVer not found")
	})
}

func TestHandleGetModuleInfoInChannel(t *testing.T) {
	reg := fake.NewRegistry("registry.example.com")
	reg.MustAddImage("deckhouse/fe/modules/stronghold/release", "alpha",
		fake.NewImageBuilder().
			WithFile("version.json", `{"version":"v1.0.1"}`).
			WithFile("module.yaml", "name: stronghold\nweight: 910\n").
			MustBuild())

	catalog := fakeRegistry(reg).Modules()

	t.Run("version line by default", func(t *testing.T) {
		out, err := captureStdout(t, func() error {
			return handleGetModuleInfoInChannel(t.Context(), catalog, "stronghold", "alpha", false)
		})
		require.NoError(t, err)
		assert.Equal(t, "Module version in channel 'alpha': v1.0.1", strings.TrimSpace(out))
	})

	t.Run("version and manifest with --all", func(t *testing.T) {
		out, err := captureStdout(t, func() error {
			return handleGetModuleInfoInChannel(t.Context(), catalog, "stronghold", "alpha", true)
		})
		require.NoError(t, err)

		var decoded struct {
			Version string `json:"version"`
			Module  struct {
				Name   string `json:"name"`
				Weight uint32 `json:"weight"`
			} `json:"module"`
		}
		require.NoError(t, json.Unmarshal([]byte(out), &decoded))
		assert.Equal(t, "v1.0.1", decoded.Version)
		assert.Equal(t, "stronghold", decoded.Module.Name)
		assert.Equal(t, uint32(910), decoded.Module.Weight)
	})

	t.Run("missing channel", func(t *testing.T) {
		_, err := captureStdout(t, func() error {
			return handleGetModuleInfoInChannel(t.Context(), catalog, "stronghold", "beta", false)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "channel 'beta' is not found")
	})

	t.Run("release without version.json", func(t *testing.T) {
		broken := fake.NewRegistry("registry.example.com")
		broken.MustAddImage("deckhouse/fe/modules/stronghold/release", "alpha",
			fake.NewImageBuilder().WithFile("unrelated.txt", "x").MustBuild())

		_, err := captureStdout(t, func() error {
			return handleGetModuleInfoInChannel(t.Context(), fakeRegistry(broken).Modules(), "stronghold", "alpha", false)
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "metadata malformed")
	})

	t.Run("release without manifest still returns version", func(t *testing.T) {
		noManifest := fake.NewRegistry("registry.example.com")
		noManifest.MustAddImage("deckhouse/fe/modules/stronghold/release", "alpha", versionImage("v1.0.1").MustBuild())

		out, err := captureStdout(t, func() error {
			return handleGetModuleInfoInChannel(t.Context(), fakeRegistry(noManifest).Modules(), "stronghold", "alpha", true)
		})
		require.NoError(t, err)

		var decoded struct {
			Version string          `json:"version"`
			Module  json.RawMessage `json:"module"`
		}
		require.NoError(t, json.Unmarshal([]byte(out), &decoded))
		assert.Equal(t, "v1.0.1", decoded.Version)
		// The manifest is omitted when the release does not carry it.
		assert.Empty(t, decoded.Module)
	})
}

// nonEmptyLines splits captured output into its non-blank lines.
func nonEmptyLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}

	return lines
}
