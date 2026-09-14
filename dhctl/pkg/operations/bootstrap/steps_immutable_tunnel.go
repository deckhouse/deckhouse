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
	"net"
	"strconv"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
)

// openImmutableChannel reaches the first master, whose address the rest of the
// bootstrap talks to. A wrong installed address fails right here, so the failure
// names both addresses just like the wait that follows it.
func (b *ClusterBootstrapper) openImmutableChannel(ctx context.Context, bctx *bootstrapContext, remotePort int, purpose string) (string, func(), error) {
	address, stop, err := b.openImmutableChannelTo(ctx, bctx.immutable.masterIP, remotePort, purpose)
	if err != nil {
		return "", nil, withBothAddresses(bctx, err)
	}
	return address, stop, nil
}

// openImmutableChannelTo returns the host:port dhctl reaches the given port of
// the given machine on, and the closer of the tunnel behind it.
func (b *ClusterBootstrapper) openImmutableChannelTo(ctx context.Context, host string, remotePort int, purpose string) (string, func(), error) {
	return immutable.OpenBastionChannel(
		ctx,
		b.SSHProviderInitializer.GetConfig(),
		b.SSHProviderInitializer.GetSettings(),
		host,
		remotePort,
		purpose,
	)
}

// masterAPIEndpoint hands the preflight check a way to reach the API port of the first master, or
// nil on a bootstrap whose master is not an immutable one. The address is an output of the
// BaseInfra phase, so it is read when the check runs rather than when the suite is built.
func (b *ClusterBootstrapper) masterAPIEndpoint(bctx *bootstrapContext) func(context.Context) (*checks.MasterAPIEndpoint, error) {
	if bctx.immutable == nil {
		return nil
	}

	return func(ctx context.Context) (*checks.MasterAPIEndpoint, error) {
		masterIP := bctx.immutable.masterIP
		if masterIP == "" {
			return nil, nil
		}

		master := net.JoinHostPort(masterIP, strconv.Itoa(immutable.APIServerPort))

		// A tunnel of its own rather than the one the rest of the bootstrap keeps open: that
		// one is opened later, by connectToImmutableMaster, and the whole point of asking
		// here is to answer before the silent wait that follows it.
		dial, stop, err := b.openImmutableChannelTo(ctx, masterIP, immutable.APIServerPort, "master API reachability")
		if err != nil {
			return nil, err
		}

		return &checks.MasterAPIEndpoint{
			Dial:    dial,
			Master:  master,
			Bastion: bastionLabel(b.SSHProviderInitializer.GetConfig()),
			Stop:    stop,
		}, nil
	}
}

// bastionLabel names the hop as the operator wrote it, or is empty when the master is reached
// directly.
func bastionLabel(connectionConfig *sshconfig.ConnectionConfig) string {
	config := immutable.BastionConfig(connectionConfig)
	if config == nil {
		return ""
	}
	if config.BastionPort != nil {
		return net.JoinHostPort(config.BastionHost, strconv.Itoa(*config.BastionPort))
	}
	return config.BastionHost
}
