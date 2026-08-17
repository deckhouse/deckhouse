// server/server.go
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

// Package server provides gRPC server bindings for the validate action.
package server

import (
	"context"

	"google.golang.org/grpc"

	protogen "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/gen"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/errs"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/validate"
)

type Server struct {
	protogen.UnimplementedValidateServiceServer
	validator validate.Validator
}

func NewServer(validator validate.Validator) *Server {
	return &Server{
		validator: validator,
	}
}

// Register registers the validate service on the given gRPC server.
func (s *Server) Register(grpcServer *grpc.Server) {
	protogen.RegisterValidateServiceServer(grpcServer, s)
}

// Validate implements the gRPC Validate method.
func (s *Server) Validate(ctx context.Context, req *protogen.ValidateRequest) (*protogen.ValidateResponse, error) {
	input, err := validate.FromPBRequest(req)
	if err != nil {
		return nil, errs.StatusInvalidRequest(err)
	}

	result, err := s.validator.Validate(ctx, input)
	if err != nil {
		return nil, errs.MapToStatus(err)
	}

	return validate.ToPBResponse(result), nil
}
