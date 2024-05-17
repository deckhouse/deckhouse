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

func (s *Service) Abort(server pb.DHCTL_AbortServer) error {
	s.shutdown(server.Context().Done())

	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()

	s.logd.Info("started")

	f := fsm.New("initial", s.abortServerTransitions())

	internalErrCh := make(chan error)
	doneCh := make(chan struct{})
	requestsCh := make(chan *pb.AbortRequest)

	phaseSwitcher := &abortPhaseSwitcher{
		server: server,
		f:      f,
		next:   make(chan error),
	}
	defer close(phaseSwitcher.next)

	s.startAborterReceiver(server, requestsCh, internalErrCh)

connectionProcessor:
	for {
		select {
		case <-doneCh:
			s.logd.Info("finished normally")
			return nil

		case err := <-internalErrCh:
			s.logd.Error("finished with internal error", logger.Err(err))
			return status.Errorf(codes.Internal, "%s", err)

		case request := <-requestsCh:
			switch message := request.Message.(type) {
			case *pb.AbortRequest_Start:
				err := f.Event("start")
				if err != nil {
					s.logd.Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				s.startAbort(ctx, server, message.Start, phaseSwitcher, internalErrCh, doneCh)

			case *pb.AbortRequest_Continue:
				err := f.Event("toNextPhase")
				if err != nil {
					s.logd.Error("got unprocessable message",
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
				s.logd.Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue connectionProcessor
			}
		}
	}
}

func (s *Service) startAborterReceiver(
	server pb.DHCTL_AbortServer,
	requestsCh chan *pb.AbortRequest,
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
			s.logd.Info(
				"processing AbortRequest",
				slog.String("message", fmt.Sprintf("%T", request.Message)),
			)
			requestsCh <- request
		}
	}()
}

func (s *Service) startAbort(
	ctx context.Context,
	server pb.DHCTL_AbortServer,
	request *pb.AbortStart,
	phaseSwitcher *abortPhaseSwitcher,
	internalErrCh chan error,
	doneCh chan struct{},
) {
	go func() {
		result := s.abort(ctx, request, phaseSwitcher, &abortLogWriter{server: server})
		err := server.Send(&pb.AbortResponse{
			Message: &pb.AbortResponse_Result{
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

func (s *Service) abort(
	_ context.Context,
	request *pb.AbortStart,
	phaseSwitcher *abortPhaseSwitcher,
	logWriter io.Writer,
) *pb.AbortResult {
	var err error

	// set global variables from options
	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream: logWriter,
		Width:     int(request.Options.LogWidth),
	})
	app.SanityCheck = request.Options.SanityCheck
	app.UseTfCache = app.UseStateCacheYes
	app.ResourcesTimeout = request.Options.ResourcesTimeout.AsDuration()
	app.DeckhouseTimeout = request.Options.DeckhouseTimeout.AsDuration()
	app.CacheDir = s.cacheDir

	log.InfoF("Task is running by DHCTL Server pod/%s\n", s.podName)
	defer func() {
		log.InfoF("Task done by DHCTL Server pod/%s, err: %v\n", s.podName, err)
	}()

	var (
		configPath    string
		resourcesPath string
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
		return &pb.AbortResult{Err: err.Error()}
	}
	defer sshClient.Stop()

	bootstrapper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{
		ConfigPaths:      []string{configPath},
		ResourcesPath:    resourcesPath,
		InitialState:     initialState,
		SSHClient:        sshClient,
		UseTfCache:       pointer.Bool(true),
		AutoApprove:      pointer.Bool(true),
		ResourcesTimeout: request.Options.ResourcesTimeout.AsDuration(),
		DeckhouseTimeout: request.Options.DeckhouseTimeout.AsDuration(),

		ResetInitialState: true,
		OnPhaseFunc:       phaseSwitcher.switchPhase,
		CommanderMode:     request.Options.CommanderMode,
		TerraformContext:  terraform.NewTerraformContext(),
	})

	abortErr := bootstrapper.Abort(false)
	state := bootstrapper.GetLastState()
	stateData, marshalErr := json.Marshal(state)
	err = errors.Join(abortErr, marshalErr)

	return &pb.AbortResult{State: string(stateData), Err: errToString(err)}
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

type abortLogWriter struct {
	server pb.DHCTL_AbortServer
}

func (w *abortLogWriter) Write(p []byte) (int, error) {
	err := w.server.Send(&pb.AbortResponse{
		Message: &pb.AbortResponse_Logs{
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

type abortPhaseSwitcher struct {
	server pb.DHCTL_AbortServer
	f      *fsm.FiniteStateMachine
	next   chan error
}

func (b *abortPhaseSwitcher) switchPhase(
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

	err = b.server.Send(&pb.AbortResponse{
		Message: &pb.AbortResponse_PhaseEnd{
			PhaseEnd: &pb.AbortPhaseEnd{
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
