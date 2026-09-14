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
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// withoutAgent makes the check see no ssh-agent. Every test that does not stand one up needs it:
// otherwise the check reads the developer's own loaded keys and the result means nothing.
func withoutAgent(t *testing.T) {
	t.Helper()
	t.Setenv("SSH_AUTH_SOCK", "")
}

// newKeyPair writes a private key to disk and returns its path and its authorized_keys line.
func newKeyPair(t *testing.T) (privateKeyPath, authorizedKey string) {
	t.Helper()

	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	block, err := ssh.MarshalPrivateKey(private, "")
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "id_ed25519")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(block), 0o600))

	signer, err := ssh.NewPublicKey(public)
	require.NoError(t, err)

	return path, string(ssh.MarshalAuthorizedKey(signer))
}

func sshKeyCheck(t *testing.T, declaredKey string, privateKeyPaths ...string) CloudSSHKeyCheck {
	t.Helper()

	providerConfig := map[string]json.RawMessage{}
	if declaredKey != "" {
		encoded, err := json.Marshal(declaredKey)
		require.NoError(t, err)
		providerConfig["sshPublicKey"] = encoded
	}

	keys := make([]sshconfig.AgentPrivateKey, 0, len(privateKeyPaths))
	for _, path := range privateKeyPaths {
		keys = append(keys, sshconfig.AgentPrivateKey{Key: path, IsPath: true})
	}

	return CloudSSHKeyCheck{
		MetaConfig: &config.MetaConfig{
			ProviderName:          "yandex",
			ProviderClusterConfig: providerConfig,
		},
		SSHProviderInitializer: providerinitializer.NewSSHProviderInitializer(nil,
			&sshconfig.ConnectionConfig{Config: &sshconfig.Config{PrivateKeys: keys}}),
	}
}

// TestCloudSSHKeyMatches is the failure this check exists for: the cloud installs one key, dhctl
// holds another, and nothing notices until the infrastructure has been created and dhctl spends
// 250 attempts waiting for an SSH connection that can never succeed.
func TestCloudSSHKeyMatches(t *testing.T) {
	withoutAgent(t)
	keyPath, authorizedKey := newKeyPair(t)

	detail, err := sshKeyCheck(t, authorizedKey, keyPath).Run(t.Context())
	require.NoError(t, err)
	assert.Contains(t, detail, "matches")
}

func TestCloudSSHKeyDoesNotMatch(t *testing.T) {
	withoutAgent(t)
	keyPath, _ := newKeyPair(t)
	_, otherAuthorizedKey := newKeyPair(t)

	_, err := sshKeyCheck(t, otherAuthorizedKey, keyPath).Run(t.Context())
	require.Error(t, err)

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "the cloud will install SHA256:")
	assert.Contains(t, failure.Observed, "dhctl holds SHA256:")
}

// TestCloudSSHKeyWithoutPrivateKeys: no key was passed and no agent is running, so there is
// nothing to compare against and nothing to report.
func TestCloudSSHKeyWithoutPrivateKeys(t *testing.T) {
	withoutAgent(t)
	_, authorizedKey := newKeyPair(t)

	_, err := sshKeyCheck(t, authorizedKey).Run(t.Context())
	assert.ErrorIs(t, err, preflight.ErrNotApplicable)
}

// TestCloudSSHKeyFromTheAgent is the case the check used to give up on. Passing keys by path is
// the minority; most operators keep theirs in an agent, and that is exactly when a mismatch went
// unnoticed until the infrastructure existed and the master would not accept a login.
func TestCloudSSHKeyFromTheAgent(t *testing.T) {
	t.Run("the agent holds the key the cloud will install", func(t *testing.T) {
		authorizedKey := startTestAgent(t, 1)

		detail, err := sshKeyCheck(t, authorizedKey).Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "ssh-agent")
	})

	t.Run("the agent holds other keys", func(t *testing.T) {
		startTestAgent(t, 2)
		_, unrelated := newKeyPair(t)

		_, err := sshKeyCheck(t, unrelated).Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		// Both agent keys are listed, and named: "dhctl holds a different key" reads very
		// differently for one the operator passed and one their agent happens to have.
		assert.Equal(t, 2, strings.Count(failure.Observed, "ssh-agent"))
	})

	t.Run("an agent that is not there is not a failure", func(t *testing.T) {
		// A stale SSH_AUTH_SOCK is common; it must not turn into a check failure.
		_, authorizedKey := newKeyPair(t)
		t.Setenv("SSH_AUTH_SOCK", filepath.Join(t.TempDir(), "not-a-socket"))

		_, err := sshKeyCheck(t, authorizedKey).Run(t.Context())

		assert.ErrorIs(t, err, preflight.ErrNotApplicable)
	})
}

// startTestAgent runs an ssh-agent holding count freshly generated keys, points SSH_AUTH_SOCK at
// it and returns the authorized_keys line of the first one.
func startTestAgent(t *testing.T, count int) string {
	t.Helper()

	// Not t.TempDir(): a unix socket path is capped near 100 bytes and the per-test directory
	// name alone is longer than that on macOS.
	dir, err := os.MkdirTemp("/tmp", "d8key")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	socket := filepath.Join(dir, "a.sock")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	keyring := agent.NewKeyring()
	first := ""
	for i := range count {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		require.NoError(t, keyring.Add(agent.AddedKey{
			PrivateKey: &private,
			Comment:    fmt.Sprintf("agent-key-%d", i),
		}))
		if i == 0 {
			signer, err := ssh.NewPublicKey(public)
			require.NoError(t, err)
			first = string(ssh.MarshalAuthorizedKey(signer))
		}
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { _ = agent.ServeAgent(keyring, conn) }()
		}
	}()

	t.Setenv("SSH_AUTH_SOCK", socket)
	return first
}

func TestCloudSSHKeyWithoutADeclaredKey(t *testing.T) {
	withoutAgent(t)
	keyPath, _ := newKeyPair(t)

	_, err := sshKeyCheck(t, "", keyPath).Run(t.Context())
	assert.ErrorIs(t, err, preflight.ErrNotApplicable)
}

// TestCloudSSHKeyThatIsNotAPublicKey: pasting the private key, or a PEM block, is a mistake the
// cloud only rejects during apply.
func TestCloudSSHKeyThatIsNotAPublicKey(t *testing.T) {
	withoutAgent(t)
	keyPath, _ := newKeyPair(t)

	_, err := sshKeyCheck(t, "-----BEGIN OPENSSH PRIVATE KEY-----", keyPath).Run(t.Context())
	require.Error(t, err)

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "not an OpenSSH public key")
}
