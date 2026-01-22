package grpc

import (
	"context"
	pb "fencing-agent/pkg/api/v1"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type Runner struct {
	logger     *log.Logger
	grpcServer *grpc.Server
	listener   net.Listener
	socketPath string
	cleanOnce  sync.Once
}

func NewRunner(socketPath string, logger *log.Logger, handler *Server, unaryLimiter *rate.Limiter, streamLimiter *rate.Limiter) (*Runner, error) {
	if err := os.RemoveAll(socketPath); err != nil {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener on %s: %w", socketPath, err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(UnaryRateLimiterInterceptor(unaryLimiter, logger)),
		grpc.StreamInterceptor(StreamServerInterceptor(streamLimiter, logger)),
	)

	pb.RegisterFencingServer(grpcServer, handler)

	reflection.Register(grpcServer)

	return &Runner{
		grpcServer: grpcServer,
		listener:   listener,
		socketPath: socketPath,
		logger:     logger,
		cleanOnce:  sync.Once{},
	}, nil
}

func (r *Runner) Run() error {
	if err := r.grpcServer.Serve(r.listener); err != nil {
		return fmt.Errorf("gRPC server failed: %w", err)
	}
	return nil
}

func (r *Runner) Stop(ctx context.Context) error {
	done := make(chan struct{})

	go func() {
		r.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info("gRPC server shutdown complete")
		return r.cleanSocket()

	case <-ctx.Done():
		r.logger.Warn("gRPC server shutdown exceeded, forced stop")
		r.grpcServer.Stop()
		_ = r.cleanSocket()
		return fmt.Errorf("gRPC graceful shutdown timeout exceeded, forced stop")
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
