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
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander/attach"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func (s *Service) CommanderAttach(server pb.DHCTL_CommanderAttachServer) error {
	ctx := operationCtx(server)

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.CommanderAttachServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.CommanderAttachRequest)
	sendCh := make(chan *pb.CommanderAttachResponse)

	phaseSwitcher := &CommanderAttachPhaseSwitcher{
		sendCh: sendCh,
		f:      f,
		next:   make(chan error),
	}

	s.startattacherReceiver(server, receiveCh, doneCh, internalErrCh)
	s.startattacherSender(server, sendCh, internalErrCh)

connectionProcessor:
	for {
		select {
		case <-doneCh:
			logger.L(ctx).Info("finished")
			return nil

		case err := <-internalErrCh:
			logger.L(ctx).Error("finished with internal error", logger.Err(err))
			return status.Errorf(codes.Internal, "%s", err)

		case request := <-receiveCh:
			logger.L(ctx).Info(
				"processing CommanderAttachRequest",
				slog.String("message", fmt.Sprintf("%T", request.Message)),
			)
			switch message := request.Message.(type) {
			case *pb.CommanderAttachRequest_Start:
				err := f.Event("start")
				if err != nil {
					logger.L(ctx).Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				s.startCommanderAttach(
					ctx, message.Start, phaseSwitcher, &CommanderAttachLogWriter{l: logger.L(ctx), sendCh: sendCh}, sendCh,
				)

			case *pb.CommanderAttachRequest_Continue:
				err := f.Event("toNextPhase")
				if err != nil {
					logger.L(ctx).Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				switch message.Continue.Continue {
				case pb.Continue_CONTINUE_UNSPECIFIED:
					phaseSwitcher.next <- errors.New("bad continue message")
				case pb.Continue_CONTINUE_NEXT_PHASE:
					phaseSwitcher.next <- nil
				case pb.Continue_CONTINUE_STOP_OPERATION:
					phaseSwitcher.next <- phases.StopOperationCondition
				case pb.Continue_CONTINUE_ERROR:
					phaseSwitcher.next <- errors.New(message.Continue.Err)
				}

			default:
				logger.L(ctx).Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue connectionProcessor
			}
		}
	}
}

func (s *Service) startattacherReceiver(
	server pb.DHCTL_CommanderAttachServer,
	receiveCh chan *pb.CommanderAttachRequest,
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

func (s *Service) startattacherSender(
	server pb.DHCTL_CommanderAttachServer,
	sendCh chan *pb.CommanderAttachResponse,
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

func (s *Service) startCommanderAttach(
	ctx context.Context,
	request *pb.CommanderAttachStart,
	phaseSwitcher *CommanderAttachPhaseSwitcher,
	logWriter *CommanderAttachLogWriter,
	sendCh chan *pb.CommanderAttachResponse,
) {
	go func() {
		result := s.CommanderAttachCluster(ctx, request, phaseSwitcher, logWriter)
		sendCh <- &pb.CommanderAttachResponse{
			Message: &pb.CommanderAttachResponse_Result{
				Result: result,
			},
		}
	}()
}

func (s *Service) CommanderAttachCluster(
	ctx context.Context,
	request *pb.CommanderAttachStart,
	phaseSwitcher *CommanderAttachPhaseSwitcher,
	logWriter io.Writer,
) *pb.CommanderAttachResult {
	var err error

	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream: logWriter,
		Width:     int(request.Options.LogWidth),
	})
	app.SanityCheck = true
	app.UseTfCache = app.UseStateCacheYes
	app.ResourcesTimeout = request.Options.ResourcesTimeout.AsDuration()
	app.DeckhouseTimeout = request.Options.DeckhouseTimeout.AsDuration()
	app.CacheDir = s.cacheDir

	log.InfoF("Task is running by DHCTL Server pod/%s\n", s.podName)
	defer func() {
		log.InfoF("Task done by DHCTL Server pod/%s\n", s.podName)
	}()

	var sshClient *ssh.Client
	err = log.Process("default", "Preparing SSH client", func() error {
		connectionConfig, err := config.ParseConnectionConfig(
			request.ConnectionConfig,
			config.NewSchemaStore(),
			config.ValidateOptionCommanderMode(request.Options.CommanderMode),
			config.ValidateOptionStrictUnmarshal(request.Options.CommanderMode),
			config.ValidateOptionValidateExtensions(request.Options.CommanderMode),
		)
		if err != nil {
			return fmt.Errorf("parsing connection config: %w", err)
		}

		sshClient, err = prepareSSHClient(connectionConfig)
		if err != nil {
			return fmt.Errorf("preparing ssh client: %w", err)
		}
		return nil
	})
	if err != nil {
		return &pb.CommanderAttachResult{Err: err.Error()}
	}
	defer sshClient.Stop()

	attacher := attach.NewAttacher(&attach.Params{
		CommanderMode:    request.Options.CommanderMode,
		CommanderUUID:    uuid.MustParse("f013cd23-6ceb-4bc5-a32d-c6f2c8cf41ea"),
		SSHClient:        sshClient,
		OnCheckResult:    onCheckResult,
		TerraformContext: terraform.NewTerraformContext(),
		OnPhaseFunc:      phaseSwitcher.switchPhase,
		AttachResources: attach.AttachResources{
			Template: request.ResourcesTemplate,
			Values:   request.ResourcesValues.AsMap(),
		},
		ScanOnly: request.ScanOnly,
	})

	result, attacherr := attacher.Attach(ctx)
	state := attacher.PhasedExecutionContext.GetLastState()
	stateData, marshalStateErr := json.Marshal(state)
	resultString, marshalResultErr := json.Marshal(result)
	err = errors.Join(attacherr, marshalStateErr, marshalResultErr)

	return &pb.CommanderAttachResult{State: string(stateData), Result: string(resultString), Err: errToString(err)}
}

func (s *Service) CommanderAttachServerTransitions() []fsm.Transition {
	return []fsm.Transition{
		{
			Event:       "start",
			Sources:     []fsm.State{"initial"},
			Destination: "running",
		},
		{
			Event:       "wait",
			Sources:     []fsm.State{"running"},
			Destination: "waiting",
		},
		{
			Event:       "toNextPhase",
			Sources:     []fsm.State{"waiting"},
			Destination: "running",
		},
	}
}

type CommanderAttachLogWriter struct {
	l      *slog.Logger
	sendCh chan *pb.CommanderAttachResponse

	m    sync.Mutex
	prev []byte
}

func (w *CommanderAttachLogWriter) Write(p []byte) (n int, err error) {
	w.m.Lock()
	defer w.m.Unlock()

	var r []string

	for _, b := range p {
		switch b {
		case '\n', '\r':
			s := string(w.prev)
			if s != "" {
				r = append(r, s)
			}
			w.prev = []byte{}
		default:
			w.prev = append(w.prev, b)
		}
	}

	if len(r) > 0 {
		for _, line := range r {
			w.l.Info(line, logTypeDHCTL)
		}
		w.sendCh <- &pb.CommanderAttachResponse{
			Message: &pb.CommanderAttachResponse_Logs{Logs: &pb.Logs{Logs: r}},
		}
	}

	return len(p), nil
}

type CommanderAttachPhaseSwitcher struct {
	sendCh chan *pb.CommanderAttachResponse
	f      *fsm.FiniteStateMachine
	next   chan error
}

func (b *CommanderAttachPhaseSwitcher) switchPhase(
	completedPhase phases.OperationPhase,
	completedPhaseState phases.DhctlState,
	phaseData attach.PhaseData,
	nextPhase phases.OperationPhase,
	nextPhaseCritical bool,
) error {
	err := b.f.Event("wait")
	if err != nil {
		return fmt.Errorf("changing state to waiting: %w", err)
	}

	phaseDataBytes, err := json.Marshal(phaseData)
	if err != nil {
		return fmt.Errorf("changing state to waiting: %w", err)
	}

	b.sendCh <- &pb.CommanderAttachResponse{
		Message: &pb.CommanderAttachResponse_PhaseEnd{
			PhaseEnd: &pb.CommanderAttachPhaseEnd{
				CompletedPhase:      string(completedPhase),
				CompletedPhaseState: completedPhaseState,
				CompletedPhaseData:  string(phaseDataBytes),
				NextPhase:           string(nextPhase),
				NextPhaseCritical:   nextPhaseCritical,
			},
		},
	}

	switchErr, ok := <-b.next
	if !ok {
		return fmt.Errorf("server stopped, cancel task")
	}
	return switchErr
}
