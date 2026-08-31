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

	protogen "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/gen"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/validate"
)

// MaxMessageSize is the limit the protocol mandates in each direction. gRPC's own
// 4 MiB default is too small: the payload carries every NodeGroup, InstanceClass
// and credential Secret of a cluster.
const MaxMessageSize = 8 * 1024 * 1024

type Client struct {
	service     protogen.ValidateServiceClient
	callOptions []grpc.CallOption
}

type options struct {
	callOptions []grpc.CallOption
}

type Option func(*options)

// WithCallOptions appends call options after the protocol's own.
func WithCallOptions(callOptions ...grpc.CallOption) Option {
	return func(o *options) {
		o.callOptions = append(o.callOptions, callOptions...)
	}
}

// NewClient does not dial: the caller keeps control of the connection's lifetime
// and of how readiness is awaited. The message-size limits are part of the
// protocol, so they are applied here rather than left for a caller to remember.
func NewClient(conn grpc.ClientConnInterface, opts ...Option) Client {
	applied := options{
		callOptions: []grpc.CallOption{
			grpc.MaxCallRecvMsgSize(MaxMessageSize),
			grpc.MaxCallSendMsgSize(MaxMessageSize),
		},
	}

	for _, opt := range opts {
		opt(&applied)
	}

	return Client{
		service:     protogen.NewValidateServiceClient(conn),
		callOptions: applied.callOptions,
	}
}

func (c Client) Validate(ctx context.Context, input validate.Input) (validate.Output, error) {
	req, err := input.ToRequest()
	if err != nil {
		return validate.Output{}, err
	}

	resp, err := c.service.Validate(ctx, req, c.callOptions...)
	if err != nil {
		return validate.Output{}, err
	}

	return validate.OutputFromResponse(resp), nil
}
