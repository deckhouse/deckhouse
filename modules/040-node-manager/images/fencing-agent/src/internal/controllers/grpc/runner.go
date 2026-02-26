/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/lib/logger/sl"
	"fencing-agent/internal/lib/validators"
	pb "fencing-agent/pkg/api/v1"
)

type Config struct {
	SocketPath  string `env:"GRPC_SOCKET_PATH" env-default:"/tmp/fencing-agent.sock"`
	UnaryRPS    int    `env:"REQUEST_RPS" env-default:"10"`
	UnaryBurst  int    `env:"REQUEST_BURST" env-default:"100"`
	StreamRPS   int    `env:"STREAM_RPS" env-default:"5"`
	StreamBurst int    `env:"STREAM_BURST" env-default:"100"`
}

func (c *Config) Validate() error {
	if unaryErr := validators.ValidateRateLimit(c.UnaryRPS, c.UnaryBurst, "Unary"); unaryErr != nil {
		return unaryErr
	}

	if streamErr := validators.ValidateRateLimit(c.StreamRPS, c.StreamBurst, "Stream"); streamErr != nil {
		return streamErr
	}

	if strings.TrimSpace(c.SocketPath) == "" {
		return errors.New("GRPC_SOCKET_PATH is empty")
	}
	return nil
}

type Runner struct {
	logger     *log.Logger
	grpcServer *grpc.Server
	listener   net.Listener
	socketPath string
	cleanOnce  sync.Once
}

func NewRunner(cfg Config, logger *log.Logger, handler *Server) (*Runner, error) {
	if err := os.RemoveAll(cfg.SocketPath); err != nil {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}

	listener, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener on %s: %w", cfg.SocketPath, err)
	}

	unaryRateLimit := rate.NewLimiter(rate.Limit(cfg.UnaryRPS), cfg.UnaryBurst)
	streamRateLimit := rate.NewLimiter(rate.Limit(cfg.StreamRPS), cfg.StreamBurst)

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(UnaryRateLimiterInterceptor(unaryRateLimit, logger)),
		grpc.StreamInterceptor(StreamServerInterceptor(streamRateLimit, logger)),
	)

	pb.RegisterFencingServer(grpcServer, handler)

	reflection.Register(grpcServer)

	return &Runner{
		grpcServer: grpcServer,
		listener:   listener,
		logger:     logger,
	}, nil
}

func (r *Runner) Run() error {
	if err := r.grpcServer.Serve(r.listener); err != nil {
		return fmt.Errorf("gRPC server failed: %w", err)
	}
	return nil
}

func (r *Runner) Stop() {
	r.grpcServer.GracefulStop()
	cleanErr := r.cleanSocket()
	if cleanErr != nil {
		r.logger.Error("failed to clean socket file", sl.Err(cleanErr))
	}
}

func (r *Runner) cleanSocket() error {
	var cleanErr error
	r.cleanOnce.Do(func() {
		if err := os.RemoveAll(r.socketPath); err != nil {
			cleanErr = fmt.Errorf("failed to remove socket file: %w", err)
		}
	})
	return cleanErr
}

func UnaryRateLimiterInterceptor(l *rate.Limiter, logger *log.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if !l.Allow() {
			logger.Warn("unary rate limit exceeded")
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

func StreamServerInterceptor(l *rate.Limiter, logger *log.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		_ *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if !l.Allow() {
			logger.Warn("stream server rate limit exceeded")
			return status.Errorf(codes.ResourceExhausted, "rate limited")
		}
		return handler(srv, ss)
	}
}
