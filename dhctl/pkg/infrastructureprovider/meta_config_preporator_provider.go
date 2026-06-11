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
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/external"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdata"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type DhctlOperation = string

const (
	DhctlOperationBootstrap DhctlOperation = providerdata.OperationBootstrap
	DhctlOperationConverge  DhctlOperation = providerdata.OperationConverge
	DhctlOperationDestroy   DhctlOperation = providerdata.OperationDestroy
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

func (p *PreparatorProviderParams) WithOperationConverge() {
	p.WithOperation(DhctlOperationConverge)
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
	operation := params.Operation
	return func(provider, downloadRootDir string) config.MetaConfigPreparator {
		return selectPreparator(provider, downloadRootDir, logger, operation)
	}
}

func selectPreparator(provider, downloadRootDir string, logger log.Logger, operation DhctlOperation) config.MetaConfigPreparator {
	switch provider {
	case "":
		// static cluster
		return config.DummyPreparatorProvider()("", "")
	case yandex.ProviderName:
		// Top-level dhctl path (bootstrap/converge/check): validate cluster
		// prefix. The hook-side caller passes false.
		return yandex.NewMetaConfigPreparator(true, logger, operation)
	case vcd.ProviderName:
		return vcd.NewMetaConfigPreparator(logger)
	default:
		if binaryPath := findExternalPreparatorBinary(downloadRootDir, provider); binaryPath != "" {
			return external.NewBinaryPreparator(binaryPath)
		}
		// External providers (DVP and any future plugin) ship a validator
		// binary inside their OCI bundle. Falling back to a prefix-only
		// validator here would silently skip every provider-specific check
		// — registry credentials, kubeconfig, layout, NodeGroup sizing —
		// and let a broken configuration reach terraform apply. Refuse to
		// proceed with a precise diagnostic instead.
		searched := ""
		if downloadRootDir != "" {
			searched = filepath.Join(downloadRootDir, provider, externalPreparatorBinaryName)
		}
		logger.LogErrorF("external validator for provider %q not found at %q\n", provider, searched)
		return &missingExternalValidatorPreparator{provider: provider, searchedPath: searched}
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

// missingExternalValidatorPreparator is the preparator returned when an
// external provider declares itself but its validator binary is absent from
// the unpacked OCI bundle. Both Validate and Prepare hard-fail so the caller
// surfaces a clear configuration error rather than a downstream
// "terraform plan diverged" or "external API rejected the request" mystery.
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

func (p *missingExternalValidatorPreparator) Prepare(_ context.Context, _ config.ProviderInput) (providerdata.PrepareResult, error) {
	return providerdata.PrepareResult{}, p.err()
}
