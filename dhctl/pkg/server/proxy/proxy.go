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

package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/mwitkow/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"

	dhctllog "github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/interceptors"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

// Serve starts GRPC server
func Serve(network, address string, parallelTasksLimit int) error {
	dhctllog.InitLoggerWithOptions("silent", dhctllog.LoggerOptions{})
	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelDebug)
	log := logger.NewLogger(lvl).With(slog.String("component", "proxy"))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	defer close(done)
	sem := make(chan struct{}, parallelTasksLimit)

	director := streamDirector{
		log: log,
	}

	log.Info(
		"starting grpc server proxy",
		slog.String("network", network),
		slog.String("address", address),
	)
	tomb.RegisterOnShutdown("server", func() {
		log.Info("stopping grpc server proxy")
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
			interceptors.UnaryParallelTasksLimiter(sem, log),
		),
		grpc.ChainStreamInterceptor(
			logging.StreamServerInterceptor(interceptors.Logger(log)),
			recovery.StreamServerInterceptor(recovery.WithRecoveryHandler(interceptors.PanicRecoveryHandler(log))),
			interceptors.StreamParallelTasksLimiter(sem, log),
		),
		grpc.UnknownServiceHandler(proxy.TransparentHandler(director.new())),
	)

	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-a-grpc-liveness-probe
	healthService := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthService)

	// grpcurl -plaintext host:port describe
	reflection.Register(s)

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

type streamDirector struct {
	log *slog.Logger
}

func (d *streamDirector) new() proxy.StreamDirector {
	return func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
		// Copy the inbound metadata explicitly.
		md, _ := metadata.FromIncomingContext(ctx)
		outCtx := metadata.NewOutgoingContext(ctx, md.Copy())

		address, err := socketPath()
		if err != nil {
			return outCtx, nil, err
		}

		d.log.Info("starting new dhctl instance", "addr", address)

		cmd := exec.Command(
			os.Args[0],
			"_server",
			"--server-network=unix",
			fmt.Sprintf("--server-address=%s", address),
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err = cmd.Start()
		if err != nil {
			return outCtx, nil, fmt.Errorf("starting dhctl server: %w", err)
		}

		conn, err := grpc.NewClient(
			"unix://"+address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return outCtx, nil, fmt.Errorf("creating client connection: %w", err)
		}

		err = checkDHCTLServer(ctx, conn)
		if err != nil {
			return outCtx, nil, fmt.Errorf("waiting for dhctl server ready: %w", err)
		}

		return outCtx, conn, err
	}
}

func checkDHCTLServer(ctx context.Context, conn grpc.ClientConnInterface) error {
	healthCl := grpc_health_v1.NewHealthClient(conn)
	loop := retry.NewSilentLoop("wait for dhctl server", 10, time.Second)
	return loop.Run(func() error {
		check, err := healthCl.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		if err != nil {
			return fmt.Errorf("checking dhctl server status: %w", err)
		}
		if check.Status != grpc_health_v1.HealthCheckResponse_SERVING {
			return fmt.Errorf("bad dhctl server status: %s", check.Status)
		}
		return nil
	})
}

func socketPath() (string, error) {
	sockUUID, err := uuid.NewUUID()
	if err != nil {
		return "", fmt.Errorf("creating uuid for socket path")
	}

	address := filepath.Join("/var/run/dhctl", sockUUID.String()+".sock")
	return address, nil
}
