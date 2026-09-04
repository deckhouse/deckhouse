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
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
)

// modeManaged is the value of `spec.settings.mode` that means the module owns the pull path.
//
// A literal rather than the constant it mirrors: dhctl is its own Go module and does not depend on the
// module's API package. The source of truth is `ModeManaged` in
// go_lib/registry/apis/deckhouse.io/v1alpha1/conditions.go — if it is ever renamed, what breaks here is
// a bundle installation quietly deciding it is not one — a silent wrong answer rather than an error —
// so the test beside this file works on a ModuleConfig of the shape the module really accepts.
const modeManaged = "Managed"

// Installing a cluster whose images come from a bundle rather than from a registry it can reach.
//
// Everything this needs already exists and is not built here: `--img-bundle-path` serves the bundle
// from an OCI registry on the operator's machine, an SSH reverse tunnel makes it reachable from the
// first master, a temporary registry on the node holds it, and registry-syncer copies it into the
// cluster's own store. All of that is switched on by one thing — the installation resolving to the
// bundle mode.
//
// What is decided here is only WHEN to resolve to it, and the decision is deliberately made from
// three separate facts rather than from a mode the operator types: a cache to hold the images, no
// upstream to fetch them from, and a bundle to take them out of. Any one of the three alone is a
// coherent installation of something else, which is exactly why a partial combination has to be
// refused rather than interpreted.
type BundleBootstrapInputs struct {
	// CacheEnabled is whether the module was asked for a store of its own.
	CacheEnabled bool

	// UpstreamConfigured is whether an upstream registry was given to pull from.
	UpstreamConfigured bool

	// BundlePath is the value of --img-bundle-path, empty when the flag was not given.
	BundlePath string

	// Upstream is where the module was told to pull from, when it was told anything.
	//
	// Read so that a cluster can be installed with no registry named anywhere but in its own
	// ModuleConfig — which is the point of the module owning the pull path, and which the installer
	// could not do: it took the registry from `InitConfiguration.deckhouse` or from the deckhouse
	// ModuleConfig, and with neither present it fell back to the public CE registry. A test that
	// removed those sections to prove the module stands on its own would therefore have proved
	// nothing: the installation would have gone to a registry nobody asked for.
	Upstream *UpstreamFacts
}

// UpstreamFacts is the primary upstream as the registry ModuleConfig states it.
type UpstreamFacts struct {
	// Scheme, Host and Path address the registry. Host is required; the rest have defaults.
	Scheme string
	Host   string
	Path   string

	// CA verifies it, empty meaning the system trust store.
	CA string

	// Username and Password authenticate to it. A license is the ordinary shape of these for
	// Deckhouse's own registry, where the token is the password.
	Username string
	Password string
}

var (
	// ErrBundleWithoutCache and the two below are refusals, not diagnostics: each one describes a
	// configuration that has two readings, and picking one silently is how an operator ends up
	// with a cluster that installed successfully and cannot pull an image.
	ErrBundleWithoutCache = errors.New(
		"a bundle was given but the registry cache is disabled: there would be nowhere to put the " +
			"images. Enable the cache, or drop --img-bundle-path to install from a registry")

	ErrBundleWithUpstream = errors.New(
		"a bundle was given together with an upstream registry: these are two different sources for " +
			"the same images and only one of them can be the source of truth. Remove the upstream to " +
			"install from the bundle, or drop --img-bundle-path to install from the upstream")

	ErrCacheWithoutSource = errors.New(
		"the registry cache is enabled with no upstream and no bundle: the cluster would have nothing " +
			"to pull from. Give an upstream, or pass --img-bundle-path to install from a bundle")
)

// IsBundleBootstrap reports whether this installation takes its images from a bundle.
//
// Returns an error for every combination that names a bundle without being able to use one, and for
// the combination that asks for a cache with nothing to fill it from. Refusing here costs a message
// at the start of an installation; not refusing costs a cluster that comes up and then cannot pull,
// with `no such host` from containerd as the only clue — which is what the store answers with when
// an image is simply absent.
func IsBundleBootstrap(in BundleBootstrapInputs) (bool, error) {
	if in.BundlePath != "" {
		if !in.CacheEnabled {
			return false, fmt.Errorf("%w (bundle: %q)", ErrBundleWithoutCache, in.BundlePath)
		}
		if in.UpstreamConfigured {
			return false, fmt.Errorf("%w (bundle: %q)", ErrBundleWithUpstream, in.BundlePath)
		}

		return true, nil
	}

	// No bundle. A cache with an upstream is the ordinary cache installation; a cache without one has
	// no source at all, and saying so now is the whole point of this branch.
	if in.CacheEnabled && !in.UpstreamConfigured {
		return false, ErrCacheWithoutSource
	}

	return false, nil
}

