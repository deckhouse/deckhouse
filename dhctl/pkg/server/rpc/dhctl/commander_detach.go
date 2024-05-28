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
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander/detach"
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

func (s *Service) CommanderDetach(server pb.DHCTL_CommanderDetachServer) error {
	ctx := operationCtx(server)

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.CommanderDetachServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.CommanderDetachRequest)
	sendCh := make(chan *pb.CommanderDetachResponse)

	s.startdetacherReceiver(server, receiveCh, doneCh, internalErrCh)
	s.startdetacherSender(server, sendCh, internalErrCh)

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
				"processing CommanderDetachRequest",
				slog.String("message", fmt.Sprintf("%T", request.Message)),
			)
			switch message := request.Message.(type) {
			case *pb.CommanderDetachRequest_Start:
				err := f.Event("start")
				if err != nil {
					logger.L(ctx).Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				s.startCommanderDetach(
					ctx, message.Start, &CommanderDetachLogWriter{l: logger.L(ctx), sendCh: sendCh}, sendCh,
				)

			default:
				logger.L(ctx).Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue connectionProcessor
			}
		}
	}
}

func (s *Service) startdetacherReceiver(
	server pb.DHCTL_CommanderDetachServer,
	receiveCh chan *pb.CommanderDetachRequest,
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

func (s *Service) startdetacherSender(
	server pb.DHCTL_CommanderDetachServer,
	sendCh chan *pb.CommanderDetachResponse,
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

func (s *Service) startCommanderDetach(
	ctx context.Context,
	request *pb.CommanderDetachStart,
	logWriter *CommanderDetachLogWriter,
	sendCh chan *pb.CommanderDetachResponse,
) {
	go func() {
		result := s.CommanderDetachCluster(ctx, request, logWriter)
		sendCh <- &pb.CommanderDetachResponse{
			Message: &pb.CommanderDetachResponse_Result{
				Result: result,
			},
		}
	}()
}

func (s *Service) CommanderDetachCluster(
	ctx context.Context,
	request *pb.CommanderDetachStart,
	logWriter io.Writer,
) *pb.CommanderDetachResult {
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
		return &pb.CommanderDetachResult{Err: err.Error()}
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
		return &pb.CommanderDetachResult{Err: err.Error()}
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
		return &pb.CommanderDetachResult{Err: err.Error()}
	}
	defer sshClient.Stop()

	var commanderUUID uuid.UUID
	if request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(request.Options.CommanderUuid)
		if err != nil {
			return &pb.CommanderDetachResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	stateCache := cache.Global()

	checker := check.NewChecker(&check.Params{
		SSHClient:     sshClient,
		StateCache:    stateCache,
		CommanderMode: request.Options.CommanderMode,
		CommanderUUID: commanderUUID,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(request.ClusterConfig),
			[]byte(request.ProviderSpecificClusterConfig),
		),
		TerraformContext: terraform.NewTerraformContext(),
	})

	detacher := detach.NewDetacher(checker, sshClient, detach.DetacherOptions{
		DetachResources: detach.DetachResources{
			Template: request.ResourcesTemplate,
			Values:   request.ResourcesValues.AsMap(),
		},
		OnCheckResult: onCheckResult,
	})

	var resErrs []error

	resErrs = append(resErrs, detacher.Detach(ctx))

	var resState string
	state, err := phases.ExtractDhctlState(stateCache)
	if err != nil {
		resErrs = append(resErrs, fmt.Errorf("unable to extract dhctl state: %w", err))
	} else {
		data, err := json.Marshal(state)
		if err != nil {
			resErrs = append(resErrs, fmt.Errorf("unable to unmarshal dhctl state: %w", err))
		}
		resState = string(data)
	}

	return &pb.CommanderDetachResult{State: resState, Err: errToString(errors.Join(resErrs...))}
}

func (s *Service) CommanderDetachServerTransitions() []fsm.Transition {
	return []fsm.Transition{
		{
			Event:       "start",
			Sources:     []fsm.State{"initial"},
			Destination: "running",
		},
	}
}

type CommanderDetachLogWriter struct {
	l      *slog.Logger
	sendCh chan *pb.CommanderDetachResponse

	m    sync.Mutex
	prev []byte
}

func (w *CommanderDetachLogWriter) Write(p []byte) (n int, err error) {
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
		w.sendCh <- &pb.CommanderDetachResponse{
			Message: &pb.CommanderDetachResponse_Logs{Logs: &pb.Logs{Logs: r}},
		}
	}

	return len(p), nil
}
