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
	"reflect"

	"github.com/google/uuid"
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
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander/detach"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/fsm"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util/callback"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type detachParams struct {
	request    *pb.CommanderDetachStart
	logOptions logger.Options
}

func (s *Service) CommanderDetach(server pb.DHCTL_CommanderDetachServer) error {
	ctx, cancel := operationCtx(server)
	defer cancel()

	logger.L(ctx).Info("started")

	f := fsm.New("initial", s.commanderDetachServerTransitions())

	doneCh := make(chan struct{})
	internalErrCh := make(chan error)
	receiveCh := make(chan *pb.CommanderDetachRequest)
	sendCh := make(chan *pb.CommanderDetachResponse)

	loggerDefault := logger.L(ctx).With(logTypeDHCTL)

	logWriter := logger.NewLogWriter(loggerDefault, sendCh,
		func(lines []string) *pb.CommanderDetachResponse {
			return &pb.CommanderDetachResponse{Message: &pb.CommanderDetachResponse_Logs{Logs: &pb.Logs{Logs: lines}}}
		},
	)

	debugWriter := logger.NewDebugLogWriter(loggerDefault)

	logOptions := logger.Options{
		DebugWriter:   debugWriter,
		DefaultWriter: logWriter,
	}

	startReceiver[*pb.CommanderDetachRequest, *pb.CommanderDetachResponse](server, receiveCh, doneCh, internalErrCh)
	startSender[*pb.CommanderDetachRequest, *pb.CommanderDetachResponse](server, sendCh, internalErrCh)

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
				go func() {
					result := s.commanderDetachSafe(ctx, detachParams{
						request:    message.Start,
						logOptions: logOptions,
					})
					sendCh <- &pb.CommanderDetachResponse{Message: &pb.CommanderDetachResponse_Result{Result: result}}
				}()

			case *pb.CommanderDetachRequest_Cancel:
				cancel()

			default:
				logger.L(ctx).Error("got unprocessable message",
					slog.String("message", fmt.Sprintf("%T", message)))
				continue connectionProcessor
			}
		}
	}
}

func (s *Service) commanderDetachSafe(ctx context.Context, p detachParams) (result *pb.CommanderDetachResult) {
	defer func() {
		if r := recover(); r != nil {
			lastState, err := panicResult(ctx, r)
			result = &pb.CommanderDetachResult{State: string(lastState), Err: err.Error()}
		}
	}()

	return s.commanderDetach(ctx, p)
}

func (s *Service) commanderDetach(ctx context.Context, p detachParams) *pb.CommanderDetachResult {
	var err error

	cleanuper := callback.NewCallback()
	defer func() { _ = cleanuper.Call() }()

	log.InitLoggerWithOptions("pretty", log.LoggerOptions{
		OutStream:   p.logOptions.DefaultWriter,
		Width:       int(p.request.Options.LogWidth),
		DebugStream: p.logOptions.DebugWriter,
	})
	app.SanityCheck = true
	app.UseTfCache = app.UseStateCacheYes
	app.ResourcesTimeout = p.request.Options.ResourcesTimeout.AsDuration()
	app.DeckhouseTimeout = p.request.Options.DeckhouseTimeout.AsDuration()
	app.CacheDir = s.params.CacheDir
	app.ApplyPreflightSkips(p.request.Options.CommonOptions.SkipPreflightChecks)

	log.InfoF("Task is running by DHCTL Server pod/%s\n", s.params.PodName)
	defer func() { log.InfoF("Task done by DHCTL Server pod/%s\n", s.params.PodName) }()

	var metaConfig *config.MetaConfig
	err = log.Process("default", "Parsing cluster config", func() error {
		metaConfig, err = config.ParseConfigFromData(
			ctx,
			input.CombineYAMLs(p.request.ClusterConfig, p.request.ProviderSpecificClusterConfig),
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(log.GetDefaultLogger()),
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
		return &pb.CommanderDetachResult{Err: err.Error()}
	}

	err = log.Process("default", "Preparing DHCTL state", func() error {
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
		return &pb.CommanderDetachResult{Err: err.Error()}
	}

	var sshClient node.SSHClient
	err = log.Process("default", "Preparing SSH client", func() error {
		connectionConfig, err := config.ParseConnectionConfig(
			p.request.ConnectionConfig,
			s.params.SchemaStore,
			config.ValidateOptionCommanderMode(p.request.Options.CommanderMode),
			config.ValidateOptionStrictUnmarshal(p.request.Options.CommanderMode),
			config.ValidateOptionValidateExtensions(p.request.Options.CommanderMode),
		)
		if err != nil {
			return fmt.Errorf("parsing connection config: %w", err)
		}

		var cleanup func() error
		sshClient, cleanup, err = helper.CreateSSHClient(connectionConfig)
		cleanuper.Add(cleanup)
		if err != nil {
			return fmt.Errorf("preparing ssh client: %w", err)
		}

		if sshClient != nil && !reflect.ValueOf(sshClient).IsNil() {
			err = sshClient.Start()
			if err != nil {
				return fmt.Errorf("cannot start sshClient: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return &pb.CommanderDetachResult{Err: err.Error()}
	}

	var commanderUUID uuid.UUID
	if p.request.Options.CommanderUuid != "" {
		commanderUUID, err = uuid.Parse(p.request.Options.CommanderUuid)
		if err != nil {
			return &pb.CommanderDetachResult{Err: fmt.Errorf("unable to parse commander uuid: %w", err).Error()}
		}
	}

	stateCache := cache.Global()
	loggerFor := log.GetDefaultLogger()

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           s.params.TmpDir,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		Logger:           loggerFor,
		IsDebug:          s.params.IsDebug,
	})

	checker := check.NewChecker(&check.Params{
		SSHClient:     sshClient,
		StateCache:    stateCache,
		CommanderMode: p.request.Options.CommanderMode,
		CommanderUUID: commanderUUID,
		CommanderModeParams: commander.NewCommanderModeParams(
			[]byte(p.request.ClusterConfig),
			[]byte(p.request.ProviderSpecificClusterConfig),
		),
		InfrastructureContext: infrastructure.NewContextWithProvider(providerGetter, loggerFor),
		TmpDir:                s.params.TmpDir,
		Logger:                loggerFor,
		IsDebug:               s.params.IsDebug,
	})

	detacher := detach.NewDetacher(checker, sshClient, detach.DetacherOptions{
		CreateDetachResources: detach.DetachResources{
			Template: p.request.CreateResourcesTemplate,
			Values:   p.request.CreateResourcesValues.AsMap(),
		},
		DeleteDetachResources: detach.DetachResources{
			Template: p.request.DeleteResourcesTemplate,
			Values:   p.request.DeleteResourcesValues.AsMap(),
		},
		OnCheckResult: onCheckResult,
	})

	detachErr := detacher.Detach(ctx)
	state, stateErr := extractLastState()

	err = errors.Join(detachErr, stateErr)

	return &pb.CommanderDetachResult{State: string(state), Err: util.ErrToString(err)}
}

func (s *Service) commanderDetachServerTransitions() []fsm.Transition {
	return []fsm.Transition{
		{
			Event:       "start",
			Sources:     []fsm.State{"initial"},
			Destination: "running",
		},
	}
}
