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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	convergectx "github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

// TODO(remove-global-app): Support all needed parameters in Params, remove usage of app.*
type Params struct {
	SSHClient  node.SSHClient
	KubeClient *client.KubernetesClient // optional

	OnPhaseFunc     phases.DefaultOnPhaseFunc
	OnProgressFunc  phases.OnProgressFunc
	ChangesSettings infrastructure.ChangeActionSettings

	*client.KubernetesInitParams

	CommanderMode bool
	CommanderUUID uuid.UUID
	*commander.CommanderModeParams
	Checker                    *check.Checker
	OnCheckResult              func(*check.CheckResult) error
	ApproveDestructiveChangeID string

	InfrastructureContext *infrastructure.Context
	ProviderGetter        infrastructure.CloudProviderGetter

	TmpDir  string
	Logger  log.Logger
	IsDebug bool

	CheckHasTerraformStateBeforeMigration bool
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

	if app.ProgressFilePath != "" {
		params.OnProgressFunc = phases.WriteProgress(app.ProgressFilePath)
	}

	return &Converger{
		Params: params,
		PhasedExecutionContext: phases.NewDefaultPhasedExecutionContext(
			phases.OperationConverge, params.OnPhaseFunc, params.OnProgressFunc,
		),
	}
}

// TODO(remove-global-app): Eliminate usage of app.* global variables,
// TODO(remove-global-app):  use explicitly passed params everywhere instead,
// TODO(remove-global-app):  applyParams will not be needed anymore then.
//
// applyParams overrides app.* options that are explicitly passed using Params struct
func (c *Converger) applyParams() error {
	if c.KubernetesInitParams != nil {
		app.KubeConfigInCluster = c.KubernetesInitParams.KubeConfigInCluster
		app.KubeConfig = c.KubernetesInitParams.KubeConfig
		app.KubeConfigContext = c.KubernetesInitParams.KubeConfigContext
	}
	return nil
}

