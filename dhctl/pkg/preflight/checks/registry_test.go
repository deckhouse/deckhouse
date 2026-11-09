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
	"encoding/json"
	"net/http"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_mocks "github.com/deckhouse/deckhouse/dhctl/pkg/config/registrymocks"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/utils"
)

func TestCheckgetProxyFromMetaConfigSuccessHTTPSProxy(t *testing.T) {
	s := require.New(t)

	metaConfig := &config.MetaConfig{
		Registry: registry_mocks.ConfigBuilder(
			registry_mocks.WithImagesRepo("registry.deckhouse.io/test"),
			registry_mocks.WithSchemeHTTPS(),
		),
		ClusterConfig: map[string]json.RawMessage{
			"clusterDomain":     []byte(`"cluster.local"`),
			"podSubnetCIDR":     []byte(`"10.0.0.0/8"`),
			"serviceSubnetCIDR": []byte(`"11.0.0.0/8"`),
			"proxy": []byte(
				`{
                   "httpsProxy": "https://login:pass@proxy.me",
                   "httpProxy":  "http://login:pass@proxy.me"
                 }`),
		},
	}

	proxyURL, noProxyList, err := utils.GetProxyFromMetaConfig(metaConfig)
	s.NoError(err)
	s.Equal("https://login:pass@proxy.me", proxyURL.String())
	s.ElementsMatch(noProxyList, []string{"127.0.0.1", "169.254.169.254", "cluster.local", "10.0.0.0/8", "11.0.0.0/8"})
}

func TestCheckgetProxyFromMetaConfigSuccessHTTPProxy(t *testing.T) {
	s := require.New(t)

	metaConfig := &config.MetaConfig{
		Registry: registry_mocks.ConfigBuilder(
			registry_mocks.WithImagesRepo("registry.deckhouse.io/test"),
			registry_mocks.WithSchemeHTTPS(),
		),
		ClusterConfig: map[string]json.RawMessage{
			"clusterDomain":     []byte(`"cluster.local"`),
			"podSubnetCIDR":     []byte(`"10.0.0.0/8"`),
			"serviceSubnetCIDR": []byte(`"11.0.0.0/8"`),
			"proxy": []byte(
				`{
                   "httpProxy":  "http://login:pass@proxy.me"
                 }`),
		},
	}

	proxyURL, noProxyList, err := utils.GetProxyFromMetaConfig(metaConfig)
	s.NoError(err)
	s.Equal("http://login:pass@proxy.me", proxyURL.String())
	s.ElementsMatch(noProxyList, []string{"127.0.0.1", "169.254.169.254", "cluster.local", "10.0.0.0/8", "11.0.0.0/8"})
}

func TestCheckgetProxyFromMetaConfigSuccessNoProxy(t *testing.T) {
	s := require.New(t)

	metaConfig := &config.MetaConfig{
		Registry: registry_mocks.ConfigBuilder(
			registry_mocks.WithImagesRepo("registry.deckhouse.io/test"),
			registry_mocks.WithSchemeHTTPS(),
		),
		ClusterConfig: map[string]json.RawMessage{
			"clusterDomain":     []byte(`"cluster.local"`),
			"podSubnetCIDR":     []byte(`"10.0.0.0/8"`),
			"serviceSubnetCIDR": []byte(`"11.0.0.0/8"`),
		},
	}

	proxyURL, noProxyList, err := utils.GetProxyFromMetaConfig(metaConfig)
	s.NoError(err)
	s.Nil(proxyURL)
	s.Nil(noProxyList)
}

func TestCheckResponseSuccess_OKResponse(t *testing.T) {
	s := require.New(t)
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}
	resp.Header.Set("Docker-Distribution-API-Version", "registry/2.0")
	s.Nil(checkResponseIsFromDockerRegistry(resp, "registry.example.com", "Direct"))
}

func TestCheckResponseSuccess_UnauthorizedResponse(t *testing.T) {
	s := require.New(t)
	resp := &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{}}
	resp.Header.Set("Docker-Distribution-API-Version", "registry/2.0")
	s.Nil(checkResponseIsFromDockerRegistry(resp, "registry.example.com", "Direct"))
}

func TestCheckResponse_NoAPIVersionHeader(t *testing.T) {
	s := require.New(t)
	resp := &http.Response{StatusCode: http.StatusUnauthorized, Header: http.Header{}}
	s.ErrorIs(checkResponseIsFromDockerRegistry(resp, "registry.example.com", "Direct"), ErrRegistryUnreachable)
}

func TestCheckResponse_WrongStatus(t *testing.T) {
	s := require.New(t)
	resp := &http.Response{StatusCode: http.StatusForbidden, Header: http.Header{}}
	s.ErrorIs(checkResponseIsFromDockerRegistry(resp, "registry.example.com", "Direct"), ErrRegistryUnreachable)
}

// TestRegistryCredentialsWithoutCredentialsIsNotApplicable: a registry used anonymously has no
// credentials to check. This test used to assert NoError and reach registry.deckhouse.io over
// real HTTPS to get it — it failed wherever the network was unavailable, and it was passing for
// the wrong reason: the CE bypass it was written for compares a repo:tag against a bare repo
// address and never matched.
func TestRegistryCredentialsWithoutCredentialsIsNotApplicable(t *testing.T) {
	registryCfg := registry_mocks.ConfigBuilder(
		registry_mocks.WithImagesRepo("registry.deckhouse.io/deckhouse/ce"),
		registry_mocks.WithSchemeHTTPS(),
	)

	check := RegistryCredentials(
		&config.MetaConfig{Registry: registryCfg},
		&config.DeckhouseInstaller{DevBranch: "pr0001", Registry: registryCfg},
	)

	_, err := check.Run(t.Context())
	assert.ErrorIs(t, err, preflight.ErrNotApplicable)
}

