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
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/kube"
	"github.com/deckhouse/lib-connection/pkg/provider"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	libretry "github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	// immutableAPIPort is where the node's own kube-apiserver listens.
	immutableAPIPort = 6443

	// immutableAPIWaitAttempts and immutableAPIWaitInterval bound the wait for
	// the node — 30 minutes. Nothing has happened on the VM yet when this wait
	// starts: it still has to install the OS onto its disk, reboot into it, pull
	// three system extensions, start containerd and kubelet, generate the
	// cluster PKI, pull four control-plane images and only then serve. The
	// classic path spends its budget on "sshd answers", which is a fraction of
	// that.
	//
	// The interval is short because the node now answers with its progress from
	// the moment it starts working: a poll is one cheap request, while a long
	// one is dead time on the clock of every bootstrap. At thirty seconds a node
	// that became ready right after a poll was left waiting for the rest of the
	// interval, which is most of a minute added to the wall time for nothing.
	immutableAPIWaitAttempts = 360
	immutableAPIWaitInterval = 5 * time.Second

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

	// The node returned a certificate, not credentials. They become credentials
	// here, where the key has been all along.
	material, err := immutable.LoadHandoffMaterial(ctx, bctx.stateCache)
	if err != nil {
		return err
	}
	if material == nil {
		return fmt.Errorf("the installer's client key is missing from the state cache: rerun the bootstrap so the BaseInfra phase regenerates the master payload")
	}
	complete, err := immutable.WithClientKey(kubeconfig, material.ClientKeyPEM)
	if err != nil {
		return err
	}

	// Stored before anything else is attempted. These are the only credentials
	// that will ever leave the node, and everything below — building a client,
	// waiting up to half an hour for the API server — can fail. Losing them
	// there would leave a running cluster nobody can reach.
	//
	// The operator's copy keeps the address the node put in it: its own. The
	// retargeted one below points at a tunnel this process owns, which stops
	// existing the moment the bootstrap ends — saving that one would hand over a
	// kubeconfig that works exactly until it is needed.
	if err := b.saveAdminKubeconfig(ctx, complete, bctx); err != nil {
		return err
	}

	content, err := immutable.RetargetKubeconfig(complete, server)
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

	// The credentials are stored and proven to work, so the node may stop
	// offering them.
	b.confirmImmutableHandoff(ctx, bctx)

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
	// Closed through the variable, not the value: the channel is reopened when it
	// breaks, and deferring the first stop would leave the last one running.
	defer func() {
		if stop != nil {
			stop()
		}
	}()

	input := immutable.FetchKubeconfigInput{
		Address: address,
		// The endpoint's certificate is issued for the node's name, not for the
		// address dhctl dialled: that address did not exist when the payload
		// was built.
		ServerName: bctx.masterNodeName,
		Material:   material,
	}

	// The wait is narrated rather than silent: the node answers the status
	// endpoint from the moment it starts working, so an operator watching a
	// bootstrap sees "generating the cluster PKI" instead of nothing at all,
	// and a node that fails says why instead of just staying unreachable.
	logger := dhlog.FromContext(ctx)
	var (
		kubeconfig  []byte
		lastMessage string
		// answered records that the channel has spoken at least once, which is
		// what tells "the tunnel died" apart from "the node is not up yet".
		answered bool
	)
	err = libretry.NewLoop("Waiting for the first master to bring the control plane up", immutableAPIWaitAttempts, immutableAPIWaitInterval).
		BreakIf(func(err error) bool {
			return errors.Is(err, immutable.ErrHandoffUnauthorized) || errors.Is(err, immutable.ErrHandoffAlreadyServed)
		}).
		RunContext(ctx, func() error {
			status, err := immutable.FetchStatus(ctx, input)
			if err != nil {
				// The wait lasts minutes and the channel is an SSH tunnel through a
				// bastion; a connection that drops in the middle leaves every
				// remaining attempt dialling a local port nothing listens on.
				// Reopening is what makes the retries mean anything.
				//
				// Only after the channel has answered at least once. Before that a
				// refused connection is simply the node still installing itself,
				// which is most of this wait — reopening on it would rebuild the
				// tunnel every few seconds and hammer the bastion with hundreds of
				// connections for a bootstrap that is going perfectly well.
				if answered && channelBroken(err) {
					if stop != nil {
						stop()
						stop = nil
					}
					reopened, newStop, openErr := b.openImmutableChannel(ctx, bctx, immutable.HandoffPort, "credentials handoff")
					if openErr != nil {
						return fmt.Errorf("the channel to the first master broke and could not be reopened: %w", openErr)
					}
					stop = newStop
					input.Address = reopened
					logger.InfoContext(ctx, "The channel to the first master broke; reopened it")
				}
				return err
			}
			answered = true
			if message := statusLine(status); message != lastMessage {
				lastMessage = message
				logger.InfoContext(ctx, fmt.Sprintf("The first master reports: %s", message))
			}
			if status.Phase != immutable.PhaseReady && status.Phase != immutable.PhaseCollected {
				return fmt.Errorf("the first master is not ready to hand the credentials over: %s", statusLine(status))
			}

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

// statusLine renders what the node reports into one readable line.
func statusLine(status *immutable.Status) string {
	if status.Message == "" {
		return status.Phase
	}
	return status.Phase + ": " + status.Message
}

// confirmImmutableHandoff tells the node the credentials are safely stored, and
// only then does the node stop offering them. Reported rather than returned:
// the kubeconfig is already on disk by this point, so a failed confirmation
// costs a node that keeps its channel open, not a lost cluster.
func (b *ClusterBootstrapper) confirmImmutableHandoff(ctx context.Context, bctx *bootstrapContext) {
	material, err := immutable.LoadHandoffMaterial(ctx, bctx.stateCache)
	if err != nil || material == nil {
		return
	}

	address, stop, err := b.openImmutableChannel(ctx, bctx, immutable.HandoffPort, "handoff confirmation")
	if err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("could not confirm the handover to the first master: %v", err))
		return
	}
	if stop != nil {
		defer stop()
	}

	input := immutable.FetchKubeconfigInput{Address: address, ServerName: bctx.masterNodeName, Material: material}
	if err := immutable.ConfirmCollected(ctx, input); err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("could not confirm the handover to the first master, it will keep the bootstrap channel open: %v", err))
		return
	}
	dhlog.FromContext(ctx).InfoContext(ctx, "Confirmed the handover; the first master closed its bootstrap channel.")

	// The material has done its job, and what it holds is the private key behind
	// a cluster-admin certificate. Kept, it outlives the bootstrap in the state
	// cache — which for dhctl-server is a Secret in the management cluster, for
	// as long as that object exists. The confirmation is the right moment: the
	// kubeconfig is already on disk and the channel is shut.
	bctx.stateCache.Delete(ctx, immutable.HandoffCacheKey)
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
func (b *ClusterBootstrapper) saveAdminKubeconfig(ctx context.Context, content []byte, bctx *bootstrapContext) error {
	path := b.Options.Bootstrap.KubeconfigOut
	if path == "" {
		// Only the CLI gets a default path. In server mode TmpDir is one directory
		// for the whole process, so the default would put the cluster-admin
		// credentials of every cluster this server ever bootstraps into a single
		// long-lived file, each run silently overwriting the last. The caller there
		// receives the kubeconfig in the response and does not need a copy on disk.
		if b.CommanderMode {
			return nil
		}
		path = filepath.Join(b.TmpDir, cache.AdminKubeconfigName)
	}

	// Removed rather than truncated, then created fresh with O_EXCL. Writing
	// into whatever is already at that path would inherit its mode: a rerun over
	// a file left at 0644 puts cluster-admin credentials on disk world-readable
	// for as long as it takes to widen them back, and follows a symlink someone
	// else put there. O_EXCL is safe here because the path was just cleared.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("replace the admin kubeconfig %s: %w", path, err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create the admin kubeconfig %s: %w", path, err)
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return fmt.Errorf("write the admin kubeconfig to %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("write the admin kubeconfig to %s: %w", path, err)
	}
	bctx.adminKubeconfigPath = path
	b.printHowToReachTheCluster(ctx, path, bctx)
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

// printHowToReachTheCluster says where the credentials are and what to do with
// them. It is the equivalent of the SSH line the classic bootstrap prints: on a
// cluster of immutable nodes there is no SSH, and this file is the only way in.
//
// Called twice on purpose. Once when the file appears, which is where it belongs
// if the run stops there, and once when the bootstrap is over — the ten minutes
// and two thousand lines between the two are exactly how a line printed only at
// the first point gets lost.
func (b *ClusterBootstrapper) printHowToReachTheCluster(ctx context.Context, kubeconfigPath string, bctx *bootstrapContext) {
	logger := dhlog.FromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Admin kubeconfig written to %s — cluster-admin credentials, "+
		"and on a cluster of immutable nodes the only way in.", kubeconfigPath))
	logger.InfoContext(ctx, fmt.Sprintf("To use the cluster:  export KUBECONFIG=%s && kubectl get nodes", kubeconfigPath))

	// With a bastion the address in that file is reachable from the bastion and
	// from nowhere else, so the line above is true only for someone already
	// inside the network. Print how to get there rather than leave the operator
	// to work out that the file needs a tunnel and to guess its shape.
	if line := bastionForwardLine(b.SSHProviderInitializer, bctx.masterIP, kubeconfigPath); line != "" {
		logger.InfoContext(ctx, "The master has no public address; reach it through the bastion first:")
		logger.InfoContext(ctx, "  "+line)
	}
}

