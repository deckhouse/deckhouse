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
	"context"
	"strings"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry/types"
)

var (
	_ Mode = &DirectMode{}
	_ Mode = &UnmanagedMode{}
)

type Mode interface {
	Mode() string
	IsModuleRequired() bool
	RemoteImagesRepo() string
	InClusterImagesRepo() string
	RemoteData() types.Data
	InClusterData(ctx context.Context, pki PKIProvider) (types.Data, error)
	BashibleMirrors(
		ctx context.Context,
		pki PKIProvider,
	) (string, []bashible.ContextMirrorHost, []bashible.ConfigMirrorHost, error)
}

type DirectMode struct {
	Remote types.Data
}

type UnmanagedMode struct {
	Remote types.Data
}

func (m *DirectMode) Mode() registry_const.ModeType {
	return registry_const.ModeDirect
}

func (m *DirectMode) IsModuleRequired() bool {
	return true
}

func (m *DirectMode) RemoteImagesRepo() string {
	return m.Remote.ImagesRepo
}

func (m *DirectMode) InClusterImagesRepo() string {
	return registry_const.HostWithPath
}

func (m *DirectMode) RemoteData() types.Data {
	return m.Remote
}

func (m *DirectMode) InClusterData(
	ctx context.Context,
	pkiProvider PKIProvider,
) (types.Data, error) {
	pki, err := pkiProvider.Get(ctx)
	if err != nil {
		return types.Data{}, err
	}

	remote := m.RemoteData()

	return types.Data{
		ImagesRepo: registry_const.HostWithPath,
		Scheme:     types.SchemeHTTPS,
		Username:   remote.Username,
		Password:   remote.Password,
		CA:         pki.CA.Cert,
	}, nil
}

func (m *DirectMode) BashibleMirrors(
	_ context.Context,
	_ PKIProvider,
) (string, []bashible.ContextMirrorHost, []bashible.ConfigMirrorHost, error) {
	host, path := m.Remote.AddressAndPath()
	username, password := m.Remote.Username, m.Remote.Password
	scheme := strings.ToLower(string(m.Remote.Scheme))
	ca := m.Remote.CA

	from := registry_const.PathRegexp
	to := strings.TrimLeft(path, "/")

	ctxMirrors := []bashible.ContextMirrorHost{{
		Host:   host,
		Scheme: scheme,
		CA:     ca,
		Auth:   bashible.ContextAuth{Username: username, Password: password},
		Rewrites: []bashible.ContextRewrite{{
			From: from,
			To:   to,
		}},
	}}

	cfgMirrors := []bashible.ConfigMirrorHost{{
		Host:   host,
		Scheme: scheme,
		CA:     ca,
		Auth:   bashible.ConfigAuth{Username: username, Password: password},
		Rewrites: []bashible.ConfigRewrite{{
			From: from,
			To:   to,
		}},
	}}
	return registry_const.Host, ctxMirrors, cfgMirrors, nil
}

func (m *UnmanagedMode) Mode() registry_const.ModeType {
	return registry_const.ModeUnmanaged
}

func (m *UnmanagedMode) IsModuleRequired() bool {
	return false
}

func (m *UnmanagedMode) RemoteImagesRepo() string {
	return m.Remote.ImagesRepo
}

func (m *UnmanagedMode) InClusterImagesRepo() string {
	return m.Remote.ImagesRepo
}

func (m *UnmanagedMode) RemoteData() types.Data {
	return m.Remote
}

func (m *UnmanagedMode) InClusterData(
	_ context.Context,
	_ PKIProvider,
) (types.Data, error) {
	return m.Remote, nil
}

func (m *UnmanagedMode) BashibleMirrors(
	_ context.Context,
	_ PKIProvider,
) (string, []bashible.ContextMirrorHost, []bashible.ConfigMirrorHost, error) {
	host, _ := m.Remote.AddressAndPath()
	username, password := m.Remote.Username, m.Remote.Password
	scheme := strings.ToLower(string(m.Remote.Scheme))
	ca := m.Remote.CA

	ctxMirrors := []bashible.ContextMirrorHost{{
		Host:   host,
		Scheme: scheme,
		CA:     ca,
		Auth:   bashible.ContextAuth{Username: username, Password: password},
	}}

	cfgMirrors := []bashible.ConfigMirrorHost{{
		Host:   host,
		Scheme: scheme,
		CA:     ca,
		Auth:   bashible.ConfigAuth{Username: username, Password: password},
	}}
	return host, ctxMirrors, cfgMirrors, nil
}
