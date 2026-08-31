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

	protogen "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/gen"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/errs"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/validate"
)

type Validator interface {
	Validate(ctx context.Context, input validate.Input) (validate.Output, error)
}

func NewValidateService(validator Validator) Service {
	return &validateService{validator: validator}
}

type validateService struct {
	protogen.UnimplementedValidateServiceServer
	validator Validator
}

func (s *validateService) Register(registrar grpc.ServiceRegistrar) {
	protogen.RegisterValidateServiceServer(registrar, s)
}

func (s *validateService) Validate(ctx context.Context, req *protogen.ValidateRequest) (*protogen.ValidateResponse, error) {
	input, err := validate.FromPBRequest(req)
	if err != nil {
		return nil, errs.StatusInvalidRequest(err)
	}

	if err := input.Validate(); err != nil {
		return nil, errs.MapToStatus(err)
	}

	out, err := s.validator.Validate(ctx, input)
	if err != nil {
		return nil, errs.MapToStatus(err)
	}

	return validate.ToPBResponse(out), nil
}
