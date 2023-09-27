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
	"fmt"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

// TODO(remove-global-app): Support all needed parameters in Params, remove usage of app.*
type Params struct {
	SSHClient    *ssh.Client
	InitialState phases.DhctlState
	OnPhaseFunc  phases.OnPhaseFunc

	*client.KubernetesInitParams
}

type Converger struct {
	*Params
	*phases.PhasedExecutionContext
}

func NewConverger(params *Params) *Converger {
	return &Converger{
		Params:                 params,
		PhasedExecutionContext: phases.NewPhasedExecutionContext(params.OnPhaseFunc),
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

// FIXME(dhctl-for-commander): optionally initialize config-path from parameter, use specified config instead of in-cluster
func (c *Converger) Converge() error {
	if err := c.applyParams(); err != nil {
		return err
	}

	kubeCl, err := operations.ConnectToKubernetesAPI(c.SSHClient)
	if err != nil {
		return err
	}

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

	err = cache.InitWithOptions(cacheIdentity, cache.CacheOptions{InitialState: c.InitialState})
	if err != nil {
		return fmt.Errorf("unable to initialize cache %s: %w", cacheIdentity, err)
	}
	inLockRunner := converge.NewInLockLocalRunner(kubeCl, "local-converger")

	stateCache := cache.Global()

	if err := c.PhasedExecutionContext.Init(stateCache); err != nil {
		return err
	}
	defer c.PhasedExecutionContext.Finalize(stateCache)

	runner := converge.NewRunner(kubeCl, inLockRunner, stateCache, converge.RunnerOptions{PhasedExecutionContext: c.PhasedExecutionContext})
	runner.WithChangeSettings(&terraform.ChangeActionSettings{
		AutoDismissDestructive: false,
	})

	err = runner.RunConverge()
	if err != nil {
		return fmt.Errorf("converge problem: %v", err)
	}

	return c.PhasedExecutionContext.Complete(stateCache)
}

func (c *Converger) AutoConverge() error {
	c.applyParams()

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

	runner := converge.NewRunner(kubeCl, inLockRunner, cache.Global(), converge.RunnerOptions{}).
		WithChangeSettings(&terraform.ChangeActionSettings{
			AutoDismissDestructive: true,
			AutoApprove:            true,
		}).
		WithExcludedNodes([]string{app.RunningNodeName}).
		WithSkipPhases([]converge.Phase{converge.PhaseAllNodes})

	converger := NewAutoConverger(runner, app.AutoConvergeListenAddress, app.ApplyInterval)
	return converger.Start()
}
