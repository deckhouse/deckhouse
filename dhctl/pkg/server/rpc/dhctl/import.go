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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/import"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Service) Import(server pb.DHCTL_ImportServer) error {
	ctx := operationCtx(server)

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.importServerTransitions())

	internalErrCh := make(chan error)
	doneCh := make(chan struct{})
	requestsCh := make(chan *pb.ImportRequest)

	phaseSwitcher := &importPhaseSwitcher{
		server: server,
		f:      f,
		next:   make(chan error),
	}
	defer close(phaseSwitcher.next)

	s.startImporterReceiver(server, requestsCh, internalErrCh)

connectionProcessor:
	for {
		select {
		case <-doneCh:
			logger.L(ctx).Info("finished")
			return nil

		case err := <-internalErrCh:
			logger.L(ctx).Error("finished with internal error", logger.Err(err))
			return status.Errorf(codes.Internal, "%s", err)

		case request := <-requestsCh:
			logger.L(ctx).Info(
				"processing ImportRequest",
				slog.String("message", fmt.Sprintf("%T", request.Message)),
			)
			switch message := request.Message.(type) {
			case *pb.ImportRequest_Start:
				err := f.Event("start")
				if err != nil {
					logger.L(ctx).Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				s.startImport(
					ctx, server, message.Start, phaseSwitcher, &importLogWriter{l: logger.L(ctx), server: server},
					internalErrCh, doneCh,
				)

			case *pb.ImportRequest_Continue:
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

func (s *Service) startImporterReceiver(
	server pb.DHCTL_ImportServer,
	requestsCh chan *pb.ImportRequest,
	internalErrCh chan error,
) {
	go func() {
		for {
			request, err := server.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				internalErrCh <- fmt.Errorf("receiving message: %w", err)
				return
			}
			requestsCh <- request
		}
	}()
}

func (s *Service) startImport(
	ctx context.Context,
	server pb.DHCTL_ImportServer,
	request *pb.ImportStart,
	phaseSwitcher *importPhaseSwitcher,
	logWriter *importLogWriter,
	internalErrCh chan error,
	doneCh chan struct{},
) {
	go func() {
		result := s.importCluster(ctx, request, phaseSwitcher, logWriter)
		err := server.Send(&pb.ImportResponse{
			Message: &pb.ImportResponse_Result{
				Result: result,
			},
		})
		if err != nil {
			internalErrCh <- fmt.Errorf("sending message: %w", err)
			return
		}

		doneCh <- struct{}{}
	}()
}

func (s *Service) importCluster(
	ctx context.Context,
	request *pb.ImportStart,
	phaseSwitcher *importPhaseSwitcher,
	logWriter io.Writer,
) *pb.ImportResult {
	var err error

	// set global variables from options
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
		return &pb.ImportResult{Err: err.Error()}
	}
	defer sshClient.Stop()

	importer := _import.NewImporter(&_import.Params{
		CommanderMode:    request.Options.CommanderMode,
		SSHClient:        sshClient,
		OnCheckResult:    onCheckResult,
		TerraformContext: terraform.NewTerraformContext(),
		OnPhaseFunc:      phaseSwitcher.switchPhase,
		ImportResources: _import.ImportResources{
			Template: request.ResourcesTemplate,
			Values:   request.ResourcesValues.AsMap(),
		},
		ScanOnly: request.ScanOnly,
	})

	result, importErr := importer.Import(ctx)
	state := importer.PhasedExecutionContext.GetLastState()
	stateData, marshalStateErr := json.Marshal(state)
	resultString, marshalResultErr := json.Marshal(result)
	err = errors.Join(importErr, marshalStateErr, marshalResultErr)

	return &pb.ImportResult{State: string(stateData), Result: string(resultString), Err: errToString(err)}
}

func (s *Service) importServerTransitions() []fsm.Transition {
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

type importLogWriter struct {
	l      *slog.Logger
	server pb.DHCTL_ImportServer
}

func (w *importLogWriter) Write(p []byte) (int, error) {
	w.l.Info(string(p), logTypeDHCTL)

	err := w.server.Send(&pb.ImportResponse{
		Message: &pb.ImportResponse_Logs{
			Logs: &pb.Logs{
				Logs: p,
			},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("writing check logs: %w", err)
	}
	return len(p), nil
}

type importPhaseSwitcher struct {
	server pb.DHCTL_ImportServer
	f      *fsm.FiniteStateMachine
	next   chan error
}

func (b *importPhaseSwitcher) switchPhase(
	completedPhase phases.OperationPhase,
	completedPhaseState phases.DhctlState,
	phaseData _import.PhaseData,
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

	err = b.server.Send(&pb.ImportResponse{
		Message: &pb.ImportResponse_PhaseEnd{
			PhaseEnd: &pb.ImportPhaseEnd{
				CompletedPhase:      string(completedPhase),
				CompletedPhaseState: completedPhaseState,
				CompletedPhaseData:  string(phaseDataBytes),
				NextPhase:           string(nextPhase),
				NextPhaseCritical:   nextPhaseCritical,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("sending on phase message: %w", err)
	}

	switchErr, ok := <-b.next
	if !ok {
		return fmt.Errorf("server stopped, cancel task")
	}
	return switchErr
}
