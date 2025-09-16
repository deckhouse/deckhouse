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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/validation"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
)

func MetaConfigPreparatorProvider() config.MetaConfigPreparatorProvider {
	return func(provider string) config.MetaConfigPreparator {
		switch provider {
		// static cluster
		case "":
			return config.DummyPreparatorProvider()("")
		case yandex.ProviderName:
			return yandex.NewMetaConfigPreparator()
		case vcd.ProviderName:
			return vcd.NewMetaConfigPreparator(vcd.MetaConfigPreparatorParams{
				PrepareMetaConfig: true,
			})
		default:
			return &defaultCloudOnlyPrefixValidatorPreparator{}
		}
	}
}

type defaultCloudOnlyPrefixValidatorPreparator struct{}

func (p *defaultCloudOnlyPrefixValidatorPreparator) Validate(metaConfig *config.MetaConfig) error {
	err := validation.DefaultPrefixValidator(metaConfig.ClusterPrefix)
	if err != nil {
		return fmt.Errorf("%v for provider %s", err, metaConfig.ProviderName)
	}

	return nil
}

func (p *defaultCloudOnlyPrefixValidatorPreparator) Prepare(*config.MetaConfig) error {
	return nil
}
