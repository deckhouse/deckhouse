// Copyright 2025 Flant JSC
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

package static

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type DestroyerParams struct {
	SSHClientProvider    sshclient.SSHProvider
	KubeProvider         kube.ClientProviderWithCleanup
	State                *State
	LoggerProvider       log.LoggerProvider
	PhasedActionProvider phases.DefaultActionProvider

	TmpDir string
}

type NodesWithCredentials struct {
	*v1.NodeUserCredentials
	IPs []entity.NodeIP
}

type Destroyer struct {
	params               *DestroyerParams
	nodesWithCredentials *NodesWithCredentials
}

func NewDestroyer(params *DestroyerParams) *Destroyer {
	return &Destroyer{
		params: params,
	}
}

func (d *Destroyer) Prepare(ctx context.Context) error {
	logger := d.logger()

	logger.LogDebugLn("Starting prepare static destroyer")
	defer logger.LogDebugLn("Finished prepare static destroyer")

	var err error

	d.nodesWithCredentials, err = d.params.State.NodeUser()

	if err != nil {
		if !errors.Is(err, errNotFoundCredentials) {
			return fmt.Errorf("Error while getting node user from cache: %w", err)
		}

		d.nodesWithCredentials, err = d.createNodeUser(ctx, logger)
		if err != nil {
			return err
		}
	} else {
		logger.LogDebugLn("Found existing nodes with credentials. Saved to destroyer and skipping creating")
	}

	if d.params.State.IsNodeUserExists() {
		logger.LogDebugLn("NodeUser for static destroyer exists getting from cache")
	}

	err = entity.NewConvergerNodeUserExistsWaiter(d.params.KubeProvider).
		WaitPresentOnNodes(ctx, d.nodesWithCredentials.NodeUserCredentials)

	return err
}

func (d *Destroyer) AfterResourcesDelete(context.Context) error {
	return nil
}

func (d *Destroyer) CleanupBeforeDestroy(context.Context) error {
	d.params.KubeProvider.Cleanup(false)
	return nil
}

