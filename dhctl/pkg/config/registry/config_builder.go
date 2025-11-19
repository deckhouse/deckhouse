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
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouse-registry"
	registry_init "github.com/deckhouse/deckhouse/go_lib/registry/models/init"
	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

type PKIProvider interface {
	Get() (PKI, error)
}

type ConfigBuilder struct {
	cfg *Config
}

type ConfigBuilderWithPKI struct {
	*ConfigBuilder
	pkiProvider PKIProvider
}

func (cb *ConfigBuilder) DeckhouseSettings() (bool, map[string]interface{}, error) {
	isExist := cb.cfg.isModuleEnabled()
	if !isExist {
		return false, nil, nil
	}

	data, err := json.Marshal(cb.cfg.ModuleConfig)
	if err != nil {
		return isExist, nil, fmt.Errorf("failed to marshal deckhouse registry settings: %w", err)
	}

	var ret map[string]interface{}
	if err := json.Unmarshal(data, &ret); err != nil {
		return isExist, nil, fmt.Errorf("failed to unmarshal deckhouse registry settings: %w", err)
	}
	return isExist, ret, nil
}

func (cb *ConfigBuilder) KubeadmTplCtx() map[string]interface{} {
	address, path := addressAndPathFromImagesRepo(cb.InclusterImagesRepo())
	return map[string]interface{}{
		"address": address,
		"path":    path,
	}
}

func (cb *ConfigBuilder) InclusterImagesRepo() string {
	if cb.cfg.ModuleConfig.Unmanaged != nil {
		return cb.cfg.ModuleConfig.Unmanaged.ImagesRepo
	}
	return registry_const.HostWithPath
}

func (cb *ConfigBuilder) WithPKI(pkiProvider PKIProvider) *ConfigBuilderWithPKI {
	return &ConfigBuilderWithPKI{ConfigBuilder: cb, pkiProvider: pkiProvider}
}

func (cb *ConfigBuilder) UpstreamData() (Data, error) {
	switch cfg := cb.cfg.ModuleConfig; {
	case cfg.Unmanaged != nil:
		username, password := cfg.Unmanaged.UsernamePassword()
		return Data{
			ImagesRepo: cfg.Unmanaged.ImagesRepo,
			Scheme:     cfg.Unmanaged.Scheme,
			Username:   username,
			Password:   password,
			CA:         cfg.Unmanaged.CA,
		}, nil
	case cfg.Direct != nil:
		username, password := cfg.Direct.UsernamePassword()
		return Data{
			ImagesRepo: cfg.Direct.ImagesRepo,
			Scheme:     cfg.Direct.Scheme,
			Username:   username,
			Password:   password,
			CA:         cfg.Direct.CA,
		}, nil
	default:
		return Data{}, ErrUnknownMode
	}
}

func (cb *ConfigBuilderWithPKI) InclusterData() (Data, error) {
	switch cfg := cb.cfg.ModuleConfig; {
	case cfg.Unmanaged != nil:
		username, password := cfg.Unmanaged.UsernamePassword()
		return Data{
			ImagesRepo: cfg.Unmanaged.ImagesRepo,
			Scheme:     cfg.Unmanaged.Scheme,
			Username:   username,
			Password:   password,
			CA:         cfg.Unmanaged.CA,
		}, nil
	case cfg.Direct != nil:
		username, password := cfg.Direct.UsernamePassword()
		pki, err := cb.pkiProvider.Get()
		if err != nil {
			return Data{}, err
		}
		return Data{
			ImagesRepo: registry_const.HostWithPath,
			Scheme:     SchemeHTTPS,
			CA:         pki.CA.Cert,
			Username:   username,
			Password:   password,
		}, nil
	default:
		return Data{}, ErrUnknownMode
	}
}

func (cb *ConfigBuilderWithPKI) DeckhouseRegistrySecretData() (map[string][]byte, error) {
	data, err := cb.InclusterData()
	if err != nil {
		return nil, err
	}

	address, path := addressAndPathFromImagesRepo(data.ImagesRepo)

	dockerCfgDecoded, err := data.DockerCfg()
	if err != nil {
		return nil, err
	}

	ret := deckhouse_registry.Config{
		Address:      address,
		Path:         path,
		Scheme:       strings.ToLower(string(data.Scheme)),
		CA:           data.CA,
		DockerConfig: dockerCfgDecoded,
	}
	return ret.ToMap(), nil
}

func (cb *ConfigBuilderWithPKI) RegistryBashibleConfigSecretData() (bool, map[string][]byte, error) {
	isExist := cb.cfg.isModuleEnabled()
	if !isExist {
		return false, nil, nil
	}

	var cfg bashible.Config
	_, cfg, err := cb.bashibleContextAndConfig()
	if err != nil {
		return isExist, nil, err
	}

	cfgYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return isExist, nil, err
	}

	data := map[string][]byte{"config": cfgYaml}
	return isExist, data, nil
}

