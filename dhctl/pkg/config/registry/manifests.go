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
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouse-registry"
)

type (
	secretData  = map[string][]byte
	contextData = map[string]any
)

func newManifestBuilder(modeModel ModeModel, legacyMode bool) *ManifestBuilder {
	return &ManifestBuilder{
		modeModel:  modeModel,
		legacyMode: legacyMode,
	}
}

type ManifestBuilder struct {
	modeModel  ModeModel
	legacyMode bool
}

// =======================
// Secrets
// =======================
func (b *ManifestBuilder) DeckhouseRegistrySecretData(getPKI getPKI) (secretData, error) {
	inClusterData, err := b.modeModel.InClusterData(getPKI)
	if err != nil {
		return nil, fmt.Errorf("get incluster data: %w", err)
	}

	address, path := inClusterData.AddressAndPath()

	dockerCfg, err := inClusterData.DockerCfg()
	if err != nil {
		return nil, fmt.Errorf("get docker config: %w", err)
	}

	ret := deckhouse_registry.Config{
		Address:      address,
		Path:         path,
		Scheme:       strings.ToLower(string(inClusterData.Scheme)),
		CA:           inClusterData.CA,
		DockerConfig: dockerCfg,
	}
	return ret.ToMap(), nil
}

func (b *ManifestBuilder) RegistryBashibleConfigSecretData() (exist bool, data secretData, err error) {
	if b.legacyMode {
		return false, nil, nil
	}

	cfg, err := b.modeModel.BashibleConfig()
	if err != nil {
		return true, nil, fmt.Errorf("get bashible config: %w", err)
	}

	cfgYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return true, nil, fmt.Errorf("marshal bashible config: %w", err)
	}
	return true, secretData{"config": cfgYaml}, nil
}

// =======================
// Context
// =======================
func (b *ManifestBuilder) KubeadmTplCtx() contextData {
	address, path := helpers.SplitAddressAndPath(b.modeModel.InClusterImagesRepo)
	return contextData{
		"address": address,
		"path":    path,
	}
}

func (b *ManifestBuilder) BashibleTplCtx(getPKI getPKI) (contextData, error) {
	bashibleCfg, err := b.modeModel.BashibleConfig()
	if err != nil {
		return nil, fmt.Errorf("get bashible config: %w", err)
	}

	bashibleCtx := bashibleCfg.ToContext()

	bashibleCtx.RegistryModuleEnable = true
	if b.legacyMode {
		bashibleCtx.RegistryModuleEnable = false
	}

	ret, err := bashibleCtx.ToMap()
	if err != nil {
		return nil, fmt.Errorf("map bashible context: %w", err)
	}

	initCfg, err := getPKI()
	if err != nil {
		return nil, fmt.Errorf("get PKI: %w", err)
	}

	ret["init"] = initCfg.ToMap()
	return ret, nil
}
