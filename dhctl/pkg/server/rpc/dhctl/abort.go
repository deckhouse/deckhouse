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
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func (s *Service) Abort(server pb.DHCTL_AbortServer) error {
	ctx := operationCtx(server)

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.abortServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.AbortRequest)
	sendCh := make(chan *pb.AbortResponse)
	phaseSwitcher := &fsmPhaseSwitcher[*pb.AbortResponse, any]{
		f: f, dataFunc: s.abortSwitchPhaseData, sendCh: sendCh, next: make(chan error),
	}
	logWriter := logger.NewLogWriter(logger.L(ctx).With(logTypeDHCTL), sendCh,
		func(lines []string) *pb.AbortResponse {
			return &pb.AbortResponse{Message: &pb.AbortResponse_Logs{Logs: &pb.Logs{Logs: lines}}}
		},
	)

	startReceiver[*pb.AbortRequest, *pb.AbortResponse](server, receiveCh, doneCh, internalErrCh)
	startSender[*pb.AbortRequest, *pb.AbortResponse](server, sendCh, internalErrCh)

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
				"processing AbortRequest",
				slog.String("message", fmt.Sprintf("%T", request.Message)),
			)
			switch message := request.Message.(type) {
			case *pb.AbortRequest_Start:
				err := f.Event("start")
				if err != nil {
					logger.L(ctx).Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				go func() {
					result := s.abort(ctx, message.Start, phaseSwitcher.switchPhase, logWriter)
					sendCh <- &pb.AbortResponse{Message: &pb.AbortResponse_Result{Result: result}}
				}()

			case *pb.AbortRequest_Continue:
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

func (s *Service) abort(
	_ context.Context,
	request *pb.AbortStart,
	switchPhase phases.DefaultOnPhaseFunc,
	logWriter io.Writer,
) *pb.AbortResult {
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

	var (
		configPath    string
		resourcesPath string
		cleanup       func() error
	)
	err = log.Process("default", "Preparing configuration", func() error {
		configPath, cleanup, err = util.WriteDefaultTempFile([]byte(input.CombineYAMLs(
			request.ClusterConfig, request.InitConfig, request.ProviderSpecificClusterConfig,
		)))
		cleanuper.Add(cleanup)
		if err != nil {
			return fmt.Errorf("failed to write init configuration: %w", err)
		}

		resourcesPath, cleanup, err = util.WriteDefaultTempFile([]byte(input.CombineYAMLs(
			request.InitResources, request.Resources,
		)))
		cleanuper.Add(cleanup)
		if err != nil {
			return fmt.Errorf("failed to write resources: %w", err)
		}

		return nil
	})
	if err != nil {
		return &pb.AbortResult{Err: err.Error()}
	}

	var initialState phases.DhctlState
	err = log.Process("default", "Preparing DHCTL state", func() error {
		if request.State != "" {
			err = json.Unmarshal([]byte(request.State), &initialState)
			if err != nil {
				return fmt.Errorf("unmarshalling dhctl state: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return &pb.AbortResult{Err: err.Error()}
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

		sshClient, cleanup, err = helper.CreateSSHClient(connectionConfig)
		cleanuper.Add(cleanup)
		if err != nil {
			return fmt.Errorf("preparing ssh client: %w", err)
		}
		return nil
	})
	if err != nil {
		return &pb.AbortResult{Err: err.Error()}
	}

	var commanderUUID uuid.UUID
	if request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(request.Options.CommanderUuid)
		if err != nil {
			return &pb.AbortResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	bootstrapper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
		ConfigPaths:      []string{configPath},
		ResourcesPath:    resourcesPath,
		InitialState:     initialState,
		NodeInterface:    ssh.NewNodeInterfaceWrapper(sshClient),
		UseTfCache:       ptr.To(true),
		AutoApprove:      ptr.To(true),
		ResourcesTimeout: request.Options.ResourcesTimeout.AsDuration(),
		DeckhouseTimeout: request.Options.DeckhouseTimeout.AsDuration(),

		ResetInitialState: true,
		OnPhaseFunc:       switchPhase,
		CommanderMode:     request.Options.CommanderMode,
		CommanderUUID:     commanderUUID,
		TerraformContext:  terraform.NewTerraformContext(),
	})

	abortErr := bootstrapper.Abort(false)
	state := bootstrapper.GetLastState()
	stateData, marshalErr := json.Marshal(state)
	err = errors.Join(abortErr, marshalErr)

	return &pb.AbortResult{State: string(stateData), Err: util.ErrToString(err)}
}

func (s *Service) abortServerTransitions() []fsm.Transition {
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

func (s *Service) abortSwitchPhaseData(
	completedPhase phases.OperationPhase,
	completedPhaseState phases.DhctlState,
	_ any,
	nextPhase phases.OperationPhase,
	nextPhaseCritical bool,
) (*pb.AbortResponse, error) {
	return &pb.AbortResponse{
		Message: &pb.AbortResponse_PhaseEnd{
			PhaseEnd: &pb.AbortPhaseEnd{
				CompletedPhase:      string(completedPhase),
				CompletedPhaseState: completedPhaseState,
				NextPhase:           string(nextPhase),
				NextPhaseCritical:   nextPhaseCritical,
			},
		},
	}, nil
}
