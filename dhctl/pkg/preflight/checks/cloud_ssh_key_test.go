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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/deckhouse/lib-connection/pkg/settings"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// agentSockets carries the socket a test stood up, for the helper that builds the check. The
// check reads the socket off the settings rather than the environment — that is the whole point —
// so the test has to put it there.
var agentSockets sync.Map

// agentSocket is the socket this test stood up, or "" when it stood none up. Tests that do not
// call startTestAgent see no agent at all, which keeps the developer's own loaded keys out.
func agentSocket(t *testing.T) string {
	t.Helper()
	if socket, ok := agentSockets.Load(t.Name()); ok {
		return socket.(string)
	}
	return ""
}

// withoutAgent is what a test says when it wants no agent. It is the default, and stating it
// keeps the intent visible next to the tests that do stand one up.
func withoutAgent(t *testing.T) {
	t.Helper()
	agentSockets.Delete(t.Name())
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

	// AuthSock falls back to the environment when the settings carry no path — that is
	// lib-connection's rule and dhctl's real behaviour — so a test that stands no agent up has
	// to clear it, or it reads the developer's own loaded keys and means nothing.
	t.Setenv("SSH_AUTH_SOCK", "")

	// The socket the check reads is the one the settings carry, which is what the SSH client
	// will use.
	sett := settings.NewBaseProviders(settings.ProviderParams{AuthSock: agentSocket(t)})

	initializer := providerinitializer.NewSSHProviderInitializer(nil,
		&sshconfig.ConnectionConfig{Config: &sshconfig.Config{PrivateKeys: keys}})
	initializer.Reinitialize(t.Context(), sett, &sshconfig.ConnectionConfig{
		Config: &sshconfig.Config{PrivateKeys: keys},
	})

	return CloudSSHKeyCheck{
		MetaConfig: &config.MetaConfig{
			ProviderName:          "yandex",
			ProviderClusterConfig: providerConfig,
		},
		SSHProviderInitializer: initializer,
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

	t.Run("a key named on the flag is not rescued by the agent", func(t *testing.T) {
		// The reported case: the right key in the agent, the wrong one on the flag. The
		// connection does offer both, but the operator named a key and it is not the one the
		// cloud installs — and the bootstrap this came from failed on the master minutes later.
		authorized := startTestAgent(t, 1)
		otherKeyPath, _ := newKeyPair(t)

		_, err := sshKeyCheck(t, authorized, otherKeyPath).Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Expected, "--ssh-agent-private-keys")
		assert.Contains(t, failure.Fix, "drop the flag")
		// Only the named key is weighed; the agent is not consulted once one was named.
		// ("--ssh-agent-private-keys" is the flag, so match the source wording, not "ssh-agent".)
		assert.NotContains(t, failure.Observed, "the ssh-agent key")
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
		// A stale socket path is common; it must not turn into a check failure.
		_, authorizedKey := newKeyPair(t)
		agentSockets.Store(t.Name(), filepath.Join(t.TempDir(), "not-a-socket"))
		t.Cleanup(func() { agentSockets.Delete(t.Name()) })

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

	agentSockets.Store(t.Name(), socket)
	t.Cleanup(func() { agentSockets.Delete(t.Name()) })

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

// TestCloudSSHKeyNamesTheDocument: MetaConfig.ProviderName is lowercased while the configuration
// is parsed, so the message used to point at "yandexClusterConfiguration" — a document nobody has
// and which greps for nothing.
func TestCloudSSHKeyNamesTheDocument(t *testing.T) {
	assert.Equal(t, "YandexClusterConfiguration", providerDocumentKind("yandex"))
	assert.Equal(t, "OpenStackClusterConfiguration", providerDocumentKind("openstack"))
	assert.Equal(t, "HuaweiCloudClusterConfiguration", providerDocumentKind("huaweicloud"))
	// A provider this installer does not know still has to read as a document name.
	assert.Equal(t, "<Provider>ClusterConfiguration", providerDocumentKind("someprivatecloud"))
}

// TestCloudSSHKeyAsksTheSocketDhctlWillUse: the check used to read SSH_AUTH_SOCK out of the
// environment while the SSH client reads it off the settings, which carry an explicit path that
// overrides the environment. Two sources, and when they disagreed the check reported on an agent
// the connection never talks to — passing on a key that is never offered.
func TestCloudSSHKeyAsksTheSocketDhctlWillUse(t *testing.T) {
	t.Run("the settings say there is no agent", func(t *testing.T) {
		// An agent is running and the environment points at it, but dhctl was told not to use
		// one. Its keys are not dhctl's to offer.
		authorized := startTestAgent(t, 1)
		t.Setenv("SSH_AUTH_SOCK", agentSocket(t))
		agentSockets.Delete(t.Name())

		_, err := sshKeyCheck(t, authorized).Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable,
			"with no agent for dhctl there is nothing to compare, whatever the environment holds")
	})

	t.Run("the settings name an agent of their own", func(t *testing.T) {
		// The reverse: the settings carry a path, and that is the agent to ask.
		authorized := startTestAgent(t, 1)

		detail, err := sshKeyCheck(t, authorized).Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "ssh-agent")
	})
}
