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

package main

import (
	"context"
	"fmt"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalprotocol "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/protocol"
	dhctlproto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
)

func validate(_ context.Context, input dhctlproto.PrepareInput) error {
	cpVars, err := dhctlproto.ParseResourcesYAML(input.ResourcesYAML)
	if err != nil {
		return fmt.Errorf("parse resources: %w", err)
	}

	stateBuilder := cpvalprotocol.NewStateBuilder(
		cpvalprotocol.StateBuilderConfig{
			ModuleName:                   dvpval.ModuleName,
			NamespaceName:                dvpval.Namespace,
			InstanceClassKind:            dvpval.InstanceClassKind,
			MigrationRules:               &dvpval.MigrationRules,
		},
	)

	state, err := stateBuilder.Build(input, cpVars)
	if err != nil {
		return fmt.Errorf("build validation state: %w", err)
	}

	if cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		return nil
	}

	result := cpval.Result{}

	if input.Operation == dhctlproto.OperationBootstrap || input.Operation == dhctlproto.OperationConverge {
		result.Merge(dvpval.ValidatePreflight(state))
	}

	result.Merge(dvpval.ValidateInvariants(state))

	return result.ErrorOrNil()
}

func prepare(_ context.Context, input dhctlproto.PrepareInput) (*dhctlproto.PrepareResult, error) {
	cpVars, err := dhctlproto.ParseResourcesYAML(input.ResourcesYAML)
	if err != nil {
		return nil, fmt.Errorf("parse resources: %w", err)
	}

	cpVars.Settings = input.ModuleConfig

	return &dhctlproto.PrepareResult{
		Vars:                  cpVars,
		ProviderClusterConfig: input.ProviderClusterConfig,
	}, nil
}
