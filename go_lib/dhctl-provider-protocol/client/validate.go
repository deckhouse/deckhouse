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

package client

import (
	"context"

	"google.golang.org/grpc"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
)

type ValidateClient struct {
	service validatev1.ValidateServiceClient
	config  Config
}

// NewValidateClient creates a new client with the given configuration.
func NewValidateClient(conn grpc.ClientConnInterface, config Config) ValidateClient {
	return ValidateClient{
		service: validatev1.NewValidateServiceClient(conn),
		config:  NewConfig().Merge(config),
	}
}

func (c ValidateClient) Validate(ctx context.Context, input validatev1.Input) (*validatev1.ValidateResponse, error) {
	req, err := input.ToRequest()
	if err != nil {
		return nil, err
	}
	return c.service.Validate(ctx, req, c.config.GRPCOptions...)
}
