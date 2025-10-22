// Copyright 2025 Flant JSC
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
	"encoding/base64"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	registry_init "github.com/deckhouse/deckhouse/go_lib/registry/models/init"
)

type ConfigBuilder struct {
	registry *Registry
}

func (builder *ConfigBuilder) InclusterData() (Data, error) {
	switch {
	case builder.registry.spec.Unmanaged != nil:
		unmanaged := builder.registry.spec.Unmanaged
		return Data{
			ImagesRepo: unmanaged.ImagesRepo,
			Scheme:     unmanaged.Scheme,
			CA:         unmanaged.CA,
			Username:   unmanaged.Username,
			Password:   unmanaged.Password,
		}, nil
	case builder.registry.spec.Direct != nil:
		direct := builder.registry.spec.Direct
		pki := builder.registry.pki
		return Data{
			ImagesRepo: registry_const.HostWithPath,
			Scheme:     SchemeHTTPS,
			CA:         pki.CA.Cert,
			Username:   direct.Username,
			Password:   direct.Password,
		}, nil
	default:
		return Data{}, ErrorUnknownRegistryMode
	}
}

func (builder *ConfigBuilder) InclusterImagesRepo() string {
	if builder.registry.spec.Unmanaged != nil {
		return builder.registry.spec.Unmanaged.ImagesRepo
	}
	return registry_const.HostWithPath
}

func (builder *ConfigBuilder) BashibleTplCtx() (map[string]interface{}, error) {
	var (
		imagesBase string
		mirrorHost string
		mirrors    []bashible.ContextMirrorHost
	)

	switch {
	case builder.registry.spec.Unmanaged != nil:
		unmanaged := builder.registry.spec.Unmanaged
		imagesBase = unmanaged.ImagesRepo
		mirrorHost, mirrors = unmanagedHostMirrors(unmanaged)
	case builder.registry.spec.Direct != nil:
		direct := builder.registry.spec.Direct
		imagesBase = registry_const.HostWithPath
		mirrorHost, mirrors = directHostMirrors(direct)
	default:
		return nil, ErrorUnknownRegistryMode
	}

	ret := bashible.Context{
		Mode:       builder.registry.spec.Mode,
		ImagesBase: imagesBase,
		Hosts: map[string]bashible.ContextHosts{
			mirrorHost: {Mirrors: mirrors}},
	}

	version, err := computeHash(&ret)
	if err != nil {
		return nil, fmt.Errorf("failed to compute config version: %w", err)
	}
	ret.Version = version

	if err := ret.Validate(); err != nil {
		return nil, err
	}
	return ret.ToMap()
}

func (builder *ConfigBuilder) KubeadmTplCtx() (map[string]interface{}, error) {
	data, err := builder.InclusterData()
	if err != nil {
		return nil, err
	}

	address, path := addressAndPathFromImagesRepo(data.ImagesRepo)
	authBase64 := data.AuthBase64()
	dockerCfgBase64, err := data.DockerCfgBase64()
	if err != nil {
		return nil, err
	}

	ret := map[string]interface{}{
		"address":   address,
		"path":      path,
		"scheme":    strings.ToLower(string(data.Scheme)),
		"ca":        data.CA,
		"dockerCfg": dockerCfgBase64,
	}
	if authBase64 != "" {
		ret["auth"] = authBase64
	}
	return ret, nil
}

func (builder *ConfigBuilder) DeckhouseRegistrySecretData() (map[string][]byte, error) {
	data, err := builder.InclusterData()
	if err != nil {
		return nil, err
	}

	address, path := addressAndPathFromImagesRepo(data.ImagesRepo)
	dockerCfgBase64, err := data.DockerCfgBase64()
	if err != nil {
		return nil, err
	}
	dockerCfg, err := base64.StdEncoding.DecodeString(dockerCfgBase64)
	if err != nil {
		return nil, err
	}

	ret := map[string][]byte{
		corev1.DockerConfigJsonKey: dockerCfg,
		"address":                  []byte(address),
		"scheme":                   []byte(strings.ToLower(string(data.Scheme))),
		"imagesRegistry":           []byte(data.ImagesRepo),
	}

	if path != "" {
		ret["path"] = []byte(path)
	}

	if data.CA != "" {
		ret["ca"] = []byte(data.CA)
	}
	return ret, nil
}

func (builder *ConfigBuilder) RegistryBootstrapSecretData() (map[string][]byte, error) {
	config, err := yaml.Marshal(
		registry_init.Config{
			CA: &registry_init.CertKey{
				Cert: builder.registry.pki.CA.Cert,
				Key:  builder.registry.pki.CA.Key,
			},
			UserRW: &builder.registry.pki.UserRW,
			UserRO: &builder.registry.pki.UserRO,
		})
	if err != nil {
		return nil, err
	}

	ret := map[string][]byte{
		"config": config,
	}
	return ret, nil
}

func unmanagedHostMirrors(unmapaged *UnmanagedModeSpec) (string, []bashible.ContextMirrorHost) {
	host, _ := addressAndPathFromImagesRepo(unmapaged.ImagesRepo)
	return host, []bashible.ContextMirrorHost{{
		Host:   host,
		CA:     unmapaged.CA,
		Scheme: strings.ToLower(string(unmapaged.Scheme)),
		Auth: bashible.ContextAuth{
			Username: unmapaged.Username,
			Password: unmapaged.Password,
		},
	}}
}

func directHostMirrors(direct *DirectModeSpec) (string, []bashible.ContextMirrorHost) {
	host, path := addressAndPathFromImagesRepo(direct.ImagesRepo)
	return registry_const.Host, []bashible.ContextMirrorHost{{
		Host:   host,
		CA:     direct.CA,
		Scheme: strings.ToLower(string(direct.Scheme)),
		Auth: bashible.ContextAuth{
			Username: direct.Username,
			Password: direct.Password,
		},
		Rewrites: []bashible.ContextRewrite{{
			From: registry_const.PathRegexp,
			To:   strings.TrimLeft(path, "/"),
		}},
	}}
}
