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
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouse-registry"
	registry_init "github.com/deckhouse/deckhouse/go_lib/registry/models/init"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

type ConfigBuilder struct {
	registry *Registry
}

func (b *ConfigBuilder) DeckhouseSettings() (interface{}, error) {
	data, err := json.Marshal(b.registry.Spec)
	if err != nil {
		return nil, fmt.Errorf("unable to encode registry spec: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unable to decode registry spec into map: %w", err)
	}
	return result, nil
}

func (b *ConfigBuilder) InclusterData() (Data, error) {
	switch {
	case b.registry.Spec.Unmanaged != nil:
		unmanaged := b.registry.Spec.Unmanaged
		username, password := unmanaged.UsernamePassword()
		return Data{
			ImagesRepo: unmanaged.ImagesRepo,
			Scheme:     unmanaged.Scheme,
			CA:         unmanaged.CA,
			Username:   username,
			Password:   password,
		}, nil
	case b.registry.Spec.Direct != nil:
		direct := b.registry.Spec.Direct
		username, password := direct.UsernamePassword()
		return Data{
			ImagesRepo: registry_const.HostWithPath,
			Scheme:     SchemeHTTPS,
			CA:         b.registry.PKI.CA.Cert,
			Username:   username,
			Password:   password,
		}, nil
	default:
		return Data{}, ErrorUnknownRegistryMode
	}
}

func (b *ConfigBuilder) InclusterImagesRepo() string {
	if b.registry.Spec.Unmanaged != nil {
		return b.registry.Spec.Unmanaged.ImagesRepo
	}
	return registry_const.HostWithPath
}

func (b *ConfigBuilder) KubeadmTplCtx() (map[string]interface{}, error) {
	data, err := b.InclusterData()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster registry data: %w", err)
	}

	address, path := addressAndPathFromImagesRepo(data.ImagesRepo)
	dockerCfgBase64, err := data.DockerCfgBase64()
	if err != nil {
		return nil, fmt.Errorf("failed to encode Docker config: %w", err)
	}

	ctx := map[string]interface{}{
		"address":   address,
		"path":      path,
		"scheme":    strings.ToLower(string(data.Scheme)),
		"ca":        data.CA,
		"dockerCfg": dockerCfgBase64,
	}

	if auth := data.AuthBase64(); auth != "" {
		ctx["auth"] = auth
	}
	return ctx, nil
}

func (b *ConfigBuilder) DeckhouseRegistrySecretData() (map[string][]byte, error) {
	data, err := b.InclusterData()
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

	ret := deckhouse_registry.Config{
		Address:      address,
		Path:         path,
		Scheme:       strings.ToLower(string(data.Scheme)),
		CA:           data.CA,
		DockerConfig: dockerCfg,
	}
	return ret.ToMap(), nil
}

func (b *ConfigBuilder) RegistryInitSecretData() (map[string][]byte, error) {
	config, err := yaml.Marshal(
		registry_init.Config{
			CA: &registry_init.CertKey{
				Cert: b.registry.PKI.CA.Cert,
				Key:  b.registry.PKI.CA.Key,
			},
			UserRW: &b.registry.PKI.UserRW,
			UserRO: &b.registry.PKI.UserRO,
		})
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		"config": config,
	}, nil
}

func (b *ConfigBuilder) RegistryBashibleConfigSecretData() (map[string][]byte, error) {
	var (
		err error
		cfg bashible.Config
	)

	_, cfg, err = b.bashibleContexAndConfig(true)
	if err != nil {
		return nil, err
	}

	config, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		"config": config,
	}, nil
}

func (b *ConfigBuilder) BashibleTplCtx() (map[string]interface{}, error) {
	var (
		err error
		ctx bashible.Context
	)
	ctx, _, err = b.bashibleContexAndConfig(true)
	if err != nil {
		return nil, err
	}
	return ctx.ToMap()
}

