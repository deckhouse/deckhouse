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
	"log/slog"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/name212/govalue"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
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
	SSHClientProvider    libcon.SSHProvider
	KubeProvider         kube.ClientProviderWithCleanup
	State                *State
	Logger               *slog.Logger
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

	logger.DebugContext(ctx, "Starting prepare static destroyer")
	defer logger.DebugContext(ctx, "Finished prepare static destroyer")

	var err error

	d.nodesWithCredentials, err = d.params.State.NodeUser(ctx)

	if err != nil {
		if !errors.Is(err, errNotFoundCredentials) {
			return fmt.Errorf("Error getting node user from cache: %w", err)
		}

		d.nodesWithCredentials, err = d.createAndSaveCredentials(ctx, logger)
		if err != nil {
			return err
		}
	} else {
		logger.DebugContext(ctx, "Found existing nodes with credentials. Saved to destroyer and skipping creation")
	}

	return d.waitNodeUserExists(ctx)
}

func (d *Destroyer) AfterResourcesDelete(context.Context) error {
	return nil
}

func (d *Destroyer) CleanupBeforeDestroy(ctx context.Context) error {
	d.params.KubeProvider.Cleanup(ctx, false)
	return nil
}

func (d *Destroyer) DestroyCluster(ctx context.Context, autoApprove bool) error {
	if govalue.IsNil(d.params.SSHClientProvider) {
		return errors.New("Internal error. SSH provider was not passed")
	}

	return d.params.PhasedActionProvider().Run(ctx, phases.AllNodesPhase, true, func() (phases.DefaultContextType, error) {
		err := d.destroyCluster(ctx, autoApprove)
		if err != nil {
			return nil, err
		}

		return nil, nil
	})
}

