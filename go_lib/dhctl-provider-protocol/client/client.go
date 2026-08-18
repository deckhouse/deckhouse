// client/client.go
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
	"fmt"

	"google.golang.org/grpc"

	protogen "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/gen"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/validate"
)

const MaxMessageSize = 8 * 1024 * 1024

// Client is the gRPC client for the validate service.
type Client struct {
	validate protogen.ValidateServiceClient
	cfg      Config
}

// Config holds client configuration.
type Config struct {
	callOptions []grpc.CallOption
}

// Option configures the client.
type Option func(*Config)

// WithCallOptions adds gRPC call options to each Validate call.
func WithCallOptions(opts ...grpc.CallOption) Option {
	return func(cfg *Config) {
		cfg.callOptions = append(cfg.callOptions, opts...)
	}
}

func DefaultCallOptions() []Option {
	return []Option{
		WithCallOptions(
			grpc.MaxCallRecvMsgSize(MaxMessageSize),
			grpc.MaxCallSendMsgSize(MaxMessageSize),
		),
	}
}

// NewClient creates a new client from a gRPC connection. It does not dial, so the
// caller keeps control of the connection's lifetime and of how readiness is awaited.
func NewClient(conn grpc.ClientConnInterface, opts ...Option) Client {
	cfg := Config{}

	for _, opt := range opts {
		opt(&cfg)
	}

	return Client{
		validate: protogen.NewValidateServiceClient(conn),
		cfg:      cfg,
	}
}

// Validate asks the plugin to check the configuration.
func (c *Client) Validate(ctx context.Context, input validate.Input) (validate.Result, error) {
	req, err := validate.ToPBRequest(input)
	if err != nil {
		return validate.Result{}, err
	}

	resp, err := c.validate.Validate(ctx, req, c.cfg.callOptions...)
	if err != nil {
		return validate.Result{}, fmt.Errorf("call plugin validate: %w", err)
	}

	return validate.FromPBResponse(resp), nil
}
