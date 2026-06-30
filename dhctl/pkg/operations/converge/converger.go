// Copyright 2023 Flant JSC
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

package converge

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	convergectx "github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// TODO(remove-global-app): Support all needed parameters in Params, remove usage of app.*
type Params struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
	KubeProvider           libcon.KubeProvider

	OnPhaseFunc     phases.DefaultOnPhaseFunc
	OnProgressFunc  phases.OnProgressFunc
	ChangesSettings infrastructure.ChangeActionSettings

	CommanderMode bool
	CommanderUUID uuid.UUID
	*commander.CommanderModeParams
	Checker                    *check.Checker
	OnCheckResult              func(context.Context, *check.CheckResult) error
	ApproveDestructiveChangeID string

	InfrastructureContext *infrastructure.Context
	ProviderGetter        infrastructure.CloudProviderGetter

	TmpDir  string
	IsDebug bool

	NoSwitchToNodeUser bool

	CheckHasTerraformStateBeforeMigration bool
	CacheID                               string

	// Options carries the per-operation parsed configuration. RPC handlers
	// must populate this with a fresh *options.Options to avoid sharing global
	// state between concurrent requests.
	Options *options.Options
}

type Converger struct {
	*Params
	PhasedExecutionContext phases.DefaultPhasedExecutionContext

	// TODO(dhctl-for-commander): pass stateCache externally using params as in Destroyer, this variable will be unneeded then
	lastState phases.DhctlState
}

func NewConverger(params *Params) *Converger {
	// if params.CommanderMode {
	// FIXME(dhctl-for-commander): commander uuid currently optional, make it required later
	// if params.CommanderUUID == uuid.Nil {
	//	panic("CommanderUUID required for bootstrap operation in commander mode!")
	// }
	// }

	if params.Options != nil && params.Options.Global.ProgressFilePath != "" {
		params.OnProgressFunc = phases.WriteProgress(params.Options.Global.ProgressFilePath)
	}

	return &Converger{
		Params: params,
		PhasedExecutionContext: phases.NewDefaultPhasedExecutionContext(
			phases.OperationConverge, params.OnPhaseFunc, params.OnProgressFunc,
		),
	}
}

func (c *Converger) ConvergeMigration(ctx context.Context) error {
	{
		// TODO(dhctl-for-commander): pass stateCache externally using params as in the Destroyer, this block will be unneeded then
		state, err := phases.ExtractDhctlState(ctx, cache.Global())
		if err != nil {
			return fmt.Errorf("unable to extract dhctl state: %w", err)
		}
		c.lastState = state
	}

	if !c.CommanderMode {
		if c.CacheID == "" {
			return fmt.Errorf("Incorrect cache identity. You need to pass --ssh-host, --kube-client-from-cluster, or --kubeconfig")
		}

		err := cache.InitWithOptions(ctx, c.CacheID, cache.CacheOptions{Cache: c.Options.Cache})
		if err != nil {
			return fmt.Errorf("unable to initialize cache %s: %w", c.CacheID, err)
		}
	}

	stateCache := cache.Global()

	if err := c.PhasedExecutionContext.InitPipeline(ctx, stateCache); err != nil {
		return err
	}
	c.lastState = nil
	defer func() {
		_ = c.PhasedExecutionContext.Finalize(ctx, stateCache)
	}()

	var convergeCtx *convergectx.Context
	if c.Params.CommanderMode {
		convergeCtx = convergectx.NewCommanderContext(ctx, convergectx.Params{
			KubeProvider:           c.KubeProvider,
			SSHProviderInitializer: c.SSHProviderInitializer,
			Cache:                  stateCache,
			ChangeParams:           c.Params.ChangesSettings,
			ProviderGetter:         c.Params.ProviderGetter,
			Opts:                   &c.Options.Global,
		}, c.Params.CommanderModeParams)
	} else {
		convergeCtx = convergectx.NewContext(ctx, convergectx.Params{
			KubeProvider:           c.KubeProvider,
			SSHProviderInitializer: c.SSHProviderInitializer,
			Cache:                  stateCache,
			ChangeParams:           c.Params.ChangesSettings,
			ProviderGetter:         c.Params.ProviderGetter,
			Opts:                   &c.Options.Global,
		})
	}

	metaConfig, err := convergeCtx.MetaConfig()
	if err != nil {
		return err
	}

	provider, err := convergeCtx.ProviderGetter()(ctx, metaConfig)
	if err != nil {
		return err
	}

	defer func() {
		err := provider.Cleanup()
		if err != nil {
			dhlog.FromContext(ctx).ErrorContext(ctx, fmt.Sprintf("Error cleaning up provider: %v", err))
		}
	}()

	convergeCtx.WithPhaseContext(c.PhasedExecutionContext).
		WithInfrastructureContext(c.Params.InfrastructureContext)

	var inLockRunner *lock.InLockRunner
	// No need for converge-lock in commander mode for bootstrap and converge operations
	if !c.CommanderMode {
		inLockRunner = lock.NewInLockLocalRunner(ctx, convergeCtx, "local-converger", c.Options.SSH.User)
	}

	switcher := convergectx.NewKubeClientSwitcher(convergeCtx, nil, convergectx.KubeClientSwitcherParams{
		TmpDir:        c.TmpDir,
		GlobalOptions: &c.Options.Global,
		DisableSwitch: true,
	})

	convergeCtx.SetClientSwitcher(switcher)

	r := newRunner(inLockRunner, switcher).
		WithCommanderUUID(c.CommanderUUID)

	err = r.RunConvergeMigration(convergeCtx, c.Params.CheckHasTerraformStateBeforeMigration)
	if err != nil {
		return fmt.Errorf("converge problem: %v", err)
	}

	if err := c.PhasedExecutionContext.CompletePipeline(ctx, stateCache); err != nil {
		return err
	}

	return nil
}

