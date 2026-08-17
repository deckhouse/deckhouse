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

package v2

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	bashible_model "github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

// settingsPath is the ModuleConfig subtree the configuration is read from.
const settingsPath = "registry"

// RegistryConfig is the resolved configuration the RegistryConfig resource is rendered
// from.
//
// Resolved here rather than in the template because one field of the user's
// configuration is not a field of the resource: `auth.license` is shorthand for a
// username and password pair, and expanding shorthand in a Helm template is how
// templates become programs.
type RegistryConfig struct {
	Mode    string        `json:"mode"`
	Primary ConfigPrimary `json:"primary"`
	Storage ConfigStorage `json:"storage"`
}

type ConfigPrimary struct {
	Upstream *ConfigUpstream `json:"upstream,omitempty"`
}

type ConfigUpstream struct {
	Scheme  string         `json:"scheme,omitempty"`
	Host    string         `json:"host"`
	Path    string         `json:"path,omitempty"`
	CA      string         `json:"ca,omitempty"`
	Auth    *ConfigAuth    `json:"auth,omitempty"`
	Mirrors []ConfigMirror `json:"mirrors,omitempty"`
}

type ConfigMirror struct {
	Scheme string      `json:"scheme,omitempty"`
	Host   string      `json:"host"`
	Path   string      `json:"path,omitempty"`
	CA     string      `json:"ca,omitempty"`
	Auth   *ConfigAuth `json:"auth,omitempty"`
}

type ConfigAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth,omitempty"`
}

type ConfigStorage struct {
	Cache             bool                     `json:"cache"`
	Size              string                   `json:"size,omitempty"`
	Source            *ConfigSource            `json:"source,omitempty"`
	GarbageCollection *ConfigGarbageCollection `json:"garbageCollection,omitempty"`
}

// ConfigGarbageCollection is when the cache reclaims the disk taken by releases the cluster has
// moved past.
type ConfigGarbageCollection struct {
	Enabled  bool   `json:"enabled"`
	Schedule string `json:"schedule,omitempty"`
}

// ConfigSource is the image set the cache is expected to hold. Required in air-gap,
// where completeness is what makes the cache trustworthy as the only source.
type ConfigSource struct {
	BundleRef       string `json:"bundleRef,omitempty"`
	ExpectedDigests int32  `json:"expectedDigests,omitempty"`
}

// settings is the shape of the ModuleConfig subtree, including the shorthand this
// package resolves away.
type settings struct {
	Mode    string `json:"mode"`
	Primary struct {
		Upstream *struct {
			Scheme string `json:"scheme"`
			Host   string `json:"host"`
			Path   string `json:"path"`
			CA     string `json:"ca"`
			Auth   *struct {
				License  string `json:"license"`
				Username string `json:"username"`
				Password string `json:"password"`
			} `json:"auth"`
			Mirrors []struct {
				Scheme string `json:"scheme"`
				Host   string `json:"host"`
				Path   string `json:"path"`
				CA     string `json:"ca"`
				Auth   *struct {
					License  string `json:"license"`
					Username string `json:"username"`
					Password string `json:"password"`
				} `json:"auth"`
			} `json:"mirrors"`
		} `json:"upstream"`
	} `json:"primary"`
	Storage struct {
		Cache  bool   `json:"cache"`
		Size   string `json:"size"`
		Source *struct {
			BundleRef       string `json:"bundleRef"`
			ExpectedDigests int32  `json:"expectedDigests"`
		} `json:"source"`
		GarbageCollection *struct {
			Enabled  bool   `json:"enabled"`
			Schedule string `json:"schedule"`
		} `json:"garbageCollection"`
	} `json:"storage"`
}

// readSettings reads the module's own configuration, with schema defaults applied.
func readSettings(input *go_hook.HookInput) (settings, error) {
	var parsed settings

	raw := input.Values.Get(settingsPath)
	if !raw.Exists() {
		return parsed, fmt.Errorf("no registry settings")
	}
	if err := json.Unmarshal([]byte(raw.Raw), &parsed); err != nil {
		return parsed, fmt.Errorf("decoding the registry settings: %w", err)
	}
	return parsed, nil
}

