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

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/external"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdata"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
)

type DhctlOperation = string

const (
	DhctlOperationBootstrap DhctlOperation = proto.OperationBootstrap
	DhctlOperationConverge  DhctlOperation = proto.OperationConverge
	DhctlOperationDestroy   DhctlOperation = proto.OperationDestroy
)

type PreparatorProviderParams struct {
	logger log.Logger
}

func NewPreparatorProviderParams(logger log.Logger) PreparatorProviderParams {
	return PreparatorProviderParams{logger: logger}
}

func NewPreparatorProviderParamsWithoutLogger() PreparatorProviderParams {
	return PreparatorProviderParams{logger: log.NewSilentLogger()}
}

func MetaConfigPreparatorProvider(params PreparatorProviderParams) config.MetaConfigPreparatorProvider {
	logger := params.logger
	if govalue.IsNil(logger) {
		logger = log.NewSilentLogger()
	}
	return func(provider, downloadRootDir string) config.MetaConfigPreparator {
		return selectPreparator(provider, downloadRootDir, logger)
	}
}

func selectPreparator(provider, downloadRootDir string, logger log.Logger) config.MetaConfigPreparator {
	switch provider {
	case "":
		// static cluster
		return config.DummyPreparatorProvider()("", "")
	case yandex.ProviderName:
		// Top-level dhctl path (bootstrap/converge/check): validate cluster
		// prefix. The hook-side caller passes false.
		return yandex.NewMetaConfigPreparator(true, logger)
	case vcd.ProviderName:
		return vcd.NewMetaConfigPreparator(logger)
	default:
		if binaryPath := findExternalPreparatorBinary(downloadRootDir, provider); binaryPath != "" {
			return external.NewBinaryPreparator(binaryPath)
		}
		searched := ""
		if downloadRootDir != "" {
			searched = providerdata.ValidatorPath(downloadRootDir, provider)
		}
		logger.LogErrorF("external validator for provider %q not found at %q\n", provider, searched)
		return &missingExternalValidatorPreparator{provider: provider, searchedPath: searched}
	}
}

func findExternalPreparatorBinary(pluginsDir, providerName string) string {
	if pluginsDir == "" {
		return ""
	}
	path := providerdata.ValidatorPath(pluginsDir, providerName)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		// A non-executable file is not a usable validator; treat it as missing
		// so the caller surfaces the proper missing-validator diagnostic.
		return ""
	}
	return path
}

// missingExternalValidatorPreparator hard-fails Validate and Prepare when an
// external provider's validator binary is absent from the unpacked OCI bundle.
type missingExternalValidatorPreparator struct {
	provider     string
	searchedPath string
}

func (p *missingExternalValidatorPreparator) err() error {
	if p.searchedPath == "" {
		return fmt.Errorf("external validator for provider %q not found: provider plugins directory was not configured", p.provider)
	}
	return fmt.Errorf("external validator for provider %q not found at %q: ensure the provider OCI bundle is unpacked and contains the validator binary", p.provider, p.searchedPath)
}

func (p *missingExternalValidatorPreparator) Validate(_ context.Context, _ config.ProviderInput) error {
	return p.err()
}

func (p *missingExternalValidatorPreparator) Prepare(_ context.Context, _ config.ProviderInput) (proto.PrepareResult, error) {
	return proto.PrepareResult{}, p.err()
}
