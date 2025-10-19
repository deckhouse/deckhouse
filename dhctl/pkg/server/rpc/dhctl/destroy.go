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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util/callback"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type destroyParams struct {
	request      *pb.DestroyStart
	switchPhase  phases.DefaultOnPhaseFunc
	sendProgress phases.OnProgressFunc
	logOptions   logger.Options
}

func (s *Service) Destroy(server pb.DHCTL_DestroyServer) error {
	ctx, cancel := operationCtx(server)
	defer cancel()

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.destroyServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.DestroyRequest)
	sendCh := make(chan *pb.DestroyResponse)
	phaseSwitcher := &fsmPhaseSwitcher[*pb.DestroyResponse, any]{
		f: f, dataFunc: s.destroySwitchPhaseData, sendCh: sendCh, next: make(chan error),
	}
	pt := progressTracker[*pb.DestroyResponse]{
		sendCh: sendCh,
		dataFunc: func(progress phases.Progress) *pb.DestroyResponse {
			return &pb.DestroyResponse{Message: &pb.DestroyResponse_Progress{Progress: convertProgress(progress)}}
		},
	}

	loggerDefault := logger.L(ctx).With(logTypeDHCTL)

	logWriter := logger.NewLogWriter(loggerDefault, sendCh,
		func(lines []string) *pb.DestroyResponse {
			return &pb.DestroyResponse{Message: &pb.DestroyResponse_Logs{Logs: &pb.Logs{Logs: lines}}}
		},
	)

	debugWriter := logger.NewDebugLogWriter(loggerDefault)

	logOptions := logger.Options{
		DebugWriter:   debugWriter,
		DefaultWriter: logWriter,
	}

	startReceiver[*pb.DestroyRequest, *pb.DestroyResponse](server, receiveCh, doneCh, internalErrCh)
	startSender[*pb.DestroyRequest, *pb.DestroyResponse](server, sendCh, internalErrCh)

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
				"processing DestroyRequest",
				slog.String("message", fmt.Sprintf("%T", request.Message)),
			)
			switch message := request.Message.(type) {
			case *pb.DestroyRequest_Start:
				err := f.Event("start")
				if err != nil {
					logger.L(ctx).Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				go func() {
					result := s.destroySafe(ctx, destroyParams{
						request:      message.Start,
						switchPhase:  phaseSwitcher.switchPhase(ctx),
						sendProgress: pt.sendProgress(),
						logOptions:   logOptions,
					})
					sendCh <- &pb.DestroyResponse{Message: &pb.DestroyResponse_Result{Result: result}}
				}()

			case *pb.DestroyRequest_Continue:
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

			case *pb.DestroyRequest_Cancel:
				cancel()

			default:
				logger.L(ctx).Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue connectionProcessor
			}
		}
	}
}

func (s *Service) destroySafe(ctx context.Context, p destroyParams) (result *pb.DestroyResult) {
	defer func() {
		if r := recover(); r != nil {
			lastState, err := panicResult(ctx, r)
			result = &pb.DestroyResult{State: string(lastState), Err: err.Error()}
		}
	}()

	return s.destroy(ctx, p)
}

func (s *Service) destroy(ctx context.Context, p destroyParams) *pb.DestroyResult {
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
	app.CacheDir = s.params.CacheDir
	app.ApplyPreflightSkips(p.request.Options.CommonOptions.SkipPreflightChecks)

	loggerFor.LogInfoF("Task is running by DHCTL Server pod/%s\n", s.params.PodName)
	defer func() { loggerFor.LogInfoF("Task done by DHCTL Server pod/%s\n", s.params.PodName) }()

	var metaConfig *config.MetaConfig
	err = loggerFor.LogProcess("default", "Parsing cluster config", func() error {
		metaConfig, err = config.ParseConfigFromData(
			ctx,
			input.CombineYAMLs(p.request.ClusterConfig, p.request.InitConfig, p.request.ProviderSpecificClusterConfig),
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(log.GetDefaultLogger()),
			),
			config.ValidateOptionCommanderMode(p.request.Options.CommanderMode),
			config.ValidateOptionStrictUnmarshal(p.request.Options.CommanderMode),
			config.ValidateOptionValidateExtensions(p.request.Options.CommanderMode),
		)
		if err != nil {
			return fmt.Errorf("parsing cluster meta config: %w", err)
		}
		return nil
	})
	if err != nil {
		return &pb.DestroyResult{Err: err.Error()}
	}

	err = loggerFor.LogProcess("default", "Preparing DHCTL state", func() error {
		cachePath := metaConfig.CachePath()
		var initialState phases.DhctlState
		if p.request.State != "" {
			err = json.Unmarshal([]byte(p.request.State), &initialState)
			if err != nil {
				return fmt.Errorf("unmarshalling dhctl state: %w", err)
			}
		}
		err = cache.InitWithOptions(
			cachePath,
			cache.CacheOptions{InitialState: initialState, ResetInitialState: true},
		)
		if err != nil {
			return fmt.Errorf("initializing cache at %s: %w", cachePath, err)
		}
		return nil
	})
	if err != nil {
		return &pb.DestroyResult{Err: err.Error()}
	}

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

		return nil
	})
	if err != nil {
		return &pb.DestroyResult{Err: err.Error()}
	}

	var commanderUUID uuid.UUID
	if p.request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(p.request.Options.CommanderUuid)
		if err != nil {
			return &pb.DestroyResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	destroyer, err := destroy.NewClusterDestroyer(ctx, &destroy.Params{
		NodeInterface:  ssh.NewNodeInterfaceWrapper(sshClient),
		StateCache:     cache.Global(),
		OnPhaseFunc:    p.switchPhase,
		OnProgressFunc: p.sendProgress,
		CommanderMode:  true,
		CommanderUUID:  commanderUUID,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(p.request.ClusterConfig),
			[]byte(p.request.ProviderSpecificClusterConfig),
		),
		TmpDir:  s.params.TmpDir,
		Logger:  loggerFor,
		IsDebug: s.params.IsDebug,
	})
	if err != nil {
		return &pb.DestroyResult{Err: fmt.Errorf("unable to initialize cluster destroyer: %w", err).Error()}
	}

	destroyErr := destroyer.DestroyCluster(ctx, true)
	state, stateErr := extractLastState()

	err = errors.Join(destroyErr, stateErr)

	return &pb.DestroyResult{State: string(state), Err: util.ErrToString(err)}
}

func (s *Service) destroyServerTransitions() []fsm.Transition {
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

func (s *Service) destroySwitchPhaseData(onPhaseData phases.OnPhaseFuncData[any]) (*pb.DestroyResponse, error) {
	return &pb.DestroyResponse{
		Message: &pb.DestroyResponse_PhaseEnd{
			PhaseEnd: &pb.DestroyPhaseEnd{
				CompletedPhase:      string(onPhaseData.CompletedPhase),
				CompletedPhaseState: onPhaseData.CompletedPhaseState,
				NextPhase:           string(onPhaseData.NextPhase),
				NextPhaseCritical:   onPhaseData.NextPhaseCritical,
			},
		},
	}, nil
}
