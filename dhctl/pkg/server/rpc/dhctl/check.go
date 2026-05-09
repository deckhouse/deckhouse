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

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
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
	request      *pb.CheckStart
	sendProgress phases.OnProgressFunc
	sendCh       chan *pb.CheckResponse
}

func (p *checkParams) loggerWidth() int {
	return int(p.request.Options.LogWidth)
}

func (p *checkParams) loggerOptions(ctx context.Context) logger.Options {
	return initLoggerOptions(ctx, &initLoggerOptionsParams[*pb.CheckResponse]{
		sendCh: p.sendCh,
		consumer: func(lines []string) *pb.CheckResponse {
			return &pb.CheckResponse{Message: &pb.CheckResponse_Logs{Logs: &pb.Logs{Logs: lines}}}
		},
		attributesProvider: p,
	})
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
	pt := progressTracker[*pb.CheckResponse]{
		sendCh: sendCh,
		dataFunc: func(progress phases.Progress) *pb.CheckResponse {
			return &pb.CheckResponse{Message: &pb.CheckResponse_Progress{Progress: convertProgress(progress)}}
		},
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
					result := s.checkSafe(ctx, &checkParams{
						request:      message.Start,
						sendProgress: pt.sendProgress(),
						sendCh:       sendCh,
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

// keep named return to keep same defered recover behavior
//
//nolint:nonamedreturns
func (s *Service) checkSafe(ctx context.Context, p *checkParams) (result *pb.CheckResult) {
	defer func() {
		if r := recover(); r != nil {
			lastState, err := panicResult(ctx, r)
			result = &pb.CheckResult{State: string(lastState), Err: err.Error()}
		}
	}()
	return s.check(ctx, p)
}

func (s *Service) check(ctx context.Context, p *checkParams) *pb.CheckResult {
	var err error

	cleanuper := callback.NewCallback()
	defer func() { _ = cleanuper.Call() }()

	loggerFor := initDhctlLogger(ctx, p)

	opts := newRequestOptions(
		s.params.CacheDir,
		p.request.Options.CommonOptions.SkipPreflightChecks,
		p.request.Options.ResourcesTimeout.AsDuration(),
		p.request.Options.DeckhouseTimeout.AsDuration(),
	)

	logBeforeExit := logInformationAboutInstance(s.params, loggerFor)
	defer logBeforeExit()

	var metaConfig *config.MetaConfig
	err = loggerFor.LogProcessCtx(ctx, "default", "Parsing cluster config", func(ctx context.Context) error {
		metaConfig, err = config.ParseConfigFromData(
			ctx,
			input.CombineYAMLs(p.request.ClusterConfig, p.request.ProviderSpecificClusterConfig),
			infrastructureprovider.MetaConfigPreparatorProvider(
				infrastructureprovider.NewPreparatorProviderParams(loggerFor),
			),
			s.params.DownloadDirConfig,
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

	err = loggerFor.LogProcessCtx(ctx, "default", "Preparing DHCTL state", func(ctx context.Context) error {
		cachePath := metaConfig.CachePath()

		var initialState phases.DhctlState
		if p.request.State != "" {
			err = json.Unmarshal([]byte(p.request.State), &initialState)
			if err != nil {
				return fmt.Errorf("unmarshalling dhctl state: %w", err)
			}
		}

		err = cache.InitWithOptions(
			ctx,
			cachePath,
			cache.CacheOptions{InitialState: initialState, ResetInitialState: true, Cache: opts.Cache},
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
		DownloadDir:      s.params.DownloadDirConfig.DownloadDir,
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
		InfrastructureContext: infrastructure.NewContextWithProvider(providerGetter, loggerFor).
			WithUseTfCache(opts.Cache.UseTfCache).
			WithDebug(s.params.IsDebug),
		Logger:         loggerFor,
		IsDebug:        s.params.IsDebug,
		TmpDir:         s.params.TmpDir,
		OnPhaseFunc:    func(data phases.OnPhaseFuncData[phases.DefaultContextType]) error { return nil },
		OnProgressFunc: p.sendProgress,
		Options:        opts,
	}

	var kubeProvider libcon.KubeProvider
	err = loggerFor.LogProcess("default", "Preparing SSH client", func() error {
		var cleanup func() error
		_, kubeProvider, cleanup, err = helper.CreateProviders(ctx, p.request.ConnectionConfig, loggerFor, s.params.IsDebug, s.params.TmpDir)
		cleanuper.Add(cleanup)
		if err != nil {
			return fmt.Errorf("creating provider: %w", err)
		}

		return nil
	})
	if err != nil {
		return &pb.CheckResult{Err: err.Error()}
	}

	checkParams.KubeProvider = kubeProvider

	checker := check.NewChecker(checkParams)

	result, cleanProvider, checkErr := checker.Check(ctx)
	defer func() {
		err := cleanProvider()
		if err != nil {
			loggerFor.LogErrorF("Error cleaning up checker: %v\n", err)
		}
	}()

	resultData, marshalErr := json.Marshal(result)
	state, stateErr := extractLastState(ctx)

	err = errors.Join(checkErr, marshalErr, stateErr)

	if result != nil {
		// todo: move onCheckResult call to check.Check() func (as in converge)
		_ = onCheckResult(ctx, result)
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
