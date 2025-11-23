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
	"log/slog"

	"github.com/google/uuid"
	"github.com/name212/govalue"
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
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

type attachParams struct {
	request      *pb.CommanderAttachStart
	switchPhase  phases.OnPhaseFunc[attach.PhaseData]
	sendProgress phases.OnProgressFunc
	logOptions   logger.Options
}

func (s *Service) CommanderAttach(server pb.DHCTL_CommanderAttachServer) error {
	ctx, cancel := operationCtx(server)
	defer cancel()

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.commanderAttachServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.CommanderAttachRequest)
	sendCh := make(chan *pb.CommanderAttachResponse)
	phaseSwitcher := &fsmPhaseSwitcher[*pb.CommanderAttachResponse, attach.PhaseData]{
		f: f, dataFunc: s.attachSwitchPhaseData, sendCh: sendCh, next: make(chan error),
	}
	pt := progressTracker[*pb.CommanderAttachResponse]{
		sendCh: sendCh,
		dataFunc: func(progress phases.Progress) *pb.CommanderAttachResponse {
			return &pb.CommanderAttachResponse{Message: &pb.CommanderAttachResponse_Progress{Progress: convertProgress(progress)}}
		},
	}

	loggerDefault := logger.L(ctx).With(logTypeDHCTL)

	logWriter := logger.NewLogWriter(loggerDefault, sendCh,
		func(lines []string) *pb.CommanderAttachResponse {
			return &pb.CommanderAttachResponse{Message: &pb.CommanderAttachResponse_Logs{Logs: &pb.Logs{Logs: lines}}}
		},
	)

	debugWriter := logger.NewDebugLogWriter(loggerDefault)

	logOptions := logger.Options{
		DebugWriter:   debugWriter,
		DefaultWriter: logWriter,
	}

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
					result := s.commanderAttachSafe(ctx, attachParams{
						request:      message.Start,
						switchPhase:  phaseSwitcher.switchPhase(ctx),
						sendProgress: pt.sendProgress(),
						logOptions:   logOptions,
					})
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

			case *pb.CommanderAttachRequest_Cancel:
				cancel()

			default:
				logger.L(ctx).Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue connectionProcessor
			}
		}
	}
}

func (s *Service) commanderAttachSafe(ctx context.Context, p attachParams) (result *pb.CommanderAttachResult) {
	defer func() {
		if r := recover(); r != nil {
			lastState, err := panicResult(ctx, r)
			result = &pb.CommanderAttachResult{State: string(lastState), Err: err.Error()}
		}
	}()

	return s.commanderAttach(ctx, p)
}

func (s *Service) commanderAttach(ctx context.Context, p attachParams) *pb.CommanderAttachResult {
	var err error

	cleanuper := callback.NewCallback()
	defer func() { _ = cleanuper.Call() }()

	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream:   p.logOptions.DefaultWriter,
		Width:       int(p.request.Options.LogWidth),
		DebugStream: p.logOptions.DebugWriter,
	})

	loggerFor := log.GetDefaultLogger()

	app.SanityCheck = true
	app.UseTfCache = app.UseStateCacheYes
	app.ResourcesTimeout = p.request.Options.ResourcesTimeout.AsDuration()
	app.DeckhouseTimeout = p.request.Options.DeckhouseTimeout.AsDuration()
	app.SetCacheDir(s.params.CacheDir)
	app.ApplyPreflightSkips(p.request.Options.CommonOptions.SkipPreflightChecks)

	logBeforeExit := logInformationAboutInstance(s.params, loggerFor)
	defer logBeforeExit()

	var sshClient node.SSHClient
	err = loggerFor.LogProcess("default", "Preparing SSH client", func() error {
		connectionConfig, err := config.ParseConnectionConfig(
			p.request.ConnectionConfig,
			s.params.SchemaStore,
			config.ValidateOptionCommanderMode(p.request.Options.CommanderMode),
			config.ValidateOptionStrictUnmarshal(p.request.Options.CommanderMode),
			config.ValidateOptionValidateExtensions(p.request.Options.CommanderMode),
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

		if !govalue.IsNil(sshClient) {
			err = sshClient.Start()
			if err != nil {
				return fmt.Errorf("cannot start sshClient: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return &pb.CommanderAttachResult{Err: err.Error()}
	}

	var commanderUUID uuid.UUID
	if p.request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(p.request.Options.CommanderUuid)
		if err != nil {
			return &pb.CommanderAttachResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	attacher := attach.NewAttacher(&attach.Params{
		CommanderMode:  p.request.Options.CommanderMode,
		CommanderUUID:  commanderUUID,
		SSHClient:      sshClient,
		OnCheckResult:  onCheckResult,
		OnPhaseFunc:    p.switchPhase,
		OnProgressFunc: p.sendProgress,
		AttachResources: attach.AttachResources{
			Template: p.request.ResourcesTemplate,
			Values:   p.request.ResourcesValues.AsMap(),
		},
		ScanOnly: p.request.ScanOnly,
		TmpDir:   s.params.TmpDir,
		Logger:   loggerFor,
		IsDebug:  s.params.IsDebug,
	})

	result, attachErr := attacher.Attach(ctx)
	resultData, marshalResultErr := json.Marshal(result)
	state, stateErr := extractLastState()

	err = errors.Join(attachErr, stateErr, marshalResultErr)

	return &pb.CommanderAttachResult{State: string(state), Result: string(resultData), Err: util.ErrToString(err)}
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

func (s *Service) attachSwitchPhaseData(onPhaseData phases.OnPhaseFuncData[attach.PhaseData]) (*pb.CommanderAttachResponse, error) {
	phaseDataBytes, err := json.Marshal(onPhaseData.CompletedPhaseData)
	if err != nil {
		return nil, err
	}

	return &pb.CommanderAttachResponse{
		Message: &pb.CommanderAttachResponse_PhaseEnd{
			PhaseEnd: &pb.CommanderAttachPhaseEnd{
				CompletedPhase:      string(onPhaseData.CompletedPhase),
				CompletedPhaseState: onPhaseData.CompletedPhaseState,
				CompletedPhaseData:  string(phaseDataBytes),
				NextPhase:           string(onPhaseData.NextPhase),
				NextPhaseCritical:   onPhaseData.NextPhaseCritical,
			},
		},
	}, nil
}
