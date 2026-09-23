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
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

type SSHCredentialCheck struct {
	// NodeInterface resolves the connection at the moment the check runs, the way every other
	// node check does — see NodeInterfaceFunc.
	NodeInterface NodeInterfaceFunc
	// FreshlyCreated is set when the machine was created moments ago, which changes both how long
	// the check waits and what a rejected credential means. See SSHCredentialAfterInfra.
	FreshlyCreated bool
	// Endpoint names the machine when the connection could not be opened and there is no session
	// to read the user and the address back from. Optional; without it such a failure can only
	// say "the master node".
	Endpoint EndpointFunc
	// ProviderName names the document that declares sshPublicKey, for the failure that asks the
	// operator to compare their key against it. Empty on a static cluster, which has no such
	// document and never reaches that branch.
	ProviderName string
}

var ErrAuthSSHFailed = fmt.Errorf("authentication failed")

const (
	// SSHConnectivityCheckName is the half that asks whether the machine answers on the port at
	// all. Split out because a machine that is not there and a machine that turns the key down
	// are different problems: one is a network or an address, the other a credential.
	SSHConnectivityCheckName preflight.CheckName = "ssh-connectivity"
	SSHCredentialCheckName   preflight.CheckName = "ssh-credential"
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
			Checked:  fmt.Sprintf("TCP connection to %s", address),
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
	return "the SSH credentials are valid"
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
		// Resolving the connection is itself a login attempt: the provider starts the client it
		// hands back, with a retry loop of its own. So a credential the node refuses fails here,
		// before the probe below, and arriving as a bare lib-connection string is how it used to
		// reach the operator.
		if sshNeverConnected(err) {
			return "", c.loginFailure(nil, err)
		}
		return "", err
	}
	wrapper, ok := nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return "", preflight.NotApplicable("dhctl was given no SSH host, so it runs against the local machine")
	}

	client := wrapper.Client()

	if c.FreshlyCreated {
		return awaitFreshMachineLogin(ctx, client, c.providerDocument())
	}

	if err := client.Check().CheckAvailability(ctx); err != nil {
		return "", sshLoginFailure(hostLabelOfClient(client), err)
	}
	return fmt.Sprintf("SSH login works for %s", hostLabelOfClient(client)), nil
}

// loginFailure words the failure for the machine this check is about. A machine created moments
// ago and one that has been running for months are refusing for different reasons, and only the
// second one is refusing for good.
func (c *SSHCredentialCheck) loginFailure(client libcon.SSHClient, err error) error {
	label := hostLabelOfClient(client)
	if client == nil && c.Endpoint != nil {
		if fromConfig := c.Endpoint(); fromConfig != "" {
			label = fromConfig
		}
	}

	if c.FreshlyCreated {
		return freshMachineLoginFailure(label, c.providerDocument(), err)
	}
	return sshLoginFailure(label, err)
}

// freshMachineBudget is how long a machine the cloud has just created is given to accept a login.
// It matches the wait dhctl does on its own right after this phase (steps_ssh.sshWaitAttempts), so
// the check never gives up on a machine that the bootstrap would still have waited for.
var freshMachineBudget = struct {
	attempts int
	wait     time.Duration
}{attempts: 250, wait: 1 * time.Second}

// awaitFreshMachineLogin waits out the boot of a machine that was created seconds ago, and then
// says which of the two readings the last failure supports.
//
// The wait is the point. A new cloud instance refuses a login for a sequence of unrelated reasons
// — no route to it yet, then nothing listening on 22, then sshd up but cloud-init has not created
// the login user yet — and every one of them is temporary. The last of those is the hard part:
// "this user does not exist" is exactly what a mistyped --ssh-user produces too, and no single
// attempt can tell them apart.
//
// What separates them is time. Once the budget is spent, cloud-init has long finished, so a node
// that answers and still refuses the credential is refusing it for good — and one that has not
// answered at all was never a credential problem.
func awaitFreshMachineLogin(ctx context.Context, client libcon.SSHClient, providerDocument string) (string, error) {
	err := client.Check().WithDelaySeconds(1).AwaitAvailability(ctx, retry.NewEmptyParams(
		retry.WithWait(freshMachineBudget.wait),
		retry.WithAttempts(freshMachineBudget.attempts),
		retry.WithLogger(dhlog.FromContext(ctx)),
	))
	if err == nil {
		return fmt.Sprintf("SSH login works for %s", hostLabelOfClient(client)), nil
	}

	return "", freshMachineLoginFailure(hostLabelOfClient(client), providerDocument, err)
}

// freshMachineLoginFailure says which of the two readings the failure supports, for a machine the
// cloud created moments ago. client may be nil: the connection can fail before there is one.
func freshMachineLoginFailure(label, providerDocument string, err error) error {
	waited := roundedBudget(freshMachineBudget.attempts, freshMachineBudget.wait)

	if sshNeverConnected(err) {
		return &preflight.Failure{
			Checked:  fmt.Sprintf("SSH login to %s", label),
			Observed: fmt.Sprintf("the node answers and kept refusing the credential for %s", waited),
			Expected: "a login user on the node that accepts the key",
			// Deliberately not preflight.Permanent: the runner retrying the whole check would
			// start the wait again, which is the one thing that must not happen here.
			Fix: fmt.Sprintf("check --ssh-user against the login user of the node's image "+
				"(ubuntu, ec2-user, debian, altlinux, opensuse). Check that the key passed to dhctl "+
				"matches %s.sshPublicKey.", providerDocument),
			Err: err,
		}
	}

	return &preflight.Failure{
		Checked:  fmt.Sprintf("SSH connection to %s", label),
		Observed: fmt.Sprintf("%s for %s", classifyNetworkError(err), waited),
		Expected: "an SSH connection to the newly created machine",
		Fix: "check in the cloud console that the instance started, and that its security groups " +
			"allow 22/TCP from this host",
		Err: err,
	}
}