// buildRegistryConfig resolves the configuration into what the resource needs.
func buildRegistryConfig(from settings) RegistryConfig {
	result := RegistryConfig{
		Mode: from.Mode,
		Storage: ConfigStorage{
			Cache: from.Storage.Cache,
			Size:  from.Storage.Size,
		},
	}
	if gc := from.Storage.GarbageCollection; gc != nil {
		result.Storage.GarbageCollection = &ConfigGarbageCollection{
			Enabled:  gc.Enabled,
			Schedule: gc.Schedule,
		}
	}
	if from.Storage.Source != nil {
		result.Storage.Source = &ConfigSource{
			BundleRef:       from.Storage.Source.BundleRef,
			ExpectedDigests: from.Storage.Source.ExpectedDigests,
		}
	}
	if result.Mode == "" {
		result.Mode = string(registryv1alpha1.ModeManaged)
	}

	upstream := from.Primary.Upstream
	if upstream == nil {
		return result
	}

	resolved := &ConfigUpstream{
		Scheme: upstream.Scheme,
		Host:   upstream.Host,
		Path:   upstream.Path,
		CA:     upstream.CA,
	}
	if upstream.Auth != nil {
		resolved.Auth = resolveAuth(upstream.Auth.License, upstream.Auth.Username, upstream.Auth.Password)
	}

	for i := range upstream.Mirrors {
		mirror := &upstream.Mirrors[i]
		entry := ConfigMirror{
			Scheme: mirror.Scheme,
			Host:   mirror.Host,
			Path:   mirror.Path,
			CA:     mirror.CA,
		}
		if mirror.Auth != nil {
			entry.Auth = resolveAuth(mirror.Auth.License, mirror.Auth.Username, mirror.Auth.Password)
		}
		resolved.Mirrors = append(resolved.Mirrors, entry)
	}

	result.Primary.Upstream = resolved
	return result
}

// resolveAuth expands a license key into the credential pair it stands for.
//
// A license key is a password under a fixed username, and that is a fact about how
// Deckhouse registries are addressed rather than something a user should have to know.
// An explicit username and password always win, so a private mirror of a licensed
// registry is expressible.
func resolveAuth(license, username, password string) *ConfigAuth {
	switch {
	case username != "":
		return &ConfigAuth{Username: username, Password: password}
	case license != "":
		return &ConfigAuth{Username: registry_const.LicenseUsername, Password: license}
	default:
		return nil
	}
}

// buildBashibleConfig is what nodes are told.
//
// The one thing in it that matters most is the agent marker: it is what silences the
// bashible step that writes per-registry drop-in directories, handing that directory to
// the agent. Everything else in this structure exists because the node context is
// shared with the legacy implementation and has to remain valid.
func buildBashibleConfig(
	config RegistryConfig, access registryv1alpha1.Auth, ca string, addresses []string,
) (*bashible_model.Config, error) {
	seed, err := buildBootstrapLayout(config, access, ca, addresses)
	if err != nil {
		return nil, err
	}

	result := bashible_model.Config{
		Agent: &bashible_model.ConfigAgent{
			Endpoint:   registry_const.ProxyHost,
			DropInFile: registry_const.AgentDropInFile,
			Layout:     seed,
		},
		Mode: config.Mode,
		// Always the in-cluster address, whether or not the cache is deployed.
		//
		// This is one of the plainer consequences of the agent: a node's image
		// references name the agent's idea of "the registry", and the agent decides
		// per request whether that is the cache or the upstream. So turning the cache
		// on or off no longer rewrites every image reference on every node, which in
		// the legacy implementation was the reason that change was a node rollout.
		ImagesBase: registry_const.HostWithPath,
		// Empty, which is also how the step that installs the legacy node-side proxy
		// learns to remove itself. The agent replaces it.
		ProxyEndpoints: []string{},
		Hosts: map[string]bashible_model.ConfigHosts{
			registry_const.Host: {
				Mirrors: []bashible_model.ConfigMirrorHost{{
					Scheme: registry_const.Scheme,
					Host:   registry_const.Host,
				}},
			},
		},
	}

	version, err := registry_pki.ComputeHash(&result)
	if err != nil {
		return nil, fmt.Errorf("computing the node configuration version: %w", err)
	}
	result.Version = version

	return &result, nil
}

