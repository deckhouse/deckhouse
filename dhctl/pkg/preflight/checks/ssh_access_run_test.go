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

package checks

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// The three checks on the critical path. Nothing could be asked of a node until these pass, and
// none of them had a test of what it actually does.

// connectionTo builds the connection configuration dhctl would have been given.
func connectionTo(t *testing.T, host string, port *int, bastion string) *providerinitializer.SSHProviderInitializer {
	t.Helper()

	cfg := &sshconfig.ConnectionConfig{
		Config: &sshconfig.Config{Port: port, BastionHost: bastion},
	}
	if host != "" {
		cfg.Hosts = []sshconfig.Host{{Host: host}}
	}
	return providerinitializer.NewSSHProviderInitializer(nil, cfg)
}

// TestSSHConnectivityRun: the half that asks whether the machine answers at all. A machine that is
// not there and a machine that turns the key down are different problems, and this one dials.
func TestSSHConnectivityRun(t *testing.T) {
	t.Run("the node answers", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close()

		host, portText, err := net.SplitHostPort(listener.Addr().String())
		require.NoError(t, err)
		port, err := net.LookupPort("tcp", portText)
		require.NoError(t, err)

		check := SSHConnectivityCheck{SSHProviderInitializer: connectionTo(t, host, &port, "")}

		detail, err := check.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "accepts TCP connections")
	})

	t.Run("nothing is listening", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		host, portText, err := net.SplitHostPort(listener.Addr().String())
		require.NoError(t, err)
		port, err := net.LookupPort("tcp", portText)
		require.NoError(t, err)
		require.NoError(t, listener.Close())

		check := SSHConnectivityCheck{SSHProviderInitializer: connectionTo(t, host, &port, "")}

		_, err = check.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "refused")
		assert.Contains(t, failure.Fix, "--ssh-host")
	})

	t.Run("the connection goes through a bastion", func(t *testing.T) {
		// Dialling the node directly would answer a question nobody asked: every later
		// connection takes the bastion, and bastion-availability is what checks it.
		check := SSHConnectivityCheck{SSHProviderInitializer: connectionTo(t, "10.0.0.5", nil, "bastion.example.com")}

		_, err := check.Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
		assert.Contains(t, err.Error(), "bastion")
	})

	t.Run("no SSH host was given", func(t *testing.T) {
		check := SSHConnectivityCheck{SSHProviderInitializer: connectionTo(t, "", nil, "")}

		_, err := check.Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
	})
}

// TestSSHCredentialRun: the other half, which asks whether the credentials are accepted.
func TestSSHCredentialRun(t *testing.T) {
	t.Run("the login works", func(t *testing.T) {
		check := SSHCredentialCheck{NodeInterface: nodeInterfaceOf(newFakeSSHClient(""))}

		detail, err := check.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "ssh login works for ubuntu@10.0.0.5")
	})

	t.Run("the credentials are turned down", func(t *testing.T) {
		client := newFakeSSHClient("")
		client.reachErr = errors.New("ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey]")

		check := SSHCredentialCheck{NodeInterface: nodeInterfaceOf(client)}

		_, err := check.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "rejected every authentication method")
		// No key becomes the right key on a retry, and this check stops the whole phase.
		assert.True(t, isPermanent(err))
	})

	t.Run("dhctl is running against the local machine", func(t *testing.T) {
		// No SSH host at all: the node interface is the installer container itself, and
		// asking it about SSH credentials would answer about the wrong machine.
		check := SSHCredentialCheck{NodeInterface: FixedNodeInterface(newFakeNode())}

		_, err := check.Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
		assert.Contains(t, err.Error(), "local machine")
	})
}

// TestSingleSSHHostRun: bootstrap builds one master and converge adds the rest, so several
// --ssh-host arguments mean the operator expects something dhctl will not do.
func TestSingleSSHHostRun(t *testing.T) {
	t.Run("one host", func(t *testing.T) {
		check := SingleSSHHostCheck{NodeInterface: nodeInterfaceOf(newFakeSSHClient("", "10.0.0.5"))}

		detail, err := check.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "10.0.0.5")
	})

	t.Run("several hosts", func(t *testing.T) {
		client := newFakeSSHClient("", "10.0.0.5", "10.0.0.6", "10.0.0.7")

		check := SingleSSHHostCheck{NodeInterface: nodeInterfaceOf(client)}

		_, err := check.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		// All of them, so the operator can see which list dhctl read.
		assert.Contains(t, failure.Observed, "3 hosts")
		for _, host := range []string{"10.0.0.5", "10.0.0.6", "10.0.0.7"} {
			assert.Contains(t, failure.Observed, host)
		}
		assert.Contains(t, failure.Fix, "dhctl converge")
		// The flags will not change between two attempts.
		assert.True(t, isPermanent(err))
	})

	t.Run("no SSH host at all", func(t *testing.T) {
		check := SingleSSHHostCheck{NodeInterface: FixedNodeInterface(newFakeNode())}

		_, err := check.Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
	})
}
