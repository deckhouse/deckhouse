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

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
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