func (c *Converger) Converge(ctx context.Context) (*ConvergeResult, error) {
	{
		// TODO(dhctl-for-commander): pass stateCache externally using params as in the Destroyer, this block will be unneeded then
		state, err := phases.ExtractDhctlState(ctx, cache.Global())
		if err != nil {
			return nil, fmt.Errorf("unable to extract dhctl state: %w", err)
		}
		c.lastState = state
	}

	if !c.CommanderMode {
		if c.CacheID == "" {
			return nil, fmt.Errorf("Incorrect cache identity. You need to pass --ssh-host, --kube-client-from-cluster, or --kubeconfig")
		}

		err := cache.InitWithOptions(ctx, c.CacheID, cache.CacheOptions{Cache: c.Options.Cache})
		if err != nil {
			return nil, fmt.Errorf("unable to initialize cache %s: %w", c.CacheID, err)
		}
	}

	interactive := input.IsTerminal() && !c.Options.Global.ShowProgress
	if interactive {
		progressCh, finishProgress := phases.InitProgress(ctx, dhlog.FromContext(ctx), "Converge")
		defer finishProgress()

		onUpdateFunc := func(progress phases.Progress) error {
			// Non-blocking: the pipeline's deferred Finalize can emit after the consumer has
			// stopped and the channel is no longer drained; never block or panic on it.
			select {
			case progressCh <- progress:
			default:
			}
			if c.OnProgressFunc != nil {
				return c.OnProgressFunc(progress)
			}

			return nil
		}

		c.PhasedExecutionContext = phases.NewDefaultPhasedExecutionContext(phases.OperationConverge, c.OnPhaseFunc, onUpdateFunc)
	}

	stateCache := cache.Global()

	if err := c.PhasedExecutionContext.InitPipeline(ctx, stateCache); err != nil {
		return nil, err
	}
	c.lastState = nil
	defer func() {
		_ = c.PhasedExecutionContext.Finalize(ctx, stateCache)
	}()

	hasTerraformState := false

	var convergeCtx *convergectx.Context
	if c.Params.CommanderMode {
		convergeCtx = convergectx.NewCommanderContext(ctx, convergectx.Params{
			KubeProvider:           c.KubeProvider,
			SSHProviderInitializer: c.SSHProviderInitializer,
			Cache:                  stateCache,
			ChangeParams:           c.Params.ChangesSettings,
			ProviderGetter:         c.ProviderGetter,
			Opts:                   &c.Options.Global,
		}, c.Params.CommanderModeParams)
	} else {
		convergeCtx = convergectx.NewContext(ctx, convergectx.Params{
			KubeProvider:           c.KubeProvider,
			SSHProviderInitializer: c.SSHProviderInitializer,
			Cache:                  stateCache,
			ChangeParams:           c.Params.ChangesSettings,
			ProviderGetter:         c.ProviderGetter,
			Opts:                   &c.Options.Global,
		})
	}

	metaConfig, err := convergeCtx.MetaConfig()
	if err != nil {
		return nil, err
	}

	c.PhasedExecutionContext.SetClusterConfig(phases.ClusterConfig{ClusterType: metaConfig.ClusterType})

	if c.CommanderMode {
		c.Checker.SetExternalPhasedContext(c.PhasedExecutionContext)

		if shouldStop, err := c.PhasedExecutionContext.StartPhase(ctx, phases.ConvergeCheckPhase, false, stateCache); err != nil {
			return nil, fmt.Errorf("unable to switch phase: %w", err)
		} else if shouldStop {
			return nil, nil
		}

		checkRes, cleaner, err := c.Checker.Check(ctx)
		// we cannot use provider cleanup here because we do not have metaconfig here
		cleanWithLog := func(err error) error {
			cleanErr := cleaner()
			if cleanErr != nil {
				dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Cannot cleanup after check: %v; prev error: %v", cleanErr, err))
				return fmt.Errorf("%v: %v", err, cleanErr)
			}
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Cleaning up after check succeeded: %v", err))
			return err
		}

		if err != nil {
			return nil, cleanWithLog(fmt.Errorf("check failed: %w", err))
		}

		hasTerraformState = checkRes.HasTerraformState

		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Has terraform state: %v", hasTerraformState))

		if c.Params.OnCheckResult != nil {
			if err := c.Params.OnCheckResult(ctx, checkRes); err != nil {
				return nil, cleanWithLog(err)
			}
		}

		switch checkRes.Status {
		case check.CheckStatusInSync:
			// No converge needed, exit immediately
			return &ConvergeResult{
				Status: ConvergeStatusInSync,
			}, cleanWithLog(nil)

		case check.CheckStatusOutOfSync:
			// Proceed converge operation

		case check.CheckStatusDestructiveOutOfSync:
			destructiveChangeApproved := c.Params.ApproveDestructiveChangeID == checkRes.DestructiveChangeID

			if !destructiveChangeApproved {
				// Terminate converge with check result
				return &ConvergeResult{
					Status:      ConvergeStatusNeedApproveForDestructiveChange,
					CheckResult: checkRes,
				}, cleanWithLog(nil)
			}
		}
	}

	needAutomaticTofuMigrationForCommander := false

	if c.ProviderGetter == nil {
		return nil, fmt.Errorf("Provider getter not set")
	}

	provider, err := c.ProviderGetter(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	defer func() {
		err := provider.Cleanup()
		if err != nil {
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Cannot cleanup provider after converge: %v", err))
		}
	}()

	if provider.NeedToUseTofu() {
		needAutomaticTofuMigrationForCommander = hasTerraformState && c.CommanderMode
		if !c.CommanderMode {
			convergeCtx.WithStateChecker(infrastructurestate.AskCanIConvergeTerraformStateWhenWeUseTofu)
		}
	}

	convergeCtx.WithPhaseContext(c.PhasedExecutionContext).
		WithInfrastructureContext(c.Params.InfrastructureContext)

	var inLockRunner *lock.InLockRunner
	// No need for converge-lock in commander mode for bootstrap and converge operations
	if !c.CommanderMode {
		inLockRunner = lock.NewInLockLocalRunner(ctx, convergeCtx, "local-converger", c.Options.SSH.User)
	}

	kubectlSwitcher := convergectx.NewKubeClientSwitcher(convergeCtx, inLockRunner, convergectx.KubeClientSwitcherParams{
		TmpDir:        c.TmpDir,
		GlobalOptions: &c.Options.Global,
		IsDebug:       c.IsDebug,
		DisableSwitch: c.NoSwitchToNodeUser,
	})

	convergeCtx.SetClientSwitcher(kubectlSwitcher)

	phasesToSkip := make([]phases.OperationPhase, 0)
	if !c.CommanderMode {
		phasesToSkip = []phases.OperationPhase{phases.DeckhouseConfigurationPhase}
	}

	r := newRunner(inLockRunner, kubectlSwitcher).
		WithCommanderUUID(c.CommanderUUID).
		WithSkipPhases(phasesToSkip)

	if c.CommanderMode {
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Need automatic migration for commander: %v", needAutomaticTofuMigrationForCommander))
	}

	if needAutomaticTofuMigrationForCommander {
		dhlog.FromContext(ctx).WarnContext(ctx, "Need to migrate to opentofu. Switching to migrator")
		err = r.RunConvergeMigration(convergeCtx, true)
	} else {
		err = r.RunConverge(convergeCtx)
	}

	if err != nil {
		return nil, fmt.Errorf("converge problem: %v", err)
	}

	if err := c.PhasedExecutionContext.CompletePipeline(ctx, stateCache); err != nil {
		return nil, err
	}

	return &ConvergeResult{
		Status: ConvergeStatusConverged,
	}, nil
}

func (c *Converger) AutoConverge(ctx context.Context, listenAddress string, checkInterval time.Duration) error {
	if c.Options == nil || c.Options.AutoConverge.RunningNodeName == "" {
		return fmt.Errorf("Need to pass the running node name. It may taint the infrastructure state during converge")
	}

	// Preserve the non-cancelable base for the long-running auto-converge loop, but carry the
	// slog root from the request ctx so logging keeps its file + TTY sinks.
	convergeCtx := convergectx.NewContext(dhlog.ToContext(context.Background(), dhlog.FromContext(ctx)), convergectx.Params{
		KubeProvider:           c.KubeProvider,
		SSHProviderInitializer: c.SSHProviderInitializer,
		Cache:                  cache.Global(),
		ChangeParams:           c.Params.ChangesSettings,
		ProviderGetter:         c.ProviderGetter,
		Opts:                   &c.Options.Global,
	})

	metaConfig, err := convergeCtx.MetaConfig()
	if err != nil {
		return err
	}

	if c.ProviderGetter == nil {
		return fmt.Errorf("Provider getter not set")
	}

	// todo flexible autoconverger provider getter
	providersGetterCtx, cancel := convergeCtx.WithTimeout(10 * time.Second)

	provider, err := c.ProviderGetter(providersGetterCtx, metaConfig)
	cancel()
	if err != nil {
		return err
	}

	if provider.NeedToUseTofu() {
		convergeCtx.WithStateChecker(infrastructurestate.CheckCanIConvergeTerraformStateWhenWeUseTofu)
	}

	convergeCtx.WithPhaseContext(c.PhasedExecutionContext).
		WithInfrastructureContext(c.Params.InfrastructureContext)

	inLockRunner := lock.NewInLockRunner(convergeCtx.Ctx(), convergeCtx, lock.AutoConvergerIdentity, c.Options.SSH.User).
		// never force lock
		WithForceLock(false)

	c.Options.Bootstrap.DeckhouseTimeout = 1 * time.Hour

	switcher := convergectx.NewKubeClientSwitcher(convergeCtx, inLockRunner, convergectx.KubeClientSwitcherParams{
		TmpDir:        c.TmpDir,
		GlobalOptions: &c.Options.Global,
		IsDebug:       c.IsDebug,
	})

	convergeCtx.SetClientSwitcher(switcher)

	r := newRunner(inLockRunner, switcher).
		WithCommanderUUID(c.CommanderUUID).
		WithExcludedNodes([]string{c.Options.AutoConverge.RunningNodeName}).
		WithSkipPhases([]phases.OperationPhase{phases.AllNodesPhase, phases.DeckhouseConfigurationPhase})

	converger := NewAutoConverger(r, AutoConvergerParams{
		ListenAddress: listenAddress,
		CheckInterval: checkInterval,
		TmpDir:        c.TmpDir,
	})

	return converger.Start(convergeCtx)
}

func (c *Converger) GetLastState() phases.DhctlState {
	if c.lastState != nil {
		return c.lastState
	} else {
		return c.PhasedExecutionContext.GetLastState()
	}
}