func (c *Converger) ConvergeMigration(ctx context.Context) error {
	{
		// TODO(dhctl-for-commander): pass stateCache externally using params as in the Destroyer, this block will be unneeded then
		state, err := phases.ExtractDhctlState(cache.Global())
		if err != nil {
			return fmt.Errorf("unable to extract dhctl state: %w", err)
		}
		c.lastState = state
	}

	if c.Params.SSHClient != nil {
		defer c.Params.SSHClient.Stop()
	}

	if err := c.applyParams(); err != nil {
		return err
	}

	var err error
	var kubeCl *client.KubernetesClient

	if c.KubeClient != nil {
		kubeCl = c.KubeClient
	} else {
		var sshClient node.SSHClient

		if err := terminal.AskBecomePassword(); err != nil {
			return err
		}
		if err := terminal.AskBastionPassword(); err != nil {
			return err
		}

		sshClient, err := sshclient.NewInitClientFromFlags(false)
		if err != nil {
			return err
		}

		if err != nil {
			return err
		}

		kubeCl = client.NewKubernetesClient().WithNodeInterface(ssh.NewNodeInterfaceWrapper(sshClient))
		if err := kubeCl.Init(client.AppKubernetesInitParams()); err != nil {
			return err
		}
	}

	if !c.CommanderMode {
		cacheIdentity := ""
		if app.KubeConfigInCluster {
			cacheIdentity = "in-cluster"
		}
		if c.SSHClient != nil {
			cacheIdentity = c.SSHClient.Check().String()
		}
		if app.KubeConfig != "" {
			cacheIdentity = cache.GetCacheIdentityFromKubeconfig(
				app.KubeConfig,
				app.KubeConfigContext,
			)
		}
		if cacheIdentity == "" {
			return fmt.Errorf("Incorrect cache identity. Need to pass --ssh-host or --kube-client-from-cluster or --kubeconfig")
		}

		err = cache.InitWithOptions(cacheIdentity, cache.CacheOptions{})
		if err != nil {
			return fmt.Errorf("unable to initialize cache %s: %w", cacheIdentity, err)
		}
	}

	stateCache := cache.Global()

	if err := c.PhasedExecutionContext.InitPipeline(stateCache); err != nil {
		return err
	}
	c.lastState = nil
	defer c.PhasedExecutionContext.Finalize(stateCache)

	var convergeCtx *convergectx.Context
	if c.Params.CommanderMode {
		convergeCtx = convergectx.NewCommanderContext(ctx, convergectx.Params{
			KubeClient:     kubeCl,
			Cache:          stateCache,
			ChangeParams:   c.Params.ChangesSettings,
			ProviderGetter: c.Params.ProviderGetter,
			Logger:         c.Logger,
		}, c.Params.CommanderModeParams)
	} else {
		convergeCtx = convergectx.NewContext(ctx, convergectx.Params{
			KubeClient:     kubeCl,
			Cache:          stateCache,
			ChangeParams:   c.Params.ChangesSettings,
			ProviderGetter: c.Params.ProviderGetter,
			Logger:         c.Logger,
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
			c.Logger.LogErrorF("Error cleaning up provider: %v\n", err)
		}
	}()

	convergeCtx.WithPhaseContext(c.PhasedExecutionContext).
		WithInfrastructureContext(c.Params.InfrastructureContext)

	var inLockRunner *lock.InLockRunner
	// No need for converge-lock in commander mode for bootstrap and converge operations
	if !c.CommanderMode {
		inLockRunner = lock.NewInLockLocalRunner(convergeCtx, "local-converger")
	}

	r := newRunner(inLockRunner, nil).
		WithCommanderUUID(c.CommanderUUID)

	err = r.RunConvergeMigration(convergeCtx, c.Params.CheckHasTerraformStateBeforeMigration)
	if err != nil {
		return fmt.Errorf("converge problem: %v", err)
	}

	if err := c.PhasedExecutionContext.CompletePipeline(stateCache); err != nil {
		return err
	}

	return nil
}

func (c *Converger) Converge(ctx context.Context) (*ConvergeResult, error) {
	{
		// TODO(dhctl-for-commander): pass stateCache externally using params as in the Destroyer, this block will be unneeded then
		state, err := phases.ExtractDhctlState(cache.Global())
		if err != nil {
			return nil, fmt.Errorf("unable to extract dhctl state: %w", err)
		}
		c.lastState = state
	}

	if err := c.applyParams(); err != nil {
		return nil, err
	}

	var err error
	var kubeCl *client.KubernetesClient

	if c.KubeClient != nil {
		kubeCl = c.KubeClient
	} else {
		if c.SSHClient == nil {
			return nil, fmt.Errorf("Not enough flags were passed to perform the operation.\nUse dhctl converge --help to get available flags.\nSsh host is not provided. Need to pass --ssh-host, or specify SSHHost manifest in the --connection-config file")
		}

		kubeCl, err = kubernetes.ConnectToKubernetesAPI(ctx, ssh.NewNodeInterfaceWrapper(c.SSHClient))
		if err != nil {
			return nil, fmt.Errorf("unable to connect to Kubernetes over ssh tunnel: %w", err)
		}
	}

	if !c.CommanderMode {
		cacheIdentity := ""
		if app.KubeConfigInCluster {
			cacheIdentity = "in-cluster"
		}
		if c.SSHClient != nil {
			cacheIdentity = c.SSHClient.Check().String()
		}
		if app.KubeConfig != "" {
			cacheIdentity = cache.GetCacheIdentityFromKubeconfig(
				app.KubeConfig,
				app.KubeConfigContext,
			)
		}
		if cacheIdentity == "" {
			return nil, fmt.Errorf("Incorrect cache identity. Need to pass --ssh-host or --kube-client-from-cluster or --kubeconfig")
		}

		err = cache.InitWithOptions(cacheIdentity, cache.CacheOptions{})
		if err != nil {
			return nil, fmt.Errorf("unable to initialize cache %s: %w", cacheIdentity, err)
		}
	}

	hasTerraformState := false

	if c.CommanderMode {
		checkRes, cleaner, err := c.Checker.Check(ctx)
		// we cannot use provider cleanup here because we do not have metaconfig here
		cleanWithLog := func(err error) error {
			cleanErr := cleaner()
			if cleanErr != nil {
				c.Logger.LogWarnF("Cannot cleanup after check: %v; prev error: %v\n", cleanErr, err)
				return fmt.Errorf("%v: %v", err, cleanErr)
			}
			c.Logger.LogDebugF("Cleaning up after check succeeded: %v\n", err)
			return err
		}

		if err != nil {
			return nil, cleanWithLog(fmt.Errorf("check failed: %w", err))
		}

		hasTerraformState = checkRes.HasTerraformState

		log.InfoF("Has terraform state: %v\n", hasTerraformState)

		if c.Params.OnCheckResult != nil {
			if err := c.Params.OnCheckResult(checkRes); err != nil {
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

	stateCache := cache.Global()

	if err := c.PhasedExecutionContext.InitPipeline(stateCache); err != nil {
		return nil, err
	}
	c.lastState = nil
	defer c.PhasedExecutionContext.Finalize(stateCache)

	var convergeCtx *convergectx.Context
	if c.Params.CommanderMode {
		convergeCtx = convergectx.NewCommanderContext(ctx, convergectx.Params{
			KubeClient:     kubeCl,
			Cache:          stateCache,
			ChangeParams:   c.Params.ChangesSettings,
			ProviderGetter: c.ProviderGetter,
			Logger:         c.Logger,
		}, c.Params.CommanderModeParams)
	} else {
		convergeCtx = convergectx.NewContext(ctx, convergectx.Params{
			KubeClient:     kubeCl,
			Cache:          stateCache,
			ChangeParams:   c.Params.ChangesSettings,
			ProviderGetter: c.ProviderGetter,
			Logger:         c.Logger,
		})
	}

	metaConfig, err := convergeCtx.MetaConfig()
	if err != nil {
		return nil, err
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
			c.Logger.LogWarnF("Cannot cleanup provider after converge: %v\n", err)
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
		inLockRunner = lock.NewInLockLocalRunner(convergeCtx, "local-converger")
	}

	kubectlSwitcher := convergectx.NewKubeClientSwitcher(convergeCtx, inLockRunner, convergectx.KubeClientSwitcherParams{
		TmpDir:  c.TmpDir,
		Logger:  c.Logger,
		IsDebug: c.IsDebug,
	})

	phasesToSkip := make([]phases.OperationPhase, 0)
	if !c.CommanderMode {
		phasesToSkip = []phases.OperationPhase{phases.DeckhouseConfigurationPhase}
	}

	r := newRunner(inLockRunner, kubectlSwitcher).
		WithCommanderUUID(c.CommanderUUID).
		WithSkipPhases(phasesToSkip)

	if c.CommanderMode {
		log.InfoF("Need automatic migration for commander: %v\n", needAutomaticTofuMigrationForCommander)
	}

	if needAutomaticTofuMigrationForCommander {
		log.WarnF("Need migrate to opentofu. Switch to migrator\n")
		err = r.RunConvergeMigration(convergeCtx, true)
	} else {
		err = r.RunConverge(convergeCtx)
	}

	if err != nil {
		return nil, fmt.Errorf("converge problem: %v", err)
	}

	if err := c.PhasedExecutionContext.CompletePipeline(stateCache); err != nil {
		return nil, err
	}

	return &ConvergeResult{
		Status: ConvergeStatusConverged,
	}, nil
}

func (c *Converger) AutoConverge() error {
	if err := c.applyParams(); err != nil {
		return err
	}

	if app.RunningNodeName == "" {
		return fmt.Errorf("Need to pass running node name. It is may taints infrastructure state while converge")
	}

	var err error
	var kubeCl *client.KubernetesClient

	if c.KubeClient != nil {
		kubeCl = c.KubeClient
	} else {
		if err := terminal.AskBecomePassword(); err != nil {
			return err
		}
		if err := terminal.AskBastionPassword(); err != nil {
			return err
		}

		sshClient, err := sshclient.NewInitClientFromFlags(false)
		if err != nil {
			return err
		}

		kubeCl = client.NewKubernetesClient().WithNodeInterface(ssh.NewNodeInterfaceWrapper(sshClient))
		if err := kubeCl.Init(client.AppKubernetesInitParams()); err != nil {
			return err
		}
	}

	var convergeCtx *convergectx.Context
	convergeCtx = convergectx.NewContext(context.Background(), convergectx.Params{
		KubeClient:     kubeCl,
		Cache:          cache.Global(),
		ChangeParams:   c.Params.ChangesSettings,
		Logger:         c.Logger,
		ProviderGetter: c.ProviderGetter,
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

	inLockRunner := lock.NewInLockRunner(convergeCtx, lock.AutoConvergerIdentity).
		// never force lock
		WithForceLock(false)

	app.DeckhouseTimeout = 1 * time.Hour

	r := newRunner(inLockRunner, convergectx.NewKubeClientSwitcher(convergeCtx, inLockRunner, convergectx.KubeClientSwitcherParams{
		TmpDir:  c.TmpDir,
		Logger:  c.Logger,
		IsDebug: c.IsDebug,
	})).
		WithCommanderUUID(c.CommanderUUID).
		WithExcludedNodes([]string{app.RunningNodeName}).
		WithSkipPhases([]phases.OperationPhase{phases.AllNodesPhase, phases.DeckhouseConfigurationPhase})

	converger := NewAutoConverger(r, app.AutoConvergeListenAddress, app.ApplyInterval)
	return converger.Start(convergeCtx)
}

func (c *Converger) GetLastState() phases.DhctlState {
	if c.lastState != nil {
		return c.lastState
	} else {
		return c.PhasedExecutionContext.GetLastState()
	}
}
