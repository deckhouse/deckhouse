// Copyright 2026 Flant JSC
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

package bootstrap

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/kube"
	"github.com/deckhouse/lib-connection/pkg/provider"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	libretry "github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	// immutableAPIPort is where the node's own kube-apiserver listens.
	immutableAPIPort = 6443

	// immutableWaitAttempts and immutableWaitInterval bound every wait on the
	// node bringing its control plane up: pulling the control-plane images,
	// starting the static pods and registering the Node take a few minutes on a
	// cold registry.
	immutableWaitAttempts = 250
	immutableWaitInterval = time.Second
)

// buildImmutableMasterPayload renders the cloud-init the first master boots
// with. The node has no sshd and no bashible, so everything dhctl would
// otherwise upload afterwards has to be in here.
func (b *ClusterBootstrapper) buildImmutableMasterPayload(ctx context.Context, bctx *bootstrapContext, nodeName string) (string, error) {
	var cloudConfig string

	err := dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Prepare immutable master payload", func(ctx context.Context) error {
		nodeConfig, err := immutable.BuildNodeConfig(ctx, immutable.NodeConfigInput{
			NodeName:   nodeName,
			MetaConfig: bctx.metaConfig,
		})
		if err != nil {
			return fmt.Errorf("build node config: %w", err)
		}

		controlPlaneConfig, err := immutable.BuildControlPlaneConfig(ctx, immutable.ControlPlaneInput{
			NodeName:   nodeName,
			MetaConfig: bctx.metaConfig,
			GlobalOpts: &b.Options.Global,
			StateCache: bctx.stateCache,
		})
		if err != nil {
			return fmt.Errorf("build control-plane config: %w", err)
		}

		cloudConfig, err = immutable.BuildCloudConfig(nodeConfig, controlPlaneConfig)
		if err != nil {
			return fmt.Errorf("build cloud config: %w", err)
		}

		return nil
	})

	return cloudConfig, err
}

// connectToImmutableMaster waits for the node to finish bringing its own
// control plane up and hands the rest of the bootstrap a Kubernetes client.
//
// There is no bashible pipeline here: the node issues its own certificates and
// kubeconfigs, lays the manifests down, waits for its apiserver and creates the
// first cluster objects itself. dhctl only needs a way in.
func (b *ClusterBootstrapper) connectToImmutableMaster(ctx context.Context, bctx *bootstrapContext) error {
	if bctx.masterIP == "" {
		return fmt.Errorf("the first master address is unknown: rerun the bootstrap so the BaseInfra phase reports it")
	}

	server, err := b.openImmutableAPIChannel(ctx, bctx)
	if err != nil {
		return err
	}

	kubeconfigPath, err := b.writeImmutableKubeconfig(ctx, bctx, server)
	if err != nil {
		return err
	}

	kubeProvider, err := newKubeconfigKubeProvider(ctx, b, kubeconfigPath)
	if err != nil {
		return err
	}
	b.KubeProvider = kubeProvider

	// Client() retries the connection and then polls the API until it answers,
	// which is exactly the "wait for the node to bring its control plane up"
	// step this phase replaces the bashible pipeline with.
	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return fmt.Errorf("connect to the API server of the immutable master at %s: %w", server, err)
	}

	return waitForImmutableMasterNode(ctx, kubeCl, bctx.masterNodeName)
}

// openImmutableAPIChannel returns the URL dhctl talks to the master's apiserver
// on. Without a bastion that is the master itself; with one it is the local end
// of a forward through the bastion — the bastion opens a direct-tcpip channel
// to an arbitrary address, so no sshd on the master is involved.
func (b *ClusterBootstrapper) openImmutableAPIChannel(ctx context.Context, bctx *bootstrapContext) (string, error) {
	connectionConfig := b.SSHProviderInitializer.GetConfig()
	if connectionConfig == nil || connectionConfig.Config == nil || connectionConfig.Config.BastionHost == "" {
		return "https://" + net.JoinHostPort(bctx.masterIP, strconv.Itoa(immutableAPIPort)), nil
	}

	localPort, err := freeLocalPort()
	if err != nil {
		return "", fmt.Errorf("reserve a local port for the API tunnel: %w", err)
	}

	tunnel, stop, err := b.openBastionTunnel(ctx, connectionConfig.Config, bctx.masterIP, localPort)
	if err != nil {
		return "", err
	}
	bctx.immutableTunnelStop = stop

	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(
		"API server tunnel through the bastion is up: %s", tunnel.String(),
	))

	// 127.0.0.1 is always in the SAN list of a kube-apiserver certificate, so
	// TLS verifies against the cluster CA without any extra name.
	return fmt.Sprintf("https://127.0.0.1:%d", localPort), nil
}

