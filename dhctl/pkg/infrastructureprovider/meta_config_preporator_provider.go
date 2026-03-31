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
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/dvp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/validation"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// todo it is ugly solution because we validate some filds in providers only in bootstrap
// need migration in cloud-provider-yandex in withNat layout
type DhctlPhase string

const (
	DhctlPhaseBootstrap DhctlPhase = "bootstrap"
)

type PreflightChecks struct {
	DVPValidateKubeAPI bool
}

type PreparatorAdditionalDataProvider func(provider string) (any, error)

type PreparatorProviderParams struct {
	logger                 log.Logger
	phase                  DhctlPhase
	PreflightChecks        PreflightChecks
	additionalDataProvider PreparatorAdditionalDataProvider
}

func (p *PreparatorProviderParams) WithPhase(phase DhctlPhase) {
	p.phase = phase
}

func (p *PreparatorProviderParams) WithPhaseBootstrap() {
	p.WithPhase(DhctlPhaseBootstrap)
}

func (p *PreparatorProviderParams) WithAdditionalDataProvider(provider PreparatorAdditionalDataProvider) {
	p.additionalDataProvider = provider
}

func (p *PreparatorProviderParams) AdditionalDataProvider() PreparatorAdditionalDataProvider {
	return p.additionalDataProvider
}

func (p *PreparatorProviderParams) WithPreflightChecks(checks PreflightChecks) {
	p.PreflightChecks = checks
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

	logger.LogDebugF("Additional data func: %v in provider\n", params.additionalDataProvider)

	return func(provider string) config.MetaConfigPreparator {
		switch provider {
		// static cluster
		case "":
			return config.DummyPreparatorProvider()("")
		case yandex.ProviderName:
			yandexPreparator := yandex.NewMetaConfigPreparator(true).WithLogger(logger)
			if params.phase == DhctlPhaseBootstrap {
				yandexPreparator.EnableValidateWithNATLayout()
			}
			return yandexPreparator
		case vcd.ProviderName:
			return vcd.NewMetaConfigPreparator(vcd.MetaConfigPreparatorParams{
				PrepareMetaConfig:     true,
				ValidateClusterPrefix: true,
			}, logger)
		case dvp.ProviderName:
			return providePreparatorForDVP(params, logger)
		default:
			return &defaultCloudOnlyPrefixValidatorPreparator{}
		}
	}
}

func providePreparatorForDVP(params PreparatorProviderParams, logger log.Logger) config.MetaConfigPreparator {
	prep := dvp.NewMetaConfigPreparator().WithLogger(logger)

	logger.LogDebugF("Additional data func: %v in creator\n", params.additionalDataProvider)

	additionalData, err := extractAdditionalData(dvp.PreparatorAdditionalDataFromAny, &extractAdditionalDataParams{
		providerName:     dvp.ProviderName,
		logger:           logger,
		preparatorParams: params,
	})

	if err != nil {
		panic(fmt.Errorf("Internal error during extract additional preparator data for dvp: %v", err))
	}

	// nil data handled in preparator
	prep.WithAdditionalData(additionalData)

	if params.phase == DhctlPhaseBootstrap {
		prep.EnableValidateKubeConfig(params.PreflightChecks.DVPValidateKubeAPI)
	}

	return prep
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

type extractAdditionalDataParams struct {
	preparatorParams PreparatorProviderParams
	providerName     string
	logger           log.Logger
}

func extractAdditionalData[T any](extractor func(any) (*T, error), params *extractAdditionalDataParams) (*T, error) {
	logger := params.logger
	providerName := params.providerName
	providerFunc := params.preparatorParams.additionalDataProvider

	logger.LogDebugF("Additional data func: %v in extractor\n", params.preparatorParams.additionalDataProvider)

	logSkip := func(msg string) (*T, error) {
		logger.LogDebugF("Additional data %s for '%s'. Skip\n", msg, providerName)
		return nil, nil
	}

	if govalue.IsNil(providerFunc) {
		return logSkip("provider not provided")
	}

	dataAny, err := providerFunc(providerName)
	if err != nil {
		return nil, fmt.Errorf("Additional data provider return error: %w", err)
	}

	if govalue.IsNil(dataAny) {
		return logSkip("data is not provided")
	}

	res, err := extractor(dataAny)
	if err != nil {
		return nil, fmt.Errorf("Additional data extractor return error: %w", err)
	}

	return res, nil
}
