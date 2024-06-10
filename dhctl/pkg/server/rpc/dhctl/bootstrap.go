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

func (s *Service) Bootstrap(server pb.DHCTL_BootstrapServer) error {
	ctx := operationCtx(server)

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.bootstrapServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.BootstrapRequest)
	sendCh := make(chan *pb.BootstrapResponse)
	phaseSwitcher := &bootstrapPhaseSwitcher{
		sendCh: sendCh,
		f:      f,
		next:   make(chan error),
	}

	s.startBootstrapperReceiver(server, receiveCh, doneCh, internalErrCh)
	s.startBootstrapperSender(server, sendCh, internalErrCh)

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
				s.startBootstrap(
					ctx, message.Start, phaseSwitcher, &bootstrapLogWriter{l: logger.L(ctx), sendCh: sendCh}, sendCh,
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
	receiveCh chan *pb.BootstrapRequest,
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

func (s *Service) startBootstrapperSender(
	server pb.DHCTL_BootstrapServer,
	sendCh chan *pb.BootstrapResponse,
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

func (s *Service) startBootstrap(
	ctx context.Context,
	request *pb.BootstrapStart,
	phaseSwitcher *bootstrapPhaseSwitcher,
	logWriter *bootstrapLogWriter,
	sendCh chan *pb.BootstrapResponse,
) {
	go func() {
		result := s.bootstrap(ctx, request, phaseSwitcher, logWriter)
		sendCh <- &pb.BootstrapResponse{
			Message: &pb.BootstrapResponse_Result{
				Result: result,
			},
		}
	}()
}

func (s *Service) bootstrap(
	_ context.Context,
	request *pb.BootstrapStart,
	phaseSwitcher *bootstrapPhaseSwitcher,
	logWriter io.Writer,
) *pb.BootstrapResult {
	var err error

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
		ConfigPaths:                []string{configPath},
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
	sendCh chan *pb.BootstrapResponse

	m    sync.Mutex
	prev []byte
}

func (w *bootstrapLogWriter) Write(p []byte) (n int, err error) {
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
		w.sendCh <- &pb.BootstrapResponse{
			Message: &pb.BootstrapResponse_Logs{Logs: &pb.Logs{Logs: r}},
		}
	}

	return len(p), nil
}

type bootstrapPhaseSwitcher struct {
	sendCh chan *pb.BootstrapResponse
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

	b.sendCh <- &pb.BootstrapResponse{
		Message: &pb.BootstrapResponse_PhaseEnd{
			PhaseEnd: &pb.BootstrapPhaseEnd{
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
