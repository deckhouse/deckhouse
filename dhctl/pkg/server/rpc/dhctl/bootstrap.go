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
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/pointer"
)

func (s *Service) Bootstrap(server pb.DHCTL_BootstrapServer) error {
	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()

	gr, ctx := errgroup.WithContext(ctx)

	f := fsm.New("initial", s.bootstrapServerTransitions())

	phaseSwitcher := &bootstrapPhaseSwitcher{
		server: server,
		f:      f,
		next:   make(chan struct{ err error }),
	}
	defer close(phaseSwitcher.next)

	gr.Go(func() error {
		for {
			request, err := server.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return status.Errorf(codes.Internal, "receiving message: %s", err)
			}

			switch message := request.Message.(type) {
			case *pb.BootstrapRequest_Start:
				err = f.Event("start")
				if err != nil {
					s.logb.Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue
				}
				err = s.startBootstrap(ctx, gr, server, phaseSwitcher, message.Start)

			case *pb.BootstrapRequest_Stop:
				err = f.Event("stop")
				if err != nil {
					s.logb.Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue
				}
				err = s.stopBootstrap(cancel, message.Stop)

			case *pb.BootstrapRequest_Continue:
				if message.Continue.Error != "" {
					err = f.Event("toStop")
					if err != nil {
						s.logb.Error("got unprocessable message",
							logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
						continue
					}
					phaseSwitcher.next <- struct{ err error }{err: errors.New(message.Continue.Error)}
					continue
				}

				err = f.Event("toNextPhase")
				if err != nil {
					s.logb.Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue
				}
				phaseSwitcher.next <- struct{ err error }{err: nil}

			default:
				s.logb.Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue
			}
		}
	})

	return gr.Wait()
}

func (s *Service) startBootstrap(
	ctx context.Context,
	gr *errgroup.Group,
	server pb.DHCTL_BootstrapServer,
	phaseSwitcher *bootstrapPhaseSwitcher,
	request *pb.BootstrapStart,
) error {
	gr.Go(func() error {
		result, err := s.boostrap(ctx, request, phaseSwitcher, &bootstrapLogWriter{server: server})
		if err != nil {
			return err
		}

		err = server.Send(&pb.BootstrapResponse{
			Message: &pb.BootstrapResponse_Result{
				Result: result,
			},
		})
		if err != nil {
			return status.Errorf(codes.Internal, "sending message: %s", err)
		}
		return nil
	})

	return nil
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

	// parse connection config
	connectionConfig, err := config.ParseConnectionConfig(
		request.ConnectionConfig,
		config.NewSchemaStore(),
		config.ValidateOptionCommanderMode(request.Options.CommanderMode),
		config.ValidateOptionStrictUnmarshal(request.Options.CommanderMode),
		config.ValidateOptionValidateExtensions(request.Options.CommanderMode),
	)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parsing connection config: %s", err)
	}

	// preparse ssh client
	sshClient, err := prepareSSHClient(connectionConfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer sshClient.Stop()

	// prepare config files
	configPath, err := writeTempFile([]byte(input.CombineYAMLs(
		request.ClusterConfig, request.InitConfig, request.ProviderSpecificClusterConfig,
	)))
	if err != nil {
		return nil, fmt.Errorf("failed to write init configuration: %w", err)
	}

	resourcesPath, err := writeTempFile([]byte(input.CombineYAMLs(
		request.InitResources, request.Resources,
	)))
	if err != nil {
		return nil, fmt.Errorf("failed to write resources: %w", err)
	}

	postBootstrapScriptPath, err := writeTempFile([]byte(request.PostBootstrapScript))
	if err != nil {
		return nil, fmt.Errorf("failed to write resources: %w", err)
	}

	// init dhctl state
	var initialState phases.DhctlState
	if request.State != "" {
		err = json.Unmarshal([]byte(request.State), &initialState)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "unmarshalling dhctl state: %s", err)
		}
	}

	// boostrap cluster
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
	next   chan struct{ err error }
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

	msg, ok := <-b.next
	if !ok {
		return fmt.Errorf("server stopped, cancel task")
	}
	return msg.err
}
