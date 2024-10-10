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
	"os"

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
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func (s *Service) Bootstrap(server pb.DHCTL_BootstrapServer) error {
	ctx := operationCtx(server)

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.bootstrapServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.BootstrapRequest)
	sendCh := make(chan *pb.BootstrapResponse)
	phaseSwitcher := &fsmPhaseSwitcher[*pb.BootstrapResponse, any]{
		f: f, dataFunc: s.bootstrapSwitchPhaseData, sendCh: sendCh, next: make(chan error),
	}
	logWriter := logger.NewLogWriter(logger.L(ctx).With(logTypeDHCTL), sendCh,
		func(lines []string) *pb.BootstrapResponse {
			return &pb.BootstrapResponse{Message: &pb.BootstrapResponse_Logs{Logs: &pb.Logs{Logs: lines}}}
		},
	)

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
					result := s.bootstrap(ctx, message.Start, phaseSwitcher.switchPhase, logWriter)
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

			default:
				logger.L(ctx).Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue connectionProcessor
			}
		}
	}
}

func (s *Service) bootstrap(
	_ context.Context,
	request *pb.BootstrapStart,
	switchPhase phases.DefaultOnPhaseFunc,
	logWriter io.Writer,
) *pb.BootstrapResult {
	var err error

	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream: logWriter,
		Width:     int(request.Options.LogWidth),
	})
	app.SanityCheck = true
	app.UseTfCache = app.UseStateCacheYes
	app.CacheDir = s.cacheDir

	log.InfoF("Task is running by DHCTL Server pod/%s\n", s.podName)
	defer func() {
		log.InfoF("Task done by DHCTL Server pod/%s\n", s.podName)
	}()

	var (
		configPath              string
		resourcesPath           string
		postBootstrapScriptPath string
	)
	err = log.Process("default", "Preparing configuration", func() error {
		configPath, err = writeTempFile([]byte(input.CombineYAMLs(
			request.ClusterConfig, request.InitConfig, request.ProviderSpecificClusterConfig,
		)))
		if err != nil {
			return fmt.Errorf("failed to write init configuration: %w", err)
		}

		resourcesPath, err = writeTempFile([]byte(input.CombineYAMLs(
			request.InitResources, request.Resources,
		)))
		if err != nil {
			return fmt.Errorf("failed to write resources: %w", err)
		}

		postBootstrapScriptPath, err = writeTempFile([]byte(request.PostBootstrapScript))
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
		return &pb.BootstrapResult{Err: err.Error()}
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

		sshClient, err = prepareSSHClient(connectionConfig)
		if err != nil {
			return fmt.Errorf("preparing ssh client: %w", err)
		}
		return nil
	})
	if err != nil {
		return &pb.BootstrapResult{Err: err.Error()}
	}
	defer sshClient.Stop()

	var commanderUUID uuid.UUID
	if request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(request.Options.CommanderUuid)
		if err != nil {
			return &pb.BootstrapResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	bootstrapper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
		NodeInterface:              ssh.NewNodeInterfaceWrapper(sshClient),
		InitialState:               initialState,
		ResetInitialState:          true,
		DisableBootstrapClearCache: true,
		OnPhaseFunc:                switchPhase,
		CommanderMode:              request.Options.CommanderMode,
		CommanderUUID:              commanderUUID,
		TerraformContext:           terraform.NewTerraformContext(),
		ConfigPaths:                []string{configPath},
		ResourcesPath:              resourcesPath,
		ResourcesTimeout:           request.Options.ResourcesTimeout.AsDuration(),
		DeckhouseTimeout:           request.Options.DeckhouseTimeout.AsDuration(),
		PostBootstrapScriptPath:    postBootstrapScriptPath,
		UseTfCache:                 ptr.To(true),
		AutoApprove:                ptr.To(true),
		KubernetesInitParams:       nil,
	})

	bootstrapErr := bootstrapper.Bootstrap()
	state := bootstrapper.GetLastState()
	stateData, marshalErr := json.Marshal(state)
	err = errors.Join(bootstrapErr, marshalErr)

	return &pb.BootstrapResult{State: string(stateData), Err: errToString(err)}
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

func (s *Service) bootstrapSwitchPhaseData(
	completedPhase phases.OperationPhase,
	completedPhaseState phases.DhctlState,
	_ any,
	nextPhase phases.OperationPhase,
	nextPhaseCritical bool,
) (*pb.BootstrapResponse, error) {
	return &pb.BootstrapResponse{
		Message: &pb.BootstrapResponse_PhaseEnd{
			PhaseEnd: &pb.BootstrapPhaseEnd{
				CompletedPhase:      string(completedPhase),
				CompletedPhaseState: completedPhaseState,
				NextPhase:           string(nextPhase),
				NextPhaseCritical:   nextPhaseCritical,
			},
		},
	}, nil
}
