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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

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
	keyPath, authorizedKey := newKeyPair(t)

	detail, err := sshKeyCheck(t, authorizedKey, keyPath).Run(t.Context())
	require.NoError(t, err)
	assert.Contains(t, detail, "matches")
}

func TestCloudSSHKeyDoesNotMatch(t *testing.T) {
	keyPath, _ := newKeyPair(t)
	_, otherAuthorizedKey := newKeyPair(t)

	_, err := sshKeyCheck(t, otherAuthorizedKey, keyPath).Run(t.Context())
	require.Error(t, err)

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "the cloud will install SHA256:")
	assert.Contains(t, failure.Observed, "dhctl holds SHA256:")
}

// TestCloudSSHKeyWithoutPrivateKeys: an ssh-agent may hold the matching key without dhctl ever
// seeing it, so this is not a failure.
func TestCloudSSHKeyWithoutPrivateKeys(t *testing.T) {
	_, authorizedKey := newKeyPair(t)

	_, err := sshKeyCheck(t, authorizedKey).Run(t.Context())
	assert.ErrorIs(t, err, preflight.ErrNotApplicable)
}

func TestCloudSSHKeyWithoutADeclaredKey(t *testing.T) {
	keyPath, _ := newKeyPair(t)

	_, err := sshKeyCheck(t, "", keyPath).Run(t.Context())
	assert.ErrorIs(t, err, preflight.ErrNotApplicable)
}

// TestCloudSSHKeyThatIsNotAPublicKey: pasting the private key, or a PEM block, is a mistake the
// cloud only rejects during apply.
func TestCloudSSHKeyThatIsNotAPublicKey(t *testing.T) {
	keyPath, _ := newKeyPair(t)

	_, err := sshKeyCheck(t, "-----BEGIN OPENSSH PRIVATE KEY-----", keyPath).Run(t.Context())
	require.Error(t, err)

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "not an OpenSSH public key")
}
