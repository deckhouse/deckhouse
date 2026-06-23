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
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouseregistry"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

// CleanModel is the clean-config equivalent of ModeModel: it produces the same
// bootstrap artifacts (bashible/kubeadm context, registry secrets, RemoteData)
// from the {cache?, upstream?} predicates instead of a four-mode enum.
//
// Fields are exported so the model round-trips through gob (the dhctl converge/
// destroy cache encodes the whole MetaConfig, which embeds registry.Config; gob
// rejects a struct with no exported fields). This mirrors ModeModel/Data.
type CleanModel struct {
	Managed  bool
	Cache    module_config.CacheSettings
	Upstream *module_config.UpstreamSettings
	// Remote is the "pull from" registry used by preflight/infra-image pull and
	// the deckhouse registry secret: upstream for connected, the d8 mirror bundle
	// for air-gap, the external imagesRepo for unmanaged.
	Remote Data
	// InClusterImagesRepo is what cluster components address (HostWithPath for
	// managed; the external imagesRepo for unmanaged).
	InClusterImagesRepo string
}

// NewCleanModel builds a CleanModel from the parsed registry ModuleConfig.
// initImagesRepo is initConfiguration.deckhouse.imagesRepo, required for the
// unmanaged (enabled:false) case. The clean settings must already be validated.
func NewCleanModel(mc module_config.RegistryModuleConfig, initImagesRepo string) (*CleanModel, error) {
	if mc.IsUnmanaged() {
		if initImagesRepo == "" {
			return nil, fmt.Errorf("registry disabled (enabled: false) requires initConfiguration.deckhouse.imagesRepo")
		}
		return &CleanModel{
			Managed:             false,
			Remote:              Data{ImagesRepo: initImagesRepo, Scheme: constant.SchemeHTTPS},
			InClusterImagesRepo: initImagesRepo,
		}, nil
	}

	s := mc.Settings
	m := &CleanModel{
		Managed:             true,
		Cache:               s.Cache,
		Upstream:            s.Upstream,
		InClusterImagesRepo: constant.HostWithPath,
	}

	if s.Upstream == nil {
		// air-gap: images come from the d8 mirror bundle during bring-up.
		m.Remote = Data{ImagesRepo: constant.BundleImagesRepo, Scheme: constant.BundleScheme}
		return m, nil
	}

	scheme := s.Upstream.Scheme
	if scheme == "" {
		scheme = constant.SchemeHTTPS
	}
	remote := Data{
		ImagesRepo: joinHostPath(s.Upstream.Host, s.Upstream.Path),
		Scheme:     scheme,
		CA:         s.Upstream.CA,
	}
	if s.Upstream.Credentials != nil {
		remote.Username = s.Upstream.Credentials.Username
		remote.Password = s.Upstream.Credentials.Password
	}
	m.Remote = remote
	return m, nil
}

func joinHostPath(host, path string) string {
	host = strings.TrimRight(host, "/")
	path = strings.TrimLeft(path, "/")
	if path == "" {
		return host
	}
	return host + "/" + path
}

var _ ManifestBuilder = (*CleanModel)(nil)

// NeedsSeed reports air-gap: managed, cache enabled, no upstream.
func (m *CleanModel) NeedsSeed() bool {
	return m.Managed && m.Cache.Enabled && m.Upstream == nil
}

// RemoteData is the "pull from" registry for preflight, infra-image pull, and the
// deckhouse registry secret.
func (m *CleanModel) RemoteData() Data { return m.Remote }

// KubeadmContext mirrors ManifestBuilder.KubeadmContext.
func (m *CleanModel) KubeadmContext() KubeadmContext {
	address, path := helpers.SplitAddressAndPath(m.InClusterImagesRepo)
	return KubeadmContext{Address: address, Path: path}
}

// DeckhouseRegistrySecretData mirrors ManifestBuilder.DeckhouseRegistrySecretData.
func (m *CleanModel) DeckhouseRegistrySecretData(p PKIProvider) (SecretData, error) {
	pki, err := p()
	if err != nil {
		return nil, err
	}
	var inCluster Data
	if m.Managed {
		// In-cluster clients reach the agent at HostWithPath over https with the
		// module CA and the RO PKI user.
		inCluster = Data{
			ImagesRepo: constant.HostWithPath,
			Scheme:     constant.SchemeHTTPS,
			Username:   pki.ROUser.Name,
			Password:   pki.ROUser.Password,
			CA:         pki.CA.Cert,
		}
	} else {
		inCluster = m.Remote
	}

	address, path := inCluster.AddressAndPath()
	dockerCfg, err := inCluster.DockerCfg()
	if err != nil {
		return nil, fmt.Errorf("get docker config: %w", err)
	}
	cfg := deckhouse_registry.Config{
		Address:      address,
		Path:         path,
		Scheme:       strings.ToLower(string(inCluster.Scheme)),
		CA:           inCluster.CA,
		DockerConfig: dockerCfg,
	}
	return cfg.ToSecretData(), nil
}

