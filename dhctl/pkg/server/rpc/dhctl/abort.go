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
	"github.com/google/uuid"
	"io"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/pointer"

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
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func (s *Service) Abort(server pb.DHCTL_AbortServer) error {
	ctx := operationCtx(server)

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.abortServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.AbortRequest)
	sendCh := make(chan *pb.AbortResponse)
	phaseSwitcher := &abortPhaseSwitcher{
		sendCh: sendCh,
		f:      f,
		next:   make(chan error),
	}

	s.startAborterReceiver(server, receiveCh, doneCh, internalErrCh)
	s.startAborterSender(server, sendCh, internalErrCh)

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
				s.startAbort(
					ctx, message.Start, phaseSwitcher, &abortLogWriter{l: logger.L(ctx), sendCh: sendCh}, sendCh,
				)

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

func (s *Service) startAborterReceiver(
	server pb.DHCTL_AbortServer,
	receiveCh chan *pb.AbortRequest,
	doneCh chan struct{},
	internalErrCh chan error,
) {
	go func() {
		for {
			request, err := server.Recv()
			if errors.Is(err, io.EOF) {
				close(doneCh)
				return
			}
			if err != nil {
				internalErrCh <- fmt.Errorf("receiving message: %w", err)
				return
			}
			receiveCh <- request
		}
	}()
}

func (s *Service) startAborterSender(
	server pb.DHCTL_AbortServer,
	sendCh chan *pb.AbortResponse,
	internalErrCh chan error,
) {
	go func() {
		for response := range sendCh {
			loop := retry.NewSilentLoop("send message", 10, time.Millisecond*100)
			err := loop.Run(func() error {
				return server.Send(response)
			})
			if err != nil {
				internalErrCh <- fmt.Errorf("sending message: %w", err)
				return
			}
		}
	}()
}

func (s *Service) startAbort(
	ctx context.Context,
	request *pb.AbortStart,
	phaseSwitcher *abortPhaseSwitcher,
	logWriter *abortLogWriter,
	sendCh chan *pb.AbortResponse,
) {
	go func() {
		result := s.abort(ctx, request, phaseSwitcher, logWriter)
		sendCh <- &pb.AbortResponse{
			Message: &pb.AbortResponse_Result{
				Result: result,
			},
		}
	}()
}

func (s *Service) abort(
	_ context.Context,
	request *pb.AbortStart,
	phaseSwitcher *abortPhaseSwitcher,
	logWriter io.Writer,
) *pb.AbortResult {
	var err error

	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream: logWriter,
		Width:     int(request.Options.LogWidth),
	})
	app.SanityCheck = true
	app.UseTfCache = app.UseStateCacheYes
	app.ResourcesTimeout = request.Options.ResourcesTimeout.AsDuration()
	app.DeckhouseTimeout = request.Options.DeckhouseTimeout.AsDuration()
	app.CacheDir = s.cacheDir

	log.InfoF("Task is running by DHCTL Server pod/%s\n", s.podName)
	defer func() {
		log.InfoF("Task done by DHCTL Server pod/%s\n", s.podName)
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
		CommanderUUID:     uuid.MustParse("f013cd23-6ceb-4bc5-a32d-c6f2c8cf41ea"),
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
	l      *slog.Logger
	sendCh chan *pb.AbortResponse

	m    sync.Mutex
	prev []byte
}

func (w *abortLogWriter) Write(p []byte) (n int, err error) {
	w.m.Lock()
	defer w.m.Unlock()

	var r []string

	for _, b := range p {
		switch b {
		case '\n', '\r':
			s := string(w.prev)
			if s != "" {
				r = append(r, s)
			}
			w.prev = []byte{}
		default:
			w.prev = append(w.prev, b)
		}
	}

	if len(r) > 0 {
		for _, line := range r {
			w.l.Info(line, logTypeDHCTL)
		}
		w.sendCh <- &pb.AbortResponse{
			Message: &pb.AbortResponse_Logs{Logs: &pb.Logs{Logs: r}},
		}
	}

	return len(p), nil
}

type abortPhaseSwitcher struct {
	sendCh chan *pb.AbortResponse
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

	b.sendCh <- &pb.AbortResponse{
		Message: &pb.AbortResponse_PhaseEnd{
			PhaseEnd: &pb.AbortPhaseEnd{
				CompletedPhase:      string(completedPhase),
				CompletedPhaseState: completedPhaseState,
				NextPhase:           string(nextPhase),
				NextPhaseCritical:   nextPhaseCritical,
			},
		},
	}

	switchErr, ok := <-b.next
	if !ok {
		return fmt.Errorf("server stopped, cancel task")
	}
	return switchErr
}
