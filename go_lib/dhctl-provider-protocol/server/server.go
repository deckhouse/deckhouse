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
	"errors"
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
	DefaultNetwork = "tcp"
)

// Service registers itself on a gRPC server. Each wire version of an action
// provides one — see validate/v1.NewService — and a caller may pass a service of
// its own (health, reflection, its own protobuf API): the transport registers
// whatever it is given and knows nothing about the actions themselves.
type Service interface {
	Register(registrar grpc.ServiceRegistrar)
}

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

// Start starts a gRPC server and returns a Server instance.
func Start(config Config, services ...Service) (*Server, error) {
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

	s := &Server{
		srv:       grpcServer,
		serveDone: make(chan error, 1),
	}

	go func() {
		s.serveDone <- grpcServer.Serve(listener)
	}()
	return s, nil
}

type Server struct {
	srv       *grpc.Server
	serveDone chan error
	stopOnce  sync.Once
	stopErr   error
}

func (s *Server) Stop() error {
	if s == nil || s.srv == nil {
		return nil
	}

	s.stopOnce.Do(func() {
		s.srv.GracefulStop()
		err := <-s.serveDone
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			s.stopErr = err
		}
	})
	return s.stopErr
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
