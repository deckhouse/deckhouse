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

// MetaConfigPreparatorProvider selects the per-provider preparator. Only the
// two in-tree providers that need one carry a dedicated implementation: vcd
// (Prepare injects legacyMode for old VCD APIs) and yandex (Validate checks the
// cluster prefix, NAT-instance layout and external-IP counts). Every other
// provider — including external ones like DVP, whose configuration is validated
// by the cloud-provider admission webhook and its OpenAPI schema — needs no
// dhctl-side preparator and gets a no-op.
func MetaConfigPreparatorProvider() config.MetaConfigPreparatorProvider {
	return selectPreparator
}

func selectPreparator(ctx context.Context, provider, downloadRootDir string) config.MetaConfigPreparator {
	switch provider {
	case yandex.ProviderName:
		return yandex.NewMetaConfigPreparator(true)
	case vcd.ProviderName:
		return vcd.NewMetaConfigPreparator()
	default:
		return config.DummyPreparatorProvider()(ctx, provider, downloadRootDir)
	}
}
