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
	"reflect"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util/callback"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type convergeParams struct {
	request      *pb.ConvergeStart
	switchPhase  phases.DefaultOnPhaseFunc
	sendProgress phases.OnProgressFunc
	logOptions   logger.Options
}

func (s *Service) Converge(server pb.DHCTL_ConvergeServer) error {
	ctx, cancel := operationCtx(server)
	defer cancel()

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.convergeServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.ConvergeRequest)
	sendCh := make(chan *pb.ConvergeResponse)
	phaseSwitcher := &fsmPhaseSwitcher[*pb.ConvergeResponse, any]{
		f: f, dataFunc: s.convergeSwitchPhaseData, sendCh: sendCh, next: make(chan error),
	}
	pt := progressTracker[*pb.ConvergeResponse]{
		sendCh: sendCh,
		dataFunc: func(progress phases.Progress) *pb.ConvergeResponse {
			return &pb.ConvergeResponse{Message: &pb.ConvergeResponse_Progress{Progress: convertProgress(progress)}}
		},
	}

	loggerDefault := logger.L(ctx).With(logTypeDHCTL)

	logWriter := logger.NewLogWriter(loggerDefault, sendCh,
		func(lines []string) *pb.ConvergeResponse {
			return &pb.ConvergeResponse{Message: &pb.ConvergeResponse_Logs{Logs: &pb.Logs{Logs: lines}}}
		},
	)

	debugWriter := logger.NewDebugLogWriter(loggerDefault)

	logOptions := logger.Options{
		DebugWriter:   debugWriter,
		DefaultWriter: logWriter,
	}

	startReceiver[*pb.ConvergeRequest, *pb.ConvergeResponse](server, receiveCh, doneCh, internalErrCh)
	startSender[*pb.ConvergeRequest, *pb.ConvergeResponse](server, sendCh, internalErrCh)

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
				"processing ConvergeRequest",
				slog.String("message", fmt.Sprintf("%T", request.Message)),
			)
			switch message := request.Message.(type) {
			case *pb.ConvergeRequest_Start:
				err := f.Event("start")
				if err != nil {
					logger.L(ctx).Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				go func() {
					result := s.convergeSafe(ctx, convergeParams{
						request:      message.Start,
						switchPhase:  phaseSwitcher.switchPhase(ctx),
						sendProgress: pt.sendProgress(),
						logOptions:   logOptions,
					})
					sendCh <- &pb.ConvergeResponse{Message: &pb.ConvergeResponse_Result{Result: result}}
				}()

			case *pb.ConvergeRequest_Continue:
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

			case *pb.ConvergeRequest_Cancel:
				cancel()

			default:
				logger.L(ctx).Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue connectionProcessor
			}
		}
	}
}

func (s *Service) convergeSafe(ctx context.Context, p convergeParams) (result *pb.ConvergeResult) {
	defer func() {
		if r := recover(); r != nil {
			lastState, err := panicResult(ctx, r)
			result = &pb.ConvergeResult{State: string(lastState), Err: err.Error()}
		}
	}()

	return s.converge(ctx, p)
}

