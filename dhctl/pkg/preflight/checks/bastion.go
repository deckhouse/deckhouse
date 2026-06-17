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

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

const BastionAvailabilityCheckName preflight.CheckName = "bastion-availability"

const defaultBastionPort = "22"

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
	return preflight.DefaultRetryPolicy
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
	host := sshCfg.BastionHost
	port := bastionPort(sshCfg)

	sshProvider, err := c.SSHProviderInitializer.GetSSHProvider(ctx)
	if err != nil && !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
		return fmt.Errorf("cannot initialize ssh provider: %w", err)
	}
	if sshProvider == nil {
		return fmt.Errorf("ssh provider is not available")
	}

	sess := session.NewSession(session.Input{
		User:       sshCfg.BastionUser,
		Port:       port,
		BecomePass: sshCfg.BastionPassword,
	})
	sess.AddAvailableHosts(session.Host{Host: host})

	client, err := sshProvider.NewStandaloneClient(ctx, sess, nil)
	if err != nil {
		return fmt.Errorf("cannot create ssh client for bastion host %s:%s: %w", host, port, err)
	}
	defer client.Stop()

	// Start performs TCP connect + SSH handshake + user authentication to the
	// bastion (transport only, no session/exec channel).
	if err := client.Start(); err != nil {
		return fmt.Errorf("cannot connect to bastion host %s:%s: %w", host, port, err)
	}

	// dhctl uses the bastion only as a ProxyJump (direct-tcpip forwarding)
	// and never runs commands on it.
	// We do NOT exec a probe command here —
	// a hardened bastion may permit forwarding while denying
	// shell/exec (ForceCommand, no-pty, command="..." in authorized_keys, a
	// restricted/kill shell), and such a bastion is still valid for our use.
	return nil
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
