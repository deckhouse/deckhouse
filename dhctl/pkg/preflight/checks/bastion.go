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
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

const BastionAvailabilityCheckName preflight.CheckName = "bastion-availability"

const (
	defaultBastionPort = "22"
	bastionDialTimeout = 10 * time.Second
)

type BastionAvailabilityCheck struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
}

// Description is the assertion that holds when the check passes. It used to describe its own
// absence ("no bastion configured, skipping…") when there was no bastion, which printed a ✓ over
// a sentence saying nothing had been checked — and cached it. The absence is now reported by the
// body as a not-applicable outcome, which is neither a pass nor remembered.
func (BastionAvailabilityCheck) Description() string {
	return "ssh connection to the bastion host is possible"
}

func (BastionAvailabilityCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (BastionAvailabilityCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.RetryPolicy{
		Attempts: 15,
		Options: []backoff.ExponentialBackOffOpts{
			backoff.WithInitialInterval(time.Second),
			backoff.WithMultiplier(2),
			backoff.WithMaxInterval(5 * time.Second),
			backoff.WithMaxElapsedTime(0),
		},
	}
}

func bastionConfigured(cfg *sshconfig.Config) bool {
	return cfg != nil && cfg.BastionHost != ""
}

func bastionPort(cfg *sshconfig.Config) string {
	if cfg == nil || cfg.BastionPort == nil {
		return defaultBastionPort
	}
	return cfg.BastionPortString()
}

func (c BastionAvailabilityCheck) Run(ctx context.Context) (string, error) {
	connCfg := c.SSHProviderInitializer.GetConfig()
	if connCfg == nil || !bastionConfigured(connCfg.Config) {
		return "", preflight.NotApplicable("no --ssh-bastion-host was given")
	}

	sshCfg := connCfg.Config
	addr := net.JoinHostPort(sshCfg.BastionHost, bastionPort(sshCfg))
	user := sshCfg.BastionUser

	authMethods, cleanup, err := bastionAuthMethods(sshCfg)
	if err != nil {
		// No key, no agent and no password: nothing about that changes on a second attempt.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the credentials for bastion %s", addr),
			Observed: err.Error(),
			Expected: "a private key, an ssh-agent identity or a password for the bastion",
			Fix:      "pass --ssh-agent-private-keys, run an ssh-agent, or pass --ask-bastion-pass",
			Err:      err,
		})
	}
	defer cleanup()

	clientCfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         bastionDialTimeout,
	}

	// 1. TCP reachability check, honouring context cancellation.
	conn, err := (&net.Dialer{Timeout: bastionDialTimeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("tcp connection to bastion %s", addr),
			Observed: classifyNetworkError(err),
			Expected: "an answer from the bastion",
			Fix:      "check --ssh-bastion-host and --ssh-bastion-port, and that the port is open from this host",
			Err:      err,
		}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(bastionDialTimeout))

	// 2. SSH transport + user authentication ONLY.
	//
	// We deliberately do NOT build a full lib-connection SSH client here: its
	// Start() spawns a keepalive goroutine that opens an SSH *session* on the
	// target. dhctl uses the bastion only as a ProxyJump (direct-tcpip
	// forwarding) and never runs commands on it, so a hardened bastion may
	// permit forwarding while denying shell/exec (ForceCommand, no-pty,
	// command="..." in authorized_keys, a restricted/kill shell). Such a
	// bastion is still valid for our use, but the keepalive session would be
	// rejected and trigger an endless reconnect loop. A bare transport+auth
	// handshake validates exactly what we rely on, nothing more.
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientCfg)
	if err != nil {
		failure := &preflight.Failure{
			Checked:  fmt.Sprintf("ssh login to bastion %s as %q", addr, user),
			Observed: "the bastion rejected every authentication method offered",
			Expected: "an authorized bastion user",
			Fix: "check --ssh-bastion-user. Authorize one of --ssh-agent-private-keys for that user " +
				"on the bastion, or pass --ask-bastion-pass",
			Err: err,
		}
		if isSSHAuthError(err) {
			return "", preflight.Permanent(failure)
		}
		failure.Observed = classifyNetworkError(err)
		return "", failure
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	// 3. The forward itself, which is the only thing dhctl actually asks of a bastion. A bastion
	// with AllowTcpForwarding no authenticates perfectly and is still useless, and that used to
	// surface minutes later as a failure to reach the master.
	if forwarded, err := client.Dial("tcp", net.JoinHostPort("127.0.0.1", "22")); err != nil {
		if strings.Contains(err.Error(), "administratively prohibited") {
			return "", preflight.Permanent(&preflight.Failure{
				Checked:  fmt.Sprintf("a direct-tcpip forward through bastion %s", addr),
				Observed: "the bastion refused to forward (administratively prohibited)",
				Expected: "a bastion that forwards TCP connections",
				Fix:      "set AllowTcpForwarding yes in sshd_config on the bastion",
				Err:      err,
			})
		}
		// Anything else here is about the address behind the bastion, which this check does not
		// own; the handshake it does own has already succeeded.
	} else {
		_ = forwarded.Close()
	}

	return fmt.Sprintf("ssh handshake with bastion %s as %q succeeded", addr, user), nil
}

