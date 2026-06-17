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

func (c BastionAvailabilityCheck) Description() string {
	connCfg := c.SSHProviderInitializer.GetConfig()
	if connCfg == nil || !bastionConfigured(connCfg.Config) {
		return "no bastion configured, skipping bastion availability check"
	}

	return "ssh connection to the bastion host is possible"
}

func (BastionAvailabilityCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (BastionAvailabilityCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.RetryPolicy{
		Attempts: 1,
		Options: []backoff.ExponentialBackOffOpts{
			backoff.WithInitialInterval(time.Second),
			backoff.WithMultiplier(2),
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

func (c BastionAvailabilityCheck) Run(ctx context.Context) error {
	connCfg := c.SSHProviderInitializer.GetConfig()
	if connCfg == nil || !bastionConfigured(connCfg.Config) {
		// No bastion configured: nothing to validate, pass silently.
		return nil
	}

	sshCfg := connCfg.Config
	addr := net.JoinHostPort(sshCfg.BastionHost, bastionPort(sshCfg))

	authMethods, cleanup, err := bastionAuthMethods(sshCfg)
	if err != nil {
		return fmt.Errorf("cannot prepare ssh auth for bastion host %s: %w", addr, err)
	}
	defer cleanup()

	clientCfg := &ssh.ClientConfig{
		User:            sshCfg.BastionUser,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         bastionDialTimeout,
	}

	// 1. TCP reachability check, honouring context cancellation.
	conn, err := (&net.Dialer{Timeout: bastionDialTimeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot reach bastion host %s: %w", addr, err)
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
		return fmt.Errorf("cannot authenticate to bastion host %s as %q: %w", addr, sshCfg.BastionUser, err)
	}
	ssh.NewClient(sshConn, chans, reqs).Close()

	return nil
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
		return nil, cleanup, fmt.Errorf("no ssh auth methods available (no private keys, ssh-agent or password)")
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
				return nil, fmt.Errorf("read private key %s: %w", k.Key, err)
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
			return nil, fmt.Errorf("parse private key: %w", err)
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
		Run:         check.Run,
	}
}
