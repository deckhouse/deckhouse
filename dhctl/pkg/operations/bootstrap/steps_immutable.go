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
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
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
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	// immutableAPIPort is where the node's own kube-apiserver listens.
	immutableAPIPort = 6443

	// immutableAPIWaitAttempts and immutableAPIWaitInterval bound the wait for
	// the very first answer from the node — 30 minutes. Nothing has happened on
	// the VM yet when this wait starts: it still has to install the OS onto its
	// disk, reboot into it, pull three system extensions, start containerd and
	// kubelet, generate the cluster PKI, pull four control-plane images and only
	// then serve. The classic path spends its budget on "sshd answers", which is
	// a fraction of that.
	immutableAPIWaitAttempts = 60
	immutableAPIWaitInterval = 30 * time.Second

	// immutableWaitAttempts and immutableWaitInterval bound everything that
	// happens after the apiserver answers. Registering the Node is the node's
	// next step, so a couple of minutes is generous.
	immutableWaitAttempts = 120
	immutableWaitInterval = time.Second
)

// buildImmutableMasterPayload renders the cloud-init the first master boots
// with. The node has no sshd and no bashible, so everything dhctl would
// otherwise upload afterwards has to be in here.
//
// The result is base64-encoded because that is what the "cloudConfig" tfvar
// carries: every provider's terraform base64decodes it before writing the
// cloud-init secret, and the only other producer of that variable (the cloud
// config secret read in kubernetes/actions/entity) encodes it too. The encoding
// happens here rather than inside BuildCloudConfig so the document itself stays
// readable — it is pinned by a golden file.
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
			StateCache: bctx.stateCache,
		})
		if err != nil {
			return fmt.Errorf("build control-plane config: %w", err)
		}

		document, err := immutable.BuildCloudConfig(nodeConfig, controlPlaneConfig)
		if err != nil {
			return fmt.Errorf("build cloud config: %w", err)
		}

		cloudConfig = base64.StdEncoding.EncodeToString([]byte(document))

		return nil
	})

	return cloudConfig, err
}

// connectToImmutableMaster collects the cluster credentials from the node and
// hands the rest of the bootstrap a Kubernetes client.
//
// There is no bashible pipeline here: the node generates the cluster PKI, lays
// the manifests down, waits for its apiserver and creates the first cluster
// objects itself. dhctl never sees a cluster key until the node gives it one.
func (b *ClusterBootstrapper) connectToImmutableMaster(ctx context.Context, bctx *bootstrapContext) error {
	if bctx.masterIP == "" {
		return fmt.Errorf("the first master address is unknown: rerun the bootstrap so the BaseInfra phase reports it")
	}

	kubeconfig, err := b.collectImmutableKubeconfig(ctx, bctx)
	if err != nil {
		return err
	}

	server, err := b.openImmutableAPIChannel(ctx, bctx)
	if err != nil {
		return err
	}

	content, err := immutable.RetargetKubeconfig(kubeconfig, server)
	if err != nil {
		return err
	}

	kubeconfigPath, err := b.writeImmutableKubeconfig(ctx, b.TmpDir, content)
	if err != nil {
		return err
	}

	kubeProvider, err := newKubeconfigKubeProvider(ctx, b, kubeconfigPath)
	if err != nil {
		return err
	}
	b.KubeProvider = kubeProvider

	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return fmt.Errorf("connect to the API server of the immutable master at %s: %w", server, err)
	}

	// Handed over only after the client above proved the credentials work, so
	// what the operator gets is never a kubeconfig that cannot connect.
	if err := b.saveAdminKubeconfig(ctx, content); err != nil {
		return err
	}

	// The file holds admin credentials and is read exactly once, here: the
	// runner behind this provider is the local one, which never reports a
	// switched connection, so no later Client() call rebuilds the client from
	// it. In dhctl-server the process outlives the bootstrap, so waiting for
	// the shutdown hook to remove it means leaving it on disk for hours.
	removeImmutableKubeconfig(ctx, kubeconfigPath)

	return waitForImmutableMasterNode(ctx, kubeCl, bctx.masterNodeName)
}