// buildBootstrapLayout is the routing a node uses before it has ever reached the API.
//
// It repeats what the controller computes, for the one window in which the controller's
// answer cannot be read: a node joining a cluster has to pull the control plane before
// there is a control plane to ask. Kept to the shape without additional routes, because
// those live in custom resources that a node in this state cannot read either — and
// nothing needed to bring a node up is behind one.
func buildBootstrapLayout(
	config RegistryConfig, access registryv1alpha1.Auth, ca string, addresses []string,
) (string, error) {
	spec := registryv1alpha1.RegistryNodeSpec{Cache: config.Storage.Cache}

	if config.Storage.Cache && len(addresses) > 0 {
		// Addresses, not the Service name. A node reading this file has no cluster DNS
		// and, at the point it reads it, no cluster network either — it is bootstrapping.
		// Every replica holds the same content, so the rest are mirrors to fail over to.
		storage := registryv1alpha1.Backend{
			Name:     registryv1alpha1.BackendStorage,
			Endpoint: storageEndpoint(addresses[0], ca, access),
		}
		for _, address := range addresses[1:] {
			storage.Mirrors = append(storage.Mirrors, storageEndpoint(address, ca, access))
		}
		spec.Backends = append(spec.Backends, storage)
	}

	if upstream := config.Primary.Upstream; upstream != nil {
		spec.Backends = append(spec.Backends, registryv1alpha1.Backend{
			Name:     registryv1alpha1.BackendUpstream,
			Endpoint: endpointOf(upstream.Scheme, upstream.Host, upstream.Path, upstream.CA, upstream.Auth),
		})

		for i := range upstream.Mirrors {
			mirror := &upstream.Mirrors[i]
			spec.Backends[len(spec.Backends)-1].Mirrors = append(
				spec.Backends[len(spec.Backends)-1].Mirrors,
				endpointOf(mirror.Scheme, mirror.Host, mirror.Path, mirror.CA, mirror.Auth))
		}
	}

	if len(spec.Backends) == 0 {
		// An air-gapped cluster with the cache turned off has no source of images at
		// all, which the configuration schema already refuses. Nothing to seed, and
		// saying so is better than writing a file the agent will reject.
		return "", nil
	}

	encoded, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("encoding the bootstrap layout: %w", err)
	}
	return string(encoded), nil
}

// storageEndpoint is one replica of the in-cluster cache.
func storageEndpoint(address, ca string, auth registryv1alpha1.Auth) registryv1alpha1.Endpoint {
	return registryv1alpha1.Endpoint{
		Scheme: registryv1alpha1.SchemeHTTPS,
		Host:   withRegistryPort(address),
		Path:   registry_const.Path,
		CA:     ca,
		Auth:   &auth,
	}
}

// withRegistryPort adds the registry port to a bare address, since the addresses are
// discovered as node addresses and the port is a constant of the design.
func withRegistryPort(address string) string {
	if strings.Contains(address, ":") && !strings.HasPrefix(address, "[") {
		return address
	}
	return fmt.Sprintf("%s:%d", address, registry_const.Port)
}

func endpointOf(scheme, host, path, ca string, auth *ConfigAuth) registryv1alpha1.Endpoint {
	endpoint := registryv1alpha1.Endpoint{
		Scheme: registryv1alpha1.Scheme(scheme),
		Host:   host,
		Path:   path,
		CA:     ca,
	}
	if endpoint.Scheme == "" {
		endpoint.Scheme = registryv1alpha1.SchemeHTTPS
	}
	if auth != nil {
		endpoint.Auth = &registryv1alpha1.Auth{
			Username: auth.Username,
			Password: auth.Password,
			Auth:     auth.Auth,
		}
	}
	return endpoint
}
