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

func (b *ManifestBuilder) DeckhouseRegistrySecretData(pkiProvider PKIProvider) (secretData, error) {
	inClusterData, err := b.modeModel.InClusterData(pkiProvider)
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

// RegistryBashibleConfigSecretData creates bashible config secret data.
// Returns:
//   - bool: true if secret exist
//   - secretData: map bytes of secret data
//   - error
func (b *ManifestBuilder) RegistryBashibleConfigSecretData() (bool, secretData, error) {
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

func (b *ManifestBuilder) KubeadmTplCtx() contextData {
	address, path := helpers.SplitAddressAndPath(b.modeModel.InClusterImagesRepo)
	return contextData{
		"address": address,
		"path":    path,
	}
}

func (b *ManifestBuilder) BashibleContext(pkiProvider PKIProvider) (BashibleContext, error) {
	cfg, err := b.modeModel.BashibleConfig()
	if err != nil {
		return BashibleContext{}, fmt.Errorf("get bashible config: %w", err)
	}

	ctx := cfg.ToContext()

	ctx.RegistryModuleEnable = true
	if b.legacyMode {
		ctx.RegistryModuleEnable = false
	}

	init, err := pkiProvider()
	if err != nil {
		return BashibleContext{}, fmt.Errorf("get PKI: %w", err)
	}
	ctx.Init = init

	return ctx, nil
}
