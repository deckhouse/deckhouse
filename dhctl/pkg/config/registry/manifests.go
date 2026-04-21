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

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouseregistry"
)

// newManifestBuilder creates a new ManifestBuilder instance.
func newManifestBuilder(modeModel ModeModel, legacyMode bool) *ManifestBuilder {
	return &ManifestBuilder{
		modeModel:  modeModel,
		legacyMode: legacyMode,
	}
}

// ManifestBuilder is responsible for building various configuration manifests.
type ManifestBuilder struct {
	modeModel  ModeModel
	legacyMode bool
}

// DeckhouseRegistrySecretData generates secret data for Deckhouse registry configuration.
// Parameters:
//   - pkiProvider: function that provides PKI data
//
// Returns:
//   - secretData: byte map containing secret data
//   - err: error from the operation
func (b *ManifestBuilder) DeckhouseRegistrySecretData(pkiProvider PKIProvider) (SecretData, error) {
	var inClusterData Data

	if !b.legacyMode {
		pki, err := pkiProvider()
		if err != nil {
			return nil, fmt.Errorf("get PKI: %w", err)
		}

		inClusterData, err = b.modeModel.InClusterData(pki)
		if err != nil {
			return nil, fmt.Errorf("get incluster data: %w", err)
		}

	} else {
		// For managed clusters in unmanaged mode, pkiProvider cannot get PKI
		// because the PKI secret doesn't exist in the cluster.
		// In this case, we use remote data directly.
		inClusterData = b.modeModel.RemoteData
	}

	address, path := inClusterData.AddressAndPath()

	dockerCfg, err := inClusterData.DockerCfg()
	if err != nil {
		return nil, fmt.Errorf("get docker config: %w", err)
	}

	cfg := deckhouse_registry.Config{
		Address:      address,
		Path:         path,
		Scheme:       strings.ToLower(string(inClusterData.Scheme)),
		CA:           inClusterData.CA,
		DockerConfig: dockerCfg,
	}
	return cfg.ToSecretData(), nil
}

// RegistryBashibleConfigSecretData creates bashible config secret data.
// Returns:
//   - secretExists: boolean indicating secret presence
//   - secretData: byte map containing secret data
//   - err: error from the operation
func (b *ManifestBuilder) RegistryBashibleConfigSecretData(pkiProvider PKIProvider) (bool, SecretData, error) {
	if b.legacyMode {
		return false, nil, nil
	}

	pki, err := pkiProvider()
	if err != nil {
		return true, nil, fmt.Errorf("get PKI: %w", err)
	}

	cfg, err := b.modeModel.BashibleConfig(pki)
	if err != nil {
		return true, nil, fmt.Errorf("get bashible config: %w", err)
	}

	cfgYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return true, nil, fmt.Errorf("marshal bashible config: %w", err)
	}
	return true, SecretData{"config": cfgYaml}, nil
}

// KubeadmContext builds kubeadm context struct.
// Returns:
//   - KubeadmContext: context structure
func (b *ManifestBuilder) KubeadmContext() KubeadmContext {
	address, path := helpers.SplitAddressAndPath(b.modeModel.InClusterImagesRepo)
	return KubeadmContext{
		Address: address,
		Path:    path,
	}
}

// BashibleContext builds bashible context struct.
// Parameters:
//   - pkiProvider: function that provides PKI data
//
// Returns:
//   - BashibleContext: context structure
//   - err: error from the operation
func (b *ManifestBuilder) BashibleContext(pkiProvider PKIProvider) (BashibleContext, error) {
	pki, err := pkiProvider()
	if err != nil {
		return BashibleContext{}, fmt.Errorf("get PKI: %w", err)
	}

	cfg, err := b.modeModel.BashibleConfig(pki)
	if err != nil {
		return BashibleContext{}, fmt.Errorf("get bashible config: %w", err)
	}

	ctx := cfg.ToContext()

	ctx.RegistryModuleEnable = true
	if b.legacyMode {
		ctx.RegistryModuleEnable = false
	}

	ctx.Bootstrap = &bashible.ContextBootstrap{
		Init: pki,
	}

	if b.modeModel.Mode == constant.ModeProxy {
		host, path := b.modeModel.RemoteData.AddressAndPath()
		ctx.Bootstrap.Proxy = &bashible.ContextBootstrapProxy{
			Host:     host,
			Path:     path,
			Scheme:   strings.ToLower(string(b.modeModel.RemoteData.Scheme)),
			CA:       b.modeModel.RemoteData.CA,
			Username: b.modeModel.RemoteData.Username,
			Password: b.modeModel.RemoteData.Password,
			TTL:      b.modeModel.TTL,
		}
	}

	return ctx, nil
}
