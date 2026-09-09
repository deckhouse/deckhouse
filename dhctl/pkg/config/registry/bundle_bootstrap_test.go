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

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
)

// TestIsBundleBootstrap covers the refusals as carefully as the success, because the refusals are the
// reason this is a function rather than three conditions written at the call site.
//
// The failure this guards against is not a wrong verdict, it is a plausible one: a configuration that
// names a bundle and cannot use it installs perfectly well and then cannot pull an image, and what an
// operator sees at that point is containerd reporting `no such host` for a registry that exists.
// Every refusal below is a message delivered minutes before that, instead of it.
func TestIsBundleBootstrap(t *testing.T) {
	cases := []struct {
		name string
		in   BundleBootstrapInputs
		want bool
		// err is the sentinel expected, nil when the combination is legitimate.
		err error
	}{{
		// The whole point: a store to fill, nothing to fill it from over the network, and a bundle
		// to fill it from instead.
		name: "a cache, no upstream and a bundle",
		in:   BundleBootstrapInputs{CacheEnabled: true, BundlePath: "/bundles/air"},
		want: true,
	}, {
		name: "a bundle with no cache to put it in",
		in:   BundleBootstrapInputs{BundlePath: "/bundles/air"},
		err:  ErrBundleWithoutCache,
	}, {
		// Two sources for one set of images. Choosing either one silently would make the other a lie.
		name: "a bundle and an upstream at once",
		in:   BundleBootstrapInputs{CacheEnabled: true, UpstreamConfigured: true, BundlePath: "/bundles/air"},
		err:  ErrBundleWithUpstream,
	}, {
		name: "a cache with nothing to fill it from",
		in:   BundleBootstrapInputs{CacheEnabled: true},
		err:  ErrCacheWithoutSource,
	}, {
		// The ordinary managed installation, which must stay ordinary.
		name: "a cache filled from an upstream",
		in:   BundleBootstrapInputs{CacheEnabled: true, UpstreamConfigured: true},
		want: false,
	}, {
		name: "an upstream and no cache",
		in:   BundleBootstrapInputs{UpstreamConfigured: true},
		want: false,
	}, {
		// Nothing configured at all is somebody else's validation to fail, not this one's: the module
		// decides what an empty configuration means, and answering "not a bundle install" is honest.
		name: "nothing configured",
		in:   BundleBootstrapInputs{},
		want: false,
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := IsBundleBootstrap(test.in)

			if test.err != nil {
				require.Error(t, err, "this combination has more than one reading and must be refused")
				assert.ErrorIs(t, err, test.err)
				assert.False(t, got, "a refused configuration must not also report itself usable")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

// TestBundleFactsFromModuleConfig reads the two facts out of the module's own configuration.
//
// The documents below are the shape the module really accepts — the cache one is what the test harness
// installs its cache variants with, field for field. That matters more than it looks: what is read here
// is two paths into somebody else's schema, and a rename on the other side would not fail, it would
// answer "no cache, no upstream" and turn a bundle installation into an ordinary one.
func TestBundleFactsFromModuleConfig(t *testing.T) {
	const managedWithUpstream = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  enabled: true
  version: 1
  settings:
    mode: Managed
    primary:
      upstream:
        scheme: HTTPS
        host: dev-registry.deckhouse.io
        path: /sys/deckhouse-oss
        auth:
          license: a-licence-key
    storage:
      cache: true
      size: 10Gi
`

	// The air-gapped shape: a cache and no upstream at all, which is what a bundle installation is
	// configured as.
	const managedNoUpstream = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  enabled: true
  version: 1
  settings:
    mode: Managed
    storage:
      cache: true
      size: 10Gi
`

	const unmanaged = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  enabled: true
  version: 1
  settings:
    mode: Unmanaged
    storage:
      cache: true
`

	const disabled = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  enabled: false
  settings:
    mode: Managed
    storage:
      cache: true
`

	// An upstream block with no host in it: a configuration on its way to being edited, and not a
	// source of anything.
	const emptyUpstream = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  enabled: true
  settings:
    mode: Managed
    primary:
      upstream:
        scheme: HTTPS
    storage:
      cache: true
`

	cases := []struct {
		name     string
		doc      string
		cache    bool
		upstream bool
	}{
		{name: "managed with a cache and an upstream", doc: managedWithUpstream, cache: true, upstream: true},
		{name: "managed with a cache and no upstream", doc: managedNoUpstream, cache: true},
		{name: "unmanaged manages nothing, so no cache", doc: unmanaged},
		{name: "a disabled module has no cache", doc: disabled},
		{name: "an upstream block without a host is not an upstream", doc: emptyUpstream, cache: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			in, err := BundleFactsFromModuleConfig([]byte(test.doc))
			require.NoError(t, err)

			assert.Equal(t, test.cache, in.CacheEnabled, "cache")
			assert.Equal(t, test.upstream, in.UpstreamConfigured, "upstream")
		})
	}
}

// TestBundleFactsAndTheRuleTogether is the combination that will actually be run at install time, and
// it is worth one test of its own: the facts come from the module's configuration and the path comes
// from the command line, and neither half means anything without the other.
func TestBundleFactsAndTheRuleTogether(t *testing.T) {
	const airGapped = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  enabled: true
  settings:
    mode: Managed
    storage:
      cache: true
`

	in, err := BundleFactsFromModuleConfig([]byte(airGapped))
	require.NoError(t, err)

	// Without the flag this configuration has no source of images, and saying so is the point.
	_, err = IsBundleBootstrap(in)
	assert.ErrorIs(t, err, ErrCacheWithoutSource)

	in.BundlePath = "/bundles/air"
	got, err := IsBundleBootstrap(in)
	require.NoError(t, err)
	assert.True(t, got, "a cache, no upstream and a bundle is a bundle installation")
}

// TestBundleRefusalsNameTheBundle: an operator reading the refusal has to be able to tell which path
// was rejected, because the flag is often passed by a script rather than typed.
func TestBundleRefusalsNameTheBundle(t *testing.T) {
	for _, in := range []BundleBootstrapInputs{
		{BundlePath: "/bundles/air"},
		{CacheEnabled: true, UpstreamConfigured: true, BundlePath: "/bundles/air"},
	} {
		_, err := IsBundleBootstrap(in)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/bundles/air")
	}
}

// TestTheInstallerTakesTheRegistryFromTheModuleThatOwnsIt is what lets a cluster be installed with no
// registry named anywhere except in the module's own configuration.
//
// The installer used to read it from `InitConfiguration.deckhouse` or from the deckhouse ModuleConfig,
// and with neither present it fell back to the public CE registry. So a configuration that states its
// registry once, in the object that owns the pull path, was installed from somewhere nobody named —
// and the test meant to prove the module stands on its own proved nothing, because the installation
// never depended on the module in the first place.
func TestTheInstallerTakesTheRegistryFromTheModuleThatOwnsIt(t *testing.T) {
	const managedWithUpstream = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  enabled: true
  version: 1
  settings:
    mode: Managed
    primary:
      upstream:
        scheme: HTTPS
        host: dev-registry.deckhouse.io
        path: /sys/deckhouse-oss
        ca: |
          -----BEGIN CERTIFICATE-----
        auth:
          username: someone
          password: secret
    storage:
      cache: true
      size: 10Gi
`

	facts, err := BundleFactsFromModuleConfig([]byte(managedWithUpstream))
	require.NoError(t, err)
	require.NotNil(t, facts.Upstream)

	settings, options := facts.Resolve(nil)
	require.NotNil(t, settings)

	// One option and not two: the cache is a store the installer has to wait for, but the images
	// still come from the upstream, so this is not a bundle installation.
	provider := NewConfigProvider(nil, settings, options...)
	assert.True(t, provider.storeExpected, "a configured cache is a store the installation waits for")
	assert.False(t, provider.bundleBootstrap, "an upstream is not a bundle installation")

	require.Equal(t, constant.ModeDirect, settings.Mode,
		"the module manages the pull path, so the installation is not unmanaged")
	require.NotNil(t, settings.Direct)
	assert.Equal(t, "dev-registry.deckhouse.io/sys/deckhouse-oss", settings.Direct.ImagesRepo,
		"host and path make one address, which is how images are named")
	assert.Equal(t, constant.SchemeHTTPS, settings.Direct.Scheme)
	assert.Equal(t, "someone", settings.Direct.Username)
	assert.Equal(t, "secret", settings.Direct.Password)
	assert.Contains(t, settings.Direct.CA, "BEGIN CERTIFICATE")
}

// TestALicenseIsCredentialsToo: for Deckhouse's own registry the token is the password, under a fixed
// user name, and a configuration that states a license states credentials.
//
// Nested under `auth`, as the module's schema puts it — and these fixtures said otherwise until a
// cluster disagreed: read a level too shallow the credentials came back empty, the installer sent an
// unauthenticated request, and the preflight blamed the registry
// ("registry-credentials failed. reason: authentication failed"). A fixture in the wrong shape is why a
// passing test proved nothing.
func TestALicenseIsCredentialsToo(t *testing.T) {
	const withLicense = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  enabled: true
  settings:
    mode: Managed
    primary:
      upstream:
        host: registry.deckhouse.io
        path: /deckhouse/ee
        auth:
          license: a-license-token
`

	facts, err := BundleFactsFromModuleConfig([]byte(withLicense))
	require.NoError(t, err)

	settings, _ := facts.Resolve(nil)
	require.NotNil(t, settings.Direct)
	assert.Equal(t, "license-token", settings.Direct.Username)
	assert.Equal(t, "a-license-token", settings.Direct.Password)
	assert.Equal(t, constant.SchemeHTTPS, settings.Direct.Scheme, "the default the module's schema states")
}

// TestTheDeckhouseModuleConfigStillWins keeps the precedence the installer documented: this path only
// supplies a registry where the legacy configuration expressed none.
func TestTheDeckhouseModuleConfigStillWins(t *testing.T) {
	stated := &module_config.DeckhouseSettings{Mode: constant.ModeUnmanaged}

	facts := BundleBootstrapInputs{
		UpstreamConfigured: true,
		Upstream:           &UpstreamFacts{Host: "example.com"},
	}

	settings, options := facts.Resolve(stated)
	assert.Same(t, stated, settings)
	assert.Empty(t, options)
}