func (d *Destroyer) DestroyCluster(ctx context.Context, autoApprove bool) error {
	if !autoApprove {
		if !input.NewConfirmation().WithMessage("Do you really want to cleanup control-plane nodes?").Ask() {
			return fmt.Errorf("Cleanup master nodes disallow")
		}
	}

	logger := d.logger()

	sshClient, err := d.params.SSHClientProvider()
	if err != nil {
		return err
	}

	logger.LogDebugLn("Starting static cluster destroy process")
	masterHosts := sshClient.Session().AvailableHosts()
	stdOutErrHandler := func(l string) {
		logger.LogWarnLn(l)
	}

	logger.LogDebugLn("Discovering additional master nodes")
	hostToExclude := ""

	ips := make([]entity.NodeIP, 0)
	if d.nodesWithCredentials != nil {
		ips = d.nodesWithCredentials.IPs
	}

	if len(ips) > 0 {
		file := sshClient.File()
		bytes, err := file.DownloadBytes(ctx, "/var/lib/bashible/discovered-node-ip")
		if err != nil {

			return err
		}
		hostToExclude = strings.TrimSpace(string(bytes))
	}

	var additionalMastersHosts []session.Host
	for _, ip := range ips {
		ok := true
		if ip.InternalIP == hostToExclude {
			ok = false
		}
		h := session.Host{Name: ip.InternalIP, Host: ip.InternalIP}
		for _, host := range masterHosts {
			if host.Host == ip.ExternalIP || host.Host == ip.InternalIP {
				ok = false
			}
		}

		if ok {
			additionalMastersHosts = append(additionalMastersHosts, h)
		}
	}

	cmd := "test -f /var/lib/bashible/cleanup_static_node.sh || exit 0 && bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing"

	if len(additionalMastersHosts) > 0 {
		logger.LogDebugF("Found %d additional masters, destroying\n", len(additionalMastersHosts))
		settings := sshClient.Session().Copy()
		if settings.BastionHost == "" {
			settings.BastionHost = settings.AvailableHosts()[0].Host
			settings.BastionPort = settings.Port
		}

		for _, host := range additionalMastersHosts {
			settings.SetAvailableHosts([]session.Host{host})
			sshClient, err = d.switchToNodeUser(ctx, sshClient, settings)
			if err != nil {
				return err
			}

			err = d.processStaticHost(ctx, sshClient, host, stdOutErrHandler, cmd)
			if err != nil {

				return err
			}

			logger.LogDebugF("host %s was cleaned up successfully\n", host.Host)
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

			sshClient, err = d.switchToNodeUser(ctx, sshClient, settings)
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

func (d *Destroyer) processStaticHost(ctx context.Context, sshClient node.SSHClient, host session.Host, stdOutErrHandler func(l string), cmd string) error {
	logger := d.logger()

	logger.LogDebugF("Starting cleanup process for host %s\n", host)
	err := retry.NewLoop(fmt.Sprintf("Clear master %s", host), 5, 30*time.Second).RunContext(ctx, func() error {
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

func (d *Destroyer) switchToNodeUser(ctx context.Context, oldSSHClient node.SSHClient, settings *session.Session) (node.SSHClient, error) {
	if d.nodesWithCredentials == nil {
		return nil, fmt.Errorf("Internal error. No nodes with credentials in destroyer. Probably Prepare did not call or try destroy when abort")
	}

	if d.params.TmpDir == "" {
		return nil, fmt.Errorf("Internal error. No tmp dir passed")
	}

	logger := d.logger()

	tmpDir := filepath.Join(d.params.TmpDir, "destroy")

	logger.LogDebugF("Starting replacing SSH client. Key directory: %s\n", tmpDir)

	err := os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache directory for NodeUser: %w", err)
	}

	logger.LogDebugF("Tempdir '%s' created for SSH client\n", tmpDir)

	n := rand.New(rand.NewSource(time.Now().UnixNano())).Int()

	privateKeyPath := filepath.Join(tmpDir, fmt.Sprintf("id_rsa_converger.%d", n))

	privateKey := session.AgentPrivateKey{
		Key:        privateKeyPath,
		Passphrase: d.nodesWithCredentials.Password,
	}

	err = os.WriteFile(privateKeyPath, []byte(d.nodesWithCredentials.PrivateKey), 0o600)
	if err != nil {
		return nil, fmt.Errorf("Failed to write private key for NodeUser: %w", err)
	}

	logger.LogDebugLn("Private key written")

	if sshclient.IsModernMode() {
		logger.LogDebugF("Old SSH Client: %-v\n", oldSSHClient)
		logger.LogDebugLn("Stopping old SSH client")
		oldSSHClient.Stop()

		// wait for keep-alive goroutine will exit
		time.Sleep(15 * time.Second)
	}

	sess := session.NewSession(session.Input{
		User:           d.nodesWithCredentials.Name,
		Port:           settings.Port,
		BastionHost:    settings.BastionHost,
		BastionPort:    settings.BastionPort,
		BastionUser:    d.nodesWithCredentials.Name,
		ExtraArgs:      settings.ExtraArgs,
		AvailableHosts: settings.AvailableHosts(),
		BecomePass:     d.nodesWithCredentials.Password,
	})

	newSSHClient := sshclient.NewClient(ctx, sess, []session.AgentPrivateKey{privateKey})

	logger.LogDebugF("New SSH Client: %-v\n", newSSHClient)
	err = newSSHClient.Start()
	if err != nil {
		return nil, fmt.Errorf("Failed to start SSH client: %w", err)
	}

	// adding keys to agent is actual only in legacy mode
	if sshclient.IsLegacyMode() {
		err = newSSHClient.(*clissh.Client).Agent.AddKeys(newSSHClient.PrivateKeys())
		if err != nil {
			return nil, fmt.Errorf("Failed to add keys to ssh agent: %w", err)
		}

		logger.LogDebugLn("Private keys added for replacing kube client")
	}

	return newSSHClient, nil
}

func (d *Destroyer) waitNodeUserExists(ctx context.Context) error {
	if d.params.PhasedActionProvider == nil {
		return fmt.Errorf("Internal error. PhasedActionProvider not initialized. Probably you try to destroy when need abort")
	}

	if d.nodesWithCredentials == nil {
		return fmt.Errorf("Internal error. nodesWithCredentials not initialized. Probably you try to destroy when need abort")
	}

	return d.params.PhasedActionProvider().Run(phases.WaitStaticDestroyerNodeUserPhase, false, func() (phases.DefaultContextType, error) {
		err := entity.NewConvergerNodeUserExistsWaiter(d.params.KubeProvider).
			WaitPresentOnNodes(ctx, d.nodesWithCredentials.NodeUserCredentials)
		if err != nil {
			return nil, err
		}

		return nil, d.params.State.SetNodeUserExists()
	})
}

func (d *Destroyer) createNodeUser(ctx context.Context, logger log.Logger) (*NodesWithCredentials, error) {
	if d.params.PhasedActionProvider == nil {
		return nil, fmt.Errorf("Internal error. PhasedActionProvider not initialized. Probably you try to destroy when need abort")
	}

	nodeIPs, err := entity.GetMasterNodesIPs(ctx, d.params.KubeProvider)
	if err != nil {
		return nil, err
	}

	logger.LogDebugF("Found master node IPs: %+v\n", nodeIPs)

	// always create node user
	var nodesWithCredentials *NodesWithCredentials

	err = d.params.PhasedActionProvider().Run(phases.CreateStaticDestroyerNodeUserPhase, false, func() (phases.DefaultContextType, error) {
		nodeUser, nodeUserCredentials, err := v1.GenerateNodeUser(v1.ConvergerNodeUser())
		if err != nil {
			return nil, fmt.Errorf("failed to generate NodeUser: %w", err)
		}

		err = entity.CreateNodeUser(ctx, d.params.KubeProvider, nodeUser)
		if err != nil {
			return nil, err
		}

		logger.LogDebugF("Node user created %s\n", nodeUserCredentials.Name)

		nodesWithCredentials = &NodesWithCredentials{
			NodeUserCredentials: nodeUserCredentials,
			IPs:                 nodeIPs,
		}

		if err := d.params.State.SaveNodeUser(nodesWithCredentials); err != nil {
			return nil, err
		}

		logger.LogDebugF("Node user saved to cache and to destroyer %s\n", nodeUserCredentials.Name)

		return nil, nil
	})

	if err != nil {
		return nil, err
	}

	return nodesWithCredentials, nil
}

func (d *Destroyer) logger() log.Logger {
	return log.SafeProvideLogger(d.params.LoggerProvider)
}
