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
	"log/slog"
	"net"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/mwitkow/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	dhctllog "github.com/deckhouse/deckhouse/dhctl/pkg/log"
	pbdhctl "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/interceptors"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/rpc/validation"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

// full method example: /dhctl.DHCTL/Check
const singlethreadedMethodsPrefix = "/dhctl.DHCTL"

// Serve starts GRPC server
func Serve(network, address string, parallelTasksLimit int) error {
	dhctllog.InitLoggerWithOptions("silent", dhctllog.LoggerOptions{})
	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelDebug)
	log := logger.NewLogger(lvl).With(slog.String("component", "server"))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	defer close(done)
	sem := make(chan struct{}, parallelTasksLimit)

	director := NewStreamDirector(log, singlethreadedMethodsPrefix)

	log.Info(
		"starting grpc server",
		slog.String("network", network),
		slog.String("address", address),
	)
	tomb.RegisterOnShutdown("server", func() {
		log.Info("stopping grpc server")
		cancel()
		<-done
		log.Info("grpc server stopped")
	})

	listener, err := net.Listen(network, address)
	if err != nil {
		log.Error("failed to listen", logger.Err(err))
		return err
	}
	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptors.UnaryLogger(log),
			logging.UnaryServerInterceptor(interceptors.Logger()),
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandlerContext(interceptors.PanicRecoveryHandler())),
			interceptors.UnaryParallelTasksLimiter(sem, singlethreadedMethodsPrefix),
		),
		grpc.ChainStreamInterceptor(
			interceptors.StreamLogger(log),
			logging.StreamServerInterceptor(interceptors.Logger()),
			recovery.StreamServerInterceptor(recovery.WithRecoveryHandlerContext(interceptors.PanicRecoveryHandler())),
			interceptors.StreamParallelTasksLimiter(sem, singlethreadedMethodsPrefix),
		),
		grpc.UnknownServiceHandler(proxy.TransparentHandler(director.Director())),
	)

	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-a-grpc-liveness-probe
	healthService := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthService)

	// grpcurl -plaintext host:port describe
	reflection.Register(s)

	// init services
	validationService := validation.New(config.NewSchemaStore())

	// register services
	pbdhctl.RegisterValidationServer(s, validationService)

	go func() {
		<-ctx.Done()

		s.GracefulStop()
	}()

	if err = s.Serve(listener); err != nil {
		log.Error("failed to serve", logger.Err(err))
		return err
	}
	return nil
}
