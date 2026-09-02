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

package immutable

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"time"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/provider"
	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/gossh"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// OpenBastionChannel returns the host:port dhctl reaches the given port of the
// given machine on, and the closer of the tunnel behind it — a no-op without a
// bastion. The forward is a direct-tcpip channel, so no sshd there is involved.
func OpenBastionChannel(
	ctx context.Context,
	connectionConfig *sshconfig.ConnectionConfig,
	sett settings.Settings,
	host string,
	remotePort int,
	purpose string,
) (string, func(), error) {
	sshConfig := BastionConfig(connectionConfig)
	if sshConfig == nil {
		return net.JoinHostPort(host, strconv.Itoa(remotePort)), func() {}, nil
	}

	localPort, err := freeLocalPort()
	if err != nil {
		return "", nil, fmt.Errorf("reserve a local port for the %s tunnel: %w", purpose, err)
	}

	stop, err := openBastionTunnel(ctx, sshConfig, sett, host, remotePort, localPort, purpose)
	if err != nil {
		return "", nil, err
	}

	// 127.0.0.1 is always in the SAN list of a kube-apiserver certificate, and
	// the handoff endpoint is verified by name rather than by address, so
	// neither channel needs the local end to be nameable.
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)), stop, nil
}

// BastionConfig returns the SSH config a tunnel to the master is built from, or
// nil when no bastion is configured and the master is reached directly.
func BastionConfig(connectionConfig *sshconfig.ConnectionConfig) *sshconfig.Config {
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
func openBastionTunnel(ctx context.Context, sshConfig *sshconfig.Config, sett settings.Settings, masterIP string, remotePort, localPort int, purpose string) (func(), error) {
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

	// The provider narrates where this context narrates: a wait opens a channel
	// every attempt, and the retry loop inside the provider prints three lines
	// per open — on top of the one line that carries the node's own progress.
	sshProvider := provider.NewDefaultSSHProvider(
		channelSettings{Settings: sett, logger: dhlog.FromContext(ctx)},
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

	goSSHClient, ok := sshClient.(*gossh.Client)
	if !ok {
		cleanupSSHProvider(ctx, sshProvider)
		return nil, fmt.Errorf("the bastion forward needs the modern SSH backend, got %T", sshClient)
	}

	local := net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort))
	listener, err := net.Listen("tcp", local)
	if err != nil {
		cleanupSSHProvider(ctx, sshProvider)
		return nil, fmt.Errorf("listen on %s for the %s forward through the bastion %s: %w", local, purpose, sshConfig.BastionHost, err)
	}

	remote := net.JoinHostPort(masterIP, strconv.Itoa(remotePort))
	// Debug: a wait rebuilds this channel per attempt — up to 360 of them over
	// half an hour — and one line each would bury the node's own progress.
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf(
		"%s forward through the bastion is up: %s -> %s", purpose, local, remote,
	))

	forwarded, stopForwarding := context.WithCancel(ctx)
	go forwardThroughBastion(forwarded, goSSHClient, listener, remote)

	return func() {
		stopForwarding()
		listener.Close()
		cleanupSSHProvider(ctx, sshProvider)
	}, nil
}

// bastionDialTimeout bounds one dial from the bastion to the master. It is the
// deadline gossh puts on the same dial, kept because it is proven, without the
// part that ends the whole forward when it fires.
const bastionDialTimeout = 5 * time.Second

// forwardThroughBastion serves the local end of the forward until it is closed.
// gossh's own forward is not used for this: a dial of its that hits the deadline
// ends its accept loop for good, and the port stays bound with nobody accepting
// it — here that dial costs the one connection it was made for.
func forwardThroughBastion(ctx context.Context, sshClient *gossh.Client, listener net.Listener, remote string) {
	logger := dhlog.FromContext(ctx)

	for {
		local, err := listener.Accept()
		if err != nil {
			return
		}

		go func() {
			defer local.Close()

			if err := pipeThroughBastion(ctx, sshClient, local, remote); err != nil {
				logger.DebugContext(ctx, fmt.Sprintf("forward a connection to %s through the bastion: %v", remote, err))
			}
		}()
	}
}

// pipeThroughBastion carries one connection over a channel of its own. The SSH
// client is taken per connection because the library replaces it on a reconnect.
func pipeThroughBastion(ctx context.Context, sshClient *gossh.Client, local net.Conn, remote string) error {
	client := sshClient.GetClient()
	if client == nil {
		return errors.New("the connection to the bastion is not live")
	}

	dialCtx, cancel := context.WithTimeout(ctx, bastionDialTimeout)
	defer cancel()

	remoteConn, err := client.DialContext(dialCtx, "tcp", remote)
	if err != nil {
		return fmt.Errorf("dial %s from the bastion: %w", remote, err)
	}
	defer remoteConn.Close()

	logger := dhlog.FromContext(ctx)
	go func() {
		if _, err := io.Copy(remoteConn, local); err != nil {
			logger.DebugContext(ctx, fmt.Sprintf("carry a request to %s through the bastion: %v", remote, err))
		}
	}()

	if _, err := io.Copy(local, remoteConn); err != nil {
		return fmt.Errorf("carry the answer of %s through the bastion: %w", remote, err)
	}

	return nil
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

// channelSettings is the SSH settings of this bootstrap with one thing replaced:
// the logger the plumbing narrates into. Embedded because every other setting
// must stay exactly what the rest of dhctl was given.
type channelSettings struct {
	settings.Settings
	logger *slog.Logger
}

func (s channelSettings) Logger() *slog.Logger { return s.logger }
