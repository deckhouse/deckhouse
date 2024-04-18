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
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/logger"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Service) Check(server pb.DHCTL_CheckServer) error {
	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()

	gr, ctx := errgroup.WithContext(ctx)

	f := fsm.New("initial", s.checkServerTransitions())

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
			case *pb.CheckRequest_Start:
				err = f.Event("start")
				if err != nil {
					s.log.Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue
				}
				err = s.startCheck(ctx, gr, server, message.Start)

			case *pb.CheckRequest_Stop:
				err = f.Event("stop")
				if err != nil {
					s.log.Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue
				}
				err = s.stopCheck(cancel, message.Stop)

			default:
				s.log.Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue
			}
		}
	})

	return gr.Wait()
}

func (s *Service) startCheck(
	ctx context.Context,
	gr *errgroup.Group,
	server pb.DHCTL_CheckServer,
	request *pb.CheckStart,
) error {
	gr.Go(func() error {
		result, err := s.check(ctx, request, &checkLogWriter{server: server})
		if err != nil {
			return err
		}

		err = server.Send(&pb.CheckResponse{
			Message: &pb.CheckResponse_Result{
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

func (s *Service) stopCheck(
	cancel context.CancelFunc,
	_ *pb.CheckStop,
) error {
	cancel()
	return nil
}

func (s *Service) check(
	ctx context.Context,
	request *pb.CheckStart,
	logWriter io.Writer,
) (*pb.CheckResult, error) {
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

	log.InfoF("Task has been started by DHCTL Server pod/%s\n", s.podName)

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

	// parse meta config
	metaConfig, err := config.ParseConfigFromData(
		combineYAMLs(request.ClusterConfig, request.ProviderSpecificClusterConfig),
		config.ValidateOptionCommanderMode(request.Options.CommanderMode),
		config.ValidateOptionStrictUnmarshal(request.Options.CommanderMode),
		config.ValidateOptionValidateExtensions(request.Options.CommanderMode),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "parsing meta config: %s", err)
	}

	// init dhctl cache
	cachePath := metaConfig.CachePath()
	var initialState phases.DhctlState
	if request.State != "" {
		err = json.Unmarshal([]byte(request.State), &initialState)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "unmarshalling dhctl state: %s", err)
		}
	}
	err = cache.InitWithOptions(
		cachePath,
		cache.CacheOptions{InitialState: initialState, ResetInitialState: true},
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "initializing cache at %s: %s", cachePath, err)
	}

	// preparse ssh client
	sshClient, err := prepareSSHClient(connectionConfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
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
		return nil, status.Errorf(codes.Internal, "checking cluster state: %s", err)
	}

	resultString, err := json.Marshal(result)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshalling check result: %s", err)
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
			Destination: "started",
		},
		{
			Event:       "stop",
			Sources:     []fsm.State{"started"},
			Destination: "stopped",
		},
	}
}
