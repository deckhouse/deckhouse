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
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
	ycval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/validation"
	ycpreflight "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/validation/preflight"
)

func validate(_ context.Context, input proto.ValidateInput) error {
	if input.Operation == proto.OperationDestroy {
		return nil
	}

	stateBuilderFactory := ycval.NewProtocolStateBuilderFactory(cpvalprotocol.StateBuilderConfig{
		ModuleName:    ycmeta.ModuleName,
		NamespaceName: ycmeta.Namespace,
	})

	state, err := stateBuilderFactory.CreateBuilder().Build(input)
	if err != nil {
		return fmt.Errorf("internal error: build validation state: %w", err)
	}

	return ycpreflight.ValidatePreflight(state, input.Operation).ErrorOrNil()
}