// collectImmutableKubeconfig waits for the node's one-shot handoff endpoint and
// reads the admin kubeconfig out of it.
//
// This is the whole "wait for the node to install itself and bring a control
// plane up" step: the endpoint does not answer before that is done. The channel
// to it is closed again as soon as the credentials are in hand — it serves
// once, so there is nothing left to reach.
func (b *ClusterBootstrapper) collectImmutableKubeconfig(ctx context.Context, bctx *bootstrapContext) ([]byte, error) {
	material, err := immutable.LoadHandoffMaterial(ctx, bctx.stateCache)
	if err != nil {
		return nil, err
	}
	if material == nil {
		return nil, fmt.Errorf("the bootstrap handoff credentials are missing from the state cache: rerun the bootstrap so the BaseInfra phase regenerates the master payload")
	}

	address, stop, err := b.openImmutableChannel(ctx, bctx, immutable.HandoffPort, "credentials handoff")
	if err != nil {
		return nil, err
	}
	if stop != nil {
		defer stop()
	}

	input := immutable.FetchKubeconfigInput{
		Address: address,
		// The endpoint's certificate is issued for the node's name, not for the
		// address dhctl dialled: that address did not exist when the payload
		// was built.
		ServerName: bctx.masterNodeName,
		Material:   material,
	}

	var kubeconfig []byte
	err = libretry.NewLoop("Waiting for the first master to hand over the cluster credentials", immutableAPIWaitAttempts, immutableAPIWaitInterval).
		BreakIf(func(err error) bool {
			return errors.Is(err, immutable.ErrHandoffUnauthorized) || errors.Is(err, immutable.ErrHandoffAlreadyServed)
		}).
		RunContext(ctx, func() error {
			collected, err := immutable.FetchKubeconfig(ctx, input)
			if err != nil {
				return err
			}
			kubeconfig = collected
			return nil
		})
	if err != nil {
		return nil, err
	}

	return kubeconfig, nil
}

// openImmutableAPIChannel returns the URL dhctl talks to the master's apiserver
// on and keeps the tunnel behind it open for the rest of the bootstrap.
func (b *ClusterBootstrapper) openImmutableAPIChannel(ctx context.Context, bctx *bootstrapContext) (string, error) {
	address, stop, err := b.openImmutableChannel(ctx, bctx, immutableAPIPort, "API server")
	if err != nil {
		return "", err
	}
	if stop != nil {
		bctx.immutableTunnelStop = stop
	}

	return "https://" + address, nil
}

// openImmutableChannel returns the host:port dhctl reaches the given port of
// the master on, and the function that closes the tunnel behind it — nil when
// there is no tunnel.
//
// Without a bastion that is the master itself; with one it is the local end of
// a forward through the bastion, which opens a direct-tcpip channel to an
// arbitrary address, so no sshd on the master is involved.
func (b *ClusterBootstrapper) openImmutableChannel(ctx context.Context, bctx *bootstrapContext, remotePort int, purpose string) (string, func(), error) {
	connectionConfig := b.SSHProviderInitializer.GetConfig()
	if connectionConfig == nil || connectionConfig.Config == nil || connectionConfig.Config.BastionHost == "" {
		return net.JoinHostPort(bctx.masterIP, strconv.Itoa(remotePort)), nil, nil
	}

	localPort, err := freeLocalPort()
	if err != nil {
		return "", nil, fmt.Errorf("reserve a local port for the %s tunnel: %w", purpose, err)
	}

	tunnel, stop, err := b.openBastionTunnel(ctx, connectionConfig.Config, bctx.masterIP, remotePort, localPort)
	if err != nil {
		return "", nil, err
	}

	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(
		"%s tunnel through the bastion is up: %s", purpose, tunnel.String(),
	))

	// 127.0.0.1 is always in the SAN list of a kube-apiserver certificate, and
	// the handoff endpoint is verified by name rather than by address, so
	// neither channel needs the local end to be nameable.
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)), stop, nil
}

// openBastionTunnel forwards a local port to the given port of the master
// through the bastion.
func (b *ClusterBootstrapper) openBastionTunnel(ctx context.Context, sshConfig *sshconfig.Config, masterIP string, remotePort, localPort int) (libcon.Tunnel, func(), error) {
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

	tunnel := sshClient.Tunnel(fmt.Sprintf("%s:%d:127.0.0.1:%d", masterIP, remotePort, localPort))
	if err := tunnel.Up(ctx); err != nil {
		_ = sshProvider.Cleanup(ctx)
		return nil, nil, fmt.Errorf("forward %d to %s:%d through the bastion %s: %w", localPort, masterIP, remotePort, sshConfig.BastionHost, err)
	}

	stopDrain := drainTunnelErrors(ctx, tunnel)

	stop := func() {
		// Stop() first: it releases HealthMonitor, which is what the drain
		// reads from.
		tunnel.Stop()
		stopDrain()
		_ = sshProvider.Cleanup(ctx)
	}

	return tunnel, stop, nil
}

