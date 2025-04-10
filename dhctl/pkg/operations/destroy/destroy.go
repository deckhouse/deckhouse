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
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	infra "github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh/frontend"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh/session"
	tf "github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type Destroyer interface {
	DestroyCluster(ctx context.Context, autoApprove bool) error
}

type Params struct {
	NodeInterface          node.Interface
	StateCache             dhctlstate.Cache
	OnPhaseFunc            phases.DefaultOnPhaseFunc
	PhasedExecutionContext phases.DefaultPhasedExecutionContext

	SkipResources bool

	CommanderMode bool
	CommanderUUID uuid.UUID
	*commander.CommanderModeParams

	TerraformContext *tf.TerraformContext
}

type ClusterDestroyer struct {
	state           *State
	stateCache      dhctlstate.Cache
	terrStateLoader infra.StateLoader

	d8Destroyer       *DeckhouseDestroyer
	cloudClusterInfra *infra.ClusterInfra

	skipResources bool

	staticDestroyer *StaticMastersDestroyer

	PhasedExecutionContext phases.DefaultPhasedExecutionContext

	CommanderMode bool
	CommanderUUID uuid.UUID
}

func NewClusterDestroyer(params *Params) (*ClusterDestroyer, error) {
	state := NewDestroyState(params.StateCache)

	var pec phases.DefaultPhasedExecutionContext
	if params.PhasedExecutionContext != nil {
		pec = params.PhasedExecutionContext
	} else {
		pec = phases.NewDefaultPhasedExecutionContext(params.OnPhaseFunc)
	}

	wrapper, ok := params.NodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nil, fmt.Errorf("cluster destruction requires usage of ssh node interface")
	}

	d8Destroyer := NewDeckhouseDestroyer(wrapper.Client(), state, DeckhouseDestroyerOptions{CommanderMode: params.CommanderMode})

	var terraStateLoader terraform.StateLoader

	if params.CommanderMode {
		// FIXME(dhctl-for-commander): commander uuid currently optional, make it required later
		// if params.CommanderUUID == uuid.Nil {
		//	panic("CommanderUUID required for destroy operation in commander mode!")
		// }

		metaConfig, err := commander.ParseMetaConfig(state.cache, params.CommanderModeParams)
		if err != nil {
			return nil, fmt.Errorf("unable to parse meta configuration: %w", err)
		}
		terraStateLoader = terraform.NewFileTerraStateLoader(state.cache, metaConfig)
	} else {
		terraStateLoader = terraform.NewLazyTerraStateLoader(terraform.NewCachedTerraStateLoader(d8Destroyer, state.cache))
	}

	clusterInfra := infra.NewClusterInfraWithOptions(terraStateLoader, state.cache, params.TerraformContext, infra.ClusterInfraOptions{PhasedExecutionContext: pec})

	staticDestroyer := NewStaticMastersDestroyer(wrapper.Client(), []NodeIP{})

	return &ClusterDestroyer{
		state:           state,
		stateCache:      params.StateCache,
		terrStateLoader: terraStateLoader,

		d8Destroyer:       d8Destroyer,
		cloudClusterInfra: clusterInfra,

		skipResources:   params.SkipResources,
		staticDestroyer: staticDestroyer,

		PhasedExecutionContext: pec,
		CommanderMode:          params.CommanderMode,
		CommanderUUID:          params.CommanderUUID,
	}, nil
}

