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
	"os"
	"sync"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	dhctllog "github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/interceptors"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/logger"
	pbdhctl "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/rpc/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

// Serve starts GRPC server
func Serve(network, address string) error {
	dhctllog.InitLoggerWithOptions("silent", dhctllog.LoggerOptions{})
	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelDebug)
	log := logger.NewLogger(lvl)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	defer close(done)
	globalLock := &sync.Mutex{}

	podName := os.Getenv("HOSTNAME")

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
			logging.UnaryServerInterceptor(interceptors.Logger(log)),
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(interceptors.PanicRecoveryHandler(log))),
			interceptors.UnaryServerSinglefligt(globalLock, log),
		),
		grpc.ChainStreamInterceptor(
			logging.StreamServerInterceptor(interceptors.Logger(log)),
			recovery.StreamServerInterceptor(recovery.WithRecoveryHandler(interceptors.PanicRecoveryHandler(log))),
			interceptors.StreamServerSinglefligt(globalLock, log),
		),
	)

	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-a-grpc-liveness-probe
	healthService := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthService)

	// grpcurl -plaintext host:port describe
	reflection.Register(s)

	// services
	dhctlService := dhctl.New(podName, address, log)

	// register services
	pbdhctl.RegisterDHCTLServer(s, dhctlService)

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
