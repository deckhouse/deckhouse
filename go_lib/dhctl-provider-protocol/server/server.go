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
	"log/slog"
	"net"
	"runtime/debug"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// Service registers itself on a gRPC server. Each wire version of an action
// provides one — see validate/v1.NewService — and a caller may pass a service of
// its own (health, reflection, its own protobuf API): the transport registers
// whatever it is given and knows nothing about the actions themselves.
type Service interface {
	Register(registrar grpc.ServiceRegistrar)
}

// Start starts a gRPC server and returns a Server instance.
func Start(config Config, services ...Service) (*Server, error) {
	config = NewConfig().Merge(config)
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	logger := config.Logger

	options := append(
		[]grpc.ServerOption{
			grpc.ChainUnaryInterceptor(recoverPanic(logger)),
		},
		config.GRPCOptions...,
	)
	grpcServer := grpc.NewServer(options...)

	for _, service := range services {
		service.Register(grpcServer)
	}
	reflection.Register(grpcServer)

	listener, err := net.Listen(config.Network, config.Address)
	if err != nil {
		return nil, fmt.Errorf("listen on %s %s: %w", config.Network, config.Address, err)
	}

	s := &Server{
		srv:       grpcServer,
		addr:      listener.Addr(),
		serveDone: make(chan error, 1),
	}

	logger.Info(ListeningLine(listener.Addr().Network(), listener.Addr().String()))

	go func() {
		s.serveDone <- grpcServer.Serve(listener)
	}()
	return s, nil
}

type Server struct {
	srv       *grpc.Server
	addr      net.Addr
	serveDone chan error
	stopOnce  sync.Once
	stopErr   error
}

func (s *Server) Addr() net.Addr {
	if s == nil {
		return nil
	}

	return s.addr
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

func recoverPanic(logger *slog.Logger) func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	//nolint:nonamedreturns // the deferred recover has to replace the returned error
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.ErrorContext(ctx, "panic in gRPC handler",
					slog.String("method", info.FullMethod),
					slog.Any("panic", recovered),
					slog.String("stack", string(debug.Stack())),
				)
				err = status.Errorf(codes.Internal, "panic in %s: %v", info.FullMethod, recovered)
			}
		}()
		return handler(ctx, req)
	}
}
