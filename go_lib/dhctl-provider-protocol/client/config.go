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
	"slices"

	"google.golang.org/grpc"
)

const (
	// MaxMessageSize is the limit the protocol mandates in each direction.
	// gRPC's own 4 MiB default is too small for a payload carrying every
	// NodeGroup, InstanceClass and credential Secret of a cluster.
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
		c.GRPCOptions = slices.Concat(c.GRPCOptions, other.GRPCOptions)
	}
	return c
}
