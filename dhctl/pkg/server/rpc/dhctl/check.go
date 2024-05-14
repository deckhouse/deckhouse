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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/logger"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func (s *Service) Check(server pb.DHCTL_CheckServer) error {
	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()

	fmt.Printf("Check processing started!\n")
	f := fsm.New("initial", s.checkServerTransitions())

	userErrCh := make(chan error, 0)
	internalErrCh := make(chan error, 0)
	doneCh := make(chan struct{}, 0)
	requestsCh := make(chan *pb.CheckRequest, 0)

	s.startReceiver(ctx, server, requestsCh, internalErrCh)

connectionProcessor:
	for {
		select {
		case <-doneCh:
			fmt.Printf("Done check processing!\n")
			return nil

		case err := <-internalErrCh:
			return status.Errorf(codes.Internal, "%s", err)

		case err := <-userErrCh:
			sendErr := server.Send(&pb.CheckResponse{
				Message: &pb.CheckResponse_Err{
					Err: err.Error(),
				},
			})
			if sendErr != nil {
				return status.Errorf(codes.Internal, "sending message: %s", sendErr)
			}
			fmt.Printf("Sent error to server: done check processing!\n")
			return nil

		case request := <-requestsCh:
			fmt.Printf("Check request received!\n")

			switch message := request.Message.(type) {
			case *pb.CheckRequest_Start:
				err := f.Event("start")
				if err != nil {
					s.log.Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				fmt.Printf("CheckRequest_Start ...\n")
				s.startChecker(ctx, server, message.Start, userErrCh, internalErrCh, doneCh)
				fmt.Printf("CheckRequest_Start DONE\n")

			case *pb.CheckRequest_Stop:
				log.WarnF("[multiversion-debug] process CheckRequest_Stop...\n")
				err := f.Event("stop")
				if err != nil {
					s.log.Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				s.stopCheck(cancel, message.Stop)
				log.WarnF("[multiversion-debug] process CheckRequest_Stop OK\n")

			default:
				s.log.Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue connectionProcessor
			}
		}
	}
}

func (s *Service) startReceiver(_ context.Context, server pb.DHCTL_CheckServer, requestsCh chan *pb.CheckRequest, internalErrCh chan error) {
	go func() {
		for {
			fmt.Printf("Awaiting for request from check stream...\n")
			request, err := server.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				internalErrCh <- status.Errorf(codes.Internal, "receiving message: %w", err)
				return
			}
			fmt.Printf("Got request form check stream: %#v", request)
			requestsCh <- request
		}
	}()
}

func (s *Service) startChecker(
	ctx context.Context,
	server pb.DHCTL_CheckServer,
	request *pb.CheckStart,
	userErrCh chan error,
	internalErrCh chan error,
	doneCh chan struct{},
) {
	go func() {
		result, err := s.check(ctx, request, &checkLogWriter{server: server})
		if err != nil {
			userErrCh <- err
			return
		}

		err = server.Send(&pb.CheckResponse{
			Message: &pb.CheckResponse_Result{
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

func (s *Service) stopCheck(
	cancel context.CancelFunc,
	_ *pb.CheckStop,
) {
	cancel()
}

func (s *Service) check(
	ctx context.Context,
	request *pb.CheckStart,
	logWriter io.Writer,
) (*pb.CheckResult, error) {
	var err error

	fmt.Printf("Service::check BEGIN\n")

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
	fmt.Printf("Task is running by DHCTL Server pod/%s\n", s.podName)
	defer func() {
		log.WarnF("[multiversion-debug] Task done by DHCTL Server pod/%s, err: %v\n", s.podName, err)
	}()

	// parse connection config
	connectionConfig, err := config.ParseConnectionConfig(
		request.ConnectionConfig,
		config.NewSchemaStore(),
		config.ValidateOptionCommanderMode(request.Options.CommanderMode),
		config.ValidateOptionStrictUnmarshal(request.Options.CommanderMode),
		config.ValidateOptionValidateExtensions(request.Options.CommanderMode),
	)
	if err != nil {
		return nil, fmt.Errorf("parsing connection config: %w", err)
	}

	// parse meta config
	metaConfig, err := config.ParseConfigFromData(
		input.CombineYAMLs(request.ClusterConfig, request.ProviderSpecificClusterConfig),
		config.ValidateOptionCommanderMode(request.Options.CommanderMode),
		config.ValidateOptionStrictUnmarshal(request.Options.CommanderMode),
		config.ValidateOptionValidateExtensions(request.Options.CommanderMode),
	)
	if err != nil {
		return nil, fmt.Errorf("parsing meta config: %w", err)
	}

	// init dhctl cache
	cachePath := metaConfig.CachePath()
	var initialState phases.DhctlState
	if request.State != "" {
		err = json.Unmarshal([]byte(request.State), &initialState)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling dhctl state: %w", err)
		}
	}
	err = cache.InitWithOptions(
		cachePath,
		cache.CacheOptions{InitialState: initialState, ResetInitialState: true},
	)
	fmt.Printf("cache.InitWIthOptions -> err=%v\n", err)
	if err != nil {
		return nil, fmt.Errorf("initializing cache at %s: %w", cachePath, err)
	}

	// preparse ssh client
	sshClient, err := prepareSSHClient(connectionConfig)
	if err != nil {
		return nil, fmt.Errorf("preparing ssh client: %w", err)
	}
	defer sshClient.Stop()

	// check cluster state
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

	result, err := checker.Check(ctx)
	if err != nil {
		return nil, fmt.Errorf("checking cluster state: %s", err)
	}

	resultString, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshalling check result: %s", err)
	}

	return &pb.CheckResult{Result: string(resultString)}, nil
}

type checkLogWriter struct {
	server pb.DHCTL_CheckServer
}

func (w *checkLogWriter) Write(p []byte) (int, error) {
	err := w.server.Send(&pb.CheckResponse{
		Message: &pb.CheckResponse_Logs{
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

func (s *Service) checkServerTransitions() []fsm.Transition {
	return []fsm.Transition{
		{
			Event:       "start",
			Sources:     []fsm.State{"initial"},
			Destination: "running",
		},
		{
			Event:       "stop",
			Sources:     []fsm.State{"running"},
			Destination: "stopped",
		},
	}
}