func (cb *ConfigBuilderWithPKI) BashibleTplCtx() (map[string]interface{}, error) {
	var ctx bashible.Context
	ctx, _, err := cb.bashibleContextAndConfig()
	if err != nil {
		return nil, err
	}

	mapCtx, err := ctx.ToMap()
	if err != nil {
		return nil, err
	}

	initCfg, err := cb.initConfig()
	if err != nil {
		return nil, err
	}

	mapInitCfg, err := initCfg.ToMap()
	if err != nil {
		return nil, err
	}

	mapCtx["init"] = mapInitCfg
	return mapCtx, nil
}

func (cb *ConfigBuilderWithPKI) initConfig() (registry_init.Config, error) {
	pki, err := cb.pkiProvider.Get()
	if err != nil {
		return registry_init.Config{}, err
	}
	return registry_init.Config(pki), nil
}

func (cb *ConfigBuilderWithPKI) bashibleContextAndConfig() (bashible.Context, bashible.Config, error) {
	var (
		imagesBase string
		ctxHosts   []bashible.ContextMirrorHost
		cfgHosts   []bashible.ConfigMirrorHost
		mirrorHost string
	)

	switch {
	case cb.cfg.ModuleConfig.Unmanaged != nil:
		unmanaged := cb.cfg.ModuleConfig.Unmanaged
		imagesBase = unmanaged.ImagesRepo
		mirrorHost, ctxHosts, cfgHosts = extractUnmanagedMirrors(unmanaged)

	case cb.cfg.ModuleConfig.Direct != nil:
		direct := cb.cfg.ModuleConfig.Direct
		imagesBase = registry_const.HostWithPath
		mirrorHost, ctxHosts, cfgHosts = extractDirectMirrors(direct)

	default:
		return bashible.Context{}, bashible.Config{}, ErrUnknownMode
	}

	ctx := bashible.Context{
		Mode:                 cb.cfg.ModuleConfig.Mode,
		ImagesBase:           imagesBase,
		RegistryModuleEnable: cb.cfg.isModuleEnabled(),
		Hosts:                map[string]bashible.ContextHosts{mirrorHost: {Mirrors: ctxHosts}},
	}

	cfg := bashible.Config{
		Mode:       cb.cfg.ModuleConfig.Mode,
		ImagesBase: imagesBase,
		Hosts:      map[string]bashible.ConfigHosts{mirrorHost: {Mirrors: cfgHosts}},
	}

	// Only from config!
	version, err := pki.ComputeHash(&cfg)
	if err != nil {
		return bashible.Context{}, bashible.Config{}, fmt.Errorf("failed to compute version: %w", err)
	}
	cfg.Version, ctx.Version = version, version

	if err := cfg.Validate(); err != nil {
		return bashible.Context{}, bashible.Config{}, err
	}
	if err := ctx.Validate(); err != nil {
		return bashible.Context{}, bashible.Config{}, err
	}

	return ctx, cfg, nil
}

func extractUnmanagedMirrors(cfg *UnmanagedModeConfig) (string, []bashible.ContextMirrorHost, []bashible.ConfigMirrorHost) {
	host, _ := addressAndPathFromImagesRepo(cfg.ImagesRepo)
	username, password := cfg.UsernamePassword()
	scheme := strings.ToLower(string(cfg.Scheme))
	ca := cfg.CA

	ctx := []bashible.ContextMirrorHost{{
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

	return host, ctx, cfgMirrors
}

func extractDirectMirrors(cfg *DirectModeConfig) (string, []bashible.ContextMirrorHost, []bashible.ConfigMirrorHost) {
	host, path := addressAndPathFromImagesRepo(cfg.ImagesRepo)
	username, password := cfg.UsernamePassword()
	scheme := strings.ToLower(string(cfg.Scheme))
	ca := cfg.CA

	from := registry_const.PathRegexp
	to := strings.TrimLeft(path, "/")

	ctx := []bashible.ContextMirrorHost{{
		Host:     host,
		Scheme:   scheme,
		CA:       ca,
		Auth:     bashible.ContextAuth{Username: username, Password: password},
		Rewrites: []bashible.ContextRewrite{{From: from, To: to}},
	}}

	cfgMirrors := []bashible.ConfigMirrorHost{{
		Host:     host,
		Scheme:   scheme,
		CA:       ca,
		Auth:     bashible.ConfigAuth{Username: username, Password: password},
		Rewrites: []bashible.ConfigRewrite{{From: from, To: to}},
	}}

	return registry_const.Host, ctx, cfgMirrors
}