// roundedBudget states the wait in a unit a reader thinks in.
func roundedBudget(attempts int, wait time.Duration) string {
	return (time.Duration(attempts) * wait).Round(time.Minute).String()
}

// sshLoginFailure separates the two things that go wrong here, because they have nothing to do
// with each other: the credentials were rejected, or the machine was never reached. Both used to
// arrive as one sentence with the raw x/crypto text appended.
func sshLoginFailure(label string, err error) error {
	failure := &preflight.Failure{
		Checked: fmt.Sprintf("SSH login to %s", label),
		Err:     err,
	}

	if isSSHAuthError(err) {
		// No key, password or agent identity is going to become the right one on a second try.
		failure.Observed = "the server rejected every authentication method offered"
		failure.Expected = "an SSH user authorized on the node"
		failure.Fix = "check --ssh-user, and that one of --ssh-agent-private-keys is authorized for it on the node. " +
			"Pass --ask-become-pass if the user authenticates by password."
		return preflight.Permanent(failure)
	}

	// The legacy backend only. dhctl runs the Go client by default, and that one says what went
	// wrong — the branch above reads it. clissh shells out to the ssh binary instead, which exits
	// 255 for everything that stopped it opening a session and puts the reason on a stderr the
	// caller does not always keep, so here the causes can only be named together. The credential
	// goes first: cloud images disagree about the login user (ubuntu, ec2-user, debian, altlinux,
	// opensuse) and picking the wrong one is the common mistake.
	if status, ok := exitStatus(err); ok && status == sshCouldNotOpenSession {
		failure.Observed = "the ssh client could not open a session (exit status 255)"
		failure.Expected = "an SSH session the node accepts"
		failure.Fix = "check --ssh-user against the login user of the node's image " +
			"(ubuntu, ec2-user, debian, altlinux, opensuse — it differs per image), that one of " +
			"--ssh-agent-private-keys is authorized for that user, and that 22/TCP is open from this host"
		return failure
	}

	failure.Observed = classifyNetworkError(err)
	failure.Expected = "an SSH connection the node accepts"
	failure.Fix = "check --ssh-host and --ssh-port, and that 22/TCP is open from this host to the node"
	return failure
}

// sshCouldNotOpenSession is what the ssh binary exits with when it never got as far as a session.
// It covers authentication, the connection and the host key alike — OpenSSH does not separate
// them by status. Only the legacy clissh backend can produce it.
const sshCouldNotOpenSession = 255

// sshNeverConnected reports whether ssh gave up before there was a session at all. Everything that
// runs over ssh fails in its own terms when this happens — a port-forward reports a forwarding
// problem, a command reports a command problem — and none of those are what went wrong.
//
// The default backend answers through isSSHAuthError; the exit status is the legacy backend's
// only signal.
func sshNeverConnected(err error) bool {
	if isSSHAuthError(err) {
		return true
	}
	status, ok := exitStatus(err)
	return ok && status == sshCouldNotOpenSession
}

// isSSHAuthError reports whether the server answered and turned the credentials down.
//
// This is the default backend's case: gossh returns what x/crypto produced, which says in words
// what went wrong. There is no typed error to match on, so the text is what there is.
//
// "handshake failed" is deliberately not one of the markers. x/crypto uses it as the prefix for
// every handshake failure, and an agreement failure — an sshd too old to share a key exchange or
// cipher — is not a credential problem and must not be answered with "check --ssh-user". The auth
// case always says so itself. (Host keys cannot be the cause: the backend sets
// InsecureIgnoreHostKey.)
func isSSHAuthError(err error) bool {
	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"unable to authenticate",
		"no supported methods remain",
		"permission denied",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// SSHCredentialAfterInfra is the same question asked of a machine the cloud created moments ago.
//
// It waits instead of answering at once, and it does not call a rejected credential permanent:
// until cloud-init has run, the login user does not exist yet, and a refusal means nothing.
// Everything else in this phase is asked over this connection, so the wait belongs here — the
// phase runs before dhctl's own wait for SSH on the master.
func SSHCredentialAfterInfra(nodeInterface NodeInterfaceFunc, endpoint EndpointFunc, providerName string) preflight.Check {
	check := SSHCredentialCheck{NodeInterface: nodeInterface, Endpoint: endpoint, FreshlyCreated: true, ProviderName: providerName}
	built := SSHCredential(nodeInterface, endpoint)
	// The waiting is inside. Letting the runner retry the check on top of that would multiply a
	// four-minute wait by the retry count.
	built.Retry = preflight.NoRetry
	built.Timeout = preflight.LongCheckTimeout
	built.Run = check.Run
	return built
}

func SSHCredential(nodeInterface NodeInterfaceFunc, endpoint EndpointFunc) preflight.Check {
	check := SSHCredentialCheck{NodeInterface: nodeInterface, Endpoint: endpoint}
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

// providerDocument names the document that declares sshPublicKey.
func (c SSHCredentialCheck) providerDocument() string {
	return providerDocumentKind(c.ProviderName)
}