// RegistryBashibleConfigSecretData mirrors ManifestBuilder.RegistryBashibleConfigSecretData.
// Only managed clusters carry the registry-bashible-config secret.
func (m *CleanModel) RegistryBashibleConfigSecretData(p PKIProvider) (bool, SecretData, error) {
	if !m.Managed {
		return false, nil, nil
	}
	pki, err := p()
	if err != nil {
		return true, nil, err
	}
	cfg, err := m.bashibleConfig(pki)
	if err != nil {
		return true, nil, err
	}
	cfgYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return true, nil, fmt.Errorf("marshal bashible config: %w", err)
	}
	return true, SecretData{"config": cfgYaml}, nil
}

// BashibleContext mirrors ManifestBuilder.BashibleContext.
func (m *CleanModel) BashibleContext(p PKIProvider) (BashibleContext, error) {
	pki, err := p()
	if err != nil {
		return BashibleContext{}, err
	}
	cfg, err := m.bashibleConfig(pki)
	if err != nil {
		return BashibleContext{}, err
	}
	ctx := cfg.ToContext()
	if !m.Managed {
		return ctx, nil
	}
	ctx.RegistryModuleEnable = true
	ctx.Bootstrap = &bashible.ContextBootstrap{
		Init: pki,
		Seed: m.NeedsSeed(),
	}
	return ctx, nil
}

// bashibleConfig builds the BashibleConfig (hosts + imagesBase + version) for the
// clean model. Air-gap → seed+cache hosts; connected/direct → upstream mirror.
func (m *CleanModel) bashibleConfig(pki PKI) (BashibleConfig, error) {
	cfg := BashibleConfig{
		Mode:       "Managed", // clean marker; not a four-mode ModeType
		ImagesBase: m.InClusterImagesRepo,
	}
	if !m.Managed {
		cfg.Mode = "Unmanaged"
		cfg.Hosts = unmanagedHosts(m.Remote)
	} else if m.NeedsSeed() {
		cfg.Hosts = toConfigHosts(bashible.BootstrapSeedHostsLocal(pki.CA.Cert, pki.ROUser.Name, pki.ROUser.Password))
	} else {
		// connected (with or without cache): forward to upstream during bring-up.
		host, path := m.Remote.AddressAndPath()
		cfg.Hosts = toConfigHosts(bashible.BootstrapUpstreamHosts(
			host,
			strings.ToLower(string(m.Remote.Scheme)),
			m.Remote.CA,
			m.Remote.Username,
			m.Remote.Password,
			strings.TrimLeft(path, "/"),
		))
	}
	version, err := registry_pki.ComputeHash(&cfg)
	if err != nil {
		return BashibleConfig{}, fmt.Errorf("compute version: %w", err)
	}
	cfg.Version = version
	return cfg, cfg.Validate()
}

func unmanagedHosts(remote Data) map[string]bashible.ConfigHosts {
	host, _ := remote.AddressAndPath()
	return map[string]bashible.ConfigHosts{
		host: {Mirrors: []bashible.ConfigMirrorHost{{
			Host:   host,
			Scheme: strings.ToLower(string(remote.Scheme)),
			CA:     remote.CA,
			Auth:   bashible.ConfigAuth{Username: remote.Username, Password: remote.Password},
		}}},
	}
}

// toConfigHosts converts Context hosts (returned by the bootstrap host builders)
// into Config hosts for BashibleConfig.
func toConfigHosts(in map[string]bashible.ContextHosts) map[string]bashible.ConfigHosts {
	out := make(map[string]bashible.ConfigHosts, len(in))
	for host, h := range in {
		mirrors := make([]bashible.ConfigMirrorHost, 0, len(h.Mirrors))
		for _, mh := range h.Mirrors {
			rewrites := make([]bashible.ConfigRewrite, 0, len(mh.Rewrites))
			for _, r := range mh.Rewrites {
				rewrites = append(rewrites, bashible.ConfigRewrite{From: r.From, To: r.To})
			}
			mirrors = append(mirrors, bashible.ConfigMirrorHost{
				Host:     mh.Host,
				Scheme:   mh.Scheme,
				CA:       mh.CA,
				Auth:     bashible.ConfigAuth{Username: mh.Auth.Username, Password: mh.Auth.Password},
				Rewrites: rewrites,
			})
		}
		out[host] = bashible.ConfigHosts{Mirrors: mirrors}
	}
	return out
}
