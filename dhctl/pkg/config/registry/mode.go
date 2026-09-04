// Copyright 2025 Flant JSC
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
package registry

import (
	"encoding/json"
	"fmt"
	"strings"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

const (
	discoveredNodeIP = "${discovered_node_ip}"
)

type ModeSettings struct {
	Mode       constant.ModeType
	RemoteData Data
	TTL        string
}

func newModeSettings(settings module_config.DeckhouseSettings) (ModeSettings, error) {
	switch {
	case settings.Direct != nil:
		var remote Data
		remote.fromRegistrySettings(*settings.Direct)

		return ModeSettings{
			Mode:       constant.ModeDirect,
			RemoteData: remote,
		}, nil

	case settings.Proxy != nil:
		var remote Data
		remote.fromRegistrySettings(settings.Proxy.RegistrySettings)

		return ModeSettings{
			Mode:       constant.ModeProxy,
			RemoteData: remote,
			TTL:        settings.Proxy.TTL,
		}, nil

	case settings.Unmanaged != nil:
		var remote Data
		remote.fromRegistrySettings(*settings.Unmanaged)

		return ModeSettings{
			Mode:       constant.ModeUnmanaged,
			RemoteData: remote,
		}, nil

	case settings.Mode == constant.ModeLocal:
		remote := Data{
			ImagesRepo: constant.BundleImagesRepo,
			Scheme:     constant.BundleScheme,
		}

		return ModeSettings{
			Mode:       constant.ModeLocal,
			RemoteData: remote,
		}, nil

	default:
		return ModeSettings{}, ErrUnknownMode
	}
}

func (s ModeSettings) ToModel() ModeModel {
	switch s.Mode {
	case constant.ModeDirect:
		return s.toDirectModel()

	case constant.ModeProxy:
		return s.toProxyModel()

	case constant.ModeLocal:
		return s.toLocalModel()

	case constant.ModeUnmanaged:
		return s.toUnmanagedModel()

	default:
		panic(ErrUnknownMode)
	}
}

func (s ModeSettings) toDirectModel() ModeModel {
	return ModeModel{
		Mode:                constant.ModeDirect,
		InClusterImagesRepo: constant.HostWithPath,
		RemoteImagesRepo:    s.RemoteData.ImagesRepo,
		RemoteData:          s.RemoteData,
	}
}

func (s ModeSettings) toProxyModel() ModeModel {
	return ModeModel{
		Mode:                constant.ModeProxy,
		InClusterImagesRepo: constant.HostWithPath,
		RemoteImagesRepo:    s.RemoteData.ImagesRepo,
		RemoteData:          s.RemoteData,
		TTL:                 s.TTL,
	}
}

func (s ModeSettings) toLocalModel() ModeModel {
	return ModeModel{
		Mode:                constant.ModeLocal,
		InClusterImagesRepo: constant.HostWithPath,
		RemoteImagesRepo:    s.RemoteData.ImagesRepo,
		RemoteData:          s.RemoteData,
	}
}

func (s ModeSettings) toUnmanagedModel() ModeModel {
	return ModeModel{
		Mode:                constant.ModeUnmanaged,
		InClusterImagesRepo: s.RemoteData.ImagesRepo,
		RemoteImagesRepo:    s.RemoteData.ImagesRepo,
		RemoteData:          s.RemoteData,
	}
}

func (s *ModeSettings) DeepCopyInto(out *ModeSettings) {
	*out = *s
	s.RemoteData.DeepCopyInto(&out.RemoteData)
}

func (s *ModeSettings) DeepCopy() *ModeSettings {
	if s == nil {
		return nil
	}
	out := new(ModeSettings)
	s.DeepCopyInto(out)
	return out
}

type ModeModel struct {
	Mode                constant.ModeType
	InClusterImagesRepo string
	RemoteImagesRepo    string
	RemoteData          Data
	TTL                 string

	// AgentOwnsRuntime means the first master is installed with the node agent already on it,
	// rather than with the container runtime pointed straight at the upstream.
	//
	// It is what closes the deployment circle. The agent is a static pod whose image comes from a
	// registry package, and every package a node fetches comes through registry-packages-proxy —
	// which on a running cluster reaches the registry THROUGH the agent. A cluster whose first
	// master has no agent therefore has no way to install one: measured on a cache-less cluster as
	// `[registry-agent] attempt 6 failed` on the master and thirty failed `rpp-get` attempts on
	// every worker after it, with no node ever joining.
	//
	// The installer is the one party outside that circle: its own proxy serves packages over the
	// dhctl tunnel, from the upstream named in the configuration, needing nothing in the cluster.
	// So the first master gets its agent from there, and from then on the proxy on that master
	// serves every other node through the agent beside it.
	AgentOwnsRuntime bool
}

func (m ModeModel) InClusterData(pki PKI) (Data, error) {
	switch m.Mode {
	case constant.ModeDirect:
		return m.toDirectInClusterData(pki), nil

	case constant.ModeProxy, constant.ModeLocal:
		return m.toProxyLocalInClusterData(pki), nil

	case constant.ModeUnmanaged:
		return m.RemoteData, nil

	default:
		return Data{}, ErrUnknownMode
	}
}

func (m ModeModel) BashibleConfig(pki PKI) (BashibleConfig, error) {
	if m.AgentOwnsRuntime {
		return m.agentBashibleConfig()
	}

	var (
		mirrors   map[string]bashible.ConfigHosts
		endpoints []string
	)

	switch m.Mode {
	case constant.ModeDirect:
		mirrors = m.toDirectBashibleHosts()

	case constant.ModeUnmanaged:
		mirrors = m.toUnmanagedBashibleHosts()

	case constant.ModeProxy, constant.ModeLocal:
		mirrors = m.toProxyLocalBashibleHosts(pki)
		endpoints = m.toProxyLocalEndpoints()

	default:
		return BashibleConfig{}, ErrUnknownMode
	}

	cfg := BashibleConfig{
		Mode:           string(m.Mode),
		ImagesBase:     m.InClusterImagesRepo,
		ProxyEndpoints: endpoints,
		Hosts:          mirrors,
	}

	version, err := registry_pki.ComputeHash(&cfg)
	if err != nil {
		return BashibleConfig{}, fmt.Errorf("compute version: %w", err)
	}

	cfg.Version = version
	return cfg, cfg.Validate()
}

// agentBashibleConfig is what the first master is told when it comes up with the agent on it.
//
// The same shape the module writes for every node once it is running — the agent marker, the
// in-cluster address, no proxy endpoints and one host with no credentials on it — because a node
// whose configuration changes the moment the module takes over is a node that rolls its container
// runtime for no reason. The credentials are deliberately absent here: they belong to the layout
// the agent routes by, and nothing else on the node has to hold them.
func (m ModeModel) agentBashibleConfig() (BashibleConfig, error) {
	layout, err := m.agentBootstrapLayout()
	if err != nil {
		return BashibleConfig{}, err
	}

	cfg := BashibleConfig{
		Agent: &bashible.ConfigAgent{
			Endpoint:   constant.ProxyHost,
			DropInFile: constant.AgentDropInFile,
			Layout:     layout,
		},
		Mode:       string(m.Mode),
		ImagesBase: constant.HostWithPath,
		// Empty rather than absent: this is also how the step that installs the legacy node-side
		// proxy learns to remove itself.
		ProxyEndpoints: []string{},
		Hosts: map[string]bashible.ConfigHosts{
			constant.Host: {
				Mirrors: []bashible.ConfigMirrorHost{{
					Scheme: constant.Scheme,
					Host:   constant.Host,
				}},
			},
		},
	}

	version, err := registry_pki.ComputeHash(&cfg)
	if err != nil {
		return BashibleConfig{}, fmt.Errorf("compute version: %w", err)
	}

	cfg.Version = version
	return cfg, cfg.Validate()
}

// agentBootstrapLayout is the routing the agent uses before it has ever reached an API server.
//
// On the first master there is no API server to ask yet — it is what this node is about to bring
// up — so the upstream from the installation configuration is written into the layout directly.
// The controller replaces it with the cluster's own answer as soon as there is one, and from then
// on this file is not consulted again.
//
// Only the upstream, and no storage backend: the store does not exist during the installation
// even when the cluster is going to have one, and naming an address that answers nothing would
// cost every pull a failed attempt before its fallback.
func (m ModeModel) agentBootstrapLayout() (string, error) {
	host, path := m.RemoteData.AddressAndPath()

	endpoint := layoutEndpoint{
		Scheme: string(m.RemoteData.Scheme),
		Host:   host,
		Path:   path,
		CA:     m.RemoteData.CA,
	}
	if m.RemoteData.Username != "" || m.RemoteData.Password != "" {
		endpoint.Auth = &layoutAuth{
			Username: m.RemoteData.Username,
			Password: m.RemoteData.Password,
		}
	}

	spec := bootstrapLayout{
		Backends: []layoutBackend{{
			Name:           backendUpstream,
			layoutEndpoint: endpoint,
		}},
	}

	encoded, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("encode the bootstrap layout: %w", err)
	}
	return string(encoded), nil
}

