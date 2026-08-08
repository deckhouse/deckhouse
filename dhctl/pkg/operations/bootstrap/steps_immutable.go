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
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	// 30 minutes, because nothing has happened on the VM yet: it still has to
	// install its OS, reboot, pull three system extensions, start kubelet,
	// generate the PKI and pull four control-plane images before it can answer.
	immutableAPIWaitAttempts = 360
	immutableAPIWaitInterval = 5 * time.Second

	// Everything after the apiserver answers. Registering the Node is the node's
	// next step, so a couple of minutes is generous.
	immutableWaitAttempts = 120
	immutableWaitInterval = time.Second

	// The client's own wait for /version — a restarting static pod or a rebuilt
	// forward, not the install, which is over by then. Five minutes.
	immutableReadyWaitAttempts = 60
)

// errImmutableMasterFailed is the node reporting that it has given up: nothing
// is still working towards a control plane, so the wait ends with the node's
// own message instead of half an hour of polling a dead node.
var errImmutableMasterFailed = errors.New("the first master gave up bringing the control plane up")

// buildImmutableMasterPayload renders the cloud-init the first master boots
// with. A progress line of its own: it decides what the machine will be, and it
// runs before the machine exists.
func (b *ClusterBootstrapper) buildImmutableMasterPayload(ctx context.Context, bctx *bootstrapContext, nodeName string) (string, error) {
	var cloudConfig string

	err := dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Prepare immutable master payload", func(ctx context.Context) error {
		payload, err := immutable.BuildMasterPayload(ctx, immutable.MasterPayloadInput{
			NodeName:      nodeName,
			MetaConfig:    bctx.metaConfig,
			StateCache:    bctx.stateCache,
			CandiDir:      b.Options.Global.CandiDir,
			GlobalOptions: &b.Options.Global,
		})
		if err != nil {
			return err
		}

		cloudConfig = payload

		return nil
	})

	return cloudConfig, err
}

// connectToImmutableMaster collects the cluster credentials from the node and
// hands the rest of the bootstrap a Kubernetes client. No bashible pipeline:
// dhctl never sees a cluster key until the node gives it one.
func (b *ClusterBootstrapper) connectToImmutableMaster(ctx context.Context, bctx *bootstrapContext) error {
	if bctx.masterIP == "" {
		return errors.New("the first master address is unknown: rerun the bootstrap so the BaseInfra phase reports it")
	}

	// A rerun does not come back through the channel: once an attempt has
	// collected the credentials that channel is closed, and what it served is on
	// disk. Reading from there beats waiting on a listener that may be gone.
	complete, collectedPath, err := adminKubeconfigFromCache(ctx, bctx.stateCache)
	if err != nil {
		return err
	}
	if collectedPath != "" {
		bctx.adminKubeconfigPath = collectedPath
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(
			"A previous attempt already collected the credentials; reusing the admin kubeconfig at %s", collectedPath,
		))
	}

	server, err := b.openImmutableAPIChannel(ctx, bctx)
	if err != nil {
		return err
	}

	if complete == nil {
		complete, err = b.collectImmutableCredentials(ctx, bctx)
		if err != nil {
			return err
		}
	}

	content, err := immutable.RetargetKubeconfig(ctx, complete, server)
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
	// offering them. Attempted on a rerun too: a node that was already told
	// answers that it was.
	b.confirmImmutableHandoff(ctx, bctx)

	// Read exactly once, here — no later Client() call rebuilds from it. In
	// dhctl-server the process outlives the bootstrap, so leaving it to the
	// shutdown hook means admin credentials on disk for hours.
	removeImmutableKubeconfig(ctx, kubeconfigPath)

	return waitForImmutableMasterNode(ctx, kubeCl, bctx.masterNodeName)
}

// adminKubeconfigFromCache returns the admin kubeconfig a previous attempt
// collected, or nil when there is nothing to reuse. A recorded path that no
// longer resolves is warned about: refusing would make the bootstrap dead.
func adminKubeconfigFromCache(ctx context.Context, stateCache state.Cache) ([]byte, string, error) {
	path, err := immutable.LoadCollectedKubeconfig(ctx, stateCache)
	if err != nil {
		return nil, "", err
	}
	if path == "" {
		return nil, "", nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
			"read the admin kubeconfig %s a previous attempt left behind: %v; collecting it from the node again", path, err,
		))
		return nil, "", nil
	}
	return content, path, nil
}

