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
func newManifestBuilder(modeModel ModeModel, legacyMode, bundleBootstrap bool) *ManifestBuilder {
	return &ManifestBuilder{
		modeModel:       modeModel,
		legacyMode:      legacyMode,
		bundleBootstrap: bundleBootstrap,
	}
}

// ManifestBuilder is responsible for building various configuration manifests.
type ManifestBuilder struct {
	modeModel  ModeModel
	legacyMode bool

	// bundleBootstrap reaches the node through the bashible context, where it tells the bootstrap
	// steps that this cluster's registry will be owned by the current implementation and not by the
	// one whose steps are being borrowed to fill it.
	bundleBootstrap bool
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

// ImageAddressData is what the installer publishes as the registry module's answer to "which
// address do image references name", or nothing when the module has to reach that answer itself.
//
// Published only where the first master comes up with the node agent on it. There the answer is
// already given — that master pulls through the agent from its first image, and every node after it
// is installed the same way — while the module can give it only once every node's agent reports the
// layout it applied. Left to the module, the question stays open for as long as some node is still
// joining, and in the meantime image references fall back to the upstream: on a cluster installed
// this way that re-rendered every workload onto the upstream and back while the second master was
// joining, and the roll of registry-packages-proxy it caused closed the only port the third master
// could fetch its first package from.
//
// Everywhere else — a cluster whose nodes pull straight from the upstream — the module's own
// question is the right one, and it is asked from nothing.
func (b *ManifestBuilder) ImageAddressData() (bool, map[string]string) {
	if b.legacyMode || !b.modeModel.AgentOwnsRuntime {
		return false, nil
	}
	return true, map[string]string{constant.ImageAddressConfigMapKey: constant.HostWithPath}
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
		Init:       pki,
		FromBundle: b.bundleBootstrap,
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
