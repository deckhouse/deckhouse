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
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libcon "github.com/deckhouse/lib-connection/pkg"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

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
		assert.Contains(t, detail, "SSH login works for ubuntu@10.0.0.5")
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

// shortFreshBudget keeps the boot wait to something a test can sit through. The real one matches
// dhctl's own wait for SSH on the master, which is minutes.
func shortFreshBudget(t *testing.T, attempts int, wait time.Duration) {
	t.Helper()

	original := freshMachineBudget
	freshMachineBudget.attempts = attempts
	freshMachineBudget.wait = wait
	t.Cleanup(func() { freshMachineBudget = original })
}

// TestSSHCredentialAfterInfra is the machine the cloud created a moment ago.
//
// The phase this runs in sits between creating the instance and waiting for it, so the first
// refusals are the machine booting: no route to it yet, then nothing listening on 22, then sshd up
// but cloud-init has not created the login user. That last one is word for word what a mistyped
// --ssh-user produces, and no single attempt tells them apart — only the wait does, which is why
// the check has to carry one.
func TestSSHCredentialAfterInfra(t *testing.T) {
	authRefused := errors.New("ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey]")

	t.Run("the check waits for the boot instead of answering at once", func(t *testing.T) {
		shortFreshBudget(t, 250, time.Second)

		var asked retry.Params
		client := newFakeSSHClient("")
		client.onAwait = func(params retry.Params) { asked = params }

		check := SSHCredentialCheck{NodeInterface: nodeInterfaceOf(client), FreshlyCreated: true}
		detail, err := check.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "SSH login works for ubuntu@10.0.0.5")
		// A machine that was created seconds ago needs minutes, not the handful of seconds a
		// network retry allows — and the budget must not fall short of the wait the bootstrap
		// does on its own straight after this phase.
		require.NotNil(t, asked)
		// 250 one-second attempts is what the bootstrap itself waits straight after this
		// phase; giving the check less would make it fail on machines the bootstrap would
		// still have waited for.
		assert.GreaterOrEqual(t, asked.Attempts(), 250)
	})

	t.Run("the credential is still refused once the wait is over", func(t *testing.T) {
		// cloud-init has long finished by now, so a node that answers and still refuses is
		// refusing for good.
		shortFreshBudget(t, 250, time.Second)

		client := newFakeSSHClient("")
		client.reachErr = authRefused

		check := SSHCredentialCheck{NodeInterface: nodeInterfaceOf(client), FreshlyCreated: true}
		_, err := check.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "the node answers and kept refusing the credential")
		assert.Contains(t, failure.Observed, "4m", "the wait it sat through is what makes this a verdict")
		assert.Contains(t, failure.Fix, "--ssh-user")
		// The other reading has to be there: the operator may have the user right and an image
		// whose cloud-init never created it.
		assert.Contains(t, failure.Fix, "sshPublicKey")
	})

	t.Run("the wait is not multiplied by the runner", func(t *testing.T) {
		// The waiting is inside the check. A retry policy on top of it would restart a
		// four-minute wait several times over, and a permanent marker would be a lie besides.
		check := SSHCredentialAfterInfra(FixedNodeInterface(newFakeNode()), nil, "yandex")

		assert.Equal(t, preflight.NoRetry, check.Retry)
		assert.Equal(t, preflight.LongCheckTimeout, check.Timeout)
	})

	t.Run("the machine never answered at all", func(t *testing.T) {
		// Never reached, so the credential was never the subject — pointing at --ssh-user here
		// would be the same mistake in the other direction.
		shortFreshBudget(t, 250, time.Second)

		client := newFakeSSHClient("")
		client.reachErr = fmt.Errorf("dial tcp 10.0.0.5:22: %w", syscall.ECONNREFUSED)

		check := SSHCredentialCheck{NodeInterface: nodeInterfaceOf(client), FreshlyCreated: true}
		_, err := check.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "nothing is listening")
		assert.Contains(t, failure.Fix, "security groups")
		assert.NotContains(t, failure.Fix, "--ssh-user")
	})

	t.Run("an existing machine does not wait", func(t *testing.T) {
		// converge, destroy and the static path reach machines that have been up for months.
		// There is no cloud-init to wait out, and a refused credential is final.
		client := newFakeSSHClient("")
		client.reachErr = authRefused
		client.onAwait = func(retry.Params) { t.Error("an existing machine must not be waited for") }

		check := SSHCredentialCheck{NodeInterface: nodeInterfaceOf(client)}
		_, err := check.Run(t.Context())

		require.Error(t, err)
		assert.True(t, isPermanent(err), "nothing is going to create the user on a machine already running")
	})
}

// TestSSHCredentialWhenTheClientCannotBeBuilt is the shape a real bootstrap produced: the SSH
// provider starts the client it hands back, so a refused credential fails while the connection is
// being resolved and never reaches the probe. It used to arrive as a bare lib-connection string —
// "start client after create: Failed to connect to target directly … dial: transient error, may
// succeed on retry" — which names no remedy and ends by saying a retry might help, when nothing
// will.
func TestSSHCredentialWhenTheClientCannotBeBuilt(t *testing.T) {
	refused := errors.New("start client after create: Failed to connect to target directly " +
		"(last '89.169.149.45:22' with user 'noubuntu'): Timeout while \"Get SSH client\": last error: " +
		"ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], " +
		"no supported methods remain: dial: transient error, may succeed on retry")

	failing := func(err error) NodeInterfaceFunc {
		return func(context.Context) (libcon.Interface, error) { return nil, err }
	}

	// The machine, named from the configuration. There is no session to read it back from when
	// the connection never opened, and "the master node" told the reader nothing — the user they
	// mistyped is the whole point.
	endpoint := EndpointOfConfig(&sshconfig.ConnectionConfig{
		Config: &sshconfig.Config{User: "noubuntu", Port: intPtr(22)},
		Hosts:  []sshconfig.Host{{Host: "89.169.149.45"}},
	})

	t.Run("the machine is named from the configuration", func(t *testing.T) {
		check := SSHCredentialCheck{NodeInterface: failing(refused), Endpoint: endpoint, FreshlyCreated: true}

		_, err := check.Run(t.Context())

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Checked, "noubuntu@89.169.149.45:22")
	})

	t.Run("on a machine the cloud just created", func(t *testing.T) {
		check := SSHCredentialCheck{NodeInterface: failing(refused), FreshlyCreated: true}

		_, err := check.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Fix, "--ssh-user")
		assert.Contains(t, failure.Fix, "sshPublicKey")
		// The raw text names the user and the address, and it has to stay reachable — it is the
		// only place they appear when there is no client to ask.
		require.ErrorIs(t, err, refused)
		assert.False(t, isPermanent(err))
	})

	t.Run("on a machine that has been running", func(t *testing.T) {
		check := SSHCredentialCheck{NodeInterface: failing(refused)}

		_, err := check.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Fix, "--ssh-user")
		assert.True(t, isPermanent(err), "no retry makes a rejected key acceptable here")
	})

	t.Run("a failure that is not about ssh is passed through", func(t *testing.T) {
		// Not every way of resolving the connection is a login; a configuration error must not
		// be dressed up as a credential problem.
		other := errors.New("hosts is empty in session or default config")
		check := SSHCredentialCheck{NodeInterface: failing(other), FreshlyCreated: true}

		_, err := check.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		assert.NotErrorAs(t, err, &failure)
	})
}

func intPtr(v int) *int { return &v }
