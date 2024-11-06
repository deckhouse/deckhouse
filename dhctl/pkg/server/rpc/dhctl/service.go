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

package dhctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

var logTypeDHCTL = slog.String("type", "dhctl")

type Service struct {
	pb.UnimplementedDHCTLServer

	podName  string
	cacheDir string

	schemaStore *config.SchemaStore
}

func New(podName, cacheDir string, schemaStore *config.SchemaStore) *Service {
	return &Service{
		podName:     podName,
		cacheDir:    cacheDir,
		schemaStore: schemaStore,
	}
}

func operationCtx(server grpc.ServerStream) context.Context {
	ctx := server.Context()

	var operation string
	switch server.(type) {
	case pb.DHCTL_CheckServer:
		operation = "check"
	case pb.DHCTL_BootstrapServer:
		operation = "bootstrap"
	case pb.DHCTL_ConvergeServer:
		operation = "converge"
	case pb.DHCTL_DestroyServer:
		operation = "destroy"
	case pb.DHCTL_AbortServer:
		operation = "abort"
	case pb.DHCTL_CommanderAttachServer:
		operation = "commander/attach"
	case pb.DHCTL_CommanderDetachServer:
		operation = "commander/detach"
	default:
		operation = "unknown"
	}
	go func() {
		<-ctx.Done()
		tomb.Shutdown(0)
	}()
	return logger.ToContext(
		ctx,
		logger.L(ctx).With(slog.String("operation", operation)),
	)
}

type serverStream[Request proto.Message, Response proto.Message] interface {
	Send(Response) error
	Recv() (Request, error)
	grpc.ServerStream
}

func startReceiver[Request, Response proto.Message](
	server serverStream[Request, Response],
	receiveCh chan Request,
	doneCh chan struct{},
	internalErrCh chan error,
) {
	go func() {
		for {
			request, err := server.Recv()
			if errors.Is(err, io.EOF) {
				close(doneCh)
				return
			}
			if err != nil {
				internalErrCh <- fmt.Errorf("receiving message: %w", err)
				return
			}
			receiveCh <- request
		}
	}()
}

func startSender[Request, Response proto.Message](
	server serverStream[Request, Response],
	sendCh chan Response,
	internalErrCh chan error,
) {
	go func() {
		for response := range sendCh {
			loop := retry.NewSilentLoop("send message", 10, time.Millisecond*100)
			err := loop.Run(func() error {
				return server.Send(response)
			})
			if err != nil {
				internalErrCh <- fmt.Errorf("sending message: %w", err)
				return
			}
		}
	}()
}

type fsmPhaseSwitcher[T proto.Message, OperationPhaseDataT any] struct {
	f        *fsm.FiniteStateMachine
	dataFunc func(
		completedPhase phases.OperationPhase,
		completedPhaseState phases.DhctlState,
		phaseData OperationPhaseDataT,
		nextPhase phases.OperationPhase,
		nextPhaseCritical bool,
	) (T, error)
	sendCh chan T
	next   chan error
}

func (b *fsmPhaseSwitcher[T, OperationPhaseDataT]) switchPhase(
	completedPhase phases.OperationPhase,
	completedPhaseState phases.DhctlState,
	phaseData OperationPhaseDataT,
	nextPhase phases.OperationPhase,
	nextPhaseCritical bool,
) error {
	err := b.f.Event("wait")
	if err != nil {
		return fmt.Errorf("changing state to waiting: %w", err)
	}

	data, err := b.dataFunc(
		completedPhase,
		completedPhaseState,
		phaseData,
		nextPhase,
		nextPhaseCritical,
	)
	if err != nil {
		return fmt.Errorf("switch phase data func error: %w", err)
	}

	b.sendCh <- data

	switchErr, ok := <-b.next
	if !ok {
		return fmt.Errorf("server stopped, cancel task")
	}
	return switchErr
}

func onCheckResult(checkRes *check.CheckResult) error {
	printableCheckRes := *checkRes
	printableCheckRes.StatusDetails.TerraformPlan = nil

	printableCheckResDump, err := json.MarshalIndent(printableCheckRes, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to encode check result json: %w", err)
	}

	_ = log.Process("default", "Check result", func() error {
		log.InfoF("%s\n", printableCheckResDump)
		return nil
	})

	return nil
}

func panicMessage(ctx context.Context, p any) string {
	stack := string(debug.Stack())

	logger.L(ctx).Error("recovered from panic",
		slog.Any("panic", p),
		slog.String("stack", stack),
	)
	return fmt.Sprintf("panic: %v, %s", p, stack)
}
