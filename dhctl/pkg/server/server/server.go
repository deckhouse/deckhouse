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
	rc "github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/requests_counter"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/rpc/status"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/rpc/validation"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/server/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

// Full method example: /dhctl.DHCTL/Check
const singlethreadedMethodsPrefix = "/dhctl.DHCTL"

// Serve starts GRPC server
func Serve(params settings.ServerParams) error {
	if err := params.Validate(); err != nil {
		return err
	}

	dhctllog.InitLoggerWithOptions("silent", dhctllog.LoggerOptions{})
	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelDebug)
	log := logger.NewLogger(lvl).With(slog.String("component", "server"))

	dhctlProxy, err := NewStreamDirector(StreamDirectorParams{
		MethodsPrefix: singlethreadedMethodsPrefix,
		TmpDir:        params.TmpDir,
	})

	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	defer close(done)
	sem := make(chan struct{}, params.ParallelTasksLimit)

	requestsCounter := rc.New(params.RequestsCounterMaxDuration, sem)
	requestsCounter.Run(ctx)

	log.Info(
		"starting grpc server",
		slog.String("network", params.Network),
		slog.String("address", params.Address),
		slog.String("tmp_dir", params.TmpDir),
	)
	tomb.RegisterOnShutdown("server", func() {
		log.Info("stopping grpc server")
		cancel()
		<-done
		log.Info("grpc server stopped")
	})

	listener, err := net.Listen(params.Network, params.Address)
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
			interceptors.UnaryRequestsCounter(requestsCounter),
		),
		grpc.ChainStreamInterceptor(
			interceptors.StreamLogger(log),
			logging.StreamServerInterceptor(interceptors.Logger()),
			recovery.StreamServerInterceptor(recovery.WithRecoveryHandlerContext(interceptors.PanicRecoveryHandler())),
			interceptors.StreamParallelTasksLimiter(sem, singlethreadedMethodsPrefix),
			interceptors.StreamRequestsCounter(requestsCounter),
		),
		grpc.UnknownServiceHandler(proxy.TransparentHandler(dhctlProxy.Director())),
	)

	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-a-grpc-liveness-probe
	healthService := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthService)

	// grpcurl -plaintext host:port describe
	reflection.Register(s)

	// init services
	validationService := validation.New(config.NewSchemaStore())
	statusService := status.New(requestsCounter)

	// register services
	pbdhctl.RegisterValidationServer(s, validationService)
	pbdhctl.RegisterStatusServer(s, statusService)

	go func() {
		<-ctx.Done()

		s.GracefulStop()
	}()

	if err = s.Serve(listener); err != nil {
		log.Error("failed to serve", logger.Err(err))
		return err
	}

	// wait for all dhctl instances to complete
	dhctlProxy.Wait()

	return nil
}
