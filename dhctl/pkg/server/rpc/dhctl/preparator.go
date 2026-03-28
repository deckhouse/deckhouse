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

package dhctl

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/dvp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type providerClusterConfigProvider interface {
	GetProviderSpecificClusterConfig() string
}

type provideMetaConfigPreparatorParams struct {
	providerConfigProvider providerClusterConfigProvider
	logger                 log.Logger
}

func provideMetaConfigPreparator(params *provideMetaConfigPreparatorParams) config.MetaConfigPreparatorProvider {
	preparatorParams := infrastructureprovider.NewPreparatorProviderParams(params.logger)
	preparatorParams.WithAdditionalDataProvider(func(provider string) (any, error) {
		if provider != dvp.ProviderName {
			return nil, nil
		}

		config := params.providerConfigProvider.GetProviderSpecificClusterConfig()
		return dvp.NewPreparatorAdditionalData(config), nil
	})

	return infrastructureprovider.MetaConfigPreparatorProvider(preparatorParams)
}

type simpleProviderClusterConfigProvider struct {
	config string
}

func newSimpleProviderClusterConfigProvider(config string) *simpleProviderClusterConfigProvider {
	return &simpleProviderClusterConfigProvider{
		config: config,
	}
}

func (p *simpleProviderClusterConfigProvider) GetProviderSpecificClusterConfig() string {
	return p.config
}
