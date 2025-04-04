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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	sdk "github.com/deckhouse/module-sdk/pkg/utils"
	"github.com/google/uuid"

	dhv1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/v1"
	v1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	infra "github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	convergectx "github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
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

	staticDestroyer := NewStaticMastersDestroyer(wrapper.Client(), []NodeIP{}, d8Destroyer)

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

		if len(nodeIPs) > 1 {
			nodeUser, nodeUserCredentials, err := convergectx.GenerateNodeUser()
			if err != nil {
				return fmt.Errorf("failed to generate NodeUser: %w", err)
			}

			d.staticDestroyer.SetUserCredentials(nodeUserCredentials)

			ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
			defer cancel()

			err = createNodeUser(ctx, d.d8Destroyer.kubeCl.KubeClient, nodeUser)
			if err != nil {
				return err
			}
		}
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
	SSHClient       node.SSHClient
	IPs             []NodeIP
	d8Destroyer     *DeckhouseDestroyer
	userCredentials *convergectx.NodeUserCredentials
}

func NewStaticMastersDestroyer(c node.SSHClient, ips []NodeIP, d8destroyer *DeckhouseDestroyer) *StaticMastersDestroyer {
	return &StaticMastersDestroyer{
		SSHClient:   c,
		IPs:         ips,
		d8Destroyer: d8destroyer,
	}
}

func (d *StaticMastersDestroyer) SetUserCredentials(cr *convergectx.NodeUserCredentials) {
	d.userCredentials = cr
}

func (d *StaticMastersDestroyer) DestroyCluster(ctx context.Context, autoApprove bool) error {
	if !autoApprove {
		if !input.NewConfirmation().WithMessage("Do you really want to cleanup control-plane nodes?").Ask() {
			return fmt.Errorf("Cleanup master nodes disallow")
		}
	}

	log.DebugLn("Starting static cluster destroy process")
	masterHosts := d.SSHClient.Session().AvailableHosts()
	stdOutErrHandler := func(l string) {
		log.WarnLn(l)
	}

	log.DebugLn("Discovering additional master nodes")
	hostToExclude := ""
	if len(d.IPs) > 0 {
		file := d.SSHClient.File()
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
		for _, host := range masterHosts {
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
		log.DebugF("Found %d additional masters, destroying\n", len(additionalMastersHosts))
		settings := d.SSHClient.Session().Copy()
		settings.BastionHost = settings.AvailableHosts()[0].Host
		settings.BastionPort = settings.Port

		for _, host := range additionalMastersHosts {
			settings.SetAvailableHosts([]session.Host{host})
			err := d.switchToNodeuser(settings)
			if err != nil {
				return err
			}

			err = d.processStaticHost(ctx, host, stdOutErrHandler, cmd)
			if err != nil {

				return err
			}

			log.DebugF("host %s was cleaned up successfully", host.Host)
		}

	}

	for _, host := range masterHosts {
		if len(additionalMastersHosts) > 0 {
			settings := d.SSHClient.Session().Copy()
			settings.BastionHost = ""
			settings.BastionPort = ""
			settings.SetAvailableHosts([]session.Host{host})

			err := d.switchToNodeuser(settings)
			if err != nil {
				return err
			}
		}

		err := d.processStaticHost(ctx, host, stdOutErrHandler, cmd)
		if err != nil {

			return err
		}
	}

	return nil
}

func (d *StaticMastersDestroyer) processStaticHost(ctx context.Context, host session.Host, stdOutErrHandler func(l string), cmd string) error {
	log.DebugF("Starting cleanup process for host %s\n", host)
	err := retry.NewLoop(fmt.Sprintf("Clear master %s", host), 5, 30*time.Second).Run(func() error {
		c := d.SSHClient.Command(cmd)
		c.Sudo(ctx)
		c.WithTimeout(30 * time.Second)
		c.WithStdoutHandler(stdOutErrHandler)
		c.WithStderrHandler(stdOutErrHandler)
		err := c.Run(ctx)

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

	return err
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

func (d *StaticMastersDestroyer) switchToNodeuser(settings *session.Session) error {
	log.DebugLn("Starting replacing SSH client")

	tmpDir := filepath.Join(app.CacheDir, "destroy")

	err := os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create cache directory for NodeUser: %w", err)
	}

	log.DebugLn("Tempdir created for SSH client")

	privateKeyPath := filepath.Join(tmpDir, "id_rsa_converger")

	privateKey := session.AgentPrivateKey{
		Key:        privateKeyPath,
		Passphrase: d.userCredentials.Password,
	}

	err = os.WriteFile(privateKeyPath, []byte(d.userCredentials.PrivateKey), 0o600)
	if err != nil {
		return fmt.Errorf("failed to write private key for NodeUser: %w", err)
	}

	log.DebugLn("Private key written")

	if !app.LegacyMode {
		log.DebugF("Old SSH Client: %-v\n", d.SSHClient)
		log.DebugLn("Stopping old SSH client")
		d.SSHClient.Stop()

		// wait for keep-alive goroutine will exit
		time.Sleep(15 * time.Second)
	}

	sess := session.NewSession(session.Input{
		User:           d.userCredentials.Name,
		Port:           settings.Port,
		BastionHost:    settings.BastionHost,
		BastionPort:    settings.BastionPort,
		BastionUser:    d.userCredentials.Name,
		ExtraArgs:      settings.ExtraArgs,
		AvailableHosts: settings.AvailableHosts(),
		BecomePass:     d.userCredentials.Password,
	})

	var newSSHClient node.SSHClient
	if app.LegacyMode {
		newSSHClient = clissh.NewClient(sess, []session.AgentPrivateKey{privateKey})
		// Avoid starting a new ssh agent
		newSSHClient.(*clissh.Client).InitializeNewAgent = false
	} else {
		newSSHClient = gossh.NewClient(sess, []session.AgentPrivateKey{privateKey})
	}

	log.DebugF("New SSH Client: %-v\n", newSSHClient)
	err = newSSHClient.Start()
	if err != nil {
		return fmt.Errorf("failed to start SSH client: %w", err)
	}

	// adding keys to agent is actual only in legacy mode
	if app.LegacyMode {
		err = newSSHClient.(*clissh.Client).Agent.AddKeys(newSSHClient.PrivateKeys())
		if err != nil {
			return fmt.Errorf("failed to add keys to ssh agent: %w", err)
		}

		log.DebugLn("private keys added for replacing kube client")
	}

	d.SSHClient = newSSHClient

	return nil
}

func createNodeUser(ctx context.Context, kubeCl client.KubeClient, nodeUser *dhv1.NodeUser) error {
	nodeUserResource, err := sdk.ToUnstructured(nodeUser)
	if err != nil {
		return fmt.Errorf("failed to convert NodeUser to unstructured: %w", err)
	}

	return retry.NewLoop("Save dhctl converge NodeUser", 45, 10*time.Second).Run(func() error {
		_, err = kubeCl.Dynamic().Resource(dhv1.NodeUserGVK).Create(ctx, nodeUserResource, metav1.CreateOptions{})

		if err != nil {
			if k8errors.IsAlreadyExists(err) {
				_, err = kubeCl.Dynamic().Resource(dhv1.NodeUserGVK).Update(ctx, nodeUserResource, metav1.UpdateOptions{})
				return err
			}

			return fmt.Errorf("failed to create NodeUser: %w", err)
		}

		return nil
	})
}
