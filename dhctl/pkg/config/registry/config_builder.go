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
	"context"
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
	Get(ctx context.Context) (PKI, error)
}

type ConfigBuilder struct {
	cfg *Config
}

type ConfigBuilderWithPKI struct {
	*ConfigBuilder
	pkiProvider PKIProvider
}

func (cb *ConfigBuilder) DeckhouseSettings() (bool, map[string]interface{}, error) {
	if !cb.cfg.isModuleEnabled() {
		return false, nil, nil
	}

	deckhouseSettings, err := cb.cfg.toDeckhouseSettings()
	if err != nil {
		return true, nil, err
	}

	data, err := json.Marshal(deckhouseSettings)
	if err != nil {
		return true, nil, fmt.Errorf("failed to marshal deckhouse registry settings: %w", err)
	}

	var ret map[string]interface{}
	if err := json.Unmarshal(data, &ret); err != nil {
		return true, nil, fmt.Errorf("failed to unmarshal deckhouse registry settings: %w", err)
	}

	return true, ret, nil
}

func (cb *ConfigBuilder) InclusterImagesRepo() string {
	if cb.cfg.Mode == registry_const.ModeUnmanaged {
		return cb.cfg.ImagesRepo
	}
	return registry_const.HostWithPath
}

func (cb *ConfigBuilder) KubeadmTplCtx() map[string]interface{} {
	address, path := addressAndPathFromImagesRepo(cb.InclusterImagesRepo())
	return map[string]interface{}{
		"address": address,
		"path":    path,
	}
}

func (cb *ConfigBuilder) UpstreamData() (Data, error) {
	switch cb.cfg.Mode {
	case registry_const.ModeDirect, registry_const.ModeUnmanaged:
		return Data{
			ImagesRepo: cb.cfg.ImagesRepo,
			Scheme:     cb.cfg.Scheme,
			Username:   cb.cfg.Username,
			Password:   cb.cfg.Password,
			CA:         cb.cfg.CA,
		}, nil

	default:
		return Data{}, ErrUnknownMode
	}
}

func (cb *ConfigBuilder) WithPKI(pkiProvider PKIProvider) *ConfigBuilderWithPKI {
	return &ConfigBuilderWithPKI{
		ConfigBuilder: cb,
		pkiProvider:   pkiProvider,
	}
}

// =======================
// ConfigBuilderWithPKI
// =======================

func (cb *ConfigBuilderWithPKI) InclusterData(ctx context.Context) (Data, error) {
	switch cb.cfg.Mode {
	case registry_const.ModeUnmanaged:
		return Data{
			ImagesRepo: cb.cfg.ImagesRepo,
			Scheme:     cb.cfg.Scheme,
			Username:   cb.cfg.Username,
			Password:   cb.cfg.Password,
			CA:         cb.cfg.CA,
		}, nil

	case registry_const.ModeDirect:
		pki, err := cb.pkiProvider.Get(ctx)
		if err != nil {
			return Data{}, fmt.Errorf("failed to get PKI: %w", err)
		}
		return Data{
			ImagesRepo: registry_const.HostWithPath,
			Scheme:     SchemeHTTPS,
			Username:   cb.cfg.Username,
			Password:   cb.cfg.Password,
			CA:         pki.CA.Cert,
		}, nil

	default:
		return Data{}, ErrUnknownMode
	}
}

func (cb *ConfigBuilderWithPKI) DeckhouseRegistrySecretData(ctx context.Context) (map[string][]byte, error) {
	data, err := cb.InclusterData(ctx)
	if err != nil {
		return nil, err
	}

	address, path := addressAndPathFromImagesRepo(data.ImagesRepo)
	dockerCfg, err := data.DockerCfg()
	if err != nil {
		return nil, err
	}

	regCfg := deckhouse_registry.Config{
		Address:      address,
		Path:         path,
		Scheme:       strings.ToLower(string(data.Scheme)),
		CA:           data.CA,
		DockerConfig: dockerCfg,
	}

	return regCfg.ToMap(), nil
}

