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
	"os"

	"github.com/google/uuid"
	"github.com/name212/govalue"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util/callback"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
)

type bootstrapParams struct {
	request      *pb.BootstrapStart
	switchPhase  phases.DefaultOnPhaseFunc
	sendProgress phases.OnProgressFunc
	logOptions   logger.Options
}

func (s *Service) Bootstrap(server pb.DHCTL_BootstrapServer) error {
	ctx, cancel := operationCtx(server)
	defer cancel()

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.bootstrapServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.BootstrapRequest)
	sendCh := make(chan *pb.BootstrapResponse)
	phaseSwitcher := &fsmPhaseSwitcher[*pb.BootstrapResponse, any]{
		f: f, dataFunc: s.bootstrapSwitchPhaseData, sendCh: sendCh, next: make(chan error),
	}
	pt := progressTracker[*pb.BootstrapResponse]{
		sendCh: sendCh,
		dataFunc: func(progress phases.Progress) *pb.BootstrapResponse {
			return &pb.BootstrapResponse{Message: &pb.BootstrapResponse_Progress{Progress: convertProgress(progress)}}
		},
	}

	loggerDefault := logger.L(ctx).With(logTypeDHCTL)

	logWriter := logger.NewLogWriter(loggerDefault, sendCh,
		func(lines []string) *pb.BootstrapResponse {
			return &pb.BootstrapResponse{Message: &pb.BootstrapResponse_Logs{Logs: &pb.Logs{Logs: lines}}}
		},
	)

	debugWriter := logger.NewDebugLogWriter(loggerDefault)

	logOptions := logger.Options{
		DebugWriter:   debugWriter,
		DefaultWriter: logWriter,
	}

	startReceiver[*pb.BootstrapRequest, *pb.BootstrapResponse](server, receiveCh, doneCh, internalErrCh)
	startSender[*pb.BootstrapRequest, *pb.BootstrapResponse](server, sendCh, internalErrCh)

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
				"processing BootstrapRequest",
				slog.String("message", fmt.Sprintf("%T", request.Message)),
			)
			switch message := request.Message.(type) {
			case *pb.BootstrapRequest_Start:
				err := f.Event("start")
				if err != nil {
					logger.L(ctx).Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				go func() {
					result := s.bootstrapSafe(ctx, bootstrapParams{
						request:      message.Start,
						switchPhase:  phaseSwitcher.switchPhase(ctx),
						sendProgress: pt.sendProgress(),
						logOptions:   logOptions,
					})
					sendCh <- &pb.BootstrapResponse{Message: &pb.BootstrapResponse_Result{Result: result}}
				}()

			case *pb.BootstrapRequest_Continue:
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

			case *pb.BootstrapRequest_Cancel:
				cancel()

			default:
				logger.L(ctx).Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue connectionProcessor
			}
		}
	}
}

func (s *Service) bootstrapSafe(ctx context.Context, p bootstrapParams) (result *pb.BootstrapResult) {
	defer func() {
		if r := recover(); r != nil {
			lastState, err := panicResult(ctx, r)
			result = &pb.BootstrapResult{State: string(lastState), Err: err.Error()}
		}
	}()

	return s.bootstrap(ctx, p)
}