// collectImmutableCredentials waits for the node's bootstrap channel, collects
// the admin kubeconfig and stores the operator's copy. The node returns a
// certificate; it becomes credentials here, where the installer's key lives.
func (b *ClusterBootstrapper) collectImmutableCredentials(ctx context.Context, bctx *bootstrapContext) ([]byte, error) {
	kubeconfig, err := b.collectImmutableKubeconfig(ctx, bctx, immutable.HandoffPort)
	if err != nil {
		return nil, err
	}

	complete, err := immutable.CompleteAdminKubeconfig(ctx, bctx.stateCache, kubeconfig)
	if err != nil {
		return nil, err
	}

	// Stored first: these are the only credentials that will ever leave the node
	// and everything after this can fail. The operator's copy keeps the node's
	// own address; the retargeted one dies with this process's tunnel.
	if err := b.saveAdminKubeconfig(ctx, complete, bctx); err != nil {
		return nil, err
	}

	return complete, nil
}

// collectImmutableKubeconfig waits for the node's one-shot handoff endpoint and
// reads the admin kubeconfig out of it. This is the whole "wait for the node to
// install itself and bring a control plane up" step.
func (b *ClusterBootstrapper) collectImmutableKubeconfig(ctx context.Context, bctx *bootstrapContext, handoffPort int) ([]byte, error) {
	material, err := immutable.LoadHandoffMaterial(ctx, bctx.stateCache)
	if err != nil {
		return nil, err
	}
	if material == nil {
		return nil, errors.New("the bootstrap handoff credentials are missing from the state cache: rerun the bootstrap so the BaseInfra phase regenerates the master payload")
	}

	address, stop, err := b.openImmutableChannel(ctx, bctx, handoffPort, "credentials handoff")
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

	// Narrated rather than silent: the node answers the status endpoint from the
	// moment it starts working, so an operator sees what it is doing and a node
	// that fails says why instead of just staying unreachable.
	logger := dhlog.FromContext(ctx)
	var (
		kubeconfig  []byte
		lastMessage string
		// answered records that the channel has spoken at least once, which is
		// what tells "the tunnel died" apart from "the node is not up yet".
		answered bool
	)
	// State rather than a branch of the classifier: an attempt that stops the
	// tunnel and fails to reopen must leave the next attempt reopening, not
	// dialling a local port nothing listens on.
	channelGone := false
	// Both requests below go through it. A break in the kubeconfig transfer that
	// did not arm the reopen would spend every remaining attempt dialling a dead
	// local port.
	noteChannelBroken := func(err error) error {
		// Only after the channel has answered once: before that a refused
		// connection is just the node still installing itself, and reopening on it
		// would hammer the bastion throughout a healthy bootstrap.
		if !answered || !channelBroken(err) {
			return err
		}
		if stop != nil {
			stop()
			stop = nil
		}
		channelGone = true
		return err
	}

	err = libretry.NewLoop("Waiting for the first master to bring the control plane up", immutableAPIWaitAttempts, immutableAPIWaitInterval).
		BreakIf(func(err error) bool {
			return errors.Is(err, immutable.ErrHandoffUnauthorized) ||
				errors.Is(err, immutable.ErrHandoffAlreadyServed) ||
				errors.Is(err, errImmutableMasterFailed)
		}).
		RunContext(ctx, func() error {
			if channelGone {
				reopened, newStop, openErr := b.openImmutableChannel(ctx, bctx, handoffPort, "credentials handoff")
				if openErr != nil {
					return fmt.Errorf("reopen the channel to the first master: %w", openErr)
				}
				stop = newStop
				input.Address = reopened
				channelGone = false
				logger.InfoContext(ctx, "Reopened the channel to the first master")
			}

			status, err := immutable.FetchStatus(ctx, input)
			if err != nil {
				return noteChannelBroken(err)
			}
			answered = true
			if message := statusLine(status); message != lastMessage {
				lastMessage = message
				logger.InfoContext(ctx, fmt.Sprintf("The first master reports: %s", message))
			}
			// A node that says it failed says so for good: it stops working on the
			// cluster, so polling it for the rest of the half-hour budget only
			// hides the message it already gave.
			if status.Phase == immutable.PhaseFailed {
				return fmt.Errorf("%w: %s", errImmutableMasterFailed, statusLine(status))
			}
			if status.Phase != immutable.PhaseReady && status.Phase != immutable.PhaseCollected {
				return fmt.Errorf("the first master is not ready to hand the credentials over: %s", statusLine(status))
			}

			collected, err := immutable.FetchKubeconfig(ctx, input)
			if err != nil {
				return noteChannelBroken(err)
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
	address, stop, err := b.openImmutableChannel(ctx, bctx, immutable.APIServerPort, "API server")
	if err != nil {
		return "", err
	}
	if stop != nil {
		bctx.immutableTunnelStop = stop
	}

	return "https://" + address, nil
}

// openImmutableChannel returns the host:port dhctl reaches the given port on,
// and the closer of the tunnel behind it — nil without a bastion. The forward
// is a direct-tcpip channel, so no sshd on the master is involved.
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
		cleanupSSHProvider(ctx, sshProvider)
		return nil, nil, fmt.Errorf("connect to the bastion host %s: %w", sshConfig.BastionHost, err)
	}

	tunnel := sshClient.Tunnel(fmt.Sprintf("%s:%d:127.0.0.1:%d", masterIP, remotePort, localPort))
	if err := tunnel.Up(ctx); err != nil {
		cleanupSSHProvider(ctx, sshProvider)
		return nil, nil, fmt.Errorf("forward %d to %s:%d through the bastion %s: %w", localPort, masterIP, remotePort, sshConfig.BastionHost, err)
	}

	// Nothing drains the tunnel's error channel: gossh sends there non-blocking,
	// so an unread channel cannot stall the accept loop, and a request that hits
	// an error fails on its own and the caller retries.
	stop := func() {
		tunnel.Stop()
		cleanupSSHProvider(ctx, sshProvider)
	}

	return tunnel, stop, nil
}

// cleanupSSHProvider releases the connection to the bastion. Reported rather
// than returned: every caller is already on its way out with something more
// interesting to say, and a leaked control socket is worth a line in the log.
func cleanupSSHProvider(ctx context.Context, sshProvider libcon.SSHProvider) {
	if err := sshProvider.Cleanup(ctx); err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("release the connection to the bastion: %v", err))
	}
}