func (s *Service) converge(ctx context.Context, p convergeParams) *pb.ConvergeResult {
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
			input.CombineYAMLs(p.request.ClusterConfig, p.request.ProviderSpecificClusterConfig),
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(loggerFor),
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
		return &pb.ConvergeResult{Err: err.Error()}
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
		return &pb.ConvergeResult{Err: err.Error()}
	}

	tmpDir := s.params.TmpDir

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           tmpDir,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		Logger:           loggerFor,
		IsDebug:          s.params.IsDebug,
	})

	infrastructureContext := infrastructure.NewContextWithProvider(providerGetter, loggerFor)

	var commanderUUID uuid.UUID
	if p.request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(p.request.Options.CommanderUuid)
		if err != nil {
			return &pb.ConvergeResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	checkParams := &check.Params{
		StateCache:    cache.Global(),
		CommanderMode: p.request.Options.CommanderMode,
		CommanderUUID: commanderUUID,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(p.request.ClusterConfig),
			[]byte(p.request.ProviderSpecificClusterConfig),
		),
		IsDebug:               s.params.IsDebug,
		TmpDir:                tmpDir,
		Logger:                loggerFor,
		InfrastructureContext: infrastructureContext,
	}

	convergeParams := &converge.Params{
		OnPhaseFunc:    p.switchPhase,
		OnProgressFunc: p.sendProgress,
		ChangesSettings: infrastructure.ChangeActionSettings{
			AutomaticSettings: infrastructure.AutomaticSettings{
				AutoDismissDestructive: false,
				AutoDismissChanges:     false,
				AutoApproveSettings: infrastructure.AutoApproveSettings{
					AutoApprove: true,
				},
			},
			SkipChangesOnDeny: false,
		},
		CommanderMode: true,
		CommanderUUID: commanderUUID,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(p.request.ClusterConfig),
			[]byte(p.request.ProviderSpecificClusterConfig),
		),
		InfrastructureContext:      infrastructureContext,
		ApproveDestructiveChangeID: p.request.ApproveDestructionChangeId,
		OnCheckResult:              onCheckResult,
		ProviderGetter:             providerGetter,
		TmpDir:                     tmpDir,
		Logger:                     loggerFor,
		IsDebug:                    s.params.IsDebug,
	}

	kubeClient, sshClient, cleanup, err := helper.InitializeClusterConnections(ctx, helper.ClusterConnectionsOptions{
		CommanderMode: p.request.Options.CommanderMode,
		ApiServerUrl:  p.request.Options.ApiServerUrl,
		ApiServerOptions: helper.ApiServerOptions{
			Token:                    p.request.Options.ApiServerToken,
			InsecureSkipTLSVerify:    p.request.Options.ApiServerInsecureSkipTlsVerify,
			CertificateAuthorityData: util.StringToBytes(p.request.Options.ApiServerCertificateAuthorityData),
		},
		SchemaStore:         s.params.SchemaStore,
		SSHConnectionConfig: p.request.ConnectionConfig,
	})
	cleanuper.Add(cleanup)
	if err != nil {
		return &pb.ConvergeResult{Err: err.Error()}
	}

	if sshClient != nil && !reflect.ValueOf(sshClient).IsNil() {
		err = sshClient.Start(ctx)
		if err != nil {
			return &pb.ConvergeResult{Err: err.Error()}
		}
	}

	checkParams.KubeClient = kubeClient
	checkParams.SSHClient = sshClient

	convergeParams.KubeClient = kubeClient
	convergeParams.SSHClient = sshClient

	checker := check.NewChecker(checkParams)
	convergeParams.Checker = checker
	converger := converge.NewConverger(convergeParams)

	result, convergeErr := converger.Converge(ctx)
	resultData, marshalResultErr := json.Marshal(result)
	state, stateErr := extractLastState()

	err = errors.Join(convergeErr, stateErr, marshalResultErr)

	return &pb.ConvergeResult{State: string(state), Result: string(resultData), Err: util.ErrToString(err)}
}

func (s *Service) convergeServerTransitions() []fsm.Transition {
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

func (s *Service) convergeSwitchPhaseData(onPhaseData phases.OnPhaseFuncData[any]) (*pb.ConvergeResponse, error) {
	return &pb.ConvergeResponse{
		Message: &pb.ConvergeResponse_PhaseEnd{
			PhaseEnd: &pb.ConvergePhaseEnd{
				CompletedPhase:      string(onPhaseData.CompletedPhase),
				CompletedPhaseState: onPhaseData.CompletedPhaseState,
				NextPhase:           string(onPhaseData.NextPhase),
				NextPhaseCritical:   onPhaseData.NextPhaseCritical,
			},
		},
	}, nil
}
