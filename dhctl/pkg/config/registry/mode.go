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
	"strings"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/module-config"
)

type ModeSettings struct {
	Mode   constant.ModeType
	Remote Data
}

func newModeSettings(settings module_config.DeckhouseSettings) (ModeSettings, error) {
	switch {
	case settings.Direct != nil:
		remote := Data{}
		remote.FromRegistrySettings(*settings.Direct)
		return ModeSettings{
			Mode:   constant.ModeDirect,
			Remote: remote,
		}, nil
	case settings.Unmanaged != nil:
		remote := Data{}
		remote.FromRegistrySettings(*settings.Unmanaged)
		return ModeSettings{
			Mode:   constant.ModeUnmanaged,
			Remote: remote,
		}, nil
	}
	return ModeSettings{}, ErrUnknownMode
}

func (s ModeSettings) ToModel() ModeModel {
	switch s.Mode {
	case constant.ModeDirect:
		return s.directModel()
	default:
		return s.unmanagedModel()
	}
}

func (s ModeSettings) directModel() ModeModel {
	return ModeModel{
		ModuleRequired:      true,
		Mode:                constant.ModeDirect,
		InClusterImagesRepo: constant.HostWithPath,
		RemoteImagesRepo:    s.Remote.ImagesRepo,
		RemoteData:          s.Remote,
	}
}

func (s ModeSettings) unmanagedModel() ModeModel {
	return ModeModel{
		ModuleRequired:      false,
		Mode:                constant.ModeUnmanaged,
		InClusterImagesRepo: s.Remote.ImagesRepo,
		RemoteImagesRepo:    s.Remote.ImagesRepo,
		RemoteData:          s.Remote,
	}
}

type ModeModel struct {
	ModuleRequired      bool
	Mode                constant.ModeType
	InClusterImagesRepo string
	RemoteImagesRepo    string
	RemoteData          Data
}

func (m ModeModel) InClusterData(getPKI func() (PKI, error)) (Data, error) {
	switch m.Mode {
	case constant.ModeDirect:
		return m.directInClusterData(getPKI)
	default:
		return m.unmanagedInClusterData()
	}
}

func (m ModeModel) BashibleMirrors() (
	ctxHosts map[string]bashible.ContextHosts,
	cfgHosts map[string]bashible.ConfigHosts,
) {
	switch m.Mode {
	case constant.ModeDirect:
		return m.directBashibleMirrors()
	default:
		return m.unmanagedBashibleMirrors()
	}
}

func (m ModeModel) directInClusterData(getPKI func() (PKI, error)) (Data, error) {
	pki, err := getPKI()
	if err != nil {
		return Data{}, err
	}

	return Data{
		ImagesRepo: constant.HostWithPath,
		Scheme:     constant.SchemeHTTPS,
		Username:   m.RemoteData.Username,
		Password:   m.RemoteData.Password,
		CA:         pki.CA.Cert,
	}, nil
}

func (m ModeModel) unmanagedInClusterData() (Data, error) {
	return m.RemoteData, nil
}

func (m ModeModel) directBashibleMirrors() (
	map[string]bashible.ContextHosts,
	map[string]bashible.ConfigHosts,
) {
	host, path := m.RemoteData.AddressAndPath()
	scheme := strings.ToLower(string(m.RemoteData.Scheme))
	from := constant.PathRegexp
	to := strings.TrimLeft(path, "/")

	ctxMirror := bashible.ContextMirrorHost{
		Host:   host,
		Scheme: scheme,
		CA:     m.RemoteData.CA,
		Auth: bashible.ContextAuth{
			Username: m.RemoteData.Username,
			Password: m.RemoteData.Password,
		},
		Rewrites: []bashible.ContextRewrite{{
			From: from,
			To:   to,
		}},
	}

	cfgMirror := bashible.ConfigMirrorHost{
		Host:   host,
		Scheme: scheme,
		CA:     m.RemoteData.CA,
		Auth: bashible.ConfigAuth{
			Username: m.RemoteData.Username,
			Password: m.RemoteData.Password,
		},
		Rewrites: []bashible.ConfigRewrite{{
			From: from,
			To:   to,
		}},
	}

	return map[string]bashible.ContextHosts{
			constant.Host: {
				Mirrors: []bashible.ContextMirrorHost{ctxMirror}},
		}, map[string]bashible.ConfigHosts{
			constant.Host: {
				Mirrors: []bashible.ConfigMirrorHost{cfgMirror}},
		}
}

func (m ModeModel) unmanagedBashibleMirrors() (
	map[string]bashible.ContextHosts,
	map[string]bashible.ConfigHosts,
) {
	host, _ := m.RemoteData.AddressAndPath()
	scheme := strings.ToLower(string(m.RemoteData.Scheme))

	ctxMirror := bashible.ContextMirrorHost{
		Host:   host,
		Scheme: scheme,
		CA:     m.RemoteData.CA,
		Auth: bashible.ContextAuth{
			Username: m.RemoteData.Username,
			Password: m.RemoteData.Password,
		},
	}

	cfgMirror := bashible.ConfigMirrorHost{
		Host:   host,
		Scheme: scheme,
		CA:     m.RemoteData.CA,
		Auth: bashible.ConfigAuth{
			Username: m.RemoteData.Username,
			Password: m.RemoteData.Password,
		},
	}

	return map[string]bashible.ContextHosts{
			host: {
				Mirrors: []bashible.ContextMirrorHost{ctxMirror}},
		}, map[string]bashible.ConfigHosts{
			host: {
				Mirrors: []bashible.ConfigMirrorHost{cfgMirror}},
		}
}
