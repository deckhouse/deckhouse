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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

// TODO(remove-global-app): Support all needed parameters in Params, remove usage of app.*
type Params struct {
	SSHClient              *ssh.Client
	OnPhaseFunc            phases.DefaultOnPhaseFunc
	AutoDismissDestructive bool
	AutoApprove            bool

	*client.KubernetesInitParams

	CommanderMode bool
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

	if err := c.applyParams(); err != nil {
		return nil, err
	}

	kubeCl, err := operations.ConnectToKubernetesAPI(c.SSHClient)
	if err != nil {
		return nil, err
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

	var inLockRunner *converge.InLockRunner
	// No need for converge-lock in commander mode for bootstrap and converge operations
	if !c.CommanderMode {
		inLockRunner = converge.NewInLockLocalRunner(kubeCl, "local-converger")
	}

	stateCache := cache.Global()

	if err := c.PhasedExecutionContext.InitPipeline(stateCache); err != nil {
		return nil, err
	}
	c.lastState = nil
	defer c.PhasedExecutionContext.Finalize(stateCache)

	runner := converge.NewRunner(kubeCl, inLockRunner, stateCache, c.Params.TerraformContext).
		WithPhasedExecutionContext(c.PhasedExecutionContext).
		WithCommanderMode(c.Params.CommanderMode).
		WithCommanderModeParams(c.Params.CommanderModeParams).
		WithChangeSettings(&terraform.ChangeActionSettings{
			AutoDismissDestructive: c.AutoDismissDestructive,
			AutoApprove:            c.AutoApprove,
		})

	err = runner.RunConverge()
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

	sshClient, err := ssh.NewInitClientFromFlags(false)
	if err != nil {
		return err
	}

	kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
	if err := kubeCl.Init(client.AppKubernetesInitParams()); err != nil {
		return err
	}

	inLockRunner := converge.NewInLockRunner(kubeCl, converge.AutoConvergerIdentity).
		// never force lock
		WithForceLock(false)

	app.DeckhouseTimeout = 1 * time.Hour

	runner := converge.NewRunner(kubeCl, inLockRunner, cache.Global(), c.TerraformContext).
		WithChangeSettings(&terraform.ChangeActionSettings{
			AutoDismissDestructive: c.AutoDismissDestructive,
			AutoApprove:            c.AutoApprove,
		}).
		WithExcludedNodes([]string{app.RunningNodeName}).
		WithSkipPhases([]converge.Phase{converge.PhaseAllNodes})

	converger := NewAutoConverger(runner, app.AutoConvergeListenAddress, app.ApplyInterval)
	return converger.Start()
}

func (c *Converger) GetLastState() phases.DhctlState {
	if c.lastState != nil {
		return c.lastState
	} else {
		return c.PhasedExecutionContext.GetLastState()
	}
}
