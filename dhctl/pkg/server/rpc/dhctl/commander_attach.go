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
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util/callback"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

func (s *Service) CommanderAttach(server pb.DHCTL_CommanderAttachServer) error {
	ctx := operationCtx(server)

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.commanderAttachServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.CommanderAttachRequest)
	sendCh := make(chan *pb.CommanderAttachResponse)
	phaseSwitcher := &fsmPhaseSwitcher[*pb.CommanderAttachResponse, attach.PhaseData]{
		f: f, dataFunc: s.attachSwitchPhaseData, sendCh: sendCh, next: make(chan error),
	}
	logWriter := logger.NewLogWriter(logger.L(ctx).With(logTypeDHCTL), sendCh,
		func(lines []string) *pb.CommanderAttachResponse {
			return &pb.CommanderAttachResponse{Message: &pb.CommanderAttachResponse_Logs{Logs: &pb.Logs{Logs: lines}}}
		},
	)

	startReceiver[*pb.CommanderAttachRequest, *pb.CommanderAttachResponse](server, receiveCh, doneCh, internalErrCh)
	startSender[*pb.CommanderAttachRequest, *pb.CommanderAttachResponse](server, sendCh, internalErrCh)

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
				go func() {
					result := s.commanderAttach(ctx, message.Start, phaseSwitcher.switchPhase, logWriter)
					sendCh <- &pb.CommanderAttachResponse{Message: &pb.CommanderAttachResponse_Result{Result: result}}
				}()

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

func (s *Service) commanderAttach(
	ctx context.Context,
	request *pb.CommanderAttachStart,
	switchPhase phases.OnPhaseFunc[attach.PhaseData],
	logWriter io.Writer,
) *pb.CommanderAttachResult {
	var err error

	cleanuper := callback.NewCallback()
	defer cleanuper.Call()

	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream: logWriter,
		Width:     int(request.Options.LogWidth),
	})
	app.SanityCheck = true
	app.UseTfCache = app.UseStateCacheYes
	app.ResourcesTimeout = request.Options.ResourcesTimeout.AsDuration()
	app.DeckhouseTimeout = request.Options.DeckhouseTimeout.AsDuration()
	app.CacheDir = s.cacheDir
	app.ApplyPreflightSkips(request.Options.CommonOptions.SkipPreflightChecks)

	log.InfoF("Task is running by DHCTL Server pod/%s\n", s.podName)
	defer func() {
		log.InfoF("Task done by DHCTL Server pod/%s\n", s.podName)
	}()

	var sshClient *ssh.Client
	err = log.Process("default", "Preparing SSH client", func() error {
		connectionConfig, err := config.ParseConnectionConfig(
			request.ConnectionConfig,
			s.schemaStore,
			config.ValidateOptionCommanderMode(request.Options.CommanderMode),
			config.ValidateOptionStrictUnmarshal(request.Options.CommanderMode),
			config.ValidateOptionValidateExtensions(request.Options.CommanderMode),
		)
		if err != nil {
			return fmt.Errorf("parsing connection config: %w", err)
		}

		var cleanup func() error
		sshClient, cleanup, err = helper.CreateSSHClient(connectionConfig)
		cleanuper.Add(cleanup)
		if err != nil {
			return fmt.Errorf("preparing ssh client: %w", err)
		}
		return nil
	})
	if err != nil {
		return &pb.CommanderAttachResult{Err: err.Error()}
	}

	var commanderUUID uuid.UUID
	if request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(request.Options.CommanderUuid)
		if err != nil {
			return &pb.CommanderAttachResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	attacher := attach.NewAttacher(&attach.Params{
		CommanderMode:    request.Options.CommanderMode,
		CommanderUUID:    commanderUUID,
		SSHClient:        sshClient,
		OnCheckResult:    onCheckResult,
		TerraformContext: terraform.NewTerraformContext(),
		OnPhaseFunc:      switchPhase,
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

	return &pb.CommanderAttachResult{State: string(stateData), Result: string(resultString), Err: util.ErrToString(err)}
}

func (s *Service) commanderAttachServerTransitions() []fsm.Transition {
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

func (s *Service) attachSwitchPhaseData(
	completedPhase phases.OperationPhase,
	completedPhaseState phases.DhctlState,
	phaseData attach.PhaseData,
	nextPhase phases.OperationPhase,
	nextPhaseCritical bool,
) (*pb.CommanderAttachResponse, error) {
	phaseDataBytes, err := json.Marshal(phaseData)
	if err != nil {
		return nil, err
	}

	return &pb.CommanderAttachResponse{
		Message: &pb.CommanderAttachResponse_PhaseEnd{
			PhaseEnd: &pb.CommanderAttachPhaseEnd{
				CompletedPhase:      string(completedPhase),
				CompletedPhaseState: completedPhaseState,
				CompletedPhaseData:  string(phaseDataBytes),
				NextPhase:           string(nextPhase),
				NextPhaseCritical:   nextPhaseCritical,
			},
		},
	}, nil
}