func (d *Destroyer) destroyCluster(ctx context.Context, autoApprove bool) error {
	if !autoApprove {
		if !input.NewConfirmation().WithMessage("Do you really want to cleanup control-plane nodes?").Ask() {
			return fmt.Errorf("Cleaning up master nodes is not allowed")
		}
	}

	logger := d.logger()

	sshClient, err := d.params.SSHClientProvider.Client(ctx)
	if err != nil {
		return err
	}

	logger.DebugContext(ctx, "Starting static cluster destroy process")
	masterHosts := sshClient.Session().AvailableHosts()
	stdOutErrHandler := func(l string) {
		// Cleanup script streams its own `[INFO] ...`/`[ERROR] ...` lines; keep the WARN severity
		// but tag FileOnly so they stay in the debug log and never flood the compact terminal.
		// lib-connection already echoes every streamed line to the debug file (`ssh: <line>`), so
		// this handler is debug-only by design — keep it non-nil or that echo disappears too.
		logger.LogAttrs(ctx, slog.LevelWarn, l, dhlog.FileOnly())
	}

	logger.DebugContext(ctx, "Discovering additional master nodes")
	hostToExclude := ""

	ips := make([]entity.NodeIP, 0)
	// for abort we do not have nodesWithCredentials
	if d.nodesWithCredentials != nil && !isSingleMaster(d.nodesWithCredentials.IPs) {
		ips = d.nodesWithCredentials.IPs
	}

	if len(ips) > 0 {
		err := dhlog.RunProcess(ctx, logger, "Get internal node IP for passed control-plane host", func(ctx context.Context) error {
			file := sshClient.File()

			bytes, err := file.DownloadBytes(ctx, "/var/lib/bashible/discovered-node-ip")
			if err != nil {
				return err
			}

			hostToExclude = strings.TrimSpace(string(bytes))
			logger.DebugContext(ctx, fmt.Sprintf("Got internal node IP for passed control-plane host: %s", hostToExclude))

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

	cmd := `test -f /var/lib/bashible/cleanup_static_node.sh || { echo "ERROR: cleanup_static_node.sh not found"; exit 1; }; bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing`

	userPassedSSHSetting := sshClient.Session().Copy()

	if len(additionalMastersHosts) > 0 {
		logger.DebugContext(ctx, fmt.Sprintf("Found %d additional masters, destroying them", len(additionalMastersHosts)))
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
				logger.InfoContext(ctx, fmt.Sprintf("Skipping additional master host: '%s'. Host already processed", host.String()))
				continue
			}
			settings.SetAvailableHosts([]session.Host{host})
			sshClient, err = d.switchToNodeUser(ctx, d.params.SSHClientProvider, settings)
			if err != nil {
				return err
			}

			err = d.processStaticHost(ctx, sshClient, host, stdOutErrHandler, cmd)
			if err != nil {
				return err
			}

			logger.DebugContext(ctx, fmt.Sprintf("Host %s was cleaned up successfully", host.Host))
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

			sshClient, err = d.switchToNodeUser(ctx, d.params.SSHClientProvider, settings)
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

func (d *Destroyer) processStaticHost(ctx context.Context, sshClient libcon.SSHClient, host session.Host, stdOutErrHandler func(l string), cmd string) error {
	d.logger().DebugContext(ctx, fmt.Sprintf("Starting cleanup process for host %s", host))

	err := retry.NewLoopWithParams(d.destroyMasterLoopParams(host)).RunContext(ctx, func() error {
		c := sshClient.Command(cmd)
		c.Sudo(ctx)
		c.WithTimeout(30 * time.Second)
		c.WithStdoutHandler(stdOutErrHandler)
		c.WithStderrHandler(stdOutErrHandler)
		err := c.Run(ctx)
		if err != nil {
			if ee, ok := errors.AsType[*exec.ExitError](err); ok {
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

	return d.addHostAsProcessed(ctx, host)
}

func (d *Destroyer) switchToNodeUser(ctx context.Context, sshProvider libcon.SSHProvider, settings *session.Session) (libcon.SSHClient, error) {
	if d.nodesWithCredentials == nil {
		return nil, fmt.Errorf("Internal error. No nodes with credentials in destroyer. Probably Prepare was not called, or destroy was attempted during an abort")
	}

	if d.params.TmpDir == "" {
		return nil, fmt.Errorf("Internal error. No tmp dir passed")
	}

	logger := d.logger()

	logger.InfoContext(ctx, "Switch to node user for next control-plane host")

	tmpDir := filepath.Join(d.params.TmpDir, "destroy")

	logger.DebugContext(ctx, fmt.Sprintf("Starting replacing SSH client. Key directory: %s", tmpDir))

	err := os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache directory for NodeUser: %w", err)
	}

	logger.DebugContext(ctx, fmt.Sprintf("Tempdir '%s' created for SSH client", tmpDir))

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

	logger.DebugContext(ctx, "Private key written")

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

	oldSSHClient, err := sshProvider.Client(ctx)
	if err != nil {
		return nil, err
	}

	oldPrivateKeys := oldSSHClient.PrivateKeys()
	for _, oldKey := range oldPrivateKeys {
		// skip another temp keys for another hosts
		// add only user passed keys
		if !strings.HasPrefix(oldKey.Key, privateKeyPrefixPathWithoutSuffix) {
			privateKeys = append(privateKeys, oldKey)
		}
	}

	newSSHClient, err := d.params.SSHClientProvider.SwitchClient(ctx, sess, privateKeys)
	if err != nil {
		return nil, err
	}

	logger.DebugContext(ctx, fmt.Sprintf("New SSH Client: %-v", newSSHClient))
	err = newSSHClient.Start()
	if err != nil {
		return nil, fmt.Errorf("Failed to start SSH client: %w", err)
	}

	if err := newSSHClient.RefreshPrivateKeys(); err != nil {
		return nil, fmt.Errorf("Failed to refresh private keys: %w", err)
	}

	logger.DebugContext(ctx, "Private keys refreshed for replacing kube client")

	return newSSHClient, nil
}

func (d *Destroyer) waitNodeUserExists(ctx context.Context) error {
	if d.params.PhasedActionProvider == nil {
		return fmt.Errorf("Internal error. PhasedActionProvider not initialized. Probably you tried to destroy when an abort was needed")
	}

	if d.nodesWithCredentials == nil {
		return fmt.Errorf("Internal error. nodesWithCredentials not initialized. Probably you tried to destroy when an abort was needed")
	}

	if len(d.nodesWithCredentials.IPs) == 0 {
		return fmt.Errorf("Internal error. nodesWithCredentials IPs are empty")
	}

	logger := d.logger()

	if d.params.State.IsNodeUserExists(ctx) {
		logger.DebugContext(ctx, "NodeUser for static destroyer exists getting from cache")
		return nil
	}

	return d.params.PhasedActionProvider().Run(ctx, phases.WaitStaticDestroyerNodeUserPhase, false, func() (phases.DefaultContextType, error) {
		if !isSingleMaster(d.nodesWithCredentials.IPs) {
			// waiter checks if nil params
			waiter := entity.NewConvergerNodeUserExistsWaiter(d.params.KubeProvider).
				WithParams(d.params.Loops.NodeUser)

			if err := waiter.WaitPresentOnNodes(ctx, d.nodesWithCredentials.NodeUser); err != nil {
				return nil, err
			}
		} else {
			logger.DebugContext(ctx, "No wait NodeUser for single-master cluster")
		}

		return nil, d.params.State.SetNodeUserExists(ctx)
	})
}

func (d *Destroyer) createNodeUserCredentials(ctx context.Context, ips []entity.NodeIP, logger *slog.Logger) (*v1.NodeUserCredentials, error) {
	if isSingleMaster(ips) {
		logger.DebugContext(ctx, "Has single master. Skip creating node user and returns empty credentials for save")
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

	logger.DebugContext(ctx, fmt.Sprintf("Node user created via API %s", nodeUserCredentials.Name))

	return nodeUserCredentials, nil
}

func (d *Destroyer) createAndSaveCredentials(ctx context.Context, logger *slog.Logger) (*NodesWithCredentials, error) {
	if d.params.PhasedActionProvider == nil {
		return nil, fmt.Errorf("Internal error. PhasedActionProvider not initialized. Probably you tried to destroy when an abort was needed")
	}

	nodeIPs, err := entity.GetMasterNodesIPs(ctx, d.params.KubeProvider, d.params.Loops.GetMastersIPs)
	if err != nil {
		return nil, err
	}

	if len(nodeIPs) == 0 {
		return nil, fmt.Errorf("Failed to get master nodes IPs: got empty nodes")
	}

	logger.DebugContext(ctx, fmt.Sprintf("Found master node IPs: %+v", nodeIPs))

	// always create node user creds so we have only master
	var nodesWithCredentials *NodesWithCredentials

	err = d.params.PhasedActionProvider().Run(ctx, phases.CreateStaticDestroyerNodeUserPhase, false, func() (phases.DefaultContextType, error) {
		nodeUserCredentials, err := d.createNodeUserCredentials(ctx, nodeIPs, logger)
		if err != nil {
			return nil, err
		}

		nodesWithCredentials = &NodesWithCredentials{
			NodeUser: nodeUserCredentials,
			IPs:      nodeIPs,
		}

		if err := d.params.State.SaveNodeUser(ctx, nodesWithCredentials); err != nil {
			return nil, err
		}

		logger.DebugContext(ctx, fmt.Sprintf("Node user '%s' saved to cache and to destroyer. Empty is correct for single master ", nodeUserCredentials.Name))

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

func (d *Destroyer) addHostAsProcessed(ctx context.Context, host session.Host) error {
	// for abort we do not have nodesWithCredentials
	if d.nodesWithCredentials == nil {
		return nil
	}

	return d.params.PhasedActionProvider().Run(ctx, phases.UpdateStaticDestroyerIPs, false, func() (phases.DefaultContextType, error) {
		d.nodesWithCredentials.AddToProcessed(host)

		if err := d.params.State.SaveNodeUser(ctx, d.nodesWithCredentials); err != nil {
			return nil, err
		}

		d.logger().DebugContext(ctx, fmt.Sprintf("Host %+v saved as processed to cache. Have processed hosts %+v", host, d.nodesWithCredentials.ProcessedIPS))

		return nil, nil
	})
}

func (d *Destroyer) logger() *slog.Logger {
	return d.params.Logger
}

var getDestroyMastersDefaultOpts = retry.AttemptsWithWaitOpts(75, 1*time.Second)

func (d *Destroyer) destroyMasterLoopParams(host session.Host) retry.Params {
	return retry.SafeCloneOrNewParams(d.params.Loops.DestroyMaster, getDestroyMastersDefaultOpts...).
		WithName(fmt.Sprintf("Clear master %s", host.String()))
}

func isSingleMaster(ips []entity.NodeIP) bool {
	return len(ips) == 1
}
