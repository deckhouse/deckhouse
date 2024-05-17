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
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/logger"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Service) Converge(server pb.DHCTL_ConvergeServer) error {
	s.shutdown(server.Context().Done())

	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()

	s.logd.Info("started")

	f := fsm.New("initial", s.convergeServerTransitions())

	internalErrCh := make(chan error)
	doneCh := make(chan struct{})
	requestsCh := make(chan *pb.ConvergeRequest)

	phaseSwitcher := &convergePhaseSwitcher{
		server: server,
		f:      f,
		next:   make(chan error),
	}
	defer close(phaseSwitcher.next)

	s.startConvergeerReceiver(server, requestsCh, internalErrCh)

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
			case *pb.ConvergeRequest_Start:
				err := f.Event("start")
				if err != nil {
					s.logd.Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				s.startConverge(ctx, server, message.Start, phaseSwitcher, internalErrCh, doneCh)

			case *pb.ConvergeRequest_Continue:
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

func (s *Service) startConvergeerReceiver(
	server pb.DHCTL_ConvergeServer,
	requestsCh chan *pb.ConvergeRequest,
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
				"processing ConvergeRequest",
				slog.String("message", fmt.Sprintf("%T", request.Message)),
			)
			requestsCh <- request
		}
	}()
}

func (s *Service) startConverge(
	ctx context.Context,
	server pb.DHCTL_ConvergeServer,
	request *pb.ConvergeStart,
	phaseSwitcher *convergePhaseSwitcher,
	internalErrCh chan error,
	doneCh chan struct{},
) {
	go func() {
		result := s.converge(ctx, request, phaseSwitcher, &convergeLogWriter{server: server})
		err := server.Send(&pb.ConvergeResponse{
			Message: &pb.ConvergeResponse_Result{
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

func (s *Service) converge(
	ctx context.Context,
	request *pb.ConvergeStart,
	phaseSwitcher *convergePhaseSwitcher,
	logWriter io.Writer,
) *pb.ConvergeResult {
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
	app.PreflightSkipDeckhouseVersionCheck = true
	app.PreflightSkipAll = true

	log.InfoF("Task is running by DHCTL Server pod/%s\n", s.podName)
	defer func() {
		log.InfoF("Task done by DHCTL Server pod/%s, err: %v\n", s.podName, err)
	}()

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
		return &pb.ConvergeResult{Err: err.Error()}
	}
	defer sshClient.Stop()

	terraformContext := terraform.NewTerraformContext()

	checker := check.NewChecker(&check.Params{
		SSHClient:     sshClient,
		StateCache:    cache.Global(),
		CommanderMode: request.Options.CommanderMode,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(request.ClusterConfig),
			[]byte(request.ProviderSpecificClusterConfig),
		),
		TerraformContext: terraform.NewTerraformContext(),
	})

	converger := converge.NewConverger(&converge.Params{
		SSHClient:              sshClient,
		OnPhaseFunc:            phaseSwitcher.switchPhase,
		AutoApprove:            true,
		AutoDismissDestructive: false,
		CommanderMode:          true,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(request.ClusterConfig),
			[]byte(request.ProviderSpecificClusterConfig),
		),
		TerraformContext:           terraformContext,
		Checker:                    checker,
		ApproveDestructiveChangeID: request.ApproveDestructionChangeId,
		OnCheckResult:              onCheckResult,
	})

	result, convergeErr := converger.Converge(ctx)
	state := converger.GetLastState()
	stateData, marshalStateErr := json.Marshal(state)
	resultString, marshalResultErr := json.Marshal(result)
	err = errors.Join(convergeErr, marshalStateErr, marshalResultErr)

	return &pb.ConvergeResult{State: string(stateData), Result: string(resultString), Err: errToString(err)}
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

type convergeLogWriter struct {
	server pb.DHCTL_ConvergeServer
}

func (w *convergeLogWriter) Write(p []byte) (int, error) {
	err := w.server.Send(&pb.ConvergeResponse{
		Message: &pb.ConvergeResponse_Logs{
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

type convergePhaseSwitcher struct {
	server pb.DHCTL_ConvergeServer
	f      *fsm.FiniteStateMachine
	next   chan error
}

func (b *convergePhaseSwitcher) switchPhase(
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

	err = b.server.Send(&pb.ConvergeResponse{
		Message: &pb.ConvergeResponse_PhaseEnd{
			PhaseEnd: &pb.ConvergePhaseEnd{
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