func (m ModeModel) toDirectInClusterData(pki PKI) Data {
	return Data{
		ImagesRepo: constant.HostWithPath,
		Scheme:     constant.SchemeHTTPS,
		Username:   m.RemoteData.Username,
		Password:   m.RemoteData.Password,
		CA:         pki.CA.Cert,
	}
}

func (m ModeModel) toProxyLocalInClusterData(pki PKI) Data {
	return Data{
		ImagesRepo: constant.HostWithPath,
		Scheme:     constant.SchemeHTTPS,
		Username:   pki.ROUser.Name,
		Password:   pki.ROUser.Password,
		CA:         pki.CA.Cert,
	}
}

func (m ModeModel) toDirectBashibleHosts() map[string]bashible.ConfigHosts {
	host, path := m.RemoteData.AddressAndPath()
	scheme := strings.ToLower(string(m.RemoteData.Scheme))
	from := constant.PathRegexp
	to := strings.TrimLeft(path, "/")

	ret := map[string]bashible.ConfigHosts{
		constant.Host: {
			Mirrors: []bashible.ConfigMirrorHost{
				{
					Host:   host,
					Scheme: scheme,
					CA:     m.RemoteData.CA,
					Auth: bashible.ConfigAuth{
						Username: m.RemoteData.Username,
						Password: m.RemoteData.Password,
					},
					Rewrites: []bashible.ConfigRewrite{
						{
							From: from,
							To:   to,
						},
					},
				},
			},
		},
	}

	return ret
}

