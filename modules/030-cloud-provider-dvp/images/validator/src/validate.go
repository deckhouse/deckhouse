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

	cpvalprotocol "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/protocol"
	dhctlproto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/meta"
	dvppreflight "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation/preflight"
)

func validate(_ context.Context, input dhctlproto.PrepareInput) error {
	if input.Operation == dhctlproto.OperationDestroy {
		return nil
	}

	stateBuilder := cpvalprotocol.NewStateBuilder(
		cpvalprotocol.StateBuilderConfig{
			ModuleName:        dvpmeta.ModuleName,
			NamespaceName:     dvpmeta.Namespace,
			InstanceClassKind: dvpmeta.InstanceClassKind,
		},
	)

	state, err := stateBuilder.Build(input)
	if err != nil {
		return fmt.Errorf("internal error: build validation state: %w", err)
	}

	return dvppreflight.ValidatePreflight(state).ErrorOrNil()
}

func prepare(_ context.Context, input dhctlproto.PrepareInput) (*dhctlproto.PrepareResult, error) {
	return &dhctlproto.PrepareResult{
		Vars:                  input.Vars,
		ProviderClusterConfig: input.ProviderClusterConfig,
	}, nil
}
