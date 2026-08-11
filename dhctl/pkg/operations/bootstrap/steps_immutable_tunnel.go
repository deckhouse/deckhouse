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
	"strconv"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/provider"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// openImmutableChannel returns the host:port dhctl reaches the given port on,
// and the closer of the tunnel behind it — a no-op without a bastion. The
// forward is a direct-tcpip channel, so no sshd on the master is involved.
func (b *ClusterBootstrapper) openImmutableChannel(ctx context.Context, bctx *bootstrapContext, remotePort int, purpose string) (string, func(), error) {
	sshConfig := bastionConfig(b.SSHProviderInitializer.GetConfig())
	if sshConfig == nil {
		return net.JoinHostPort(bctx.masterIP, strconv.Itoa(remotePort)), func() {}, nil
	}

	localPort, err := freeLocalPort()
	if err != nil {
		return "", nil, fmt.Errorf("reserve a local port for the %s tunnel: %w", purpose, err)
	}

	stop, err := b.openBastionTunnel(ctx, sshConfig, bctx.masterIP, remotePort, localPort, purpose)
	if err != nil {
		return "", nil, err
	}

	// 127.0.0.1 is always in the SAN list of a kube-apiserver certificate, and
	// the handoff endpoint is verified by name rather than by address, so
	// neither channel needs the local end to be nameable.
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)), stop, nil
}

// bastionConfig returns the SSH config a tunnel to the master is built from, or
// nil when no bastion is configured and the master is reached directly.
func bastionConfig(connectionConfig *sshconfig.ConnectionConfig) *sshconfig.Config {
	if connectionConfig == nil || connectionConfig.Config == nil {
		return nil
	}
	if connectionConfig.Config.BastionHost == "" {
		return nil
	}
	return connectionConfig.Config
}

// openBastionTunnel forwards a local port to the given port of the master
// through the bastion, and returns the closer of that forward.
func (b *ClusterBootstrapper) openBastionTunnel(ctx context.Context, sshConfig *sshconfig.Config, masterIP string, remotePort, localPort int, purpose string) (func(), error) {
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
		return nil, fmt.Errorf("connect to the bastion host %s: %w", sshConfig.BastionHost, err)
	}

	tunnel := sshClient.Tunnel(fmt.Sprintf("%s:%d:127.0.0.1:%d", masterIP, remotePort, localPort))
	if err := tunnel.Up(ctx); err != nil {
		cleanupSSHProvider(ctx, sshProvider)
		return nil, fmt.Errorf("forward %d to %s:%d through the bastion %s: %w", localPort, masterIP, remotePort, sshConfig.BastionHost, err)
	}

	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(
		"%s tunnel through the bastion is up: %s", purpose, tunnel.String(),
	))

	// Nothing drains the tunnel's error channel: gossh sends there non-blocking,
	// so an unread channel cannot stall the accept loop, and a request that hits
	// an error fails on its own and the caller retries.
	return func() {
		tunnel.Stop()
		cleanupSSHProvider(ctx, sshProvider)
	}, nil
}

// cleanupSSHProvider releases the connection to the bastion. Reported rather
// than returned: every caller is already on its way out with something more
// interesting to say, and a leaked control socket is worth a line in the log.
func cleanupSSHProvider(ctx context.Context, sshProvider libcon.SSHProvider) {
	if err := sshProvider.Cleanup(ctx); err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("release the connection to the bastion: %v", err))
	}
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