// BundleFactsFromModuleConfig reads the two facts the rule needs out of the registry ModuleConfig.
//
// Two fields and not a model of the whole object, deliberately. The module's settings are described by
// its own openapi schema and validated against it inside the cluster; a Go struct here would be a
// second, silent copy of that schema, and the copy would go out of date the first time a field was
// added without anybody noticing — the installer would then read a configuration it half understands.
// What is read instead is exactly what the decision consumes, by path, and anything unexpected simply
// leaves the facts false.
//
// A module that is switched off, or one that manages nothing, has no cache to speak of: both answer
// "no cache" rather than an error, because neither is a bundle installation and neither is a mistake.
func BundleFactsFromModuleConfig(doc []byte) (BundleBootstrapInputs, error) {
	var in BundleBootstrapInputs

	var obj unstructured.Unstructured
	if err := yaml.Unmarshal(doc, &obj); err != nil {
		return in, fmt.Errorf("parse the registry ModuleConfig: %w", err)
	}

	if enabled, found, err := unstructured.NestedBool(obj.Object, "spec", "enabled"); err == nil && found && !enabled {
		return in, nil
	}

	// An absent mode means the module's own default, which manages the registry.
	mode, _, err := unstructured.NestedString(obj.Object, "spec", "settings", "mode")
	if err != nil {
		return in, fmt.Errorf("read the registry mode: %w", err)
	}
	if mode != "" && mode != modeManaged {
		return in, nil
	}

	cache, _, err := unstructured.NestedBool(obj.Object, "spec", "settings", "storage", "cache")
	if err != nil {
		return in, fmt.Errorf("read storage.cache: %w", err)
	}
	in.CacheEnabled = cache

	// An upstream counts as configured when it names a host. An empty block is what a configuration
	// looks like on its way to being edited, and treating it as a source would send the installation
	// looking for images at nowhere in particular.
	host, _, err := unstructured.NestedString(obj.Object, "spec", "settings", "primary", "upstream", "host")
	if err != nil {
		return in, fmt.Errorf("read primary.upstream.host: %w", err)
	}
	in.UpstreamConfigured = host != ""
	if !in.UpstreamConfigured {
		return in, nil
	}

	upstream := UpstreamFacts{Host: host}
	for field, into := range map[string]*string{
		"scheme": &upstream.Scheme,
		"path":   &upstream.Path,
		"ca":     &upstream.CA,
	} {
		value, _, err := unstructured.NestedString(
			obj.Object, "spec", "settings", "primary", "upstream", field)
		if err != nil {
			return in, fmt.Errorf("read primary.upstream.%s: %w", field, err)
		}
		*into = value
	}

	// The credentials live one level deeper, under `auth`, and reading them a level too shallow was a
	// measured failure rather than a tidiness point: the fields came back empty, the installer sent an
	// unauthenticated request, and the preflight refused the installation with
	// "preflight check \"registry-credentials\" failed. reason: authentication failed" — naming the
	// registry as the culprit for what was a misread of our own configuration. It went unnoticed while
	// `InitConfiguration.deckhouse` still carried a registry, because then nothing needed this.
	for field, into := range map[string]*string{
		"username": &upstream.Username,
		"password": &upstream.Password,
	} {
		value, _, err := unstructured.NestedString(
			obj.Object, "spec", "settings", "primary", "upstream", "auth", field)
		if err != nil {
			return in, fmt.Errorf("read primary.upstream.auth.%s: %w", field, err)
		}
		*into = value
	}

	// A license is how Deckhouse's own registry is authenticated: the token is the password, under
	// a fixed user name. Read as an alternative to the pair rather than in addition to it, and the
	// module's own schema says the same thing more strongly — it refuses a configuration carrying both
	// ("'license' and 'username'/'password' are mutually exclusive").
	if upstream.Username == "" && upstream.Password == "" {
		license, _, err := unstructured.NestedString(
			obj.Object, "spec", "settings", "primary", "upstream", "auth", "license")
		if err != nil {
			return in, fmt.Errorf("read primary.upstream.auth.license: %w", err)
		}
		if license != "" {
			upstream.Username, upstream.Password = licenseUser, license
		}
	}

	in.Upstream = &upstream
	return in, nil
}