func (m ModeModel) toUnmanagedBashibleHosts() map[string]bashible.ConfigHosts {
	host, _ := m.RemoteData.AddressAndPath()
	scheme := strings.ToLower(string(m.RemoteData.Scheme))

	ret := map[string]bashible.ConfigHosts{
		host: {
			Mirrors: []bashible.ConfigMirrorHost{
				{
					Host:   host,
					Scheme: scheme,
					CA:     m.RemoteData.CA,
					Auth: bashible.ConfigAuth{
						Username: m.RemoteData.Username,
						Password: m.RemoteData.Password,
					},
				},
			},
		},
	}

	return ret
}

func (m ModeModel) toProxyLocalBashibleHosts(pki PKI) map[string]bashible.ConfigHosts {
	endpoints := m.toProxyLocalEndpoints()

	hosts := make([]string, 0, 1+len(endpoints))

	// ProxyHost is the main endpoint for accessing the registry-proxy service.
	hosts = append(hosts, constant.ProxyHost)

	// Append endpoints for direct access.
	// These are used when the registry-proxy is not yet running (during bootstrap).
	hosts = append(hosts, endpoints...)

	scheme := strings.ToLower(string(constant.SchemeHTTPS))
	mirrors := make([]bashible.ConfigMirrorHost, 0, len(hosts))

	for _, host := range hosts {
		mirrors = append(mirrors,
			bashible.ConfigMirrorHost{
				Host:   host,
				Scheme: scheme,
				CA:     pki.CA.Cert,
				Auth: bashible.ConfigAuth{
					Username: pki.ROUser.Name,
					Password: pki.ROUser.Password,
				},
			},
		)
	}

	ret := map[string]bashible.ConfigHosts{
		constant.Host: {
			Mirrors: mirrors,
		},
	}

	return ret
}

func (m ModeModel) toProxyLocalEndpoints() []string {
	return constant.GenerateProxyEndpoints([]string{discoveredNodeIP})
}
