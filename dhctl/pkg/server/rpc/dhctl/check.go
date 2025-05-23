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
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util/callback"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func (s *Service) Check(server pb.DHCTL_CheckServer) error {
	ctx, cancel := operationCtx(server)
	defer cancel()

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.checkServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.CheckRequest)
	sendCh := make(chan *pb.CheckResponse)

	loggerDefault := logger.L(ctx).With(logTypeDHCTL)

	logWriter := logger.NewLogWriter(loggerDefault, sendCh,
		func(lines []string) *pb.CheckResponse {
			return &pb.CheckResponse{Message: &pb.CheckResponse_Logs{Logs: &pb.Logs{Logs: lines}}}
		},
	)

	debugWriter := logger.NewDebugLogWriter(loggerDefault)

	logOptions := logger.Options{
		DebugWriter:   debugWriter,
		DefaultWriter: logWriter,
	}

	startReceiver[*pb.CheckRequest, *pb.CheckResponse](server, receiveCh, doneCh, internalErrCh)
	startSender[*pb.CheckRequest, *pb.CheckResponse](server, sendCh, internalErrCh)

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
				"processing CheckRequest",
				slog.String("message", fmt.Sprintf("%T", request.Message)),
			)
			switch message := request.Message.(type) {
			case *pb.CheckRequest_Start:
				err := f.Event("start")
				if err != nil {
					logger.L(ctx).Error("got unprocessable message",
						logger.Err(err), slog.String("message", fmt.Sprintf("%T", message)))
					continue connectionProcessor
				}
				go func() {
					result := s.checkSafe(ctx, message.Start, logOptions)
					sendCh <- &pb.CheckResponse{Message: &pb.CheckResponse_Result{Result: result}}
				}()

			case *pb.CheckRequest_Cancel:
				cancel()

			default:
				logger.L(ctx).Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue connectionProcessor
			}
		}
	}
}

func (s *Service) checkSafe(ctx context.Context, request *pb.CheckStart, options logger.Options) (result *pb.CheckResult) {
	defer func() {
		if r := recover(); r != nil {
			result = &pb.CheckResult{Err: panicMessage(ctx, r)}
		}
	}()

	return s.check(ctx, request, options)
}

func (s *Service) check(ctx context.Context, request *pb.CheckStart, options logger.Options) *pb.CheckResult {
	var err error

	cleanuper := callback.NewCallback()
	defer func() { _ = cleanuper.Call() }()

	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream:   options.DefaultWriter,
		Width:       int(request.Options.LogWidth),
		DebugStream: options.DebugWriter,
	})

	app.SanityCheck = true
	app.UseTfCache = app.UseStateCacheYes
	app.ResourcesTimeout = request.Options.ResourcesTimeout.AsDuration()
	app.DeckhouseTimeout = request.Options.DeckhouseTimeout.AsDuration()
	app.CacheDir = s.cacheDir
	app.ApplyPreflightSkips(request.Options.CommonOptions.SkipPreflightChecks)

	log.InfoF("Task is running by DHCTL Server pod/%s\n", s.podName)
	defer func() { log.InfoF("Task done by DHCTL Server pod/%s\n", s.podName) }()

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
		return &pb.CheckResult{Err: err.Error()}
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
		return &pb.CheckResult{Err: err.Error()}
	}

	var commanderUUID uuid.UUID
	if request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(request.Options.CommanderUuid)
		if err != nil {
			return &pb.CheckResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	checkParams := &check.Params{
		StateCache:    cache.Global(),
		CommanderMode: request.Options.CommanderMode,
		CommanderUUID: commanderUUID,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(request.ClusterConfig),
			[]byte(request.ProviderSpecificClusterConfig),
		),
		InfrastructureContext: infrastructure.NewContextWithProvider(infrastructureprovider.ExecutorProvider(metaConfig)),
	}

	kubeClient, sshClient, cleanup, err := helper.InitializeClusterConnections(ctx, helper.ClusterConnectionsOptions{
		CommanderMode: request.Options.CommanderMode,
		ApiServerUrl:  request.Options.ApiServerUrl,
		ApiServerOptions: helper.ApiServerOptions{
			Token:                    request.Options.ApiServerToken,
			InsecureSkipTLSVerify:    request.Options.ApiServerInsecureSkipTlsVerify,
			CertificateAuthorityData: util.StringToBytes(request.Options.ApiServerCertificateAuthorityData),
		},
		SchemaStore:         s.schemaStore,
		SSHConnectionConfig: request.ConnectionConfig,
	})
	cleanuper.Add(cleanup)
	if err != nil {
		return &pb.CheckResult{Err: err.Error()}
	}

	checkParams.KubeClient = kubeClient
	checkParams.SSHClient = sshClient

	checker := check.NewChecker(checkParams)

	result, checkErr := checker.Check(ctx)
	resultData, marshalErr := json.Marshal(result)
	state, extractStateErr := phases.ExtractDhctlState(cache.Global())
	stateData, marshalStateErr := json.Marshal(state)

	err = errors.Join(checkErr, marshalErr, extractStateErr, marshalStateErr)

	if result != nil {
		// todo: move onCheckResult call to check.Check() func (as in converge)
		_ = onCheckResult(result)
	}

	return &pb.CheckResult{
		Result: string(resultData),
		Err:    util.ErrToString(err),
		State:  string(stateData),
	}
}

func (s *Service) checkServerTransitions() []fsm.Transition {
	return []fsm.Transition{
		{
			Event:       "start",
			Sources:     []fsm.State{"initial"},
			Destination: "running",
		},
	}
}
