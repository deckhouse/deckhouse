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
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	convergectx "github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

// TODO(remove-global-app): Support all needed parameters in Params, remove usage of app.*
type Params struct {
	SSHClient  node.SSHClient
	KubeClient *client.KubernetesClient // optional

	OnPhaseFunc            phases.DefaultOnPhaseFunc
	AutoDismissDestructive bool
	AutoApprove            bool

	*client.KubernetesInitParams

	CommanderMode bool
	CommanderUUID uuid.UUID
	*commander.CommanderModeParams
	Checker                    *check.Checker
	OnCheckResult              func(*check.CheckResult) error
	ApproveDestructiveChangeID string

	TerraformContext *terraform.TerraformContext
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

	return &Converger{
		Params:                 params,
		PhasedExecutionContext: phases.NewDefaultPhasedExecutionContext(params.OnPhaseFunc),
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

func (c *Converger) Converge(ctx context.Context) (*ConvergeResult, error) {
	{
		// TODO(dhctl-for-commander): pass stateCache externally using params as in the Destroyer, this block will be unneeded then
		state, err := phases.ExtractDhctlState(cache.Global())
		if err != nil {
			return nil, fmt.Errorf("unable to extract dhctl state: %w", err)
		}
		c.lastState = state
	}

	defer c.Params.SSHClient.Stop()

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

	if c.CommanderMode {
		checkRes, err := c.Checker.Check(ctx)
		if err != nil {
			return nil, fmt.Errorf("check failed: %w", err)
		}

		if c.Params.OnCheckResult != nil {
			if err := c.Params.OnCheckResult(checkRes); err != nil {
				return nil, err
			}
		}

		switch checkRes.Status {
		case check.CheckStatusInSync:
			// No converge needed, exit immediately
			return &ConvergeResult{
				Status: ConvergeStatusInSync,
			}, nil

		case check.CheckStatusOutOfSync:
			// Proceed converge operation

		case check.CheckStatusDestructiveOutOfSync:
			destructiveChangeApproved := c.Params.ApproveDestructiveChangeID == checkRes.DestructiveChangeID

			if !destructiveChangeApproved {
				// Terminate converge with check result
				return &ConvergeResult{
					Status:      ConvergeStatusNeedApproveForDestructiveChange,
					CheckResult: checkRes,
				}, nil
			}
		}
	}

	stateCache := cache.Global()

	if err := c.PhasedExecutionContext.InitPipeline(stateCache); err != nil {
		return nil, err
	}
	c.lastState = nil
	defer c.PhasedExecutionContext.Finalize(stateCache)

	changesSettings := terraform.ChangeActionSettings{
		AutoDismissDestructive: c.AutoDismissDestructive,
		AutoApprove:            c.AutoApprove,
	}

	var convergeCtx *convergectx.Context
	if c.Params.CommanderMode {
		convergeCtx = convergectx.NewCommanderContext(ctx, kubeCl, stateCache, c.Params.CommanderModeParams, changesSettings)
	} else {
		convergeCtx = convergectx.NewContext(ctx, kubeCl, stateCache, changesSettings)
	}

	convergeCtx.WithPhaseContext(c.PhasedExecutionContext).
		WithTerraformContext(c.Params.TerraformContext)

	var inLockRunner *lock.InLockRunner
	// No need for converge-lock in commander mode for bootstrap and converge operations
	if !c.CommanderMode {
		inLockRunner = lock.NewInLockLocalRunner(convergeCtx, "local-converger")
	}

	kubectlSwitcher := convergectx.NewKubeClientSwitcher(convergeCtx, inLockRunner)

	r := newRunner(inLockRunner, kubectlSwitcher).
		WithCommanderUUID(c.CommanderUUID)

	err = r.RunConverge(convergeCtx)
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
		return fmt.Errorf("Need to pass running node name. It is may taints terraform state while converge")
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

		if app.SSHLegacyMode {
			sshClient, err = clissh.NewInitClientFromFlags(false)
		} else {
			sshClient, err = gossh.NewInitClientFromFlags(false)
		}

		if err != nil {
			return err
		}

		kubeCl = client.NewKubernetesClient().WithNodeInterface(ssh.NewNodeInterfaceWrapper(sshClient))
		if err := kubeCl.Init(client.AppKubernetesInitParams()); err != nil {
			return err
		}
	}

	var convergeCtx *convergectx.Context
	convergeCtx = convergectx.NewContext(context.Background(), kubeCl, cache.Global(), terraform.ChangeActionSettings{
		AutoDismissDestructive: c.AutoDismissDestructive,
		AutoApprove:            c.AutoApprove,
	})

	convergeCtx.WithPhaseContext(c.PhasedExecutionContext).
		WithTerraformContext(c.Params.TerraformContext)

	inLockRunner := lock.NewInLockRunner(convergeCtx, lock.AutoConvergerIdentity).
		// never force lock
		WithForceLock(false)

	app.DeckhouseTimeout = 1 * time.Hour

	r := newRunner(inLockRunner, convergectx.NewKubeClientSwitcher(convergeCtx, inLockRunner)).
		WithCommanderUUID(c.CommanderUUID).
		WithExcludedNodes([]string{app.RunningNodeName}).
		WithSkipPhases([]phases.OperationPhase{phases.AllNodesPhase})

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