// bastionForwardLine builds the command that makes the saved kubeconfig usable
// from outside, or "" when the master is directly reachable and it already is.
//
// The forward lands on 127.0.0.1, which every kube-apiserver certificate carries
// in its SAN list — so the kubeconfig needs no tls-server-name and differs from
// the saved one by the server line alone.
func bastionForwardLine(initializer *providerinitializer.SSHProviderInitializer, masterIP, kubeconfigPath string) string {
	if initializer == nil {
		return ""
	}
	connectionConfig := initializer.GetConfig()
	if connectionConfig == nil || connectionConfig.Config == nil || connectionConfig.Config.BastionHost == "" {
		return ""
	}
	cfg := connectionConfig.Config

	bastionPort := 0
	if cfg.BastionPort != nil {
		bastionPort = *cfg.BastionPort
	}
	return buildBastionForwardLine(cfg.BastionUser, cfg.BastionHost, bastionPort, masterIP, kubeconfigPath)
}

// buildBastionForwardLine is split out to be testable: the value of this line
// is that it can be pasted, and that is worth a test.
func buildBastionForwardLine(bastionUser, bastionHost string, bastionPort int, masterIP, kubeconfigPath string) string {
	bastion := bastionHost
	if bastionUser != "" {
		bastion = bastionUser + "@" + bastion
	}
	port := ""
	if bastionPort != 0 {
		port = fmt.Sprintf(" -p %d", bastionPort)
	}

	// 6445 rather than 6443: the port is opened on the operator's own machine,
	// which may well be running a cluster of its own.
	const localPort = 6445
	return fmt.Sprintf("ssh -f -N%s -L %d:%s:%d %s  &&  sed -i.bak 's|https://%s:%d|https://127.0.0.1:%d|' %s",
		port, localPort, masterIP, immutableAPIPort, bastion,
		masterIP, immutableAPIPort, localPort, kubeconfigPath)
}

// channelBroken reports whether err is the local end of the channel having gone
// away rather than the node saying something. A refused dial, a reset
// connection or a stream that ends mid-answer all mean the tunnel is gone; a
// node answering "not ready yet" does not.
func channelBroken(err error) bool {
	// ECONNREFUSED is deliberately absent: a refused dial is what a node that has
	// not finished installing itself looks like, and it is also what a closed
	// local forward looks like. Treating it as a break would rebuild the tunnel
	// throughout a healthy bootstrap.
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE)
}
