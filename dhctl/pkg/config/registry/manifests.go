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
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouse-registry"
	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

func NewManifestBuilder(modeModel ModeModel, moduleEnable bool) *ManifestBuilder {
	return &ManifestBuilder{
		modeModel:     modeModel,
		moduleEnabled: moduleEnable,
	}
}

type ManifestBuilder struct {
	modeModel     ModeModel
	moduleEnabled bool
}

// =======================
// Secrets
// =======================
func (b *ManifestBuilder) DeckhouseRegistrySecretData(getPKI func() (PKI, error)) (map[string][]byte, error) {
	inClusterData, err := b.modeModel.InClusterData(getPKI)
	if err != nil {
		return nil, err
	}

	address, path := inClusterData.AddressAndPath()
	dockerCfg, err := inClusterData.DockerCfg()
	if err != nil {
		return nil, err
	}
	regCfg := deckhouse_registry.Config{
		Address:      address,
		Path:         path,
		Scheme:       strings.ToLower(string(inClusterData.Scheme)),
		CA:           inClusterData.CA,
		DockerConfig: dockerCfg,
	}
	return regCfg.ToMap(), nil
}

func (b *ManifestBuilder) RegistryBashibleConfigSecretData() (bool, map[string][]byte, error) {
	if !b.moduleEnabled {
		return false, nil, nil
	}

	_, cfg, err := b.bashibleContextAndConfig()
	if err != nil {
		return true, nil, err
	}

	cfgYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return true, nil, fmt.Errorf("marshal bashible config: %w", err)
	}
	return true, map[string][]byte{"config": cfgYaml}, nil
}

// =======================
// Context
// =======================
func (b *ManifestBuilder) KubeadmTplCtx() map[string]interface{} {
	address, path := helpers.SplitAddressAndPath(b.modeModel.InClusterImagesRepo)
	return map[string]interface{}{
		"address": address,
		"path":    path,
	}
}

func (b *ManifestBuilder) BashibleTplCtx(getPKI func() (PKI, error)) (map[string]interface{}, error) {
	bashibleCtx, _, err := b.bashibleContextAndConfig()
	if err != nil {
		return nil, err
	}

	mapCtx, err := bashibleCtx.ToMap()
	if err != nil {
		return nil, err
	}

	initCfg, err := getPKI()
	if err != nil {
		return nil, fmt.Errorf("get PKI: %w", err)
	}

	mapInitCfg, err := initCfg.ToMap()
	if err != nil {
		return nil, err
	}

	mapCtx["init"] = mapInitCfg
	return mapCtx, nil
}

func (b *ManifestBuilder) bashibleContextAndConfig() (bashible.Context, bashible.Config, error) {
	ctxMirrors, cfgMirrors := b.modeModel.BashibleMirrors()

	bashibleCtx := bashible.Context{
		Mode:                 b.modeModel.Mode,
		ImagesBase:           b.modeModel.InClusterImagesRepo,
		RegistryModuleEnable: b.moduleEnabled,
		Hosts:                ctxMirrors,
	}

	bashibleCfg := bashible.Config{
		Mode:       b.modeModel.Mode,
		ImagesBase: b.modeModel.InClusterImagesRepo,
		Hosts:      cfgMirrors,
	}

	version, err := pki.ComputeHash(&bashibleCfg)
	if err != nil {
		return bashible.Context{}, bashible.Config{}, fmt.Errorf("compute version: %w", err)
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
