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

	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
)

type DhctlOperation = string

const (
	DhctlOperationBootstrap DhctlOperation = proto.OperationBootstrap
	DhctlOperationConverge  DhctlOperation = proto.OperationConverge
	DhctlOperationDestroy   DhctlOperation = proto.OperationDestroy
)

// MetaConfigValidatorProvider selects the per-provider validator. Only two
// in-tree providers carry one: yandex (cluster prefix, NAT-instance layout,
// external-IP counts) and vcd (provider.server format, plus the legacyMode
// patch it applies through PatchProviderClusterConfig). Every other provider —
// including external ones like DVP, whose config is checked by the
// cloud-provider admission webhook and its OpenAPI schema — needs no
// dhctl-side validation and gets a no-op.
func MetaConfigValidatorProvider() config.MetaConfigValidatorProvider {
	return selectValidator
}

func selectValidator(ctx context.Context, provider string) config.MetaConfigValidator {
	switch provider {
	case yandex.ProviderName:
		// Top-level dhctl paths validate the cluster prefix; the yandex hook
		// builds its own validator with that check off (the prefix of a running
		// cluster is already a fact).
		return yandex.NewMetaConfigValidator(true)
	case vcd.ProviderName:
		return vcd.NewMetaConfigValidator()
	default:
		return config.DummyValidatorProvider()(ctx, provider)
	}
}
