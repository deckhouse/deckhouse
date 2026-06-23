// Copyright 2026 Flant JSC
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

// ManifestBuilder produces the bootstrap artifacts. Two implementations: the
// legacy four-mode modeManifestBuilder and the clean CleanModel.
type ManifestBuilder interface {
	DeckhouseRegistrySecretData(pkiProvider PKIProvider) (SecretData, error)
	RegistryBashibleConfigSecretData(pkiProvider PKIProvider) (bool, SecretData, error)
	KubeadmContext() KubeadmContext
	BashibleContext(pkiProvider PKIProvider) (BashibleContext, error)
}

var _ ManifestBuilder = (*modeManifestBuilder)(nil)

// managedMode reports whether m is a managed registry mode (Direct, Proxy, or
// Local). Unmanaged mode leaves the in-cluster registry untouched, so managed-
// only fields (RegistryModuleEnable, Bootstrap.Init, air-gap seed Hosts) must
// not be set for it.
func managedMode(m constant.ModeType) bool {
	return constant.ModuleRequired(m)
}

// newManifestBuilder creates a new modeManifestBuilder instance.
func newManifestBuilder(modeModel ModeModel, legacyMode bool) *modeManifestBuilder {
	return &modeManifestBuilder{
		modeModel:  modeModel,
		legacyMode: legacyMode,
	}
}

// modeManifestBuilder is responsible for building various configuration manifests
// using the legacy four-mode registry model.
type modeManifestBuilder struct {
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
func (b *modeManifestBuilder) DeckhouseRegistrySecretData(pkiProvider PKIProvider) (SecretData, error) {
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
func (b *modeManifestBuilder) RegistryBashibleConfigSecretData(pkiProvider PKIProvider) (bool, SecretData, error) {
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
func (b *modeManifestBuilder) KubeadmContext() KubeadmContext {
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
func (b *modeManifestBuilder) BashibleContext(pkiProvider PKIProvider) (BashibleContext, error) {
	pki, err := pkiProvider()
	if err != nil {
		return BashibleContext{}, fmt.Errorf("get PKI: %w", err)
	}

	cfg, err := b.modeModel.BashibleConfig(pki)
	if err != nil {
		return BashibleContext{}, fmt.Errorf("get bashible config: %w", err)
	}

	ctx := cfg.ToContext()

	// Managed modes (Direct, Proxy, Local) enable the registry module and supply
	// a Bootstrap.Init PKI bundle. Unmanaged mode leaves registry infrastructure
	// untouched, so these fields must not be set for it.
	if !b.legacyMode && managedMode(b.modeModel.Mode) {
		ctx.RegistryModuleEnable = true

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

		// Air-gap (Local mode) bootstrap seed:
		// Replace the FSM Proxy/Local host list from BashibleConfig with two local
		// mirrors: cache first (preferred once up), then the on-node seed process
		// (available from the start, before the cache is ready). Both use https
		// with the module CA. No path rewrite: both are rooted at system/deckhouse
		// and imagesBase already carries the path (constant.HostWithPath).
		//
		// The seed is filled once over the SSH reverse tunnel (registry-syncer
		// 127.0.0.1:5511 -> 127.0.0.1:5010), then the tunnel is never in the
		// pull path. For Direct/Proxy modes ToContext() already produced the
		// correct upstream mirror with the ^system/deckhouse rewrite; we keep it.
		//
		// After bring-up the registry-agent takes over registry.d and re-renders
		// these mirrors with the permanent in-cluster entries.
		if b.modeModel.Mode == constant.ModeLocal {
			ctx.Hosts = bashible.BootstrapSeedHostsLocal(pki.CA.Cert, pki.ROUser.Name, pki.ROUser.Password)
			ctx.ProxyEndpoints = nil
		}
	}

	return ctx, nil
}
