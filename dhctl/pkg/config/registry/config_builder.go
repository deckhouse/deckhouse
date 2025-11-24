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
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouse-registry"
	"github.com/deckhouse/deckhouse/go_lib/registry/pki"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry/helpers"
)

type ConfigBuilder struct {
	registryMode  RegistryMode
	moduleEnabled bool
}

type ConfigBuilderWithPKI struct {
	ConfigBuilder
	pkiProvider PKIProvider
}

func (cb *ConfigBuilder) KubeadmTplCtx() map[string]interface{} {
	address, path := helpers.SplitAddressAndPath(cb.registryMode.InClusterImagesRepo())
	return map[string]interface{}{
		"address": address,
		"path":    path,
	}
}

func (cb *ConfigBuilder) WithPKI(pkiProvider PKIProvider) *ConfigBuilderWithPKI {
	return &ConfigBuilderWithPKI{
		ConfigBuilder: *cb,
		pkiProvider:   pkiProvider,
	}
}

// =======================
// ConfigBuilderWithPKI
// =======================
func (cb *ConfigBuilderWithPKI) DeckhouseRegistrySecretData(ctx context.Context) (map[string][]byte, error) {
	data, err := cb.registryMode.InClusterData(ctx, cb.pkiProvider)
	if err != nil {
		return nil, err
	}

	address, path := data.AddressAndPath()
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
	if !cb.moduleEnabled {
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

	initCfg, err := cb.pkiProvider.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get PKI: %w", err)
	}

	mapInitCfg, err := initCfg.ToMap()
	if err != nil {
		return nil, err
	}

	mapCtx["init"] = mapInitCfg
	return mapCtx, nil
}

func (cb *ConfigBuilderWithPKI) bashibleContextAndConfig(ctx context.Context) (bashible.Context, bashible.Config, error) {
	mirrorHost, ctxMirrors, cfgMirrors, err := cb.registryMode.BashibleMirrors(ctx, cb.pkiProvider)
	if err != nil {
		return bashible.Context{}, bashible.Config{}, err
	}

	bashibleCtx := bashible.Context{
		Mode:                 cb.registryMode.Mode(),
		ImagesBase:           cb.registryMode.InClusterImagesRepo(),
		RegistryModuleEnable: cb.moduleEnabled,
		Hosts: map[string]bashible.ContextHosts{
			mirrorHost: {Mirrors: ctxMirrors},
		},
	}

	bashibleCfg := bashible.Config{
		Mode:       cb.registryMode.Mode(),
		ImagesBase: cb.registryMode.InClusterImagesRepo(),
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
