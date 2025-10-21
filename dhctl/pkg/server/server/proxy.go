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
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/mwitkow/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type StreamDirector struct {
	methodsPrefix string

	wg          *sync.WaitGroup
	syncWriters *syncWriters
}

func NewStreamDirector(methodsPrefix string) *StreamDirector {
	return &StreamDirector{
		methodsPrefix: methodsPrefix,

		wg: &sync.WaitGroup{},
		syncWriters: &syncWriters{
			stdoutWriter: &syncWriter{writer: os.Stdout},
			stderrWriter: &syncWriter{writer: os.Stderr},
		},
	}
}

func (d *StreamDirector) Wait() {
	d.wg.Wait()
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

		log := logger.L(ctx).With(slog.String("addr", address))

		cmd := exec.Command(
			os.Args[0],
			"_server",
			"--server-network=unix",
			fmt.Sprintf("--server-address=%s", address),
		)

		// Add parent envs to child envs
		cmd.Env = append(cmd.Env, os.Environ()...)

		// Launch new process group so that signals (ex: SIGINT) are not sent also to the child process.
		// https://stackoverflow.com/a/66261096
		// Child process will start own child process e.g. infrastructure utility. We want them to finish normally.
		// Parent process should wait for all children.
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}

		stdOutReader, err := cmd.StdoutPipe()
		if err != nil {
			return outCtx, nil, fmt.Errorf("getting dhctl instance stdout pipe: %w", err)
		}

		stdErrReader, err := cmd.StderrPipe()
		if err != nil {
			return outCtx, nil, fmt.Errorf("getting dhctl instance stderr pipe: %w", err)
		}

		d.writeLogs(log, stdOutReader, stdErrReader)

		err = cmd.Start()
		if err != nil {
			return outCtx, nil, fmt.Errorf("starting dhctl server: %w", err)
		}

		log.Info("started new dhctl instance")

		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			exitErr := cmd.Wait()
			log.Info("stopped dhctl instance", logger.Err(exitErr))
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

func (d *StreamDirector) writeLogs(log *slog.Logger, stdOutReader, stdErrReader io.Reader) {
	d.wg.Add(2)

	go func() {
		defer d.wg.Done()

		err := d.syncWriters.stdoutWriter.copyFrom(stdOutReader)
		if err != nil {
			log.Error("writing dhctl instance stdout logs", logger.Err(err))
		}
	}()

	go func() {
		defer d.wg.Done()

		err := d.syncWriters.stderrWriter.copyFrom(stdErrReader)
		if err != nil {
			log.Error("writing dhctl instance stderr logs", logger.Err(err))
		}
	}()
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

	address := filepath.Join(app.TmpDirName, sockUUID.String()+".sock")
	return address, nil
}

type syncWriters struct {
	stdoutWriter *syncWriter
	stderrWriter *syncWriter
}

type syncWriter struct {
	writer io.Writer
	mx     sync.Mutex
}

func (sw *syncWriter) Write(p []byte) (n int, err error) {
	sw.mx.Lock()
	defer sw.mx.Unlock()
	return sw.writer.Write(p)
}

func (sw *syncWriter) copyFrom(reader io.Reader) error {
	_, err := io.Copy(sw, reader)
	return err
}
