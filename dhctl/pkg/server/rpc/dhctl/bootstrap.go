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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/pointer"
)

func (s *Service) Bootstrap(server pb.DHCTL_BootstrapServer) error {
	ctx := operationCtx(server)

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.bootstrapServerTransitions())

	internalErrCh := make(chan error)
	doneCh := make(chan struct{})
	requestsCh := make(chan *pb.BootstrapRequest)

	phaseSwitcher := &bootstrapPhaseSwitcher{
		server: server,
		f:      f,
		next:   make(chan error),
	}
	defer close(phaseSwitcher.next)

	s.startBootstrapperReceiver(server, requestsCh, internalErrCh)

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
				s.startBootstrap(
					ctx, server, message.Start, phaseSwitcher, &bootstrapLogWriter{l: logger.L(ctx), server: server},
					internalErrCh, doneCh,
				)

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

func (s *Service) startBootstrapperReceiver(
	server pb.DHCTL_BootstrapServer,
	requestsCh chan *pb.BootstrapRequest,
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

func (s *Service) startBootstrap(
	ctx context.Context,
	server pb.DHCTL_BootstrapServer,
	request *pb.BootstrapStart,
	phaseSwitcher *bootstrapPhaseSwitcher,
	logWriter *bootstrapLogWriter,
	internalErrCh chan error,
	doneCh chan struct{},
) {
	go func() {
		result := s.bootstrap(ctx, request, phaseSwitcher, logWriter)
		err := server.Send(&pb.BootstrapResponse{
			Message: &pb.BootstrapResponse_Result{
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

func (s *Service) bootstrap(
	_ context.Context,
	request *pb.BootstrapStart,
	phaseSwitcher *bootstrapPhaseSwitcher,
	logWriter io.Writer,
) *pb.BootstrapResult {
	var err error

	// set global variables from options
	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream: logWriter,
		Width:     int(request.Options.LogWidth),
	})
	app.SanityCheck = true
	app.UseTfCache = app.UseStateCacheYes
	app.PreflightSkipDeckhouseVersionCheck = true
	app.PreflightSkipAll = true
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
		return &pb.BootstrapResult{Err: err.Error()}
	}
	defer sshClient.Stop()

	bootstrapper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
		SSHClient:                  sshClient,
		InitialState:               initialState,
		ResetInitialState:          true,
		DisableBootstrapClearCache: true,
		OnPhaseFunc:                phaseSwitcher.switchPhase,
		CommanderMode:              request.Options.CommanderMode,
		TerraformContext:           terraform.NewTerraformContext(),
		ConfigPath:                 configPath,
		ResourcesPath:              resourcesPath,
		ResourcesTimeout:           request.Options.ResourcesTimeout.AsDuration(),
		DeckhouseTimeout:           request.Options.DeckhouseTimeout.AsDuration(),
		PostBootstrapScriptPath:    postBootstrapScriptPath,
		UseTfCache:                 pointer.Bool(true),
		AutoApprove:                pointer.Bool(true),
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

type bootstrapLogWriter struct {
	l      *slog.Logger
	server pb.DHCTL_BootstrapServer
}

func (w *bootstrapLogWriter) Write(p []byte) (int, error) {
	w.l.Info(string(p), logTypeDHCTL)

	err := w.server.Send(&pb.BootstrapResponse{
		Message: &pb.BootstrapResponse_Logs{
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

type bootstrapPhaseSwitcher struct {
	server pb.DHCTL_BootstrapServer
	f      *fsm.FiniteStateMachine
	next   chan error
}

func (b *bootstrapPhaseSwitcher) switchPhase(
	completedPhase phases.OperationPhase,
	completedPhaseState phases.DhctlState,
	_ interface{},
	nextPhase phases.OperationPhase,
	nextPhaseCritical bool,
) error {
	err := b.f.Event("wait")
	if err != nil {
		return fmt.Errorf("changing state to waiting: %w", err)
	}

	err = b.server.Send(&pb.BootstrapResponse{
		Message: &pb.BootstrapResponse_PhaseEnd{
			PhaseEnd: &pb.BootstrapPhaseEnd{
				CompletedPhase:      string(completedPhase),
				CompletedPhaseState: completedPhaseState,
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
