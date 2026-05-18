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
	"path/filepath"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/validation"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/external"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdata"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type DhctlOperation string

const (
	DhctlOperationBootstrap DhctlOperation = providerdata.OperationBootstrap
)

type PreparatorProviderParams struct {
	logger    log.Logger
	Operation DhctlOperation
}

func (p *PreparatorProviderParams) WithOperation(op DhctlOperation) {
	p.Operation = op
}

func (p *PreparatorProviderParams) WithOperationBootstrap() {
	p.WithOperation(DhctlOperationBootstrap)
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

func MetaConfigPreparatorProvider(params PreparatorProviderParams) config.MetaConfigPreparatorProvider {
	logger := params.logger

	if govalue.IsNil(logger) {
		logger = log.NewSilentLogger()
	}

	return func(provider, downloadRootDir string) config.MetaConfigPreparator {
		switch provider {
		// static cluster
		case "":
			return config.DummyPreparatorProvider()("", "")
		case yandex.ProviderName:
			return yandex.NewMetaConfigPreparator(true, string(params.Operation)).WithLogger(logger)
		case vcd.ProviderName:
			return vcd.NewMetaConfigPreparator(vcd.MetaConfigPreparatorParams{
				PrepareMetaConfig:     true,
				ValidateClusterPrefix: true,
			}, logger)
		default:
			if binaryPath := findExternalPreparatorBinary(downloadRootDir, provider); binaryPath != "" {
				return external.NewBinaryPreparator(binaryPath)
			}
			return &defaultCloudOnlyPrefixValidatorPreparator{}
		}
	}
}

const externalPreparatorBinaryName = "validator"

// findExternalPreparatorBinary looks for a validator binary in pluginsDir/<providerName>/.
// Returns the full path if found and is a regular file, empty string otherwise.
func findExternalPreparatorBinary(pluginsDir, providerName string) string {
	if pluginsDir == "" {
		return ""
	}
	path := filepath.Join(pluginsDir, providerName, externalPreparatorBinaryName)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return path
}

type defaultCloudOnlyPrefixValidatorPreparator struct{}

func (p *defaultCloudOnlyPrefixValidatorPreparator) Validate(_ context.Context, input config.ProviderInput) error {
	if err := validation.DefaultPrefixValidator(input.ClusterPrefix); err != nil {
		return fmt.Errorf("%v for provider %s", err, input.ProviderName)
	}
	return nil
}

func (p *defaultCloudOnlyPrefixValidatorPreparator) Prepare(_ context.Context, _ config.ProviderInput) (providerdata.PrepareResult, error) {
	return providerdata.PrepareResult{}, nil
}