func (d *ClusterDestroyer) DestroyCluster(ctx context.Context, autoApprove bool) error {
	defer d.d8Destroyer.UnlockConverge(true)

	if err := d.PhasedExecutionContext.InitPipeline(d.stateCache); err != nil {
		return err
	}
	defer d.PhasedExecutionContext.Finalize(d.stateCache)

	if d.CommanderMode {
		kubeCl, err := d.d8Destroyer.GetKubeClient(ctx)
		if err != nil {
			return err
		}

		_, err = commander.CheckShouldUpdateCommanderUUID(ctx, kubeCl, d.CommanderUUID)
		if err != nil {
			return fmt.Errorf("uuid consistency check failed: %w", err)
		}
	}

	// populate cluster state in cache
	metaConfig, err := d.terrStateLoader.PopulateMetaConfig(ctx)
	if err != nil {
		return err
	}

	clusterType := metaConfig.ClusterType
	var infraDestroyer Destroyer
	switch clusterType {
	case config.CloudClusterType:
		infraDestroyer = d.cloudClusterInfra
	case config.StaticClusterType:
		nodeIPs, err := d.GetMasterNodesIPs(ctx)
		if err != nil {
			return err
		}

		d.staticDestroyer.IPs = nodeIPs
		infraDestroyer = d.staticDestroyer
	default:
		return fmt.Errorf("Unknown cluster type '%s'", clusterType)
	}

	if !d.skipResources {
		if shouldStop, err := d.PhasedExecutionContext.StartPhase(phases.DeleteResourcesPhase, false, d.stateCache); err != nil {
			return err
		} else if shouldStop {
			return nil
		}
		if err := d.d8Destroyer.DeleteResources(ctx, clusterType); err != nil {
			return err
		}
		if err := d.PhasedExecutionContext.CompletePhase(d.stateCache, nil); err != nil {
			return err
		}
	}

	if clusterType == config.CloudClusterType {
		_, _, err = d.terrStateLoader.PopulateClusterState(ctx)
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

	if err := infraDestroyer.DestroyCluster(ctx, autoApprove); err != nil {
		return err
	}

	d.state.Clean()
	return d.PhasedExecutionContext.CompletePipeline(d.stateCache)
}

type NodeIP struct {
	internalIP string
	externalIP string
}

type StaticMastersDestroyer struct {
	SSHClient *ssh.Client
	IPs       []NodeIP
}

func NewStaticMastersDestroyer(c *ssh.Client, ips []NodeIP) *StaticMastersDestroyer {
	return &StaticMastersDestroyer{
		SSHClient: c,
		IPs:       ips,
	}
}

func (d *StaticMastersDestroyer) DestroyCluster(ctx context.Context, autoApprove bool) error {
	if !autoApprove {
		if !input.NewConfirmation().WithMessage("Do you really want to cleanup control-plane nodes?").Ask() {
			return fmt.Errorf("Cleanup master nodes disallow")
		}
	}

	mastersHosts := d.SSHClient.Settings.AvailableHosts()
	stdOutErrHandler := func(l string) {
		log.WarnLn(l)
	}

	hostToExclude := ""
	if len(d.IPs) > 0 {
		file := frontend.NewFile(d.SSHClient.Settings)
		bytes, err := file.DownloadBytes(ctx, "/var/lib/bashible/discovered-node-ip")
		if err != nil {

			return err
		}
		hostToExclude = strings.TrimSpace(string(bytes))
	}

	var additionalMastersHosts []session.Host
	for _, ip := range d.IPs {
		ok := true
		if ip.internalIP == hostToExclude {
			ok = false
		}
		h := session.Host{Name: ip.internalIP, Host: ip.internalIP}
		for _, host := range mastersHosts {
			if host.Host == ip.externalIP || host.Host == ip.internalIP {
				ok = false
			}
		}

		if ok {
			additionalMastersHosts = append(additionalMastersHosts, h)
		}
	}

	cmd := "test -f /var/lib/bashible/cleanup_static_node.sh || exit 0 && bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing"

	if len(additionalMastersHosts) > 0 {
		settings := d.SSHClient.Settings.Copy()
		settings.BastionHost = settings.AvailableHosts()[0].Host
		settings.SetAvailableHosts(additionalMastersHosts)
		err := processStaticHosts(ctx, additionalMastersHosts, settings, stdOutErrHandler, cmd)
		if err != nil {

			return err
		}
	}

	err := processStaticHosts(ctx, mastersHosts, d.SSHClient.Settings, stdOutErrHandler, cmd)
	if err != nil {

		return err
	}

	return nil
}

func processStaticHosts(ctx context.Context, hosts []session.Host, s *session.Session, stdOutErrHandler func(l string), cmd string) error {
	for _, host := range hosts {
		settings := s.Copy()
		settings.SetAvailableHosts([]session.Host{host})
		err := retry.NewLoop(fmt.Sprintf("Clear master %s", host), 5, 10*time.Second).RunContext(ctx, func() error {
			cmd := frontend.NewCommand(settings, cmd)
			cmd.Sudo()
			cmd.WithTimeout(5 * time.Minute)
			cmd.WithStdoutHandler(stdOutErrHandler)
			cmd.WithStderrHandler(stdOutErrHandler)
			err := cmd.Run(ctx)

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

func (d *ClusterDestroyer) GetMasterNodesIPs(ctx context.Context) ([]NodeIP, error) {
	var nodeIPs []NodeIP

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	kubeCl, err := d.d8Destroyer.GetKubeClient(ctx)
	if err != nil {
		log.DebugF("Cannot get kubernetes client. Got error: %v", err)
		return []NodeIP{}, err
	}

	var nodes *v1.NodeList
	err = retry.NewLoop("Get control plane nodes from Kubernetes cluster", 5, 5*time.Second).RunContext(ctx, func() error {
		nodes, err = kubeCl.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/control-plane="})
		if err != nil {
			log.DebugF("Cannot get nodes. Got error: %v", err)
			return err
		}
		return nil
	})

	if err != nil {
		log.DebugF("Cannot get nodes after 5 attemts")
		return []NodeIP{}, err
	}

	for _, node := range nodes.Items {
		var ip NodeIP

		for _, addr := range node.Status.Addresses {
			if addr.Type == "InternalIP" {
				ip.internalIP = addr.Address
			}
			if addr.Type == "ExternalIP" {
				ip.externalIP = addr.Address
			}
		}

		nodeIPs = append(nodeIPs, ip)
	}

	return nodeIPs, nil
}