// TestRegistryFieldsNameTheRegistryModuleConfigItCameFrom: a registry the registry ModuleConfig
// names, written by the user or by the installer when nothing named a registry, has its upstream
// under primary.upstream there. The failures used to send the reader to a section of the deckhouse
// ModuleConfig their configuration does not have.
func TestRegistryFieldsNameTheRegistryModuleConfigItCameFrom(t *testing.T) {
	fromModule := registry_mocks.ConfigBuilder(registry_mocks.WithModeDirect())
	fromModule.FromRegistryModule = true
	meta := &config.MetaConfig{Registry: fromModule}

	mode := configuredRegistryMode(meta)
	assert.Equal(t, registryFromModule, mode)
	assert.Equal(t, `.spec.settings.primary.upstream.host and .path in the "registry" ModuleConfig`, registryImagesRepoField(mode))
	assert.Equal(t, `.spec.settings.primary.upstream.ca in the "registry" ModuleConfig`, registryCAField(mode))
	assert.Equal(t, `.spec.settings.primary.upstream.auth.license (or username/password) in the "registry" ModuleConfig`, registryCredentialsField(mode))
	assert.NotContains(t, RegistryReachableCheck{MetaConfig: meta}.registryMode(), "direct")

	written := &config.MetaConfig{Registry: registry_mocks.ConfigBuilder(registry_mocks.WithModeDirect())}
	assert.Contains(t, registryImagesRepoField(configuredRegistryMode(written)), `.spec.settings.registry.direct.imagesRepo in the "deckhouse" ModuleConfig`)
	assert.Equal(t, "", configuredRegistryMode(nil))
}

// TestRegistryFieldsNameTheDocumentTheUserWrote: a Direct section of the deckhouse ModuleConfig, as
// Commander writes it, is installed as InitConfiguration, and InitConfiguration as legacy Unmanaged.
// The failures printed the resolved mode and sent both readers to registry.unmanaged, a section
// neither of them wrote.
func TestRegistryFieldsNameTheDocumentTheUserWrote(t *testing.T) {
	t.Run("a folded Direct section names registry.direct", func(t *testing.T) {
		folded := registry_mocks.ConfigBuilder(registry_mocks.WithModeUnmanaged(), registry_mocks.WithLegacyMode())
		folded.FoldedFromDirect = true
		meta := &config.MetaConfig{Registry: folded}

		mode := configuredRegistryMode(meta)
		assert.Equal(t, `.spec.settings.registry.direct.imagesRepo in the "deckhouse" ModuleConfig`, registryImagesRepoField(mode))
		assert.Equal(t, `.spec.settings.registry.direct.ca in the "deckhouse" ModuleConfig`, registryCAField(mode))
		assert.Equal(t, `.spec.settings.registry.direct.license (or username/password) in the "deckhouse" ModuleConfig`, registryCredentialsField(mode))
	})

	t.Run("InitConfiguration names InitConfiguration alone", func(t *testing.T) {
		stated := registry_mocks.ConfigBuilder(registry_mocks.WithModeUnmanaged(), registry_mocks.WithLegacyMode())
		stated.FromInitConfiguration = true
		meta := &config.MetaConfig{Registry: stated}

		mode := configuredRegistryMode(meta)
		assert.Equal(t, "InitConfiguration.deckhouse.imagesRepo", registryImagesRepoField(mode))
		assert.Equal(t, "InitConfiguration.deckhouse.registryCA", registryCAField(mode))
		assert.Equal(t, "InitConfiguration.deckhouse.registryDockerCfg. Key the dockercfg entry by the registry address exactly", registryCredentialsField(mode))
	})

	t.Run("an Unmanaged section written by the user keeps its own path", func(t *testing.T) {
		written := &config.MetaConfig{Registry: registry_mocks.ConfigBuilder(registry_mocks.WithModeUnmanaged())}
		assert.Contains(t, registryImagesRepoField(configuredRegistryMode(written)), `.spec.settings.registry.unmanaged.imagesRepo in the "deckhouse" ModuleConfig`)
		assert.Contains(t, registryCredentialsField(configuredRegistryMode(written)), `.spec.settings.registry.unmanaged.license (or username/password) in the "deckhouse" ModuleConfig`)
	})

	// The image check turns the same HTTP 401 into a failure of its own, so it has to name the
	// same field as the credentials check.
	t.Run("a refused image pull names the registry ModuleConfig", func(t *testing.T) {
		refused := &transport.Error{
			StatusCode: http.StatusUnauthorized,
			Errors:     []transport.Diagnostic{{Code: transport.UnauthorizedErrorCode, Message: "authentication required"}},
		}

		var failure *preflight.Failure
		require.ErrorAs(t, imageFetchFailure("registry.example.com/deckhouse/ce:v1.78.0", registryFromModule, refused), &failure)
		assert.Equal(t, `check .spec.settings.primary.upstream.auth.license (or username/password) in the "registry" ModuleConfig`, failure.Fix)
	})

	// The edition check has no configuration of its own: it reads the mode back off the image the
	// earlier check fetched, so the stand-in has to survive the trip.
	t.Run("the edition check names the same field", func(t *testing.T) {
		image := NewDeckhouseImage()
		image.set("registry.example.com/deckhouse/ee:v1.78.0", &v1.ConfigFile{}, false, registryFoldedDirect)

		edition := DhctlEditionCheck{
			BuildInfo: options.BuildInfo{AppVersion: "dev", AppEdition: "ee"},
			Image:     image,
		}

		_, err := edition.Run(t.Context())
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Fix, `.spec.settings.registry.direct.imagesRepo in the "deckhouse" ModuleConfig`)
		assert.NotContains(t, failure.Fix, "unmanaged")
	})
}