// openBastionTunnel forwards a local port to the master's API port through the
// bastion.
func (b *ClusterBootstrapper) openBastionTunnel(ctx context.Context, sshConfig *sshconfig.Config, masterIP string, localPort int) (libcon.Tunnel, func(), error) {
	// The SSH session is retargeted at the bastion itself instead of using it
	// as a jump host: the forward has to originate on the bastion, because the
	// master runs no sshd to originate it on.
	bastionConfig := sshConfig.Clone()
	bastionConfig.User = sshConfig.BastionUser
	bastionConfig.Port = sshConfig.BastionPort
	// gossh authenticates the target host with SudoPassword when no key
	// matches, so the bastion password has to travel in that field.
	bastionConfig.SudoPassword = sshConfig.BastionPassword
	bastionConfig.BastionHost = ""
	bastionConfig.BastionUser = ""
	bastionConfig.BastionPassword = ""
	// Only the gossh backend can forward without running a command on the
	// target; the legacy clissh one keeps an interactive session open there.
	bastionConfig.ForceLegacy = false
	bastionConfig.ForceModern = true

	sshProvider := provider.NewDefaultSSHProvider(
		b.SSHProviderInitializer.GetSettings(),
		&sshconfig.ConnectionConfig{
			Config: bastionConfig,
			Hosts:  []sshconfig.Host{{Host: sshConfig.BastionHost}},
		},
		provider.SSHClientWithStartAfterCreate(true),
	)

	sshClient, err := sshProvider.Client(ctx)
	if err != nil {
		_ = sshProvider.Cleanup(ctx)
		return nil, nil, fmt.Errorf("connect to the bastion host %s: %w", sshConfig.BastionHost, err)
	}

	tunnel := sshClient.Tunnel(fmt.Sprintf("%s:%d:127.0.0.1:%d", masterIP, immutableAPIPort, localPort))
	if err := tunnel.Up(ctx); err != nil {
		_ = sshProvider.Cleanup(ctx)
		return nil, nil, fmt.Errorf("forward %d to %s:%d through the bastion %s: %w", localPort, masterIP, immutableAPIPort, sshConfig.BastionHost, err)
	}

	stop := func() {
		tunnel.Stop()
		_ = sshProvider.Cleanup(ctx)
	}

	return tunnel, stop, nil
}

// writeImmutableKubeconfig mints installer credentials from the CA the node was
// given and stores them in a kubeconfig file. There is no sshd to copy
// admin.conf off the node with, so dhctl issues its own client certificate.
func (b *ClusterBootstrapper) writeImmutableKubeconfig(ctx context.Context, bctx *bootstrapContext, server string) (string, error) {
	ca, err := immutable.LoadCABundle(ctx, bctx.stateCache)
	if err != nil {
		return "", err
	}
	if len(ca) == 0 {
		return "", fmt.Errorf("the control-plane CA bundle is missing from the state cache: rerun the bootstrap so the BaseInfra phase regenerates the master payload")
	}

	content, err := immutable.BuildAdminKubeconfig(ctx, immutable.AdminKubeconfigInput{
		CACertPEM: ca["ca.crt"],
		CAKeyPEM:  ca["ca.key"],
		Server:    server,
	})
	if err != nil {
		return "", err
	}

	// os.CreateTemp opens the file with mode 0600; it holds a system:masters
	// key, so it is removed again once dhctl exits.
	file, err := os.CreateTemp(b.TmpDir, "dhctl-immutable-kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create a temporary kubeconfig: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(content); err != nil {
		return "", fmt.Errorf("write the temporary kubeconfig %s: %w", file.Name(), err)
	}

	path := file.Name()
	tomb.RegisterOnShutdown("Delete the temporary installer kubeconfig", func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("failed to remove %s: %v", path, err))
		}
	})

	return path, nil
}

// newKubeconfigKubeProvider builds a Kubernetes provider that talks to the API
// server directly through the given kubeconfig, with no SSH runner behind it.
func newKubeconfigKubeProvider(ctx context.Context, b *ClusterBootstrapper, kubeconfigPath string) (libcon.KubeProvider, error) {
	kubeConfig := &kube.Config{KubeConfig: kubeconfigPath}

	// A kubeconfig-backed client needs no SSH provider, hence the nil.
	runner, err := provider.GetRunnerInterface(ctx, kubeConfig, b.SSHProviderInitializer.GetSettings(), nil)
	if err != nil {
		return nil, fmt.Errorf("build the Kubernetes runner interface: %w", err)
	}

	waitParams := libretry.NewEmptyParams(
		libretry.WithWait(immutableWaitInterval),
		libretry.WithAttempts(immutableWaitAttempts),
		libretry.WithLogger(dhlog.FromContext(ctx)),
	)

	return provider.NewDefaultKubeProvider(b.SSHProviderInitializer.GetSettings(), kubeConfig, runner).
		WithLoopsParams(provider.KubeProviderLoopsParams{
			InitClient:   waitParams,
			WaitingReady: waitParams,
		}), nil
}

// waitForImmutableMasterNode waits until kubelet has registered the node. The
// node also creates the bootstrap RBAC, the control-plane label and taint and
// the d8-pki Secret on its own; dhctl creates none of them.
func waitForImmutableMasterNode(ctx context.Context, kubeCl libcon.KubeClient, nodeName string) error {
	return retry.NewLoop("Waiting for the first master node to register", immutableWaitAttempts, immutableWaitInterval).
		RunContext(ctx, func() error {
			_, err := kubeCl.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("get node %s: %w", nodeName, err)
			}
			return nil
		})
}

// freeLocalPort reserves an ephemeral port by binding and releasing it, so the
// tunnel can bind it right after.
func freeLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address type %T", listener.Addr())
	}
	return addr.Port, nil
}