// licenseUser is the account name a Deckhouse license authenticates as.
const licenseUser = "license-token"

// Resolve turns the facts read from the registry ModuleConfig into the arguments NewConfigProvider
// needs: the registry settings to use, and the options that go with them.
//
// One function called from both places dhctl decides this, rather than the same three lines twice.
// dhctl decides it twice because it reads the configuration twice — once over raw documents, to know
// which registry to download candi and the provider plugins from, and once over the parsed
// configuration, where the result becomes MetaConfig.Registry and from there the bashible context and
// the node manifests. The two disagreeing is neither a compile error nor a visible failure: the
// installer would fetch its own images from the bundle and then hand the nodes a configuration that
// says the registry is unmanaged. That is not hypothetical — it is exactly how the first installation
// from a bundle came up with an empty store and Deckhouse in ImagePullBackOff, because only the first
// of the two places knew about the bundle.
//
// Returns deckhouseSettings unchanged, and no options, for every configuration that is not an
// installation from a bundle. In particular the deckhouse ModuleConfig keeps the precedence documented
// on IsLocal: this path only supplies a mode where the legacy configuration expressed none.
func (in BundleBootstrapInputs) Resolve(
	deckhouseSettings *module_config.DeckhouseSettings,
) (*module_config.DeckhouseSettings, []ProviderOption) {
	// The deckhouse ModuleConfig keeps the precedence documented on IsLocal: this path only supplies
	// a registry where the legacy configuration expressed none.
	if deckhouseSettings != nil {
		return deckhouseSettings, nil
	}

	switch {
	case in.CacheEnabled && !in.UpstreamConfigured:
		// A cache with nothing to fill it from over the network is an installation from a bundle.
		return &module_config.DeckhouseSettings{Mode: constant.ModeLocal},
			[]ProviderOption{WithBundleBootstrap(), WithStore()}

	case in.Upstream != nil:
		// The registry the module was told to use, used by the installer as well.
		//
		// Without this the installer had no way to learn it: it read `InitConfiguration.deckhouse` or
		// the deckhouse ModuleConfig, and with neither present fell back to the public CE registry —
		// so a cluster whose only statement about a registry is its own ModuleConfig would have been
		// installed from somewhere nobody named. Which also made the honest air-gap test impossible:
		// removing those sections to prove the module stands on its own proved nothing.
		//
		// Direct rather than Unmanaged, because this is a module that manages the pull path; the
		// provider downgrades it by itself where the container runtime cannot support that.
		settings := module_config.New(constant.ModeDirect)
		settings.Direct = &module_config.RegistrySettings{
			ImagesRepo: in.Upstream.imagesRepo(),
			Scheme:     in.Upstream.scheme(),
			CA:         in.Upstream.CA,
			Username:   in.Upstream.Username,
			Password:   in.Upstream.Password,
		}

		// And the agent, because this is a cluster whose pull path the module owns. It is the
		// installer that has to put the first one there: see WithAgent.
		//
		// A cache alongside an upstream is not a bundle installation, but it is still a store the
		// installation is not finished without — and the store is what the installer has to wait
		// for, because nothing else reports on it any more.
		if in.CacheEnabled {
			return &settings, []ProviderOption{WithStore(), WithAgent()}
		}
		return &settings, []ProviderOption{WithAgent()}
	}

	return deckhouseSettings, nil
}

// imagesRepo is the upstream as one address, in the form the installer addresses images by.
func (u *UpstreamFacts) imagesRepo() string {
	host := strings.Trim(u.Host, "/")
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return host
	}
	return host + "/" + path
}

// scheme defaults to HTTPS, as the module's own schema does.
func (u *UpstreamFacts) scheme() constant.SchemeType {
	if u.Scheme == "" {
		return constant.SchemeHTTPS
	}
	return constant.SchemeType(strings.ToUpper(u.Scheme))
}