// drainTunnelErrors keeps the tunnel accepting connections and returns the
// function that stops the drain again.
//
// Every proxied connection posts the error it ends with — usually "use of
// closed network connection" from whichever direction lost the race to close —
// into a channel buffered for ten, and only HealthMonitor takes them out.
// Without it the eleventh connection blocks the accept loop and the API channel
// dies in the middle of installing Deckhouse. The errors themselves are of no
// interest: a request that hit one fails on its own and the client retries.
func drainTunnelErrors(ctx context.Context, tunnel libcon.Tunnel) func() {
	logger := dhlog.FromContext(ctx)

	// Buffered, so HealthMonitor is never left blocked on a hand-over after
	// the drain has stopped.
	errorCh := make(chan error, 16)
	go tunnel.HealthMonitor(errorCh)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case err := <-errorCh:
				logger.DebugContext(ctx, fmt.Sprintf("API server tunnel: %v", err))
			case <-done:
				return
			}
		}
	}()

	return func() { close(done) }
}

// writeImmutableKubeconfig stores the collected admin kubeconfig in a file the
// Kubernetes client can be built from, with its server URL pointed at the
// address dhctl reaches the API on.
func (b *ClusterBootstrapper) writeImmutableKubeconfig(ctx context.Context, dir string, content []byte) (string, error) {
	// os.CreateTemp opens the file with mode 0600; it holds admin credentials,
	// so it is removed again once dhctl exits.
	file, err := os.CreateTemp(dir, "dhctl-immutable-kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create a temporary kubeconfig: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(content); err != nil {
		return "", fmt.Errorf("write the temporary kubeconfig %s: %w", file.Name(), err)
	}

	// The happy path removes the file as soon as the client is built; this
	// covers the runs that never get that far.
	path := file.Name()
	tomb.RegisterOnShutdown("Delete the temporary installer kubeconfig", func() {
		removeImmutableKubeconfig(ctx, path)
	})

	return path, nil
}

// saveAdminKubeconfig hands the admin kubeconfig to the operator, at the path
// --kubeconfig-out names or next to the run's log and trace files.
//
// It is written by default rather than on request because a cluster whose first
// master runs an immutable OS has no second way in: the node runs no SSH server,
// so the kubeconfig the classic bootstrap leaves in /root/.kube/config on the
// master cannot be fetched from it, and the handoff endpoint this one came
// through serves once and is already closed. Defaulting to "do not keep it"
// would mean defaulting to a cluster nobody can reach.
func (b *ClusterBootstrapper) saveAdminKubeconfig(ctx context.Context, content []byte) error {
	path := b.Options.Bootstrap.KubeconfigOut
	if path == "" {
		path = filepath.Join(b.TmpDir, cache.AdminKubeconfigName)
	}

	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write the admin kubeconfig to %s: %w", path, err)
	}
	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Admin kubeconfig written to %s — it holds cluster-admin credentials, "+
		"and on a cluster of immutable nodes it is the only way in.", path))
	return nil
}

// removeImmutableKubeconfig deletes the temporary kubeconfig, tolerating a
// second call.
func removeImmutableKubeconfig(ctx context.Context, path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("failed to remove %s: %v", path, err))
	}
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

	// InitClient only builds the client out of the kubeconfig, which either
	// works at once or is broken; WaitingReady is the loop that polls
	// /version until the node's apiserver answers, and that is the one the
	// whole "the node installs itself and brings a control plane up" wait
	// hides behind.
	initParams := libretry.NewEmptyParams(
		libretry.WithWait(immutableWaitInterval),
		libretry.WithAttempts(immutableWaitAttempts),
		libretry.WithLogger(dhlog.FromContext(ctx)),
	)
	readyParams := libretry.NewEmptyParams(
		libretry.WithWait(immutableAPIWaitInterval),
		libretry.WithAttempts(immutableAPIWaitAttempts),
		libretry.WithLogger(dhlog.FromContext(ctx)),
	)

	return provider.NewDefaultKubeProvider(b.SSHProviderInitializer.GetSettings(), kubeConfig, runner).
		WithLoopsParams(provider.KubeProviderLoopsParams{
			InitClient:   initParams,
			WaitingReady: readyParams,
		}), nil
}

// waitForImmutableMasterNode waits until kubelet has registered the node. The
// node also creates the bootstrap RBAC, the control-plane label and taint and
// the d8-pki Secret on its own; dhctl creates none of them.
func waitForImmutableMasterNode(ctx context.Context, kubeCl libcon.KubeClient, nodeName string) error {
	return libretry.NewLoop("Waiting for the first master node to register", immutableWaitAttempts, immutableWaitInterval).
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
