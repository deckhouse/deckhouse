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
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

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
	logWriter := logger.NewLogWriter(logger.L(ctx).With(logTypeDHCTL), sendCh,
		func(lines []string) *pb.ConvergeResponse {
			return &pb.ConvergeResponse{Message: &pb.ConvergeResponse_Logs{Logs: &pb.Logs{Logs: lines}}}
		},
	)

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
					result := s.convergeSafe(ctx, message.Start, phaseSwitcher.switchPhase(ctx), logWriter)
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

func (s *Service) convergeSafe(
	ctx context.Context,
	request *pb.ConvergeStart,
	switchPhase phases.DefaultOnPhaseFunc,
	logWriter io.Writer,
) (result *pb.ConvergeResult) {
	defer func() {
		if r := recover(); r != nil {
			result = &pb.ConvergeResult{Err: panicMessage(ctx, r)}
		}
	}()

	return s.converge(ctx, request, switchPhase, logWriter)
}

func (s *Service) converge(
	ctx context.Context,
	request *pb.ConvergeStart,
	switchPhase phases.DefaultOnPhaseFunc,
	logWriter io.Writer,
) *pb.ConvergeResult {
	var err error

	cleanuper := callback.NewCallback()
	defer func() { _ = cleanuper.Call() }()

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
	defer func() { log.InfoF("Task done by DHCTL Server pod/%s\n", s.podName) }()

	var metaConfig *config.MetaConfig
	err = log.Process("default", "Parsing cluster config", func() error {
		metaConfig, err = config.ParseConfigFromData(
			input.CombineYAMLs(request.ClusterConfig, request.ProviderSpecificClusterConfig),
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
		return &pb.ConvergeResult{Err: err.Error()}
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
		return &pb.ConvergeResult{Err: err.Error()}
	}

	terraformContext := terraform.NewTerraformContext()

	var commanderUUID uuid.UUID
	if request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(request.Options.CommanderUuid)
		if err != nil {
			return &pb.ConvergeResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	checkParams := &check.Params{
		StateCache:    cache.Global(),
		CommanderMode: request.Options.CommanderMode,
		CommanderUUID: commanderUUID,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(request.ClusterConfig),
			[]byte(request.ProviderSpecificClusterConfig),
		),
		TerraformContext: terraform.NewTerraformContext(),
	}

	convergeParams := &converge.Params{
		OnPhaseFunc:            switchPhase,
		AutoApprove:            true,
		AutoDismissDestructive: false,
		CommanderMode:          true,
		CommanderUUID:          commanderUUID,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(request.ClusterConfig),
			[]byte(request.ProviderSpecificClusterConfig),
		),
		TerraformContext:           terraformContext,
		ApproveDestructiveChangeID: request.ApproveDestructionChangeId,
		OnCheckResult:              onCheckResult,
	}

	kubeClient, sshClient, cleanup, err := helper.InitializeClusterConnections(ctx, helper.ClusterConnectionsOptions{
		CommanderMode: request.Options.CommanderMode,
		ApiServerUrl:  request.Options.ApiServerUrl,
		ApiServerOptions: helper.ApiServerOptions{
			Token:                    request.Options.ApiServerToken,
			InsecureSkipTLSVerify:    request.Options.ApiServerInsecureSkipTlsVerify,
			CertificateAuthorityData: util.StringToBytes(request.Options.ApiServerCertificateAuthorityData),
		},
		SchemaStore:         s.schemaStore,
		SSHConnectionConfig: request.ConnectionConfig,
	})
	cleanuper.Add(cleanup)
	if err != nil {
		return &pb.ConvergeResult{Err: err.Error()}
	}

	checkParams.KubeClient = kubeClient
	checkParams.SSHClient = sshClient
	convergeParams.KubeClient = kubeClient
	convergeParams.SSHClient = sshClient

	checker := check.NewChecker(checkParams)
	convergeParams.Checker = checker
	converger := converge.NewConverger(convergeParams)

	result, convergeErr := converger.Converge(ctx)
	state := converger.GetLastState()
	stateData, marshalStateErr := json.Marshal(state)
	resultString, marshalResultErr := json.Marshal(result)
	err = errors.Join(convergeErr, marshalStateErr, marshalResultErr)

	return &pb.ConvergeResult{State: string(stateData), Result: string(resultString), Err: util.ErrToString(err)}
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

func (s *Service) convergeSwitchPhaseData(
	completedPhase phases.OperationPhase,
	completedPhaseState phases.DhctlState,
	_ any,
	nextPhase phases.OperationPhase,
	nextPhaseCritical bool,
) (*pb.ConvergeResponse, error) {
	return &pb.ConvergeResponse{
		Message: &pb.ConvergeResponse_PhaseEnd{
			PhaseEnd: &pb.ConvergePhaseEnd{
				CompletedPhase:      string(completedPhase),
				CompletedPhaseState: completedPhaseState,
				NextPhase:           string(nextPhase),
				NextPhaseCritical:   nextPhaseCritical,
			},
		},
	}, nil
}
