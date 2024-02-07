// Copyright 2024 Flant JSC
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
	"log/slog"
	"net"
	"runtime/debug"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	dhctllog "github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/logger"
	pbhello "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/hello"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/rpc/hello"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// Serve starts GRPC server
func Serve() error {
	dhctllog.InitLoggerWithOptions("silent", dhctllog.LoggerOptions{})
	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelDebug)
	log := logger.NewLogger(lvl)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	defer close(done)

	log.Info(
		"starting grpc server",
		slog.String("host", app.ServerHost),
		slog.Int("port", app.ServerPort),
	)
	tomb.RegisterOnShutdown("server", func() {
		log.Info("stopping grpc server")
		cancel()
		<-done
		log.Info("grpc server stopped")
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", app.ServerHost, app.ServerPort))
	if err != nil {
		log.Error("failed to listen", logger.Err(err))
		return err
	}
	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			logging.UnaryServerInterceptor(interceptorLogger(log)),
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler(log))),
		),
		grpc.ChainStreamInterceptor(
			logging.StreamServerInterceptor(interceptorLogger(log)),
			recovery.StreamServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler(log))),
		),
	)

	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-a-grpc-liveness-probe
	healthService := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthService)

	// grpcurl -plaintext host:port describe
	reflection.Register(s)

	// register services
	pbhello.RegisterGreeterServer(s, &hello.Service{})

	go func() {
		<-ctx.Done()

		gracefulStop(s, time.Second*10)
	}()

	if err = s.Serve(listener); err != nil {
		log.Error("failed to serve", logger.Err(err))
		return err
	}
	return nil
}

func grpcPanicRecoveryHandler(log *slog.Logger) func(p any) error {
	return func(p any) error {
		log.Error(
			"recovered from panic",
			slog.Any("panic", p),
			slog.Any("stack", string(debug.Stack())),
		)
		return status.Errorf(codes.Internal, "%s", p)
	}
}

func interceptorLogger(log *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		log.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}

func gracefulStop(s *grpc.Server, timeout time.Duration) {
	stopped := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(stopped)
	}()

	t := time.NewTimer(timeout)
	select {
	case <-t.C:
		s.Stop()
	case <-stopped:
		t.Stop()
	}
}