// statusLine renders what the node reports into one readable line.
func statusLine(status *immutable.Status) string {
	if status.Message == "" {
		return string(status.Phase)
	}
	return string(status.Phase) + ": " + status.Message
}

// confirmImmutableHandoff tells the node the credentials are stored, and only
// then does it stop offering them. Reported, not returned: the kubeconfig is
// already on disk, so a failed confirmation costs an open channel, not a cluster.
func (b *ClusterBootstrapper) confirmImmutableHandoff(ctx context.Context, bctx *bootstrapContext) {
	logger := dhlog.FromContext(ctx)

	material, err := immutable.LoadHandoffMaterial(ctx, bctx.stateCache)
	if err != nil {
		// Silence here costs a node that is never told, a cluster-admin key left
		// in the cache and a rerun that goes back to a dead channel.
		logger.WarnContext(ctx, fmt.Sprintf("load the handoff credentials to confirm the handover: %v", err))
		return
	}
	if material == nil {
		logger.WarnContext(ctx, "The handoff credentials are gone from the state cache; the first master will keep its bootstrap channel open until it times out.")
		return
	}

	address, stop, err := b.openImmutableChannel(ctx, bctx, immutable.HandoffPort, "handoff confirmation")
	if err != nil {
		logger.WarnContext(ctx, fmt.Sprintf("confirm the handover to the first master: %v", err))
		return
	}
	if stop != nil {
		defer stop()
	}

	input := immutable.FetchKubeconfigInput{Address: address, ServerName: bctx.masterNodeName, Material: material}
	switch err := immutable.ConfirmCollected(ctx, input); {
	case err == nil:
		logger.InfoContext(ctx, "Confirmed the handover; the first master closed its bootstrap channel.")
	case errors.Is(err, immutable.ErrHandoffAlreadyServed):
		// A rerun reaching a channel an earlier attempt already closed. That is
		// the confirmation, arriving as the node's memory of it.
		logger.InfoContext(ctx, "The first master had already closed its bootstrap channel.")
	default:
		logger.WarnContext(ctx, fmt.Sprintf("confirm the handover to the first master, it will keep the bootstrap channel open: %v", err))
		return
	}

	// The rest of the material stays: a rerun that found it gone would mint fresh
	// material and render a payload the master never booted with. Only the
	// client key goes — see immutable.ForgetHandoffClientKey.
	if err := immutable.ForgetHandoffClientKey(ctx, bctx.stateCache); err != nil {
		logger.WarnContext(ctx, fmt.Sprintf("drop the installer's client key from the state cache: %v", err))
	}
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

	// Closed explicitly rather than deferred: a flush that fails leaves a
	// truncated kubeconfig, which surfaces two calls later as an opaque parse
	// error from the client builder.
	if _, err := file.Write(content); err != nil {
		file.Close()
		return "", fmt.Errorf("write the temporary kubeconfig %s: %w", file.Name(), err)
	}
	if err := file.Close(); err != nil {
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

// saveAdminKubeconfig hands the admin kubeconfig to the operator. Written by
// default: the node runs no sshd and the handoff endpoint has already served
// its one read, so "do not keep it" means a cluster nobody can reach.
func (b *ClusterBootstrapper) saveAdminKubeconfig(ctx context.Context, content []byte, bctx *bootstrapContext) error {
	path := b.Options.Bootstrap.KubeconfigOut
	if path != "" {
		// The same guard the preflight applies, repeated because preflights can be
		// skipped and this one protects the only way into the cluster.
		if err := immutable.CheckKubeconfigOutSurvivesCleanup(ctx, path, b.TmpDir); err != nil {
			return err
		}
	}
	if path == "" {
		// Only the CLI gets a default path. In server mode TmpDir is shared by the
		// whole process, so the default would funnel every cluster's cluster-admin
		// credentials into one file, each run overwriting the last.
		if b.CommanderMode {
			return immutable.ErrKubeconfigOutRequired
		}
		// Named after the cluster: the write below clears the path first, so a
		// second immutable cluster from the same machine would delete the first
		// one's only credentials. The suffix keeps the tmp cleaner off it.
		name := cache.AdminKubeconfigName
		if bctx.metaConfig != nil && bctx.metaConfig.ClusterPrefix != "" {
			name = bctx.metaConfig.ClusterPrefix + "-" + name
		}
		path = filepath.Join(b.TmpDir, name)
	}

	// Removed rather than truncated, then created with O_EXCL: writing into
	// whatever is at that path inherits its mode and follows a symlink someone
	// else put there. O_EXCL is safe because the path was just cleared.
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

	// Recorded before the confirmation, not after: a run that died between the two
	// with the record unwritten would leave every rerun dialling a closed channel
	// while the file sat here unread. Returned, not warned, for the same reason.
	if err := immutable.SaveCollectedKubeconfig(ctx, bctx.stateCache, path); err != nil {
		return err
	}

	b.printHowToReachTheCluster(ctx, path, bctx)
	return nil
}

// removeImmutableKubeconfig deletes the temporary kubeconfig, tolerating a
// second call.
func removeImmutableKubeconfig(ctx context.Context, path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("remove %s: %v", path, err))
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

	// The long wait is over by now: the node has reported Ready and served its
	// kubeconfig, so what these cover is a restarting static pod or a rebuilt
	// forward — a couple of minutes, not the half hour the collection had.
	initParams := libretry.NewEmptyParams(
		libretry.WithWait(immutableWaitInterval),
		libretry.WithAttempts(immutableWaitAttempts),
		libretry.WithLogger(dhlog.FromContext(ctx)),
	)
	readyParams := libretry.NewEmptyParams(
		libretry.WithWait(immutableAPIWaitInterval),
		libretry.WithAttempts(immutableReadyWaitAttempts),
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

// printHowToReachTheCluster says where the credentials are — the equivalent of
// the SSH line the classic bootstrap prints. Called twice because two thousand
// log lines separate the points, and one print gets lost between them.
func (b *ClusterBootstrapper) printHowToReachTheCluster(ctx context.Context, kubeconfigPath string, bctx *bootstrapContext) {
	logger := dhlog.FromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Admin kubeconfig written to %s — cluster-admin credentials, "+
		"and on a cluster of immutable nodes the only way in.", kubeconfigPath))
	logger.InfoContext(ctx, fmt.Sprintf("To use the cluster:  export KUBECONFIG=%s && kubectl get nodes", kubeconfigPath))

	// With a bastion that address is reachable from the bastion and nowhere else,
	// so the line above is true only inside the network. Print how to get there
	// rather than leave the operator to guess the shape of the tunnel.
	if line := bastionForwardLine(b.SSHProviderInitializer, bctx.masterIP, kubeconfigPath); line != "" {
		logger.InfoContext(ctx, "The master has no public address; reach it through the bastion first:")
		logger.InfoContext(ctx, "  "+line)
	}
}

// bastionForwardLine builds the command that makes the saved kubeconfig usable
// from outside, or "" when the master is directly reachable. It forwards to
// 127.0.0.1, which every apiserver certificate covers.
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
		port, localPort, masterIP, immutable.APIServerPort, bastion,
		masterIP, immutable.APIServerPort, localPort, kubeconfigPath)
}

// channelBroken reports whether the local end of the channel went away rather
// than the node saying something. A reset connection or a stream that ends
// mid-answer means the tunnel is gone; "not ready yet" does not.
func channelBroken(err error) bool {
	// ECONNREFUSED is deliberately absent: it is what a node still installing
	// itself looks like as well as a closed forward, and treating it as a break
	// would rebuild the tunnel throughout a healthy bootstrap.
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE)
}