// bastionAuthMethods builds the ssh auth methods used to reach the bastion:
// public keys from the connection config, the running ssh-agent (if any) and an
// optional bastion password. The returned cleanup closes the ssh-agent socket.
func bastionAuthMethods(cfg *sshconfig.Config) ([]ssh.AuthMethod, func(), error) {
	cleanup := func() {}

	signers, err := bastionSigners(cfg.PrivateKeys)
	if err != nil {
		return nil, cleanup, err
	}

	var methods []ssh.AuthMethod
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if agentConn, err := net.Dial("unix", sock); err == nil {
			cleanup = func() { _ = agentConn.Close() }
			methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(agentConn).Signers))
		}
	}

	if cfg.BastionPassword != "" {
		methods = append(methods, ssh.Password(cfg.BastionPassword))
	}

	if len(methods) == 0 {
		return nil, cleanup, fmt.Errorf("dhctl has no private key, no ssh-agent identity and no password for the bastion")
	}

	return methods, cleanup, nil
}

// bastionSigners parses the connection config private keys into ssh signers.
// dhctl writes inline sshAgentPrivateKeys to temp files, so most keys arrive as
// paths (IsPath); inline PEM keys are handled too.
func bastionSigners(keys []sshconfig.AgentPrivateKey) ([]ssh.Signer, error) {
	signers := make([]ssh.Signer, 0, len(keys))
	for _, k := range keys {
		pemBytes := []byte(k.Key)
		if k.IsPath {
			b, err := os.ReadFile(k.Key)
			if err != nil {
				return nil, fmt.Errorf("dhctl could not read the private key %s: %w", k.Key, err)
			}
			pemBytes = b
		}

		var (
			signer ssh.Signer
			err    error
		)
		if k.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(k.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(pemBytes)
		}
		if err != nil {
			return nil, fmt.Errorf("dhctl could not parse the private key %s: %w", k.Key, err)
		}
		signers = append(signers, signer)
	}

	return signers, nil
}

func BastionAvailability(sshProviderInitializer *providerinitializer.SSHProviderInitializer) preflight.Check {
	check := BastionAvailabilityCheck{SSHProviderInitializer: sshProviderInitializer}
	return preflight.Check{
		Name:        BastionAvailabilityCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.DefaultPreflightCheckTimeout,
		Run:         check.Run,
	}
}

// BastionAvailabilityAfterInfra is the same check, asked again once the infrastructure exists.
//
// On the cloud layouts that create one — OpenStack standard, VCD with-nat, AWS withNAT — the
// bastion is not in the configuration at all until base infrastructure has been applied, so the
// pre-infra instance finds no --ssh-bastion-host and reports itself not applicable. Without a
// second look, a wrong --ssh-bastion-user or key becomes 250 one-second attempts at reaching the
// master, in which the bastion is never mentioned.
func BastionAvailabilityAfterInfra(sshProviderInitializer *providerinitializer.SSHProviderInitializer) preflight.Check {
	check := BastionAvailability(sshProviderInitializer)
	check.Phase = preflight.PhasePostInfra
	return check
}
