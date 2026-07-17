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

package infrastructureprovider

import (
	"context"
	"fmt"
	"os"

	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/validation"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/external"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
)

type DhctlOperation = string

const (
	DhctlOperationBootstrap DhctlOperation = proto.OperationBootstrap
	DhctlOperationConverge  DhctlOperation = proto.OperationConverge
	DhctlOperationDestroy   DhctlOperation = proto.OperationDestroy
)

// MetaConfigValidatorProvider selects the validator for a provider. Every cloud
// provider is validated, in-tree and external alike:
//   - yandex and vcd have dedicated in-tree validators (vcd additionally
//     rewrites the parsed config — see its PatchProviderClusterConfig);
//   - an external provider is validated by the validator binary from its
//     unpacked OCI bundle, which runs the provider's own pre-bootstrap checks
//     (DVP: kubeconfig, credential Secret, master NodeGroup, InstanceClass
//     references) — the in-cluster admission webhook cannot cover those,
//     because during bootstrap there is no cluster yet;
//   - the remaining in-tree providers get the default cluster-prefix check.
//
// An external provider whose bundle carries no usable validator is an error,
// not a free pass.
func MetaConfigValidatorProvider() config.MetaConfigValidatorProvider {
	return selectValidator
}

func selectValidator(ctx context.Context, provider, downloadRootDir string) config.MetaConfigValidator {
	switch provider {
	case "":
		// static cluster
		return config.DummyValidatorProvider()(ctx, "", "")
	case yandex.ProviderName:
		// Top-level dhctl paths validate the cluster prefix; the yandex hook
		// builds its own validator with that check off (the prefix of a running
		// cluster is already a fact).
		return yandex.NewMetaConfigValidator(true)
	case vcd.ProviderName:
		return vcd.NewMetaConfigValidator()
	default:
		if binaryPath := findExternalValidatorBinary(downloadRootDir, provider); binaryPath != "" {
			return external.NewBinaryValidator(binaryPath)
		}
		// In-tree providers ship their schemas in the image's candi and need no
		// external validator: keep the lightweight prefix-only check. Only truly
		// external providers (not in candi) require the downloaded binary.
		if providerBundledInCandi(provider) {
			return &inTreeDefaultValidator{}
		}
		searched := ""
		if downloadRootDir != "" {
			searched = providerdir.ValidatorPath(downloadRootDir, provider)
		}
		dhlog.FromContext(ctx).ErrorContext(ctx, fmt.Sprintf("external validator for provider %q not found at %q", provider, searched))
		return &missingExternalValidator{provider: provider, searchedPath: searched}
	}
}

func findExternalValidatorBinary(pluginsDir, providerName string) string {
	if pluginsDir == "" {
		return ""
	}
	path := providerdir.ValidatorPath(pluginsDir, providerName)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		// A non-executable file is not a usable validator; treat it as missing
		// so the caller surfaces the proper missing-validator diagnostic.
		return ""
	}
	return path
}

// providerBundledInCandi is a var so tests can stub the candi lookup.
var providerBundledInCandi = func(provider string) bool {
	return config.ProviderBundledInCandi(provider, nil)
}

// inTreeDefaultValidator is the fallback for in-tree providers without a
// dedicated validator: validate the cluster prefix.
type inTreeDefaultValidator struct{}

func (p *inTreeDefaultValidator) Validate(_ context.Context, input config.ProviderInput) error {
	if err := validation.DefaultPrefixValidator(input.ClusterPrefix); err != nil {
		return fmt.Errorf("validate cluster prefix for provider %s: %w", input.ProviderName, err)
	}
	return nil
}

// missingExternalValidator hard-fails when an external provider's validator
// binary is absent from the unpacked OCI bundle: silently skipping the
// provider's own pre-bootstrap checks would let a broken configuration reach
// the infrastructure.
type missingExternalValidator struct {
	provider     string
	searchedPath string
}

func (p *missingExternalValidator) Validate(_ context.Context, _ config.ProviderInput) error {
	if p.searchedPath == "" {
		return fmt.Errorf("external validator for provider %q not found: provider plugins directory was not configured", p.provider)
	}
	return fmt.Errorf("external validator for provider %q not found at %q: ensure the provider OCI bundle is unpacked and contains the validator binary", p.provider, p.searchedPath)
}
