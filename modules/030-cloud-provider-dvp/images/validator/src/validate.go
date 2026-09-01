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

	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	cpvalprotocol "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/protocol"
	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/meta"
	dvpvalidation "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
	dvppreflight "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation/preflight"
)

type Validator struct{}

func (Validator) Validate(ctx context.Context, input validatev1.Input) (*validatev1.ValidateResponse, error) {
	ret, err := validate(ctx, input)
	if err != nil {
		return nil, err
	}

	return toResponse(ret), nil
}

func validate(_ context.Context, input validatev1.Input) (cpvalapi.Result, error) {
	if input.Operation == validatev1.OperationDestroy {
		return cpvalapi.Result{}, nil
	}

	stateBuilderFactory := dvpvalidation.NewProtocolStateBuilderFactory(cpvalprotocol.StateBuilderConfig{
		ModuleName:    dvpmeta.ModuleName,
		NamespaceName: dvpmeta.Namespace,
	})

	state, err := stateBuilderFactory.CreateBuilder().Build(input)
	if err != nil {
		return cpvalapi.Result{}, fmt.Errorf("build validation state: %w", err)
	}

	return dvppreflight.ValidatePreflight(state), nil
}

func toResponse(result cpvalapi.Result) *validatev1.ValidateResponse {
	resp := &validatev1.ValidateResponse{}

	for _, violation := range result.Errors() {
		resp.Errors = append(resp.Errors, toViolation(violation))
	}

	for _, violation := range result.Warnings() {
		resp.Warnings = append(resp.Warnings, toViolation(violation))
	}

	return resp
}

func toViolation(violation cpvalapi.Violation) *validatev1.ViolationResponse {
	wire := &validatev1.ViolationResponse{
		Path:    violation.Path,
		Code:    violation.Code,
		Message: violation.Message,
	}

	if violation.Value != nil {
		wire.Value = fmt.Sprint(violation.Value)
	}

	return wire
}
