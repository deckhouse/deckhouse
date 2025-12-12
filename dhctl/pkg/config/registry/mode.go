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
	"fmt"
	"strings"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/module-config"
	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

type ModeSettings struct {
	Mode       constant.ModeType
	RemoteData Data
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

	case settings.Unmanaged != nil:
		var remote Data
		remote.fromRegistrySettings(*settings.Unmanaged)

		return ModeSettings{
			Mode:       constant.ModeUnmanaged,
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

func (s ModeSettings) toUnmanagedModel() ModeModel {
	return ModeModel{
		Mode:                constant.ModeUnmanaged,
		InClusterImagesRepo: s.RemoteData.ImagesRepo,
		RemoteImagesRepo:    s.RemoteData.ImagesRepo,
		RemoteData:          s.RemoteData,
	}
}

type ModeModel struct {
	Mode                constant.ModeType
	InClusterImagesRepo string
	RemoteImagesRepo    string
	RemoteData          Data
}

func (m ModeModel) InClusterData(pkiProvider PKIProvider) (Data, error) {
	switch m.Mode {
	case constant.ModeDirect:
		return m.toDirectInClusterData(pkiProvider)

	case constant.ModeUnmanaged:
		return m.RemoteData, nil

	default:
		return Data{}, ErrUnknownMode
	}
}

func (m ModeModel) BashibleConfig() (BashibleConfig, error) {
	var mirrors map[string]bashible.ConfigHosts

	switch m.Mode {
	case constant.ModeDirect:
		mirrors = m.toDirectBashibleHosts()

	case constant.ModeUnmanaged:
		mirrors = m.toUnmanagedBashibleHosts()

	default:
		return BashibleConfig{}, ErrUnknownMode
	}

	cfg := BashibleConfig{
		Mode:       string(m.Mode),
		ImagesBase: m.InClusterImagesRepo,
		Hosts:      mirrors,
	}

	version, err := pki.ComputeHash(&cfg)
	if err != nil {
		return BashibleConfig{}, fmt.Errorf("compute version: %w", err)
	}

	cfg.Version = version
	return cfg, cfg.Validate()
}

func (m ModeModel) toDirectInClusterData(pkiProvider PKIProvider) (Data, error) {
	pki, err := pkiProvider()
	if err != nil {
		return Data{}, fmt.Errorf("get PKI: %w", err)
	}

	return Data{
		ImagesRepo: constant.HostWithPath,
		Scheme:     constant.SchemeHTTPS,
		Username:   m.RemoteData.Username,
		Password:   m.RemoteData.Password,
		CA:         pki.CA.Cert,
	}, nil
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
