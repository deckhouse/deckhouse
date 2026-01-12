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

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type LoopsParams struct {
	NodeUser       retry.Params
	DestroyMaster  retry.Params
	GetMastersIPs  retry.Params
	CreateNodeUser retry.Params
}

type DestroyerParams struct {
	SSHClientProvider    sshclient.SSHProvider
	KubeProvider         kube.ClientProviderWithCleanup
	State                *State
	LoggerProvider       log.LoggerProvider
	PhasedActionProvider phases.DefaultActionProvider

	TmpDir string

	Loops LoopsParams
}

type NodesWithCredentials struct {
	NodeUser     *v1.NodeUserCredentials
	IPs          []entity.NodeIP
	ProcessedIPS []session.Host
}

func (c *NodesWithCredentials) SetHostAsProcessed(host session.Host) bool {
	if len(c.ProcessedIPS) == 0 {
		return false
	}

	for _, ip := range c.ProcessedIPS {
		if ip.Host == host.Host {
			return true
		}
	}

	return false
}

func (c *NodesWithCredentials) AddToProcessed(host session.Host) {
	if len(c.ProcessedIPS) == 0 {
		c.ProcessedIPS = make([]session.Host, 0, 1)
	}

	c.ProcessedIPS = append(c.ProcessedIPS, host)
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

		d.nodesWithCredentials, err = d.createAndSaveCredentials(ctx, logger)
		if err != nil {
			return err
		}
	} else {
		logger.LogDebugLn("Found existing nodes with credentials. Saved to destroyer and skipping creating")
	}

	return d.waitNodeUserExists(ctx)
}

func (d *Destroyer) AfterResourcesDelete(context.Context) error {
	return nil
}

func (d *Destroyer) CleanupBeforeDestroy(context.Context) error {
	d.params.KubeProvider.Cleanup(false)
	return nil
}

