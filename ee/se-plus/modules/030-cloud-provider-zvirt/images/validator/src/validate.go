/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"

	zmeta "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/meta"
	zval "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation"
	zpreflight "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation/preflight"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	cpvalprotocol "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/protocol"
	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
)

// Validator serves the validate action of the dhctl provider protocol.
type Validator struct{}

// Validate is the protocol entrypoint. The returned error means the validator itself failed;
// the configuration's own problems travel in the response as violations.
func (Validator) Validate(ctx context.Context, input validatev1.Input) (*validatev1.ValidateResponse, error) {
	ret, err := validate(ctx, input)
	if err != nil {
		return nil, err
	}

	return toResponse(ret), nil
}

// validate runs the module's preflight checks against the state dhctl hands over.
//
// Destroy is deliberately exempt: the checks describe a configuration a cluster can be built from,
// and refusing to tear down a cluster because its configuration is broken is the opposite of
// helpful — a broken configuration is often exactly why it is being destroyed.
func validate(_ context.Context, input validatev1.Input) (cpvalapi.Result, error) {
	if input.Operation == validatev1.OperationDestroy {
		return cpvalapi.Result{}, nil
	}

	stateBuilderFactory := zval.NewProtocolStateBuilderFactory(cpvalprotocol.StateBuilderConfig{
		ModuleName:    zmeta.ModuleName,
		NamespaceName: zmeta.Namespace,
	})

	state, err := stateBuilderFactory.CreateBuilder().Build(input)
	if err != nil {
		return cpvalapi.Result{}, fmt.Errorf("build validation state: %w", err)
	}

	return zpreflight.ValidatePreflight(state), nil
}

// toResponse keeps errors and warnings apart, so dhctl can report a warning without refusing to
// proceed.
func toResponse(result cpvalapi.Result) *validatev1.ValidateResponse {
	ret := &validatev1.ValidateResponse{}

	for _, violation := range result.Errors() {
		ret.Errors = append(ret.Errors, toViolationResponse(violation))
	}

	for _, violation := range result.Warnings() {
		ret.Warnings = append(ret.Warnings, toViolationResponse(violation))
	}

	return ret
}

func toViolationResponse(violation cpvalapi.Violation) *validatev1.ViolationResponse {
	ret := &validatev1.ViolationResponse{
		Path:    violation.Path,
		Code:    violation.Code,
		Message: violation.Message,
	}

	if violation.Value != nil {
		ret.Value = fmt.Sprint(violation.Value)
	}

	return ret
}
