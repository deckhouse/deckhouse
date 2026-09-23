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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// TestBastionAvailabilityWithNoBastionIsNotApplicable: the check used to describe its own
// absence ("no bastion configured, skipping…") and pass, so the reader saw a ✓ over a sentence
// saying nothing had been checked — and the runner cached it. The absence is now an outcome of
// its own, which is neither a pass nor remembered.
func TestBastionAvailabilityWithNoBastionIsNotApplicable(t *testing.T) {
	const desc = "ssh connection to the bastion host is possible"

	tests := []struct {
		name          string
		cfg           *sshconfig.ConnectionConfig
		notApplicable bool
	}{
		{"bastion set", &sshconfig.ConnectionConfig{Config: &sshconfig.Config{BastionHost: "10.0.0.1"}}, false},
		{"no bastion", &sshconfig.ConnectionConfig{Config: &sshconfig.Config{}}, true},
		{"nil inner config", &sshconfig.ConnectionConfig{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init := providerinitializer.NewSSHProviderInitializer(nil, tt.cfg)
			check := BastionAvailabilityCheck{SSHProviderInitializer: init}

			if got := check.Description(); got != desc {
				t.Errorf("Description() = %q, want the assertion that holds on a pass, %q", got, desc)
			}

			if !tt.notApplicable {
				return
			}
			_, err := check.Run(t.Context())
			if !errors.Is(err, preflight.ErrNotApplicable) {
				t.Errorf("Run() = %v, want a not-applicable outcome", err)
			}
			if !strings.Contains(err.Error(), "--ssh-bastion-host") {
				t.Errorf("the reason must name the flag that would configure one, got %q", err)
			}
		})
	}
}

func TestBastionConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  *sshconfig.Config
		want bool
	}{
		{"nil config", nil, false},
		{"empty bastion host", &sshconfig.Config{}, false},
		{"bastion host set", &sshconfig.Config{BastionHost: "10.0.0.1"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bastionConfigured(tt.cfg); got != tt.want {
				t.Errorf("bastionConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBastionPort(t *testing.T) {
	port := 2222
	tests := []struct {
		name string
		cfg  *sshconfig.Config
		want string
	}{
		{"nil config", nil, "22"},
		{"nil port defaults to 22", &sshconfig.Config{BastionHost: "h"}, "22"},
		{"port set", &sshconfig.Config{BastionHost: "h", BastionPort: &port}, "2222"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bastionPort(tt.cfg); got != tt.want {
				t.Errorf("bastionPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

// testPrivateKey returns a freshly generated OpenSSH private key, optionally encrypted.
func testPrivateKey(t *testing.T, passphrase string) string {
	t.Helper()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	var block *pem.Block
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(key, "")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(key, "", []byte(passphrase))
	}
	require.NoError(t, err)

	return string(pem.EncodeToMemory(block))
}

// writeKeyFile writes a key where dhctl writes it. Inline sshAgentPrivateKeys from a connection
// config end up in temp files, so most keys reach this code as paths.
func writeKeyFile(t *testing.T, pemKey string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "id_ed25519")
	require.NoError(t, os.WriteFile(path, []byte(pemKey), 0o600))
	return path
}

// TestBastionAuthMethods covers how the bastion check decides what to authenticate with. It never
// had a test, and the three sources it draws on — configured keys, a running ssh-agent and the
// bastion password — are exactly what an operator gets wrong when the handshake is refused.
func TestBastionAuthMethods(t *testing.T) {
	// Without this the developer's own agent leaks into the test and every case has a method.
	noAgent := func(t *testing.T) { t.Helper(); t.Setenv("SSH_AUTH_SOCK", "") }

	t.Run("a key given as a path", func(t *testing.T) {
		noAgent(t)
		cfg := &sshconfig.Config{PrivateKeys: []sshconfig.AgentPrivateKey{
			{Key: writeKeyFile(t, testPrivateKey(t, "")), IsPath: true},
		}}

		methods, cleanup, err := bastionAuthMethods(cfg)
		defer cleanup()

		require.NoError(t, err)
		assert.Len(t, methods, 1)
	})

	t.Run("a key given inline", func(t *testing.T) {
		noAgent(t)
		cfg := &sshconfig.Config{PrivateKeys: []sshconfig.AgentPrivateKey{
			{Key: testPrivateKey(t, "")},
		}}

		methods, cleanup, err := bastionAuthMethods(cfg)
		defer cleanup()

		require.NoError(t, err)
		assert.Len(t, methods, 1)
	})

	t.Run("an encrypted key with its passphrase", func(t *testing.T) {
		noAgent(t)
		cfg := &sshconfig.Config{PrivateKeys: []sshconfig.AgentPrivateKey{
			{Key: writeKeyFile(t, testPrivateKey(t, "secret")), IsPath: true, Passphrase: "secret"},
		}}

		methods, cleanup, err := bastionAuthMethods(cfg)
		defer cleanup()

		require.NoError(t, err)
		assert.Len(t, methods, 1)
	})

	t.Run("an encrypted key with the wrong passphrase", func(t *testing.T) {
		noAgent(t)
		cfg := &sshconfig.Config{PrivateKeys: []sshconfig.AgentPrivateKey{
			{Key: writeKeyFile(t, testPrivateKey(t, "secret")), IsPath: true, Passphrase: "not-it"},
		}}

		_, cleanup, err := bastionAuthMethods(cfg)
		defer cleanup()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not parse the private key")
	})

	t.Run("a key file that is not there", func(t *testing.T) {
		noAgent(t)
		cfg := &sshconfig.Config{PrivateKeys: []sshconfig.AgentPrivateKey{
			{Key: filepath.Join(t.TempDir(), "absent"), IsPath: true},
		}}

		_, cleanup, err := bastionAuthMethods(cfg)
		defer cleanup()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not read the private key")
	})

	t.Run("the bastion password on its own", func(t *testing.T) {
		// --ssh-bastion-pass with no key at all is a configuration that works, so it must not
		// be reported as having nothing to authenticate with.
		noAgent(t)
		cfg := &sshconfig.Config{BastionPassword: "hunter2"}

		methods, cleanup, err := bastionAuthMethods(cfg)
		defer cleanup()

		require.NoError(t, err)
		assert.Len(t, methods, 1)
	})

	t.Run("a key and a password are both offered", func(t *testing.T) {
		noAgent(t)
		cfg := &sshconfig.Config{
			PrivateKeys:     []sshconfig.AgentPrivateKey{{Key: testPrivateKey(t, "")}},
			BastionPassword: "hunter2",
		}

		methods, cleanup, err := bastionAuthMethods(cfg)
		defer cleanup()

		require.NoError(t, err)
		assert.Len(t, methods, 2)
	})

	t.Run("nothing to authenticate with", func(t *testing.T) {
		// The answer the check has to give before it dials: refusing here names the cause,
		// while dialing gets "attempted methods [none]" from the far end.
		noAgent(t)

		_, cleanup, err := bastionAuthMethods(&sshconfig.Config{})
		defer cleanup()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no private key, no ssh-agent identity and no password")
	})

	t.Run("a running ssh-agent counts as a method", func(t *testing.T) {
		// The agent is the usual way a key reaches the bastion.
		//
		// Not t.TempDir(): a unix socket path is capped near 100 bytes, and the per-test
		// directory name alone is longer than that on macOS.
		dir, err := os.MkdirTemp("/tmp", "d8ag")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dir) })

		socket := filepath.Join(dir, "a.sock")
		listener, err := net.Listen("unix", socket)
		require.NoError(t, err)
		defer listener.Close()

		t.Setenv("SSH_AUTH_SOCK", socket)

		methods, cleanup, err := bastionAuthMethods(&sshconfig.Config{})
		defer cleanup()

		require.NoError(t, err)
		assert.Len(t, methods, 1)
	})

	t.Run("SSH_AUTH_SOCK pointing at nothing", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", filepath.Join(t.TempDir(), "not-a-socket"))

		_, cleanup, err := bastionAuthMethods(&sshconfig.Config{})
		defer cleanup()

		// No agent, no keys, no password: the socket being dead adds nothing rather than
		// failing on its own.
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no private key")
	})
}
