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
	"fmt"
	"strings"
	"syscall"
	"testing"

	sshutils "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg/ssh/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// TestIsSSHAuthError separates the credentials being turned down from the machine not being
// reached. They have nothing to do with each other, and one of them is worth retrying.
func TestIsSSHAuthError(t *testing.T) {
	rejected := []string{
		"ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey]",
		"ssh: no supported methods remain",
		"Permission denied (publickey,password)",
		// The same words from a backend that shouts them.
		"SSH: UNABLE TO AUTHENTICATE",
		// The server's MaxAuthTries.
		"ssh: disconnect, reason 2: Too many authentication failures",
	}
	for _, text := range rejected {
		assert.True(t, isSSHAuthError(errors.New(text)), "should be an auth failure: %s", text)
	}
	// lib-connection marks a rejection whose text names none of the markers.
	assert.True(t, isSSHAuthError(fmt.Errorf("%w: ssh: handshake failed: EOF", sshutils.ErrSSHAuthentication)))

	notRejected := []string{
		"dial tcp 10.0.0.5:22: connect: connection refused",
		"dial tcp 10.0.0.5:22: i/o timeout",
		"lookup master-0.example.com: no such host",
	}
	for _, text := range notRejected {
		assert.False(t, isSSHAuthError(errors.New(text)), "should not be an auth failure: %s", text)
	}
}

// TestSSHLoginFailure: both causes arrived as one sentence with the raw x/crypto text appended,
// and they need different things done about them.
func TestSSHLoginFailure(t *testing.T) {
	client := newFakeSSHClient("")

	t.Run("the credentials were turned down", func(t *testing.T) {
		cause := errors.New("ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey]")

		err := sshLoginFailure(hostLabelOfClient(client), cause)

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Checked, "ubuntu@10.0.0.5")
		assert.Contains(t, failure.Observed, "rejected every authentication method")
		assert.Contains(t, failure.Fix, "--ssh-user")
		require.ErrorIs(t, err, cause, "the cause has to stay reachable for the log")

		// No key becomes the right key on a second attempt, so the runner must not spend its
		// retries here.
		assert.True(t, isPermanent(err))
	})

	t.Run("the server's MaxAuthTries ran out", func(t *testing.T) {
		cause := errors.New("ssh: disconnect, reason 2: Too many authentication failures")

		err := sshLoginFailure(hostLabelOfClient(client), cause)

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		// Either the user or the keys are wrong, or too many identities came first: both hints stay.
		assert.Contains(t, failure.Observed, "MaxAuthTries")
		assert.Contains(t, failure.Fix, "--ssh-user")
		assert.Contains(t, failure.Fix, "remove the other keys from the ssh-agent")
		assert.True(t, isPermanent(err))

		// A machine the cloud just created gets the same hint.
		err = freshMachineLoginFailure(hostLabelOfClient(client), "OpenStackClusterConfiguration", cause)
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "MaxAuthTries")
		assert.Contains(t, failure.Fix, "sshPublicKey")
		assert.Contains(t, failure.Fix, "remove the other keys from the ssh-agent")
	})

	// lib-connection marks a failure of dhctl's own credentials, such as an ssh-agent that is the
	// only source of keys and cannot list them. The server was never asked.
	t.Run("dhctl could not use its own credentials", func(t *testing.T) {
		cause := fmt.Errorf("ssh: handshake failed: %w: read SSH agent identities: EOF", sshutils.ErrSSHCredentials)

		for name, err := range map[string]error{
			"node":            sshLoginFailure(hostLabelOfClient(client), cause),
			"freshly created": freshMachineLoginFailure(hostLabelOfClient(client), "OpenStackClusterConfiguration", cause),
		} {
			var failure *preflight.Failure
			require.ErrorAs(t, err, &failure, name)
			assert.Contains(t, failure.Observed, "could not use its SSH credentials", name)
			assert.Contains(t, failure.Fix, "ssh-add -l", name)
			assert.NotContains(t, failure.Fix, "22/TCP", name)
			require.ErrorIs(t, err, cause, name)
			assert.True(t, isPermanent(err), name)
		}
		assert.True(t, sshNeverConnected(cause))
	})

	t.Run("the machine was never reached", func(t *testing.T) {
		cause := fmt.Errorf("dial tcp 10.0.0.5:22: %w", syscall.ECONNREFUSED)

		err := sshLoginFailure(hostLabelOfClient(client), cause)

		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "connection was refused")
		assert.Contains(t, failure.Fix, "22/TCP")
		// A machine that is still booting answers on the next attempt.
		assert.False(t, isPermanent(err))
	})
}

