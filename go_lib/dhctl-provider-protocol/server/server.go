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
	"fmt"
	"net"
	"runtime/debug"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// MaxMessageSize is the limit the protocol mandates in each direction.
	// gRPC's own 4 MiB default is too small for a payload carrying every
	// NodeGroup, InstanceClass and credential Secret of a cluster.
	MaxMessageSize = 8 * 1024 * 1024
	DefaultNetwork = "unix"
)

type Config struct {
	Network     string
	Address     string
	GRPCOptions []grpc.ServerOption
}

func NewConfig() Config {
	return Config{
		Network: DefaultNetwork,
		GRPCOptions: []grpc.ServerOption{
			grpc.MaxRecvMsgSize(MaxMessageSize),
			grpc.MaxSendMsgSize(MaxMessageSize),
		},
	}
}

func (c Config) Validate() error {
	if c.Network == "" {
		return fmt.Errorf("network is required")
	}

	if c.Address == "" {
		return fmt.Errorf("address is required")
	}

	return nil
}

func (c Config) Merge(other Config) Config {
	if other.Network != "" {
		c.Network = other.Network
	}

	if other.Address != "" {
		c.Address = other.Address
	}

	if len(other.GRPCOptions) > 0 {
		c.GRPCOptions = other.GRPCOptions
	}
	return c
}

// Service registers itself on a gRPC server. The generated RegisterXxxServer
// functions in api/gen take a grpc.ServiceRegistrar, so an action's service is a
// thin wrapper over one of them — see NewValidateService.
//
// A caller may also pass a service of its own (health, reflection, its own protobuf
// API): Start registers whatever it is given and knows nothing about the actions
// themselves.
type Service interface {
	Register(registrar grpc.ServiceRegistrar)
}

// Start listens, serves in the background and returns the function that stops it.
// The socket exists by the time Start returns, so a caller may connect right away.
//
// Both the returned function and cancelling ctx stop the server gracefully —
// in-flight calls finish. stop is idempotent and reports what Serve returned.
//
// An action whose service is not passed answers Unimplemented, which is how a
// caller learns it is missing.
func Start(ctx context.Context, config Config, services ...Service) (func() error, error) {
	config = NewConfig().Merge(config)
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	options := append(
		[]grpc.ServerOption{grpc.ChainUnaryInterceptor(recoverPanic)},
		config.GRPCOptions...,
	)
	grpcServer := grpc.NewServer(options...)

	for _, service := range services {
		service.Register(grpcServer)
	}

	listener, err := net.Listen(config.Network, config.Address)
	if err != nil {
		return nil, fmt.Errorf("listen on %s %s: %w", config.Network, config.Address, err)
	}

	served := make(chan error, 1)
	go func() {
		served <- grpcServer.Serve(listener)
	}()

	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			grpcServer.GracefulStop()
		case <-stopped:
		}
	}()

	var (
		once     sync.Once
		serveErr error
	)

	stop := func() error {
		once.Do(func() {
			close(stopped)
			grpcServer.GracefulStop()
			serveErr = <-served
		})

		return serveErr
	}

	return stop, nil
}

//nolint:nonamedreturns // the deferred recover has to replace the returned error
func recoverPanic(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = status.Errorf(codes.Internal, "panic in %s: %v\n%s", info.FullMethod, recovered, debug.Stack())
		}
	}()
	return handler(ctx, req)
}
