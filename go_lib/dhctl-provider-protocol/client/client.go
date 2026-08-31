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

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/v1/validate"
)

const (
	MaxMessageSize = 8 * 1024 * 1024
)

type Config struct {
	GRPCOptions []grpc.CallOption
}

func NewConfig() Config {
	return Config{
		GRPCOptions: []grpc.CallOption{
			grpc.MaxCallRecvMsgSize(MaxMessageSize),
			grpc.MaxCallSendMsgSize(MaxMessageSize),
		},
	}
}

func (c Config) Validate() error {
	return nil
}

func (c Config) Merge(other Config) Config {
	if len(other.GRPCOptions) > 0 {
		c.GRPCOptions = other.GRPCOptions
	}
	return c
}

type Client struct {
	service validatev1.ValidateServiceClient
	config  Config
}

// NewClient creates a new client with the given configuration.
func NewClient(conn grpc.ClientConnInterface, config Config) Client {
	return Client{
		service: validatev1.NewValidateServiceClient(conn),
		config:  NewConfig().Merge(config),
	}
}

func (c Client) Validate(ctx context.Context, input validatev1.Input) (validatev1.Output, error) {
	req, err := input.ToRequest()
	if err != nil {
		return validatev1.Output{}, err
	}

	resp, err := c.service.Validate(ctx, req, c.config.GRPCOptions...)
	if err != nil {
		return validatev1.Output{}, err
	}

	return validatev1.OutputFromResponse(resp), nil
}
