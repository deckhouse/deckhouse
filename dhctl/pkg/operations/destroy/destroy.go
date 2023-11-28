// Copyright 2021 Flant JSC
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

package destroy

import (
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	infra "github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type Destroyer interface {
	DestroyCluster(autoApprove bool) error
}

type Params struct {
	SSHClient   *ssh.Client
	StateCache  dhctlstate.Cache
	OnPhaseFunc phases.OnPhaseFunc

	SkipResources bool
}

type ClusterDestroyer struct {
	state           *State
	stateCache      dhctlstate.Cache
	terrStateLoader infra.StateLoader

	d8Destroyer       *DeckhouseDestroyer
	cloudClusterInfra *infra.ClusterInfra

	skipResources bool

	staticDestroyer *StaticMastersDestroyer

	*phases.PhasedExecutionContext
}

func NewClusterDestroyer(params *Params) *ClusterDestroyer {
	state := NewDestroyState(params.StateCache)
	pec := phases.NewPhasedExecutionContext(params.OnPhaseFunc)
	d8Destroyer := NewDeckhouseDestroyer(params.SSHClient, state)
	terraStateLoader := terraform.NewLazyTerraStateLoader(terraform.NewCachedTerraStateLoader(d8Destroyer, state.cache))
	clusterInfra := infra.NewClusterInfraWithOptions(terraStateLoader, state.cache, infra.ClusterInfraOptions{PhasedExecutionContext: pec})

	staticDestroyer := NewStaticMastersDestroyer(params.SSHClient)

	return &ClusterDestroyer{
		state:           state,
		stateCache:      params.StateCache,
		terrStateLoader: terraStateLoader,

		d8Destroyer:       d8Destroyer,
		cloudClusterInfra: clusterInfra,

		skipResources: params.SkipResources,

		PhasedExecutionContext: pec,

		staticDestroyer: staticDestroyer,
	}
}

func (d *ClusterDestroyer) DestroyCluster(autoApprove bool) error {
	defer d.d8Destroyer.UnlockConverge(true)

	if err := d.PhasedExecutionContext.Init(d.stateCache); err != nil {
		return err
	}
	defer d.PhasedExecutionContext.Finalize(d.stateCache)

	// populate cluster state in cache
	metaConfig, err := d.terrStateLoader.PopulateMetaConfig()
	if err != nil {
		return err
	}

	clusterType := metaConfig.ClusterType
	var infraDestroyer Destroyer
	switch clusterType {
	case config.CloudClusterType:
		infraDestroyer = d.cloudClusterInfra
	case config.StaticClusterType:
		infraDestroyer = d.staticDestroyer
	default:
		return fmt.Errorf("Unknown cluster type '%s'", clusterType)
	}

	if !d.skipResources {
		if shouldStop, err := d.PhasedExecutionContext.StartPhase(phases.DeleteResourcesPhase, false); err != nil {
			return err
		} else if shouldStop {
			return nil
		}
		if err := d.d8Destroyer.DeleteResources(clusterType); err != nil {
			return err
		}
		if err := d.PhasedExecutionContext.CommitState(d.stateCache); err != nil {
			return err
		}
	}

	if clusterType == config.CloudClusterType {
		_, _, err = d.terrStateLoader.PopulateClusterState()
		if err != nil {
			return err
		}
	}

	// only after load and save all states into cache
	// set resources as deleted
	if err := d.state.SetResourcesDestroyed(); err != nil {
		return err
	}

	// why only unwatch lock without request unlock
	// user may not delete resources and converge still working in cluster
	// all node groups removing may still in long time run and
	// we get race (destroyer destroy node group, auto applayer create nodes)
	d.d8Destroyer.UnlockConverge(false)
	// Stop proxy because we have already got all info from kubernetes-api
	d.d8Destroyer.StopProxy()

	if err := infraDestroyer.DestroyCluster(autoApprove); err != nil {
		return err
	}

	d.state.Clean()
	return d.PhasedExecutionContext.Complete()
}

type StaticMastersDestroyer struct {
	SSHClient *ssh.Client
}

func NewStaticMastersDestroyer(c *ssh.Client) *StaticMastersDestroyer {
	return &StaticMastersDestroyer{
		SSHClient: c,
	}
}

func (d *StaticMastersDestroyer) DestroyCluster(autoApprove bool) error {
	if !autoApprove {
		if !input.NewConfirmation().WithMessage("Do you really want to cleanup control-plane nodes?").Ask() {
			return fmt.Errorf("Cleanup master nodes disallow")
		}
	}

	mastersHosts := d.SSHClient.Settings.AvailableHosts()
	stdOutErrHandler := func(l string) {
		log.WarnLn(l)
	}

	cmd := "test -f /var/lib/bashible/cleanup_static_node.sh || exit 0 && bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing"
	for _, host := range mastersHosts {
		settings := d.SSHClient.Settings.Copy()
		settings.SetAvailableHosts([]string{host})
		err := retry.NewLoop(fmt.Sprintf("Clear master %s", host), 5, 10*time.Second).Run(func() error {
			err := frontend.NewCommand(settings, cmd).
				Sudo().
				WithTimeout(5 * time.Minute).
				WithStdoutHandler(stdOutErrHandler).
				WithStderrHandler(stdOutErrHandler).
				Run()

			if err != nil {
				var ee *exec.ExitError
				if errors.As(err, &ee) {
					// script reboot node
					if ee.ExitCode() == 255 {
						return nil
					}
				}

				return err
			}

			return err
		})

		if err != nil {
			return err
		}
	}

	return nil
}
