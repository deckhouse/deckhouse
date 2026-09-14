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
	"fmt"
	"net"
	"strings"
	"time"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

type SSHCredentialCheck struct {
	// NodeInterface resolves the connection at the moment the check runs, the way every other
	// node check does — see NodeInterfaceFunc.
	NodeInterface NodeInterfaceFunc
}

var ErrAuthSSHFailed = fmt.Errorf("authentication failed")

const (
	// SSHConnectivityCheckName is the half that asks whether the machine answers on the port at
	// all. Split out because a machine that is not there and a machine that turns the key down
	// are different problems: one is a network or an address, the other a credential.
	SSHConnectivityCheckName preflight.CheckName = "static-ssh-connectivity"
	SSHCredentialCheckName   preflight.CheckName = "static-ssh-credential"
)

// SSHConnectivityCheck dials the SSH port and goes no further.
type SSHConnectivityCheck struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
}

func (*SSHConnectivityCheck) Description() string {
	return "the node answers on its SSH port"
}

func (*SSHConnectivityCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (*SSHConnectivityCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NetworkRetry
}

func (c *SSHConnectivityCheck) Run(ctx context.Context) (string, error) {
	sess := c.SSHProviderInitializer.GetConfig()
	if sess == nil || sess.Config == nil || len(sess.Hosts) == 0 {
		return "", preflight.NotApplicable("dhctl was given no SSH host")
	}

	host := sess.Hosts[0].Host
	port := "22"
	if sess.Config.Port != nil {
		port = sess.Config.PortString()
	}
	address := net.JoinHostPort(host, port)

	// Through the bastion when there is one: that is the path every later connection takes, and
	// dialling the node directly would answer a question nobody asked.
	if sess.Config.BastionHost != "" {
		return "", preflight.NotApplicable("the connection goes through a bastion, which bastion-availability checks")
	}

	conn, err := (&net.Dialer{Timeout: sshDialTimeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("tcp connection to %s", address),
			Observed: classifyNetworkError(err),
			Expected: "sshd listening on the node",
			Fix:      "check --ssh-host and --ssh-port, and that the machine is up and reachable from this host",
			Err:      err,
		}
	}
	_ = conn.Close()

	return fmt.Sprintf("%s accepts TCP connections", address), nil
}

const sshDialTimeout = 10 * time.Second

func SSHConnectivity(sshProvider *providerinitializer.SSHProviderInitializer) preflight.Check {
	check := SSHConnectivityCheck{SSHProviderInitializer: sshProvider}
	return preflight.Check{
		Name:        SSHConnectivityCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.DefaultPreflightCheckTimeout,
		// The critical path: everything else in this phase is asked over this connection, so a
		// port that does not answer ends the phase here rather than above a page of records
		// saying nothing could be asked.
		StopsPhaseOnFailure: true,
		Run:                 check.Run,
	}
}

func (*SSHCredentialCheck) Description() string {
	return "ssh credentials are valid"
}

func (*SSHCredentialCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (*SSHCredentialCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NetworkRetry
}

func (c *SSHCredentialCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := c.NodeInterface(ctx)
	if err != nil {
		return "", err
	}
	wrapper, ok := nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return "", preflight.NotApplicable("dhctl was given no SSH host: it is running against the local machine")
	}

	client := wrapper.Client()
	if err := client.Check().CheckAvailability(ctx); err != nil {
		return "", sshLoginFailure(client, err)
	}
	return fmt.Sprintf("ssh login works for %s", hostLabelOfClient(client)), nil
}

// sshLoginFailure separates the two things that go wrong here, because they have nothing to do
// with each other: the credentials were rejected, or the machine was never reached. Both used to
// arrive as one sentence with the raw x/crypto text appended.
func sshLoginFailure(client libcon.SSHClient, err error) error {
	failure := &preflight.Failure{
		Checked: fmt.Sprintf("ssh login to %s", hostLabelOfClient(client)),
		Err:     err,
	}

	if isSSHAuthError(err) {
		// No key, password or agent identity is going to become the right one on a second try.
		failure.Observed = "the server rejected every authentication method offered"
		failure.Expected = "the SSH user to be authorized on the node"
		failure.Fix = "check --ssh-user, and that one of --ssh-agent-private-keys is authorized for it on the node; " +
			"pass --ask-become-pass if the user authenticates by password"
		return preflight.Permanent(failure)
	}

	failure.Observed = classifyNetworkError(err)
	failure.Expected = "the node to accept an SSH connection"
	failure.Fix = "check --ssh-host and --ssh-port, and that 22/TCP is open from this host to the node"
	return failure
}

// isSSHAuthError reports whether the server answered and turned the credentials down. x/crypto
// gives no typed error for this, so the text it produces is what there is to match on.
func isSSHAuthError(err error) bool {
	text := err.Error()
	for _, marker := range []string{
		"unable to authenticate",
		"no supported methods remain",
		"permission denied",
		"handshake failed",
	} {
		if strings.Contains(strings.ToLower(text), marker) {
			return true
		}
	}
	return false
}

func SSHCredential(nodeInterface NodeInterfaceFunc) preflight.Check {
	check := SSHCredentialCheck{NodeInterface: nodeInterface}
	return preflight.Check{
		Name:        SSHCredentialCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		// The critical path: credentials the node turns down mean no node check can be asked at
		// all, and the bootstrap would not get past this either.
		StopsPhaseOnFailure: true,
		Run:                 check.Run,
	}
}