func (s *Service) bootstrap(ctx context.Context, p bootstrapParams) *pb.BootstrapResult {
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
	app.CacheDir = s.params.CacheDir
	app.ApplyPreflightSkips(p.request.Options.CommonOptions.SkipPreflightChecks)

	logBeforeExit := logInformationAboutInstance(s.params, loggerFor)
	defer logBeforeExit()

	var (
		configPaths             []string
		configPath              string
		postBootstrapScriptPath string
		cleanup                 func() error
	)
	err = loggerFor.LogProcess("default", "Preparing configuration", func() error {
		for _, cfg := range []string{
			p.request.ClusterConfig,
			p.request.InitConfig,
			p.request.ProviderSpecificClusterConfig,
			p.request.InitResources,
			p.request.Resources,
		} {
			if len(cfg) == 0 {
				continue
			}

			configPath, cleanup, err = util.WriteDefaultTempFile([]byte(cfg))
			cleanuper.Add(cleanup)
			if err != nil {
				return fmt.Errorf("failed to write configuration: %w", err)
			}

			configPaths = append(configPaths, configPath)
		}

		postBootstrapScriptPath, cleanup, err = util.WriteDefaultTempFile([]byte(p.request.PostBootstrapScript))
		cleanuper.Add(cleanup)
		if err != nil {
			return fmt.Errorf("failed to write post bootstrap script: %w", err)
		}
		postBootstrapScript, err := os.Open(postBootstrapScriptPath)
		if err != nil {
			return fmt.Errorf("failed to open post bootstrap script: %w", err)
		}
		err = postBootstrapScript.Chmod(0555)
		if err != nil {
			return fmt.Errorf("failed to chmod post bootstrap script: %w", err)
		}

		return nil
	})
	if err != nil {
		return &pb.BootstrapResult{Err: err.Error()}
	}

	var initialState phases.DhctlState
	err = loggerFor.LogProcess("default", "Preparing DHCTL state", func() error {
		if p.request.State != "" {
			err = json.Unmarshal([]byte(p.request.State), &initialState)
			if err != nil {
				return fmt.Errorf("unmarshalling dhctl state: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return &pb.BootstrapResult{Err: err.Error()}
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

		sshClient, cleanup, err = helper.CreateSSHClient(connectionConfig)
		cleanuper.Add(cleanup)
		if err != nil {
			return fmt.Errorf("preparing ssh client: %w", err)
		}

		if !govalue.IsNil(sshClient) && len(connectionConfig.SSHHosts) > 0 {
			err = sshClient.Start()
			if err != nil {
				return fmt.Errorf("cannot start sshClient: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return &pb.BootstrapResult{Err: err.Error()}
	}

	var commanderUUID uuid.UUID
	if p.request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(p.request.Options.CommanderUuid)
		if err != nil {
			return &pb.BootstrapResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	bootstrapper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
		NodeInterface:              ssh.NewNodeInterfaceWrapper(sshClient),
		InitialState:               initialState,
		ResetInitialState:          true,
		DisableBootstrapClearCache: false,
		OnPhaseFunc:                p.switchPhase,
		OnProgressFunc:             p.sendProgress,
		CommanderMode:              p.request.Options.CommanderMode,
		CommanderUUID:              commanderUUID,
		ConfigPaths:                configPaths,
		ResourcesTimeout:           p.request.Options.ResourcesTimeout.AsDuration(),
		DeckhouseTimeout:           p.request.Options.DeckhouseTimeout.AsDuration(),
		PostBootstrapScriptPath:    postBootstrapScriptPath,
		UseTfCache:                 ptr.To(true),
		AutoApprove:                ptr.To(true),
		KubernetesInitParams:       nil,
		TmpDir:                     s.params.TmpDir,
		Logger:                     loggerFor,
		IsDebug:                    s.params.IsDebug,
	})

	bootstrapErr := bootstrapper.Bootstrap(ctx)
	state, stateErr := extractLastState()
	err = errors.Join(bootstrapErr, stateErr)

	return &pb.BootstrapResult{State: string(state), Err: util.ErrToString(err)}
}

func (s *Service) bootstrapServerTransitions() []fsm.Transition {
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

func (s *Service) bootstrapSwitchPhaseData(onPhaseData phases.OnPhaseFuncData[any]) (*pb.BootstrapResponse, error) {
	return &pb.BootstrapResponse{
		Message: &pb.BootstrapResponse_PhaseEnd{
			PhaseEnd: &pb.BootstrapPhaseEnd{
				CompletedPhase:      string(onPhaseData.CompletedPhase),
				CompletedPhaseState: onPhaseData.CompletedPhaseState,
				NextPhase:           string(onPhaseData.NextPhase),
				NextPhaseCritical:   onPhaseData.NextPhaseCritical,
			},
		},
	}, nil
}
