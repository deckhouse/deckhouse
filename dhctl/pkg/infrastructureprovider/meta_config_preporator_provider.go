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

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/validation"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type PreparatorProviderParams struct {
	logger log.Logger
}

func NewPreparatorProviderParams(logger log.Logger) PreparatorProviderParams {
	return PreparatorProviderParams{
		logger: logger,
	}
}

func NewPreparatorProviderParamsWithoutLogger() PreparatorProviderParams {
	return PreparatorProviderParams{
		logger: log.NewSilentLogger(),
	}
}

// looger can be nil if nil will use silent logger
func MetaConfigPreparatorProvider(params PreparatorProviderParams) config.MetaConfigPreparatorProvider {
	logger := params.logger

	if govalue.IsNil(logger) {
		logger = log.NewSilentLogger()
	}

	return func(provider string) config.MetaConfigPreparator {
		switch provider {
		// static cluster
		case "":
			return config.DummyPreparatorProvider()("")
		case yandex.ProviderName:
			return yandex.NewMetaConfigPreparator(true).WithLogger(logger)
		case vcd.ProviderName:
			return vcd.NewMetaConfigPreparator(vcd.MetaConfigPreparatorParams{
				PrepareMetaConfig:     true,
				ValidateClusterPrefix: true,
			}, logger)
		default:
			return &defaultCloudOnlyPrefixValidatorPreparator{}
		}
	}
}

type defaultCloudOnlyPrefixValidatorPreparator struct{}

func (p *defaultCloudOnlyPrefixValidatorPreparator) Validate(_ context.Context, metaConfig *config.MetaConfig) error {
	err := validation.DefaultPrefixValidator(metaConfig.ClusterPrefix)
	if err != nil {
		return fmt.Errorf("%v for provider %s", err, metaConfig.ProviderName)
	}

	return nil
}

func (p *defaultCloudOnlyPrefixValidatorPreparator) Prepare(_ context.Context, _ *config.MetaConfig) error {
	return nil
}
