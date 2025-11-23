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
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/name212/govalue"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/controller"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	convergectx "github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type SSHProvider func() (node.SSHClient, error)

type Destroyer interface {
	DestroyCluster(ctx context.Context, autoApprove bool) error
}

type Params struct {
	NodeInterface          node.Interface
	StateCache             dhctlstate.Cache
	OnPhaseFunc            phases.DefaultOnPhaseFunc
	OnProgressFunc         phases.OnProgressFunc
	PhasedExecutionContext phases.DefaultPhasedExecutionContext

	SkipResources bool

	CommanderMode bool
	CommanderUUID uuid.UUID
	*commander.CommanderModeParams

	InfrastructureContext *infrastructure.Context

	TmpDir  string
	Logger  log.Logger
	IsDebug bool
}

type ClusterDestroyer struct {
	state           *State
	stateCache      dhctlstate.Cache
	terrStateLoader controller.StateLoader

	d8Destroyer       *DeckhouseDestroyer
	cloudClusterInfra *controller.ClusterInfra

	skipResources bool

	staticDestroyer *StaticMastersDestroyer

	PhasedExecutionContext phases.DefaultPhasedExecutionContext

	CommanderMode bool
	CommanderUUID uuid.UUID

	tmpDir  string
	logger  log.Logger
	isDebug bool
}

// NewClusterDestroyer
// params.SSHClient should not START!
func NewClusterDestroyer(ctx context.Context, params *Params) (*ClusterDestroyer, error) {
	state := NewDestroyState(params.StateCache)

	logger := params.Logger
	if govalue.IsNil(logger) {
		logger = log.GetDefaultLogger()
	}

	if app.ProgressFilePath != "" {
		params.OnProgressFunc = phases.WriteProgress(app.ProgressFilePath)
	}

	var pec phases.DefaultPhasedExecutionContext
	if params.PhasedExecutionContext != nil {
		pec = params.PhasedExecutionContext
	} else {
		pec = phases.NewDefaultPhasedExecutionContext(
			phases.OperationDestroy, params.OnPhaseFunc, params.OnProgressFunc,
		)
	}

	wrapper, ok := params.NodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nil, fmt.Errorf("cluster destruction requires usage of ssh node interface")
	}

	sshClientProvider := sync.OnceValues(func() (node.SSHClient, error) {
		sshClient := wrapper.Client()
		if err := sshClient.Start(); err != nil {
			return nil, err
		}

		return sshClient, nil
	})

	d8Destroyer := NewDeckhouseDestroyer(sshClientProvider, state, DeckhouseDestroyerOptions{CommanderMode: params.CommanderMode})

	var terraStateLoader controller.StateLoader

	if params.CommanderMode {
		// FIXME(dhctl-for-commander): commander uuid currently optional, make it required later
		// if params.CommanderUUID == uuid.Nil {
		//	panic("CommanderUUID required for destroy operation in commander mode!")
		// }

		metaConfig, err := commander.ParseMetaConfig(ctx, state.cache, params.CommanderModeParams, logger)
		if err != nil {
			return nil, fmt.Errorf("unable to parse meta configuration: %w", err)
		}
		terraStateLoader = infrastructurestate.NewFileTerraStateLoader(state.cache, metaConfig)
	} else {
		terraStateLoader = infrastructurestate.NewLazyTerraStateLoader(
			infrastructurestate.NewCachedTerraStateLoader(d8Destroyer, state.cache, logger),
		)
	}

	clusterInfra := controller.NewClusterInfraWithOptions(
		terraStateLoader,
		state.cache,
		params.InfrastructureContext,
		controller.ClusterInfraOptions{
			PhasedExecutionContext: pec,
			TmpDir:                 params.TmpDir,
			IsDebug:                params.IsDebug,
			Logger:                 logger,
		},
	)

	staticDestroyer := NewStaticMastersDestroyer(sshClientProvider, []NodeIP{}, state)

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

		tmpDir:  params.TmpDir,
		logger:  logger,
		isDebug: params.IsDebug,
	}, nil
}

func (d *ClusterDestroyer) lockConverge(ctx context.Context) error {
	if d.CommanderMode {
		d.logger.LogDebugLn("Locking converge skipped for commander")
		return nil
	}

	locked, err := d.state.IsConvergeLocked()
	if err != nil {
		return err
	}

	if locked {
		d.logger.LogDebugLn("Locking converge skipped because locked in previous run")
		return nil
	}

	if err := d.d8Destroyer.LockConverge(ctx); err != nil {
		return err
	}

	if err := d.state.SetConvergeLocked(); err != nil {
		// try to unlock because we cannot save in state
		d.d8Destroyer.UnlockConverge(true)
		return err
	}

	d.logger.LogDebugLn("Converge was locked successfully and write to state")

	return nil
}

func (d *ClusterDestroyer) DestroyCluster(ctx context.Context, autoApprove bool) error {
	// we do not need unlock converge because we save lock in state

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
		if err := d.lockConverge(ctx); err != nil {
			return err
		}
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

			err = entity.CreateNodeUser(ctx, d.d8Destroyer, nodeUser)
			if err != nil {
				return err
			}

			err = d.staticDestroyer.waitNodeUserPresent(global.ConvergeNodeUserName, ctx)
			if err != nil {
				return err
			}

			// wait for other nodes NodeUser will be created as well
			time.Sleep(20 * time.Second)
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

	d.logger.LogDebugF("Resources were destroyed set\n")

	// Stop proxy because we have already got all info from kubernetes-api
	// also stop ssh client for cloud clusters
	d.d8Destroyer.Cleanup(clusterType == config.CloudClusterType)

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
	state             *State
	sshClientProvider SSHProvider
	IPs               []NodeIP
	userCredentials   *convergectx.NodeUserCredentials
}

