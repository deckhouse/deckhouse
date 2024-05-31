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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mwitkow/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type StreamDirector struct {
	methodsPrefix string
	log           *slog.Logger
}

func NewStreamDirector(log *slog.Logger, methodsPrefix string) *StreamDirector {
	return &StreamDirector{
		methodsPrefix: methodsPrefix,
		log:           log,
	}
}

func (d *StreamDirector) Director() proxy.StreamDirector {
	return func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
		// Copy the inbound metadata explicitly.
		md, _ := metadata.FromIncomingContext(ctx)
		outCtx := metadata.NewOutgoingContext(ctx, md.Copy())

		if !strings.HasPrefix(fullMethodName, d.methodsPrefix) {
			return outCtx, nil, status.Errorf(codes.Unimplemented, "Unknown method")
		}

		address, err := socketPath()
		if err != nil {
			return outCtx, nil, err
		}

		cmd := exec.Command(
			os.Args[0],
			"_server",
			"--server-network=unix",
			fmt.Sprintf("--server-address=%s", address),
		)

		// todo: handle logs from parallel server instances
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err = cmd.Start()
		if err != nil {
			return outCtx, nil, fmt.Errorf("starting dhctl server: %w", err)
		}

		logger.L(ctx).Info("started new dhctl instance", slog.String("addr", address))

		go func() {
			exitErr := cmd.Wait()
			logger.L(ctx).
				Info("stopped dhctl instance", slog.String("addr", address), logger.Err(exitErr))
		}()

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