// TestNoConnection names what stopped the connection for every check behind ssh-credential.
func TestNoConnection(t *testing.T) {
	for name, tc := range map[string]struct {
		cause    error
		observed string
		fix      string
	}{
		"dhctl's own credentials": {
			cause:    fmt.Errorf("ssh: handshake failed: %w: read SSH agent identities: EOF", sshutils.ErrSSHCredentials),
			observed: "could not use its SSH credentials",
			fix:      "ssh-add -l",
		},
		"the node turned the credentials down": {
			cause:    errors.New("ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey]"),
			observed: "did not accept the credentials",
			fix:      "--preflight-skip-check=ssh-credential",
		},
		"the node was never reached": {
			cause:    fmt.Errorf("dial tcp 10.0.0.5:22: %w", syscall.ECONNREFUSED),
			observed: "connection was refused",
			fix:      "--preflight-skip-check=ssh-credential",
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := noConnection("", tc.cause)

			var failure *preflight.Failure
			require.ErrorAs(t, err, &failure)
			assert.Contains(t, failure.Observed, tc.observed)
			assert.Contains(t, failure.Fix, tc.fix)
			assert.True(t, failure.StopsPhase)
			assert.True(t, isPermanent(err))
			require.ErrorIs(t, err, tc.cause)
		})
	}
}

// TestOneLineError keeps a per-instance line to one line: a dozen multi-line causes in one block
// is unreadable, and the full text is in the debug log.
func TestOneLineError(t *testing.T) {
	assert.Equal(t, "connection refused", oneLineError(errors.New("connection refused")))
	assert.Equal(t, "connection refused …", oneLineError(errors.New("connection refused\nand a second line")))
	assert.Equal(t, "trimmed", oneLineError(errors.New("  trimmed  \n")))
}

func TestHealthURL(t *testing.T) {
	// The reverse tunnel's near end is this host's loopback, whatever the node's address is.
	assert.Equal(t, "http://127.0.0.1:27322/healthz", healthURL(defaultTunnelRemotePort))
}

// TestInvalidCIDRFailure: the message names the field to edit, not just the value that failed.
func TestInvalidCIDRFailure(t *testing.T) {
	err := invalidCIDRFailure("podSubnetCIDR", "10.111.0.0")

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	// Bare, not qualified with a document: #22688 lets it be declared in either, and the fix
	// below is what names them.
	assert.Equal(t, "podSubnetCIDR", failure.Checked)
	assert.Contains(t, failure.Fix, "control-plane-manager")
	assert.Contains(t, failure.Observed, `"10.111.0.0" is not a CIDR`)
	assert.Contains(t, failure.Expected, "10.111.0.0/16")
	// A malformed CIDR does not become well formed on a second read.
	assert.True(t, isPermanent(err))
}

// TestInstanceClassProviderRun: an OpenStackInstanceClass in an AWS cluster is filed away as a
// plain resource and configures nothing, and the node group that references it never gets nodes.
func TestInstanceClassProviderRun(t *testing.T) {
	const awsInstanceClass = `
apiVersion: deckhouse.io/v1
kind: AWSInstanceClass
metadata:
  name: worker
spec:
  instanceType: m5.large
`
	const openstackInstanceClass = `
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: worker
spec:
  flavorName: m1.large
`

	t.Run("the instance class matches the provider", func(t *testing.T) {
		check := InstanceClassProviderCheck{MetaConfig: &config.MetaConfig{
			ProviderName:  "aws",
			ResourcesYAML: awsInstanceClass,
		}}

		detail, err := check.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, `provider "aws"`)
	})

	t.Run("the instance class is another provider's", func(t *testing.T) {
		check := InstanceClassProviderCheck{MetaConfig: &config.MetaConfig{
			ProviderName:  "aws",
			ResourcesYAML: strings.TrimSpace(awsInstanceClass) + "\n---" + openstackInstanceClass,
		}}

		_, err := check.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "OpenStackInstanceClass")
		assert.Contains(t, failure.Fix, "change the kind")
		// The document is not going to change between two attempts.
		assert.True(t, isPermanent(err))
	})

	t.Run("a static cluster has no provider", func(t *testing.T) {
		check := InstanceClassProviderCheck{MetaConfig: &config.MetaConfig{}}

		_, err := check.Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
	})

	t.Run("no configuration at all", func(t *testing.T) {
		_, err := InstanceClassProviderCheck{}.Run(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "the cluster configuration was not passed to this check")
	})
}