func (d *Destroyer) DestroyCluster(ctx context.Context, autoApprove bool) error {
	if govalue.IsNil(d.params.SSHClientProvider) {
		return errors.New("Internal error. SSH provider did not pass")
	}

	if !autoApprove {
		if !input.NewConfirmation().WithMessage("Do you really want to cleanup control-plane nodes?").Ask() {
			return fmt.Errorf("Cleanup master nodes disallow")
		}
	}

	logger := d.logger()

	sshClient, err := d.params.SSHClientProvider.Client()
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
	// for abort we do not have nodesWithCredentials
	if d.nodesWithCredentials != nil && !isSingleMaster(d.nodesWithCredentials.IPs) {
		ips = d.nodesWithCredentials.IPs
	}

	if len(ips) > 0 {
		err := logger.LogProcess("default", "Get internal node IP for passed control-plane host", func() error {
			file := sshClient.File()
			bytes, err := file.DownloadBytes(ctx, "/var/lib/bashible/discovered-node-ip")
			if err != nil {

				return err
			}
			hostToExclude = strings.TrimSpace(string(bytes))
			logger.LogDebugF("Got internal node IP for passed control-plane host: %s\n", hostToExclude)
			return nil
		})
		if err != nil {
			return err
		}
	}

	var additionalMastersHosts []session.Host
	for _, ip := range ips {
		ok := true
		if ip.InternalIP == hostToExclude {
			ok = false
		}
		h := session.Host{Name: ip.Name(), Host: ip.InternalIP}
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

	userPassedSSHSetting := sshClient.Session().Copy()

	if len(additionalMastersHosts) > 0 {
		logger.LogDebugF("Found %d additional masters, destroying them\n", len(additionalMastersHosts))
		settings := userPassedSSHSetting.Copy()
		// if bastion passed - use user bastion, because master passed by user and another masters in one network
		// else connect over passed host, because additional masters will have private network address
		if settings.BastionHost == "" {
			settings.BastionHost = userPassedSSHSetting.AvailableHosts()[0].Host
			settings.BastionPort = userPassedSSHSetting.Port
			settings.BastionUser = userPassedSSHSetting.User
		}

		for _, host := range additionalMastersHosts {
			if d.hostProcessed(host) {
				logger.LogInfoF("Skipping additional master host: '%s'. Host already processed\n", host.String())
				continue
			}
			settings.SetAvailableHosts([]session.Host{host})
			sshClient, err = d.switchToNodeUser(ctx, sshClient, settings)
			if err != nil {
				return err
			}

			err = d.processStaticHost(ctx, sshClient, host, stdOutErrHandler, cmd)
			if err != nil {

				return err
			}

			logger.LogDebugF("Host %s was cleaned up successfully\n", host.Host)
		}

	}

	for _, host := range masterHosts {
		// if we have additional masters hosts (multimaster) we should switch to node user
		// because it was created
		// else we will process with setting passed by user because we did not switch above
		if len(additionalMastersHosts) > 0 {
			// for last master (it master was user connected in destroy/abort)
			// revert to passed settings and switch to node user for reconnect to last host
			// node user was created for all master hosts and we can switch save to it
			// without use passed user
			settings := userPassedSSHSetting.Copy()
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
	d.logger().LogDebugF("Starting cleanup process for host %s\n", host)

	err := retry.NewLoopWithParams(d.destroyMasterLoopParams(host)).RunContext(ctx, func() error {
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

	if err != nil {
		return err
	}

	return d.addHostAsProcessed(host)
}

func (d *Destroyer) switchToNodeUser(ctx context.Context, oldSSHClient node.SSHClient, settings *session.Session) (node.SSHClient, error) {
	if d.nodesWithCredentials == nil {
		return nil, fmt.Errorf("Internal error. No nodes with credentials in destroyer. Probably Prepare did not call or try destroy when abort")
	}

	if d.params.TmpDir == "" {
		return nil, fmt.Errorf("Internal error. No tmp dir passed")
	}

	logger := d.logger()

	logger.LogInfoF("Switch to node user for next control-plane host\n")

	tmpDir := filepath.Join(d.params.TmpDir, "destroy")

	logger.LogDebugF("Starting replacing SSH client. Key directory: %s\n", tmpDir)

	err := os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache directory for NodeUser: %w", err)
	}

	logger.LogDebugF("Tempdir '%s' created for SSH client\n", tmpDir)

	privateKeyPrefixPathWithoutSuffix := filepath.Join(tmpDir, "id_rsa_destroyer.key")

	n := rand.New(rand.NewSource(time.Now().UnixNano())).Int()
	privateKeyPath := fmt.Sprintf("%s.%d", privateKeyPrefixPathWithoutSuffix, n)

	convergerPrivateKey := session.AgentPrivateKey{
		Key:        privateKeyPath,
		Passphrase: d.nodesWithCredentials.NodeUser.Password,
	}

	err = os.WriteFile(privateKeyPath, []byte(d.nodesWithCredentials.NodeUser.PrivateKey), 0o600)
	if err != nil {
		return nil, fmt.Errorf("Failed to write private key for NodeUser: %w", err)
	}

	logger.LogDebugLn("Private key written")

	sess := session.NewSession(session.Input{
		User: d.nodesWithCredentials.NodeUser.Name,
		// use input because we cannot discovery sshd port for another hosts
		// and we hope that user use same port for all nodes
		Port: settings.Port,
		// use passed bastion host because if we do not have bastion
		// for additional master nodes we connect over first master
		// because additional master will have private network address
		// this will set in setting before run switchToNodeUser
		BastionHost: settings.BastionHost,
		BastionPort: settings.BastionPort,
		BastionUser: settings.BastionUser,

		ExtraArgs: settings.ExtraArgs,
		// input setting have one host to connect
		AvailableHosts: settings.AvailableHosts(),
		BecomePass:     d.nodesWithCredentials.NodeUser.Password,
	})

	privateKeys := []session.AgentPrivateKey{convergerPrivateKey}

	oldPrivateKeys := oldSSHClient.PrivateKeys()
	for _, oldKey := range oldPrivateKeys {
		// skip another temp keys for another hosts
		// add only user passed keys
		if !strings.HasPrefix(oldKey.Key, privateKeyPrefixPathWithoutSuffix) {
			privateKeys = append(privateKeys, oldKey)
		}
	}

	newSSHClient, err := d.params.SSHClientProvider.SwitchClient(ctx, sess, privateKeys, oldSSHClient)
	if err != nil {
		return nil, err
	}

	logger.LogDebugF("New SSH Client: %-v\n", newSSHClient)
	err = newSSHClient.Start()
	if err != nil {
		return nil, fmt.Errorf("Failed to start SSH client: %w", err)
	}

	if err := newSSHClient.RefreshPrivateKeys(); err != nil {
		return nil, fmt.Errorf("Failed to refresh private keys: %w", err)
	}

	logger.LogDebugLn("Private keys refreshed for replacing kube client")

	return newSSHClient, nil
}

func (d *Destroyer) waitNodeUserExists(ctx context.Context) error {
	if d.params.PhasedActionProvider == nil {
		return fmt.Errorf("Internal error. PhasedActionProvider not initialized. Probably you try to destroy when need abort")
	}

	if d.nodesWithCredentials == nil {
		return fmt.Errorf("Internal error. nodesWithCredentials not initialized. Probably you try to destroy when need abort")
	}

	if len(d.nodesWithCredentials.IPs) == 0 {
		return fmt.Errorf("Internal error. nodesWithCredentials ips is empty")
	}

	logger := d.logger()

	if d.params.State.IsNodeUserExists() {
		logger.LogDebugLn("NodeUser for static destroyer exists getting from cache")
		return nil
	}

	return d.params.PhasedActionProvider().Run(phases.WaitStaticDestroyerNodeUserPhase, false, func() (phases.DefaultContextType, error) {
		if !isSingleMaster(d.nodesWithCredentials.IPs) {
			// waiter checks if nil params
			waiter := entity.NewConvergerNodeUserExistsWaiter(d.params.KubeProvider).
				WithParams(d.params.Loops.NodeUser)

			if err := waiter.WaitPresentOnNodes(ctx, d.nodesWithCredentials.NodeUser); err != nil {
				return nil, err
			}
		} else {
			logger.LogDebugLn("No wait NodeUser for single-master cluster")
		}

		return nil, d.params.State.SetNodeUserExists()
	})
}

func (d *Destroyer) createNodeUserCredentials(ctx context.Context, ips []entity.NodeIP, logger log.Logger) (*v1.NodeUserCredentials, error) {
	if isSingleMaster(ips) {
		logger.LogDebugLn("Has single master. Skip creating node user and returns empty credentials for save")
		return &v1.NodeUserCredentials{}, nil
	}

	nodeUser, nodeUserCredentials, err := v1.GenerateNodeUser(v1.ConvergerNodeUser())
	if err != nil {
		return nil, fmt.Errorf("Failed to generate NodeUser: %w", err)
	}

	err = entity.CreateOrUpdateNodeUser(ctx, d.params.KubeProvider, nodeUser, d.params.Loops.CreateNodeUser)
	if err != nil {
		return nil, err
	}

	logger.LogDebugF("Node user created via API %s\n", nodeUserCredentials.Name)

	return nodeUserCredentials, nil
}

func (d *Destroyer) createAndSaveCredentials(ctx context.Context, logger log.Logger) (*NodesWithCredentials, error) {
	if d.params.PhasedActionProvider == nil {
		return nil, fmt.Errorf("Internal error. PhasedActionProvider not initialized. Probably you try to destroy when need abort")
	}

	nodeIPs, err := entity.GetMasterNodesIPs(ctx, d.params.KubeProvider, d.params.Loops.GetMastersIPs)
	if err != nil {
		return nil, err
	}

	if len(nodeIPs) == 0 {
		return nil, fmt.Errorf("Failed to get master nodes IPs: got empty nodes")
	}

	logger.LogDebugF("Found master node IPs: %+v\n", nodeIPs)

	// always create node user creds so we have only master
	var nodesWithCredentials *NodesWithCredentials

	err = d.params.PhasedActionProvider().Run(phases.CreateStaticDestroyerNodeUserPhase, false, func() (phases.DefaultContextType, error) {
		nodeUserCredentials, err := d.createNodeUserCredentials(ctx, nodeIPs, logger)
		if err != nil {
			return nil, err
		}

		nodesWithCredentials = &NodesWithCredentials{
			NodeUser: nodeUserCredentials,
			IPs:      nodeIPs,
		}

		if err := d.params.State.SaveNodeUser(nodesWithCredentials); err != nil {
			return nil, err
		}

		logger.LogDebugF("Node user '%s' saved to cache and to destroyer. Empty is correct for single master \n", nodeUserCredentials.Name)

		return nil, nil
	})

	if err != nil {
		return nil, err
	}

	return nodesWithCredentials, nil
}

func (d *Destroyer) hostProcessed(host session.Host) bool {
	if d.nodesWithCredentials == nil {
		return false
	}

	return d.nodesWithCredentials.SetHostAsProcessed(host)
}

func (d *Destroyer) addHostAsProcessed(host session.Host) error {
	// for abort we do not have nodesWithCredentials
	if d.nodesWithCredentials == nil {
		return nil
	}

	return d.params.PhasedActionProvider().Run(phases.UpdateStaticDestroyerIPs, false, func() (phases.DefaultContextType, error) {
		d.nodesWithCredentials.AddToProcessed(host)

		if err := d.params.State.SaveNodeUser(d.nodesWithCredentials); err != nil {
			return nil, err
		}

		d.logger().LogDebugF("Host %+v saved as processed to cache. Have processed hosts %+v\n", host, d.nodesWithCredentials.ProcessedIPS)

		return nil, nil
	})
}

func (d *Destroyer) logger() log.Logger {
	return log.SafeProvideLogger(d.params.LoggerProvider)
}

var getDestroyMastersDefaultOpts = retry.AttemptsWithWaitOpts(5, 15*time.Second)

func (d *Destroyer) destroyMasterLoopParams(host session.Host) retry.Params {
	return retry.SafeCloneOrNewParams(d.params.Loops.DestroyMaster, getDestroyMastersDefaultOpts...).
		WithName(fmt.Sprintf("Clear master %s", host.String()))
}

func isSingleMaster(ips []entity.NodeIP) bool {
	return len(ips) == 1
}
