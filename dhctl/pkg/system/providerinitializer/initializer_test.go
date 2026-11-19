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

package providerinitializer

import (
	"context"
	"errors"
	"testing"

	libcon "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg"
	"github.com/stretchr/testify/require"
)

type fakeSSHProvider struct {
	libcon.SSHProvider

	client libcon.SSHClient
	err    error
	calls  *int
}

func (p fakeSSHProvider) Client(context.Context) (libcon.SSHClient, error) {
	*p.calls++
	return p.client, p.err
}

type fakeSSHClient struct{ libcon.SSHClient }

// TestTheSSHConnectionIsOpenedOnlyWhenAskedFor: a command that reaches Kubernetes with a kubeconfig
// and also names an SSH host must not wait on that host unless something needs a node.
func TestTheSSHConnectionIsOpenedOnlyWhenAskedFor(t *testing.T) {
	t.Run("the provider's client, asked for on the call", func(t *testing.T) {
		sshClient := &fakeSSHClient{}
		calls := new(int)
		i := &SSHProviderInitializer{provider: fakeSSHProvider{client: sshClient, calls: calls}}

		connect := i.SSHConnect()
		require.NotNil(t, connect)
		require.Zero(t, *calls)

		got, err := connect(t.Context())
		require.NoError(t, err)
		require.Same(t, sshClient, got)
		require.Equal(t, 1, *calls)
	})

	t.Run("a connection that cannot be had is an error", func(t *testing.T) {
		i := &SSHProviderInitializer{provider: fakeSSHProvider{err: errors.New("ssh: handshake failed"), calls: new(int)}}

		_, err := i.SSHConnect()(t.Context())
		require.ErrorContains(t, err, "ssh: handshake failed")
	})

	t.Run("no SSH hosts, or no SSH at all", func(t *testing.T) {
		require.Nil(t, NewSSHProviderInitializer(nil, nil).SSHConnect())

		var none *SSHProviderInitializer
		require.Nil(t, none.SSHConnect())
	})
}
