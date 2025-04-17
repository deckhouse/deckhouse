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
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

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
					result := s.destroySafe(ctx, message.Start, phaseSwitcher.switchPhase(ctx), logOptions)
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

func (s *Service) destroySafe(ctx context.Context, request *pb.DestroyStart, switchPhase phases.DefaultOnPhaseFunc, options logger.Options) (result *pb.DestroyResult) {
	defer func() {
		if r := recover(); r != nil {
			result = &pb.DestroyResult{Err: panicMessage(ctx, r)}
		}
	}()

	return s.destroy(ctx, request, switchPhase, options)
}

func (s *Service) destroy(ctx context.Context, request *pb.DestroyStart, switchPhase phases.DefaultOnPhaseFunc, options logger.Options) *pb.DestroyResult {
	var err error

	cleanuper := callback.NewCallback()
	defer func() { _ = cleanuper.Call() }()

	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream:   options.DefaultWriter,
		Width:       int(request.Options.LogWidth),
		DebugStream: options.DebugWriter,
	})

	app.SanityCheck = true
	app.UseTfCache = app.UseStateCacheYes
	app.ResourcesTimeout = request.Options.ResourcesTimeout.AsDuration()
	app.DeckhouseTimeout = request.Options.DeckhouseTimeout.AsDuration()
	app.CacheDir = s.cacheDir
	app.ApplyPreflightSkips(request.Options.CommonOptions.SkipPreflightChecks)

	log.InfoF("Task is running by DHCTL Server pod/%s\n", s.podName)
	defer func() { log.InfoF("Task done by DHCTL Server pod/%s\n", s.podName) }()

	var metaConfig *config.MetaConfig
	err = log.Process("default", "Parsing cluster config", func() error {
		metaConfig, err = config.ParseConfigFromData(
			input.CombineYAMLs(request.ClusterConfig, request.InitConfig, request.ProviderSpecificClusterConfig),
			config.ValidateOptionCommanderMode(request.Options.CommanderMode),
			config.ValidateOptionStrictUnmarshal(request.Options.CommanderMode),
			config.ValidateOptionValidateExtensions(request.Options.CommanderMode),
		)
		if err != nil {
			return fmt.Errorf("parsing cluster meta config: %w", err)
		}
		return nil
	})
	if err != nil {
		return &pb.DestroyResult{Err: err.Error()}
	}

	err = log.Process("default", "Preparing DHCTL state", func() error {
		cachePath := metaConfig.CachePath()
		var initialState phases.DhctlState
		if request.State != "" {
			err = json.Unmarshal([]byte(request.State), &initialState)
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
		return &pb.DestroyResult{Err: err.Error()}
	}

	var commanderUUID uuid.UUID
	if request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(request.Options.CommanderUuid)
		if err != nil {
			return &pb.DestroyResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	destroyer, err := destroy.NewClusterDestroyer(&destroy.Params{
		NodeInterface: ssh.NewNodeInterfaceWrapper(sshClient),
		StateCache:    cache.Global(),
		OnPhaseFunc:   switchPhase,
		CommanderMode: true,
		CommanderUUID: commanderUUID,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(request.ClusterConfig),
			[]byte(request.ProviderSpecificClusterConfig),
		),
	})
	if err != nil {
		return &pb.DestroyResult{Err: fmt.Errorf("unable to initialize cluster destroyer: %w", err).Error()}
	}

	destroyErr := destroyer.DestroyCluster(ctx, true)
	state := destroyer.PhasedExecutionContext.GetLastState()
	data, marshalErr := json.Marshal(state)
	err = errors.Join(destroyErr, marshalErr)

	return &pb.DestroyResult{State: string(data), Err: util.ErrToString(err)}
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

func (s *Service) destroySwitchPhaseData(
	completedPhase phases.OperationPhase,
	completedPhaseState phases.DhctlState,
	_ any,
	nextPhase phases.OperationPhase,
	nextPhaseCritical bool,
) (*pb.DestroyResponse, error) {
	return &pb.DestroyResponse{
		Message: &pb.DestroyResponse_PhaseEnd{
			PhaseEnd: &pb.DestroyPhaseEnd{
				CompletedPhase:      string(completedPhase),
				CompletedPhaseState: completedPhaseState,
				NextPhase:           string(nextPhase),
				NextPhaseCritical:   nextPhaseCritical,
			},
		},
	}, nil
}
