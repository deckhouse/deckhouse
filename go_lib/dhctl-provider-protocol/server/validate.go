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

package server

import (
	"context"

	"google.golang.org/grpc"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/v1/validate"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/errs"
)

// Validator is what the validator implements: the check itself, in plain Go. An
// Output says what is wrong with the configuration; an error means the check could
// not be made and reaches the caller as Internal.
type Validator interface {
	Validate(ctx context.Context, input validatev1.Input) (validatev1.Output, error)
}

func NewValidateService(validator Validator) Service {
	return &validateService{validator: validator}
}

type validateService struct {
	validatev1.UnimplementedValidateServiceServer
	validator Validator
}

func (s *validateService) Register(registrar grpc.ServiceRegistrar) {
	validatev1.RegisterValidateServiceServer(registrar, s)
}

func (s *validateService) Validate(ctx context.Context, req *validatev1.ValidateRequest) (*validatev1.ValidateResponse, error) {
	input, err := validatev1.InputFromRequest(req)
	if err != nil {
		return nil, errs.StatusInvalidRequest(err)
	}

	if err := input.Validate(); err != nil {
		return nil, errs.ToStatus(err)
	}

	output, err := s.validator.Validate(ctx, input)
	if err != nil {
		return nil, errs.ToStatus(err)
	}

	return output.ToResponse(), nil
}