func (b *ConfigBuilder) bashibleContexAndConfig(registryModuleEnable bool) (bashible.Context, bashible.Config, error) {
	var (
		imagesBase string
		mirrorHost string
		ctxMirrors []bashible.ContextMirrorHost
		cfgMirrors []bashible.ConfigMirrorHost
	)

	switch {
	case b.registry.Spec.Unmanaged != nil:
		unmanaged := b.registry.Spec.Unmanaged
		imagesBase = unmanaged.ImagesRepo
		mirrorHost, ctxMirrors, cfgMirrors = unmanagedHostMirrors(unmanaged)
	case b.registry.Spec.Direct != nil:
		direct := b.registry.Spec.Direct
		imagesBase = registry_const.HostWithPath
		mirrorHost, ctxMirrors, cfgMirrors = directHostMirrors(direct)
	default:
		return bashible.Context{}, bashible.Config{}, ErrorUnknownRegistryMode
	}

	ctx := bashible.Context{
		RegistryModuleEnable: registryModuleEnable,
		Mode:                 b.registry.Spec.Mode,
		ImagesBase:           imagesBase,
		Hosts: map[string]bashible.ContextHosts{
			mirrorHost: {Mirrors: ctxMirrors}},
	}

	cfg := bashible.Config{
		Mode:       b.registry.Spec.Mode,
		ImagesBase: imagesBase,
		Hosts: map[string]bashible.ConfigHosts{
			mirrorHost: {Mirrors: cfgMirrors}},
	}

	// Version only from config!
	version, err := registry_pki.ComputeHash(&cfg)
	if err != nil {
		return bashible.Context{}, bashible.Config{}, fmt.Errorf("failed to compute version: %w", err)
	}
	cfg.Version = version
	ctx.Version = version

	if err := cfg.Validate(); err != nil {
		return bashible.Context{}, bashible.Config{}, err
	}

	if err := ctx.Validate(); err != nil {
		return bashible.Context{}, bashible.Config{}, err
	}

	return ctx, cfg, nil
}

func unmanagedHostMirrors(unmapaged *UnmanagedModeSpec) (string, []bashible.ContextMirrorHost, []bashible.ConfigMirrorHost) {
	host, _ := addressAndPathFromImagesRepo(unmapaged.ImagesRepo)
	username, password := unmapaged.UsernamePassword()
	scheme := strings.ToLower(string(unmapaged.Scheme))
	return host,
		[]bashible.ContextMirrorHost{{
			Host:   host,
			CA:     unmapaged.CA,
			Scheme: scheme,
			Auth: bashible.ContextAuth{
				Username: username,
				Password: password,
			},
		}},
		[]bashible.ConfigMirrorHost{{
			Host:   host,
			CA:     unmapaged.CA,
			Scheme: scheme,
			Auth: bashible.ConfigAuth{
				Username: username,
				Password: password,
			},
		}}
}

func directHostMirrors(direct *DirectModeSpec) (string, []bashible.ContextMirrorHost, []bashible.ConfigMirrorHost) {
	host, path := addressAndPathFromImagesRepo(direct.ImagesRepo)
	username, password := direct.UsernamePassword()
	scheme := strings.ToLower(string(direct.Scheme))
	rewriteFrom := registry_const.PathRegexp
	rewriteTo := strings.TrimLeft(path, "/")
	return registry_const.Host,
		[]bashible.ContextMirrorHost{{
			Host:   host,
			CA:     direct.CA,
			Scheme: scheme,
			Auth: bashible.ContextAuth{
				Username: username,
				Password: password,
			},
			Rewrites: []bashible.ContextRewrite{{
				From: rewriteFrom,
				To:   rewriteTo,
			}},
		}},
		[]bashible.ConfigMirrorHost{{
			Host:   host,
			CA:     direct.CA,
			Scheme: scheme,
			Auth: bashible.ConfigAuth{
				Username: username,
				Password: password,
			},
			Rewrites: []bashible.ConfigRewrite{{
				From: rewriteFrom,
				To:   rewriteTo,
			}},
		}}
}