func (cb *ConfigBuilderWithPKI) RegistryBashibleConfigSecretData(ctx context.Context) (bool, map[string][]byte, error) {
	if !cb.cfg.isModuleEnabled() {
		return false, nil, nil
	}

	_, cfg, err := cb.bashibleContextAndConfig(ctx)
	if err != nil {
		return true, nil, err
	}

	cfgYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return true, nil, fmt.Errorf("failed to marshal bashible config: %w", err)
	}

	return true, map[string][]byte{"config": cfgYaml}, nil
}

func (cb *ConfigBuilderWithPKI) BashibleTplCtx(ctx context.Context) (map[string]interface{}, error) {
	bashibleCtx, _, err := cb.bashibleContextAndConfig(ctx)
	if err != nil {
		return nil, err
	}

	mapCtx, err := bashibleCtx.ToMap()
	if err != nil {
		return nil, err
	}

	initCfg, err := cb.initConfig(ctx)
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

func (cb *ConfigBuilderWithPKI) initConfig(ctx context.Context) (registry_init.Config, error) {
	pki, err := cb.pkiProvider.Get(ctx)
	if err != nil {
		return registry_init.Config{}, fmt.Errorf("failed to get PKI: %w", err)
	}
	return registry_init.Config(pki), nil
}

func (cb *ConfigBuilderWithPKI) bashibleContextAndConfig(_ context.Context) (bashible.Context, bashible.Config, error) {
	imagesBase := cb.InclusterImagesRepo()

	mirrorHost, ctxMirrors, cfgMirrors, err := cb.mirrors()
	if err != nil {
		return bashible.Context{}, bashible.Config{}, err
	}

	bashibleCtx := bashible.Context{
		Mode:                 cb.cfg.Mode,
		ImagesBase:           imagesBase,
		RegistryModuleEnable: cb.cfg.isModuleEnabled(),
		Hosts: map[string]bashible.ContextHosts{
			mirrorHost: {Mirrors: ctxMirrors},
		},
	}

	bashibleCfg := bashible.Config{
		Mode:       cb.cfg.Mode,
		ImagesBase: imagesBase,
		Hosts: map[string]bashible.ConfigHosts{
			mirrorHost: {Mirrors: cfgMirrors},
		},
	}

	version, err := pki.ComputeHash(&bashibleCfg)
	if err != nil {
		return bashible.Context{}, bashible.Config{}, fmt.Errorf("failed to compute version: %w", err)
	}

	bashibleCfg.Version = version
	bashibleCtx.Version = version

	if err := bashibleCfg.Validate(); err != nil {
		return bashible.Context{}, bashible.Config{}, err
	}
	if err := bashibleCtx.Validate(); err != nil {
		return bashible.Context{}, bashible.Config{}, err
	}
	return bashibleCtx, bashibleCfg, nil
}

func (cb *ConfigBuilderWithPKI) mirrors() (string, []bashible.ContextMirrorHost, []bashible.ConfigMirrorHost, error) {
	switch cb.cfg.Mode {
	case registry_const.ModeUnmanaged:
		host, ctxHosts, cfgHosts := unmanagedMirrors(cb.cfg)
		return host, ctxHosts, cfgHosts, nil

	case registry_const.ModeDirect:
		host, ctxHosts, cfgHosts := directMirrors(cb.cfg)
		return host, ctxHosts, cfgHosts, nil

	default:
		return "", nil, nil, ErrUnknownMode
	}
}

func unmanagedMirrors(cfg *Config) (string, []bashible.ContextMirrorHost, []bashible.ConfigMirrorHost) {
	host, _ := addressAndPathFromImagesRepo(cfg.ImagesRepo)

	username, password := cfg.Username, cfg.Password
	scheme := strings.ToLower(string(cfg.Scheme))
	ca := cfg.CA

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

	return host, ctxMirrors, cfgMirrors
}

func directMirrors(cfg *Config) (string, []bashible.ContextMirrorHost, []bashible.ConfigMirrorHost) {
	host, path := addressAndPathFromImagesRepo(cfg.ImagesRepo)
	username, password := cfg.Username, cfg.Password
	scheme := strings.ToLower(string(cfg.Scheme))
	ca := cfg.CA

	from := registry_const.PathRegexp
	to := strings.TrimLeft(path, "/")

	ctxMirrors := []bashible.ContextMirrorHost{{
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

	return registry_const.Host, ctxMirrors, cfgMirrors
}