func NewStaticMastersDestroyer(sshClientProvider SSHProvider, ips []NodeIP, state *State) *StaticMastersDestroyer {
	return &StaticMastersDestroyer{
		sshClientProvider: sshClientProvider,
		IPs:               ips,
		state:             state,
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

	sshClient, err := d.sshClientProvider()
	if err != nil {
		return err
	}

	log.DebugLn("Starting static cluster destroy process")
	masterHosts := sshClient.Session().AvailableHosts()
	stdOutErrHandler := func(l string) {
		log.WarnLn(l)
	}

	log.DebugLn("Discovering additional master nodes")
	hostToExclude := ""
	if len(d.IPs) > 0 {
		file := sshClient.File()
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
		settings := sshClient.Session().Copy()
		if settings.BastionHost == "" {
			settings.BastionHost = settings.AvailableHosts()[0].Host
			settings.BastionPort = settings.Port
		}

		for _, host := range additionalMastersHosts {
			settings.SetAvailableHosts([]session.Host{host})
			sshClient, err = d.switchToNodeUser(sshClient, settings)
			if err != nil {
				return err
			}

			err = d.processStaticHost(ctx, sshClient, host, stdOutErrHandler, cmd)
			if err != nil {

				return err
			}

			log.DebugF("host %s was cleaned up successfully", host.Host)
		}

	}

	for _, host := range masterHosts {
		if len(additionalMastersHosts) > 0 {
			settings := sshClient.Session().Copy()
			if settings.BastionHost == settings.AvailableHosts()[0].Host {
				settings.BastionHost = ""
				settings.BastionPort = ""
			}

			settings.SetAvailableHosts([]session.Host{host})

			sshClient, err = d.switchToNodeUser(sshClient, settings)
			if err != nil {
				return err
			}
		}

		err := d.processStaticHost(ctx, sshClient, host, stdOutErrHandler, cmd)
		if err != nil {

			return err
		}
	}

	return nil
}

func (d *StaticMastersDestroyer) processStaticHost(ctx context.Context, sshClient node.SSHClient, host session.Host, stdOutErrHandler func(l string), cmd string) error {
	log.DebugF("Starting cleanup process for host %s\n", host)
	err := retry.NewLoop(fmt.Sprintf("Clear master %s", host), 5, 30*time.Second).Run(func() error {
		c := sshClient.Command(cmd)
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

func (d *StaticMastersDestroyer) switchToNodeUser(oldSSHClient node.SSHClient, settings *session.Session) (node.SSHClient, error) {
	log.DebugLn("Starting replacing SSH client")

	tmpDir := filepath.Join(d.state.StateDir(), "destroy")

	err := os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache directory for NodeUser: %w", err)
	}

	log.DebugLn("Tempdir created for SSH client")

	privateKeyPath := filepath.Join(tmpDir, "id_rsa_converger")

	privateKey := session.AgentPrivateKey{
		Key:        privateKeyPath,
		Passphrase: d.userCredentials.Password,
	}

	err = os.WriteFile(privateKeyPath, []byte(d.userCredentials.PrivateKey), 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to write private key for NodeUser: %w", err)
	}

	log.DebugLn("Private key written")

	if sshclient.IsModernMode() {
		log.DebugF("Old SSH Client: %-v\n", oldSSHClient)
		log.DebugLn("Stopping old SSH client")
		oldSSHClient.Stop()

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

	newSSHClient := sshclient.NewClient(sess, []session.AgentPrivateKey{privateKey})

	log.DebugF("New SSH Client: %-v\n", newSSHClient)
	err = newSSHClient.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start SSH client: %w", err)
	}

	// adding keys to agent is actual only in legacy mode
	if sshclient.IsLegacyMode() {
		err = newSSHClient.(*clissh.Client).Agent.AddKeys(newSSHClient.PrivateKeys())
		if err != nil {
			return nil, fmt.Errorf("failed to add keys to ssh agent: %w", err)
		}

		log.DebugLn("private keys added for replacing kube client")
	}

	return newSSHClient, nil
}

var errSSHClientDidNotGet = errors.New("Failed to get ssh client")

func (d *StaticMastersDestroyer) waitNodeUserPresent(name string, ctx context.Context) error {
	command := "stat /home/deckhouse/" + name + "/.ssh/authorized_keys"

	err := retry.NewLoop("Checking if NodeUser present on node", 20, 5*time.Second).
		BreakIf(retry.IsErr(errSSHClientDidNotGet)).
		RunContext(ctx, func() error {
			sshClient, err := d.sshClientProvider()
			if err != nil {
				return fmt.Errorf("%w: %w", errSSHClientDidNotGet, err)
			}
			cmd := sshClient.Command(command)
			cmd.Sudo(ctx)
			return cmd.Run(ctx)
		})

	return err
}
