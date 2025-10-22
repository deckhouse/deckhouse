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
	"github.com/name212/govalue"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
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

type checkParams struct {
	request    *pb.CheckStart
	logOptions logger.Options
}

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
					result := s.checkSafe(ctx, checkParams{
						request:    message.Start,
						logOptions: logOptions,
					})
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

func (s *Service) checkSafe(ctx context.Context, p checkParams) (result *pb.CheckResult) {
	defer func() {
		if r := recover(); r != nil {
			lastState, err := panicResult(ctx, r)
			result = &pb.CheckResult{State: string(lastState), Err: err.Error()}
		}
	}()

	return s.check(ctx, p)
}

func (s *Service) check(ctx context.Context, p checkParams) *pb.CheckResult {
	var err error

	cleanuper := callback.NewCallback()
	defer func() { _ = cleanuper.Call() }()

	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream:   p.logOptions.DefaultWriter,
		Width:       int(p.request.Options.LogWidth),
		DebugStream: p.logOptions.DebugWriter,
	})

	loggerFor := log.GetDefaultLogger()

	app.SanityCheck = true
	app.UseTfCache = app.UseStateCacheYes
	app.ResourcesTimeout = p.request.Options.ResourcesTimeout.AsDuration()
	app.DeckhouseTimeout = p.request.Options.DeckhouseTimeout.AsDuration()
	app.CacheDir = s.params.CacheDir
	app.ApplyPreflightSkips(p.request.Options.CommonOptions.SkipPreflightChecks)

	logBeforeExit := logInformationAboutInstance(s.params, loggerFor)
	defer logBeforeExit()

	var metaConfig *config.MetaConfig
	err = loggerFor.LogProcess("default", "Parsing cluster config", func() error {
		metaConfig, err = config.ParseConfigFromData(
			ctx,
			input.CombineYAMLs(p.request.ClusterConfig, p.request.ProviderSpecificClusterConfig),
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(loggerFor),
			),
			config.ValidateOptionCommanderMode(p.request.Options.CommanderMode),
			config.ValidateOptionStrictUnmarshal(p.request.Options.CommanderMode),
			config.ValidateOptionValidateExtensions(p.request.Options.CommanderMode),
		)
		if err != nil {
			return fmt.Errorf("parsing cluster meta config: %w", err)
		}
		return nil
	})
	if err != nil {
		return &pb.CheckResult{Err: err.Error()}
	}

	err = loggerFor.LogProcess("default", "Preparing DHCTL state", func() error {
		cachePath := metaConfig.CachePath()
		var initialState phases.DhctlState
		if p.request.State != "" {
			err = json.Unmarshal([]byte(p.request.State), &initialState)
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
	if p.request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(p.request.Options.CommanderUuid)
		if err != nil {
			return &pb.CheckResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           s.params.TmpDir,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		Logger:           loggerFor,
		IsDebug:          s.params.IsDebug,
	})

	checkParams := &check.Params{
		StateCache:    cache.Global(),
		CommanderMode: p.request.Options.CommanderMode,
		CommanderUUID: commanderUUID,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(p.request.ClusterConfig),
			[]byte(p.request.ProviderSpecificClusterConfig),
		),
		InfrastructureContext: infrastructure.NewContextWithProvider(providerGetter, loggerFor),
		Logger:                loggerFor,
		IsDebug:               s.params.IsDebug,
		TmpDir:                s.params.TmpDir,
	}

	kubeClient, sshClient, cleanup, err := helper.InitializeClusterConnections(ctx, helper.ClusterConnectionsOptions{
		CommanderMode: p.request.Options.CommanderMode,
		ApiServerUrl:  p.request.Options.ApiServerUrl,
		ApiServerOptions: helper.ApiServerOptions{
			Token:                    p.request.Options.ApiServerToken,
			InsecureSkipTLSVerify:    p.request.Options.ApiServerInsecureSkipTlsVerify,
			CertificateAuthorityData: util.StringToBytes(p.request.Options.ApiServerCertificateAuthorityData),
		},
		SchemaStore:         s.params.SchemaStore,
		SSHConnectionConfig: p.request.ConnectionConfig,
	})
	cleanuper.Add(cleanup)
	if err != nil {
		return &pb.CheckResult{Err: err.Error()}
	}

	if !govalue.IsNil(sshClient) {
		err = sshClient.Start()
		if err != nil {
			return &pb.CheckResult{Err: err.Error()}
		}
	}

	checkParams.KubeClient = kubeClient
	checkParams.SSHClient = sshClient

	checker := check.NewChecker(checkParams)

	result, cleanProvider, checkErr := checker.Check(ctx)
	defer func() {
		err := cleanProvider()
		if err != nil {
			loggerFor.LogErrorF("Error cleaning up checker: %v\n", err)
		}
	}()

	resultData, marshalErr := json.Marshal(result)
	state, stateErr := extractLastState()

	err = errors.Join(checkErr, marshalErr, stateErr)

	if result != nil {
		// todo: move onCheckResult call to check.Check() func (as in converge)
		_ = onCheckResult(result)
	}

	return &pb.CheckResult{
		Result: string(resultData),
		Err:    util.ErrToString(err),
		State:  string(state),
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
