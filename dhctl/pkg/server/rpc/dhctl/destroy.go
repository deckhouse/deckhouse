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
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func (s *Service) Destroy(server pb.DHCTL_DestroyServer) error {
	ctx := operationCtx(server)

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.destroyServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.DestroyRequest)
	sendCh := make(chan *pb.DestroyResponse)
	phaseSwitcher := &destroyPhaseSwitcher{
		sendCh: sendCh,
		f:      f,
		next:   make(chan error),
	}

	s.startDestroyerReceiver(server, receiveCh, doneCh, internalErrCh)
	s.startDestroyerSender(server, sendCh, internalErrCh)

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
				"processing DestroyRequest",
				slog.String("message", fmt.Sprintf("%T", request.Message)),
			)
			switch message := request.Message.(type) {
			case *pb.DestroyRequest_Start:
				err := f.Event("start")
				if err != nil {
					logger.L(ctx).Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				s.startDestroy(
					ctx, message.Start, phaseSwitcher, &destroyLogWriter{l: logger.L(ctx), sendCh: sendCh}, sendCh,
				)

			case *pb.DestroyRequest_Continue:
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

func (s *Service) startDestroyerReceiver(
	server pb.DHCTL_DestroyServer,
	receiveCh chan *pb.DestroyRequest,
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

func (s *Service) startDestroyerSender(
	server pb.DHCTL_DestroyServer,
	sendCh chan *pb.DestroyResponse,
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

func (s *Service) startDestroy(
	ctx context.Context,
	request *pb.DestroyStart,
	phaseSwitcher *destroyPhaseSwitcher,
	logWriter *destroyLogWriter,
	sendCh chan *pb.DestroyResponse,
) {
	go func() {
		result := s.destroy(ctx, request, phaseSwitcher, logWriter)
		sendCh <- &pb.DestroyResponse{
			Message: &pb.DestroyResponse_Result{
				Result: result,
			},
		}
	}()
}

func (s *Service) destroy(
	_ context.Context,
	request *pb.DestroyStart,
	phaseSwitcher *destroyPhaseSwitcher,
	logWriter io.Writer,
) *pb.DestroyResult {
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

	var metaConfig *config.MetaConfig
	err = log.Process("default", "Parsing cluster config", func() error {
		metaConfig, err = config.ParseConfigFromData(
			input.CombineYAMLs(request.ClusterConfig, request.InitConfig, request.ProviderSpecificClusterConfig),
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
		return &pb.DestroyResult{Err: err.Error()}
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
		return &pb.DestroyResult{Err: err.Error()}
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
		return &pb.DestroyResult{Err: err.Error()}
	}
	defer sshClient.Stop()

	var commanderUUID uuid.UUID
	if request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(request.Options.CommanderUuid)
		if err != nil {
			return &pb.DestroyResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	destroyer, err := destroy.NewClusterDestroyer(&destroy.Params{
		SSHClient:     sshClient,
		StateCache:    cache.Global(),
		OnPhaseFunc:   phaseSwitcher.switchPhase,
		CommanderMode: true,
		CommanderUUID: commanderUUID,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(request.ClusterConfig),
			[]byte(request.ProviderSpecificClusterConfig),
		),
		TerraformContext: terraform.NewTerraformContext(),
	})
	if err != nil {
		return &pb.DestroyResult{Err: fmt.Errorf("unable to initialize cluster destroyer: %w", err).Error()}
	}

	destroyErr := destroyer.DestroyCluster(true)
	state := destroyer.PhasedExecutionContext.GetLastState()
	data, marshalErr := json.Marshal(state)
	err = errors.Join(destroyErr, marshalErr)

	return &pb.DestroyResult{State: string(data), Err: errToString(err)}
}

func (s *Service) destroyServerTransitions() []fsm.Transition {
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

type destroyLogWriter struct {
	l      *slog.Logger
	sendCh chan *pb.DestroyResponse

	m    sync.Mutex
	prev []byte
}

func (w *destroyLogWriter) Write(p []byte) (n int, err error) {
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
		w.sendCh <- &pb.DestroyResponse{
			Message: &pb.DestroyResponse_Logs{Logs: &pb.Logs{Logs: r}},
		}
	}

	return len(p), nil
}

type destroyPhaseSwitcher struct {
	sendCh chan *pb.DestroyResponse
	f      *fsm.FiniteStateMachine
	next   chan error
}

func (b *destroyPhaseSwitcher) switchPhase(
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

	b.sendCh <- &pb.DestroyResponse{
		Message: &pb.DestroyResponse_PhaseEnd{
			PhaseEnd: &pb.DestroyPhaseEnd{
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
