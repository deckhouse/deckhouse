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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/logger"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/pointer"
)

func (s *Service) Bootstrap(server pb.DHCTL_BootstrapServer) error {
	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()

	s.logb.Info("started")

	f := fsm.New("initial", s.bootstrapServerTransitions())

	dhctlErrCh := make(chan error)
	internalErrCh := make(chan error)
	doneCh := make(chan struct{})
	requestsCh := make(chan *pb.BootstrapRequest)

	phaseSwitcher := &bootstrapPhaseSwitcher{
		server: server,
		f:      f,
		next:   make(chan error),
	}
	defer close(phaseSwitcher.next)

connectionProcessor:
	for {
		select {
		case <-doneCh:
			s.logc.Info("finished normally")
			return nil

		case err := <-internalErrCh:
			s.logc.Error("finished with internal error", logger.Err(err))
			return status.Errorf(codes.Internal, "%s", err)

		case err := <-dhctlErrCh:
			sendErr := server.Send(&pb.BootstrapResponse{
				Message: &pb.BootstrapResponse_Err{
					Err: err.Error(),
				},
			})
			if sendErr != nil {
				s.logc.Error("finished with internal error", logger.Err(sendErr))
				return status.Errorf(codes.Internal, "sending message: %s", sendErr)
			}

			s.logc.Info("finished with dhctl error", logger.Err(err))
			return nil

		case request := <-requestsCh:
			switch message := request.Message.(type) {
			case *pb.BootstrapRequest_Start:
				err := f.Event("start")
				if err != nil {
					s.logb.Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				s.startBootstrap(ctx, server, message.Start, phaseSwitcher, dhctlErrCh, internalErrCh, doneCh)

			case *pb.BootstrapRequest_Stop:
				err := f.Event("stop")
				if err != nil {
					s.logb.Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				err = s.stopBootstrap(cancel, message.Stop)

			case *pb.BootstrapRequest_Continue:
				err := f.Event("toNextPhase")
				if err != nil {
					s.logb.Error("got unprocessable message",
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
				s.logb.Error("got unprocessable message",
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
			s.logc.Info(
				"processing BootstrapRequest",
				slog.String("message", fmt.Sprintf("%T", request.Message)),
			)
			requestsCh <- request
		}
	}()
}

func (s *Service) startBootstrap(
	ctx context.Context,
	server pb.DHCTL_BootstrapServer,
	request *pb.BootstrapStart,
	phaseSwitcher *bootstrapPhaseSwitcher,
	dhctlErrCh chan error,
	internalErrCh chan error,
	doneCh chan struct{},
) {
	go func() {
		result, err := s.boostrap(ctx, request, phaseSwitcher, &bootstrapLogWriter{server: server})
		if err != nil {
			dhctlErrCh <- err
			return
		}

		err = server.Send(&pb.BootstrapResponse{
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

func (s *Service) stopBootstrap(
	cancel context.CancelFunc,
	_ *pb.BootstrapStop,
) error {
	cancel()
	return nil
}

func (s *Service) boostrap(
	_ context.Context,
	request *pb.BootstrapStart,
	phaseSwitcher *bootstrapPhaseSwitcher,
	logWriter io.Writer,
) (*pb.BootstrapResult, error) {
	var err error

	// set global variables from options
	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream: logWriter,
		Width:     int(request.Options.LogWidth),
	})
	app.SanityCheck = request.Options.SanityCheck
	app.UseTfCache = app.UseStateCacheYes
	app.PreflightSkipDeckhouseVersionCheck = true
	app.PreflightSkipAll = true
	app.CacheDir = s.cacheDir

	log.InfoF("Task is running by DHCTL Server pod/%s\n", s.podName)
	defer func() {
		log.InfoF("Task done by DHCTL Server pod/%s, err: %v\n", s.podName, err)
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
			return fmt.Errorf("failed to write resources: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
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
		return nil, err
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
		return nil, err
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
	data, marshalErr := json.Marshal(state)

	return &pb.BootstrapResult{State: string(data)}, errors.Join(bootstrapErr, marshalErr)
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
		{
			Event:       "toStop",
			Sources:     []fsm.State{"waiting"},
			Destination: "stopped",
		},
		{
			Event:       "stop",
			Sources:     []fsm.State{"running", "waiting"},
			Destination: "stopped",
		},
	}
}

type bootstrapLogWriter struct {
	server pb.DHCTL_BootstrapServer
}

func (w *bootstrapLogWriter) Write(p []byte) (int, error) {
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
